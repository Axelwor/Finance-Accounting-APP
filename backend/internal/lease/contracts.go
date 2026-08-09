package lease

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// US-111: Lease Contract Registration (PSAK 73)
//   Initial journal: Dr 1701 Right-of-Use Asset / Cr 2301 Lease Liability (at PV)
//   PV = payment * [1 - (1+r)^-n] / r
// ---------------------------------------------------------------------------

type CreateLeaseContractRequest struct {
	LesseeName         string `json:"lessee_name"`
	LessorName         string `json:"lessor_name"`
	StartDate          string `json:"start_date"`
	EndDate            string `json:"end_date"`
	PaymentAmountCents int64  `json:"payment_amount_cents"`
	PaymentFrequency   string `json:"payment_frequency"`
	TotalPayments      int    `json:"total_payments"`
	DiscountRate       string `json:"discount_rate"`
	PaymentAccountCode string `json:"payment_account_code"`
	Description        string `json:"description"`
}

type leasePaymentScheduleResponse struct {
	PaymentNo               int    `json:"payment_no"`
	PaymentDate             string `json:"payment_date"`
	PaymentAmountCents      int64  `json:"payment_amount_cents"`
	PrincipalCents          int64  `json:"principal_cents"`
	InterestCents           int64  `json:"interest_cents"`
	RemainingLiabilityCents int64  `json:"remaining_liability_cents"`
	JournalEntryID          int64  `json:"journal_entry_id,omitempty"`
	Posted                  bool   `json:"posted"`
}

type leaseContractResponse struct {
	ID                       int64                          `json:"id"`
	Number                   string                         `json:"number"`
	LesseeName               string                         `json:"lessee_name"`
	LessorName               string                         `json:"lessor_name,omitempty"`
	StartDate                string                         `json:"start_date"`
	EndDate                  string                         `json:"end_date"`
	PaymentAmountCents       int64                          `json:"payment_amount_cents"`
	PaymentFrequency         string                         `json:"payment_frequency"`
	TotalPayments            int                            `json:"total_payments"`
	DiscountRate             string                         `json:"discount_rate"`
	ROUAssetAccountID        int64                          `json:"rou_asset_account_id"`
	LeaseLiabilityAccountID  int64                          `json:"lease_liability_account_id"`
	InterestExpenseAccountID int64                          `json:"interest_expense_account_id"`
	Status                   string                         `json:"status"`
	InitialROUCents          int64                          `json:"initial_rou_cents"`
	InitialLiabilityCents    int64                          `json:"initial_liability_cents"`
	JournalEntryID           int64                          `json:"journal_entry_id,omitempty"`
	Schedule                 []leasePaymentScheduleResponse `json:"schedule,omitempty"`
}

func (service *Service) CreateLeaseContract(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateLeaseContractRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateLeaseContractRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	uid := userID(request)

	var result leaseContractResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Idempotent replay.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			lc, ferr := fetchLeaseByJournal(request.Context(), tx, tenant, existing.ID)
			if ferr == nil {
				result = *lc
				return nil
			}
		} else if !isNoRows(err) {
			return err
		}

		// Resolve accounts.
		rouAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, rouAssetAccountCode)
		if err != nil {
			return err
		}
		liabilityAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, leaseLiabilityAccountCode)
		if err != nil {
			return err
		}
		interestAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, interestExpenseAccountCode)
		if err != nil {
			return err
		}

		// Calculate PV of lease payments.
		rate := parseDiscountRate(req.DiscountRate)
		pvCents := presentValueCents(req.PaymentAmountCents, rate, req.TotalPayments)
		if pvCents <= 0 {
			return fmt.Errorf("lease PV is zero or negative; check discount rate and payments")
		}

		// Build journal: Dr 1701 ROU Asset / Cr 2301 Lease Liability.
		journalLines := []accounting.Line{
			{AccountID: rouAcctID, DebitCents: pvCents, SourceLineRef: "rou-asset"},
			{AccountID: liabilityAcctID, CreditCents: pvCents, SourceLineRef: "lease-liability"},
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("LEASE-INIT-%s", req.LesseeName)
		desc := req.Description
		if desc == "" {
			desc = fmt.Sprintf("Lease commencement: %s", req.LesseeName)
		}
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType(intentLeaseInitial),
			EntryDate:   req.StartDate,
			Description: desc,
			Lines:       journalLines,
		}
		entryID, err := postJournal(request.Context(), tx, tenant, journal, idem, uid)
		if err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "lease.initial.posted", mustJSON(map[string]any{
			"journal_id": entryID, "lessee": req.LesseeName, "pv_cents": pvCents,
		})); err != nil {
			return err
		}

		// Allocate lease number.
		leaseNumber, err := nextDocNumber(request.Context(), tx, tenant, "LEASE", "LS")
		if err != nil {
			return err
		}
		startDate, _ := parseDate(req.StartDate)
		endDate, _ := parseDate(req.EndDate)
		rateValue := pgtype.Numeric{}
		_ = rateValue.Scan(req.DiscountRate)

		var leaseID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO lease_contracts (
				tenant_id, number, lessee_name, lessor_name, start_date, end_date,
				payment_amount_cents, payment_frequency, total_payments, discount_rate,
				rou_asset_account_id, lease_liability_account_id, interest_expense_account_id,
				status, initial_rou_cents, initial_liability_cents, journal_entry_id, created_by
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'ACTIVE', $14, $14, $15, $16)
			RETURNING id
		`, tenant, leaseNumber, req.LesseeName, textValueOptional(req.LessorName),
			startDate, endDate, req.PaymentAmountCents, req.PaymentFrequency,
			req.TotalPayments, rateValue, rouAcctID, liabilityAcctID, interestAcctID,
			pvCents, entryID, int8Value(uid)).Scan(&leaseID)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("lease number %s already exists", leaseNumber)
			}
			return err
		}

		// Generate payment schedule.
		schedule := buildPaymentSchedule(startDate.Time, req.PaymentFrequency, req.TotalPayments, req.PaymentAmountCents, rate, pvCents)
		for _, p := range schedule {
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO lease_payments (tenant_id, lease_id, payment_no, payment_date, payment_amount_cents, principal_cents, interest_cents, remaining_liability_cents, posted)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false)
			`, tenant, leaseID, p.PaymentNo, p.PaymentDate, p.PaymentAmountCents,
				p.PrincipalCents, p.InterestCents, p.RemainingLiabilityCents); err != nil {
				return err
			}
		}

		result = leaseContractResponse{
			ID:                       leaseID,
			Number:                   leaseNumber,
			LesseeName:               req.LesseeName,
			LessorName:               req.LessorName,
			StartDate:                req.StartDate,
			EndDate:                  req.EndDate,
			PaymentAmountCents:       req.PaymentAmountCents,
			PaymentFrequency:         req.PaymentFrequency,
			TotalPayments:            req.TotalPayments,
			DiscountRate:             req.DiscountRate,
			ROUAssetAccountID:        rouAcctID,
			LeaseLiabilityAccountID:  liabilityAcctID,
			InterestExpenseAccountID: interestAcctID,
			Status:                   statusActive,
			InitialROUCents:          pvCents,
			InitialLiabilityCents:    pvCents,
			JournalEntryID:           entryID,
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "LEASE_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ---------------------------------------------------------------------------
// List & Get (US-111)
// ---------------------------------------------------------------------------

func (service *Service) ListLeaseContracts(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, number, lessee_name, lessor_name, start_date, end_date,
		       payment_amount_cents, payment_frequency, total_payments, discount_rate,
		       rou_asset_account_id, lease_liability_account_id, interest_expense_account_id,
		       status, initial_rou_cents, initial_liability_cents, journal_entry_id
		FROM lease_contracts
		WHERE tenant_id = $1
		ORDER BY start_date DESC, id DESC
	`, tenant)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	items := make([]leaseContractResponse, 0)
	for rows.Next() {
		var lc leaseContractResponse
		var startDate, endDate pgtype.Date
		var lessorName pgtype.Text
		var journalID pgtype.Int8
		var rate pgtype.Numeric
		if err := rows.Scan(&lc.ID, &lc.Number, &lc.LesseeName, &lessorName,
			&startDate, &endDate, &lc.PaymentAmountCents, &lc.PaymentFrequency,
			&lc.TotalPayments, &rate, &lc.ROUAssetAccountID, &lc.LeaseLiabilityAccountID,
			&lc.InterestExpenseAccountID, &lc.Status, &lc.InitialROUCents,
			&lc.InitialLiabilityCents, &journalID); err != nil {
			writeError(writer, http.StatusInternalServerError, "LIST_FAILED", err.Error())
			return
		}
		lc.StartDate = dateString(startDate)
		lc.EndDate = dateString(endDate)
		lc.LessorName = textValue(lessorName)
		lc.DiscountRate = numericToString(rate)
		lc.JournalEntryID = int8ValueRaw(journalID)
		items = append(items, lc)
	}
	writeJSON(writer, http.StatusOK, items)
}

func (service *Service) GetLeaseContract(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	leaseID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var result *leaseContractResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var ferr error
		result, ferr = fetchLeaseByID(request.Context(), tx, tenant, leaseID)
		return ferr
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "LEASE_NOT_FOUND", err.Error())
		return
	}

	// Load schedule.
	schedRows, err := service.pool.Query(request.Context(), `
		SELECT payment_no, payment_date, payment_amount_cents, principal_cents,
		       interest_cents, remaining_liability_cents, journal_entry_id, posted
		FROM lease_payments
		WHERE tenant_id = $1 AND lease_id = $2
		ORDER BY payment_no
	`, tenant, leaseID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LOAD_FAILED", err.Error())
		return
	}
	defer schedRows.Close()
	schedule := make([]leasePaymentScheduleResponse, 0)
	for schedRows.Next() {
		var s leasePaymentScheduleResponse
		var payDate pgtype.Date
		var journalID pgtype.Int8
		if err := schedRows.Scan(&s.PaymentNo, &payDate, &s.PaymentAmountCents,
			&s.PrincipalCents, &s.InterestCents, &s.RemainingLiabilityCents,
			&journalID, &s.Posted); err != nil {
			writeError(writer, http.StatusInternalServerError, "LOAD_FAILED", err.Error())
			return
		}
		s.PaymentDate = dateString(payDate)
		s.JournalEntryID = int8ValueRaw(journalID)
		schedule = append(schedule, s)
	}
	result.Schedule = schedule

	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Validation + fetch helpers
// ---------------------------------------------------------------------------

func validateLeaseContractRequest(req CreateLeaseContractRequest) (string, string) {
	if req.LesseeName == "" {
		return "INVALID_REQUEST", "lessee_name is required"
	}
	if !validDate(req.StartDate) {
		return "INVALID_REQUEST", "start_date must be a valid YYYY-MM-DD date"
	}
	if !validDate(req.EndDate) {
		return "INVALID_REQUEST", "end_date must be a valid YYYY-MM-DD date"
	}
	if req.PaymentAmountCents <= 0 {
		return "INVALID_REQUEST", "payment_amount_cents must be > 0"
	}
	switch req.PaymentFrequency {
	case freqMonthly, freqQuarterly, freqAnnually:
	default:
		return "INVALID_REQUEST", "payment_frequency must be MONTHLY, QUARTERLY, or ANNUALLY"
	}
	if req.TotalPayments <= 0 {
		return "INVALID_REQUEST", "total_payments must be > 0"
	}
	rate := parseDiscountRate(req.DiscountRate)
	if rate <= 0 || rate >= 1 {
		return "INVALID_REQUEST", "discount_rate must be between 0 and 1 (exclusive)"
	}
	return "", ""
}

func fetchLeaseByID(ctx context.Context, tx pgx.Tx, tenant, leaseID int64) (*leaseContractResponse, error) {
	var lc leaseContractResponse
	var startDate, endDate pgtype.Date
	var lessorName pgtype.Text
	var journalID pgtype.Int8
	var rate pgtype.Numeric
	err := tx.QueryRow(ctx, `
		SELECT id, number, lessee_name, lessor_name, start_date, end_date,
		       payment_amount_cents, payment_frequency, total_payments, discount_rate,
		       rou_asset_account_id, lease_liability_account_id, interest_expense_account_id,
		       status, initial_rou_cents, initial_liability_cents, journal_entry_id
		FROM lease_contracts
		WHERE tenant_id = $1 AND id = $2
	`, tenant, leaseID).Scan(&lc.ID, &lc.Number, &lc.LesseeName, &lessorName,
		&startDate, &endDate, &lc.PaymentAmountCents, &lc.PaymentFrequency,
		&lc.TotalPayments, &rate, &lc.ROUAssetAccountID, &lc.LeaseLiabilityAccountID,
		&lc.InterestExpenseAccountID, &lc.Status, &lc.InitialROUCents,
		&lc.InitialLiabilityCents, &journalID)
	if err != nil {
		return nil, err
	}
	lc.StartDate = dateString(startDate)
	lc.EndDate = dateString(endDate)
	lc.LessorName = textValue(lessorName)
	lc.DiscountRate = numericToString(rate)
	lc.JournalEntryID = int8ValueRaw(journalID)
	return &lc, nil
}

func fetchLeaseByJournal(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (*leaseContractResponse, error) {
	var lc leaseContractResponse
	var startDate, endDate pgtype.Date
	var lessorName pgtype.Text
	var journalIDOut pgtype.Int8
	var rate pgtype.Numeric
	err := tx.QueryRow(ctx, `
		SELECT id, number, lessee_name, lessor_name, start_date, end_date,
		       payment_amount_cents, payment_frequency, total_payments, discount_rate,
		       rou_asset_account_id, lease_liability_account_id, interest_expense_account_id,
		       status, initial_rou_cents, initial_liability_cents, journal_entry_id
		FROM lease_contracts
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&lc.ID, &lc.Number, &lc.LesseeName, &lessorName,
		&startDate, &endDate, &lc.PaymentAmountCents, &lc.PaymentFrequency,
		&lc.TotalPayments, &rate, &lc.ROUAssetAccountID, &lc.LeaseLiabilityAccountID,
		&lc.InterestExpenseAccountID, &lc.Status, &lc.InitialROUCents,
		&lc.InitialLiabilityCents, &journalIDOut)
	if err != nil {
		return nil, err
	}
	lc.StartDate = dateString(startDate)
	lc.EndDate = dateString(endDate)
	lc.LessorName = textValue(lessorName)
	lc.DiscountRate = numericToString(rate)
	lc.JournalEntryID = int8ValueRaw(journalIDOut)
	return &lc, nil
}

// ---------------------------------------------------------------------------
// PV + payment schedule (effective interest method)
// ---------------------------------------------------------------------------

// parseDiscountRate parses a string like "0.01" into a float64.
func parseDiscountRate(raw string) float64 {
	var n pgtype.Numeric
	if err := n.Scan(raw); err != nil {
		return 0
	}
	return numericToFloat(n)
}

// presentValueCents computes PV = payment * [1 - (1+r)^-n] / r, rounded to cents.
func presentValueCents(paymentCents int64, rate float64, n int) int64 {
	if rate <= 0 || n <= 0 {
		return 0
	}
	pv := float64(paymentCents) * (1 - math.Pow(1+rate, -float64(n))) / rate
	return int64(pv + 0.5)
}

// scheduledPayment is one row of the amortization schedule.
type scheduledPayment struct {
	PaymentNo               int
	PaymentDate             time.Time
	PaymentAmountCents      int64
	PrincipalCents          int64
	InterestCents           int64
	RemainingLiabilityCents int64
}

// buildPaymentSchedule generates the full amortization schedule using the
// effective interest method: interest = remaining * rate, principal = payment - interest.
func buildPaymentSchedule(startDate time.Time, frequency string, totalPayments int, paymentCents int64, rate float64, pvCents int64) []scheduledPayment {
	schedule := make([]scheduledPayment, 0, totalPayments)
	remaining := pvCents
	stepMonths := frequencyMonths(frequency)
	for i := 1; i <= totalPayments; i++ {
		interest := int64(float64(remaining) * rate)
		principal := paymentCents - interest
		if principal < 0 {
			principal = 0
		}
		remaining -= principal
		if remaining < 0 {
			remaining = 0
		}
		// Last payment: absorb rounding residual into principal.
		if i == totalPayments && remaining > 0 {
			principal += remaining
			remaining = 0
		}
		payDate := startDate.AddDate(0, stepMonths*(i-1), 0)
		schedule = append(schedule, scheduledPayment{
			PaymentNo:               i,
			PaymentDate:             payDate,
			PaymentAmountCents:      paymentCents,
			PrincipalCents:          principal,
			InterestCents:           interest,
			RemainingLiabilityCents: remaining,
		})
	}
	return schedule
}

func frequencyMonths(frequency string) int {
	switch frequency {
	case freqMonthly:
		return 1
	case freqQuarterly:
		return 3
	case freqAnnually:
		return 12
	default:
		return 1
	}
}
