package lease

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// US-111: Lease Payment Posting (PSAK 73)
//   Journal: Dr 2301 Lease Liability (principal) + Dr 5906 Interest Expense / Cr Cash
// ---------------------------------------------------------------------------

type leasePaymentResponse struct {
	LeaseID                 int64  `json:"lease_id"`
	PaymentNo               int    `json:"payment_no"`
	PaymentDate             string `json:"payment_date"`
	PaymentAmountCents      int64  `json:"payment_amount_cents"`
	PrincipalCents          int64  `json:"principal_cents"`
	InterestCents           int64  `json:"interest_cents"`
	RemainingLiabilityCents int64  `json:"remaining_liability_cents"`
	JournalEntryID          int64  `json:"journal_entry_id,omitempty"`
	Posted                  bool   `json:"posted"`
}

func (service *Service) PostLeasePayment(writer http.ResponseWriter, request *http.Request) {
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
	leaseID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "lease id is required")
		return
	}
	paymentNo, err := pathID(chi.URLParam(request, "payment_no"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "payment_no is required")
		return
	}

	// Optional body: payment_date + payment_account_code (defaults from schedule / 1101).
	var req struct {
		PaymentDate        string `json:"payment_date"`
		PaymentAccountCode string `json:"payment_account_code"`
		Description        string `json:"description"`
	}
	_ = decodeJSON(request, &req)

	uid := userID(request)

	var result leasePaymentResponse
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
			row, ferr := fetchLeasePaymentByJournal(request.Context(), tx, tenant, existing.ID)
			if ferr == nil {
				result = row
				result.JournalEntryID = existing.ID
				return nil
			}
		} else if !isNoRows(err) {
			return err
		}

		// Load lease contract.
		lease, err := fetchLeaseByID(request.Context(), tx, tenant, leaseID)
		if err != nil {
			return fmt.Errorf("lease not found: %w", err)
		}
		if lease.Status != statusActive {
			return fmt.Errorf("lease %s is %s; only ACTIVE leases can accept payments", lease.Number, lease.Status)
		}

		// Load the payment schedule row.
		var p leasePaymentResponse
		var payDate pgtype.Date
		var journalID pgtype.Int8
		err = tx.QueryRow(request.Context(), `
			SELECT payment_no, payment_date, payment_amount_cents, principal_cents,
			       interest_cents, remaining_liability_cents, journal_entry_id, posted
			FROM lease_payments
			WHERE tenant_id = $1 AND lease_id = $2 AND payment_no = $3
		`, tenant, leaseID, paymentNo).Scan(&p.PaymentNo, &payDate, &p.PaymentAmountCents,
			&p.PrincipalCents, &p.InterestCents, &p.RemainingLiabilityCents, &journalID, &p.Posted)
		if err != nil {
			return fmt.Errorf("payment %d not found: %w", paymentNo, err)
		}
		if p.Posted {
			return fmt.Errorf("payment %d is already posted", paymentNo)
		}
		p.LeaseID = leaseID
		p.PaymentDate = dateString(payDate)

		// Resolve cash account.
		cashCode := req.PaymentAccountCode
		if cashCode == "" {
			cashCode = cashAccountCode
		}
		cashAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, cashCode)
		if err != nil {
			return err
		}

		// Entry date: override from body, else use scheduled payment date.
		entryDate := req.PaymentDate
		if entryDate == "" {
			entryDate = p.PaymentDate
		}
		if !validDate(entryDate) {
			return fmt.Errorf("payment_date must be a valid YYYY-MM-DD date")
		}

		// Build journal: Dr 2301 Lease Liability (principal) + Dr 5906 Interest / Cr Cash.
		journalLines := []accounting.Line{
			{AccountID: lease.LeaseLiabilityAccountID, DebitCents: p.PrincipalCents, SourceLineRef: "lease-principal"},
			{AccountID: lease.InterestExpenseAccountID, DebitCents: p.InterestCents, SourceLineRef: "lease-interest"},
			{AccountID: cashAcctID, CreditCents: p.PaymentAmountCents, SourceLineRef: "cash-payment"},
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		desc := req.Description
		if desc == "" {
			desc = fmt.Sprintf("Lease payment %d/%d: %s", p.PaymentNo, lease.TotalPayments, lease.LesseeName)
		}
		sourceRef := fmt.Sprintf("LEASE-PAY-%d-%d", leaseID, paymentNo)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType(intentLeasePayment),
			EntryDate:   entryDate,
			Description: desc,
			Lines:       journalLines,
		}
		entryID, err := postJournal(request.Context(), tx, tenant, journal, idem, uid)
		if err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "lease.payment.posted", mustJSON(map[string]any{
			"journal_id": entryID, "lease_id": leaseID, "payment_no": paymentNo,
			"principal_cents": p.PrincipalCents, "interest_cents": p.InterestCents,
		})); err != nil {
			return err
		}

		// Mark the payment schedule row as posted.
		if _, err := tx.Exec(request.Context(), `
			UPDATE lease_payments SET posted = true, journal_entry_id = $1
			WHERE tenant_id = $2 AND lease_id = $3 AND payment_no = $4
		`, entryID, tenant, leaseID, paymentNo); err != nil {
			return err
		}

		p.JournalEntryID = entryID
		p.Posted = true
		result = p
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "LEASE_PAYMENT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func fetchLeasePaymentByJournal(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (leasePaymentResponse, error) {
	var p leasePaymentResponse
	var payDate pgtype.Date
	var journalIDOut pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT lease_id, payment_no, payment_date, payment_amount_cents, principal_cents,
		       interest_cents, remaining_liability_cents, journal_entry_id
		FROM lease_payments
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&p.LeaseID, &p.PaymentNo, &payDate, &p.PaymentAmountCents,
		&p.PrincipalCents, &p.InterestCents, &p.RemainingLiabilityCents, &journalIDOut)
	if err != nil {
		return leasePaymentResponse{}, err
	}
	p.PaymentDate = dateString(payDate)
	p.JournalEntryID = int8ValueRaw(journalIDOut)
	p.Posted = true
	return p, nil
}
