package tax

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// ECL / Penyisihan Piutang (US-082, PSAK 48)
// ---------------------------------------------------------------------------
//
// Expected Credit Loss buckets outstanding customer receivables by age and
// applies a loss rate to each bucket:
//
//	0-30   days: 1%
//	31-60  days: 2.5%
//	61-90  days: 5%
//	>90    days: 10%  (configurable via request)
//
// The provision journal adjusts 1202 (Allowance for Doubtful Accounts, contra
// asset) against 5205 (Bad Debt Expense) to reach the target allowance:
//
//	If target > current allowance: Dr 5205 / Cr 1202 (increase provision)
//	If target < current allowance: Dr 1202 / Cr 5205 (release provision)
//
// A write-off removes a specific receivable from the books, consuming any
// allowance already booked against it:
//
//	Dr 1202 / Cr 1201  (write off the receivable against the allowance)

// ECLBucket is one aging bucket in the ECL calculation.
type ECLBucket struct {
	Label          string  `json:"label"`
	MinDays        int     `json:"min_days"`
	MaxDays        int     `json:"max_days"`
	RatePct        float64 `json:"rate_pct"`
	BalanceCents   int64   `json:"balance_cents"`
	ProvisionCents int64   `json:"provision_cents"`
}

// CalculateECLRequest is the POST /ecl/calculate body.
type CalculateECLRequest struct {
	AsOfDate  string `json:"as_of_date"`
	EntryDate string `json:"entry_date"`
	Notes     string `json:"notes"`
	// Rates override the defaults (percentages). Omitted keys fall back.
	Rates map[string]float64 `json:"rates"`
}

// CalculateECLResult is the response for POST /ecl/calculate.
type CalculateECLResult struct {
	JournalEntryID        int64       `json:"journal_entry_id"`
	Number                string      `json:"number"`
	IntentType            string      `json:"intent_type"`
	EntryDate             string      `json:"entry_date"`
	Description           string      `json:"description"`
	AsOfDate              string      `json:"as_of_date"`
	Buckets               []ECLBucket `json:"buckets"`
	TargetAllowanceCents  int64       `json:"target_allowance_cents"`
	CurrentAllowanceCents int64       `json:"current_allowance_cents"`
	AdjustmentCents       int64       `json:"adjustment_cents"`
}

// CalculateECL computes the ECL provision from the AR aging and posts the
// adjustment journal. Idempotent via the Idempotency-Key header.
func (service *Service) CalculateECL(writer http.ResponseWriter, request *http.Request) {
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
	var req CalculateECLRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := validateECLRequest(req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := service.postECLProvision(request.Context(), tenant, idem, req)
	if err != nil {
		if errors.Is(err, errNoAdjustment) {
			writeError(writer, http.StatusUnprocessableEntity, "NO_ECL_ADJUSTMENT", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "ECL_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) postECLProvision(ctx context.Context, tenant int64, idem string, req CalculateECLRequest) (CalculateECLResult, error) {
	result := CalculateECLResult{}
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if err := withTenant(ctx, tx, tenant); err != nil {
			return err
		}
		existing, err := db.New(tx).GetJournalByIdempotencyKey(ctx, db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			r, ferr := fetchECLJournal(ctx, tx, tenant, existing.ID)
			if ferr != nil {
				return ferr
			}
			result = r
			return nil
		} else if !isNoRows(err) {
			return err
		}

		buckets, target, err := computeECLBuckets(ctx, tx, tenant, req.AsOfDate, req.Rates)
		if err != nil {
			return err
		}
		allowanceID, err := resolveAccountByCode(ctx, tx, tenant, allowanceAccountCode) // 1202
		if err != nil {
			return err
		}
		badDebtID, err := resolveAccountByCode(ctx, tx, tenant, badDebtExpenseCode) // 5205
		if err != nil {
			return err
		}
		// Current allowance balance (credit-normal contra asset: balance is
		// credits - debits).
		currentCredit, err := accountBalance(ctx, tx, tenant, allowanceID) // returns debit - credit
		if err != nil {
			return err
		}
		currentAllowance := -currentCredit // flip to credit-normal
		adjustment := target - currentAllowance
		if adjustment == 0 {
			return errNoAdjustment
		}

		var lines []accounting.Line
		if adjustment > 0 {
			// Increase provision: Dr 5205 / Cr 1202.
			lines = []accounting.Line{
				{AccountID: badDebtID, DebitCents: adjustment, SourceLineRef: "ecl-provision-expense"},
				{AccountID: allowanceID, CreditCents: adjustment, SourceLineRef: "ecl-provision-allowance"},
			}
		} else {
			// Release provision: Dr 1202 / Cr 5205.
			amt := -adjustment
			lines = []accounting.Line{
				{AccountID: allowanceID, DebitCents: amt, SourceLineRef: "ecl-release-allowance"},
				{AccountID: badDebtID, CreditCents: amt, SourceLineRef: "ecl-release-expense"},
			}
		}

		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   "ECL/" + req.AsOfDate,
			IntentType:  accounting.IntentType("ECL_PROVISION"),
			EntryDate:   req.EntryDate,
			Description: fmt.Sprintf("ECL provision as of %s%s", req.AsOfDate, optionalNote(req.Notes)),
			Lines:       lines,
		}
		posted, err := postJournal(ctx, tx, tenant, idem, journal, userIDFromCtx(ctx))
		if err != nil {
			return err
		}

		result = CalculateECLResult{
			JournalEntryID:        posted.ID,
			Number:                posted.Number,
			IntentType:            string(journal.IntentType),
			EntryDate:             journal.EntryDate,
			Description:           journal.Description,
			AsOfDate:              req.AsOfDate,
			Buckets:               buckets,
			TargetAllowanceCents:  target,
			CurrentAllowanceCents: currentAllowance,
			AdjustmentCents:       adjustment,
		}
		return nil
	})
	if err != nil {
		return CalculateECLResult{}, err
	}
	return result, nil
}

func validateECLRequest(req CalculateECLRequest) error {
	if !validDate(req.AsOfDate) {
		return errors.New("as_of_date is required (YYYY-MM-DD)")
	}
	if !validDate(req.EntryDate) {
		return errors.New("entry_date is required (YYYY-MM-DD)")
	}
	return nil
}

// computeECLBuckets ages the open AR (1201) into buckets and applies the loss
// rate to each. Returns the buckets (for display) and the target allowance.
func computeECLBuckets(ctx context.Context, tx pgx.Tx, tenantID int64, asOf string, overrides map[string]float64) ([]ECLBucket, int64, error) {
	asOfDate, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		return nil, 0, err
	}
	defaults := []ECLBucket{
		{Label: "0-30", MinDays: 0, MaxDays: 30, RatePct: 1.0},
		{Label: "31-60", MinDays: 31, MaxDays: 60, RatePct: 2.5},
		{Label: "61-90", MinDays: 61, MaxDays: 90, RatePct: 5.0},
		{Label: ">90", MinDays: 91, MaxDays: 999999, RatePct: 10.0},
	}
	for i := range defaults {
		if r, ok := overrides[defaults[i].Label]; ok && r >= 0 {
			defaults[i].RatePct = r
		}
	}

	// Load open invoices (issued, not fully paid) with their outstanding amount.
	// The invoices table stores the open receivable as receivable_cents; status
	// is one of DRAFT, ISSUED, PARTIALLY_PAID, PAID, VOID (see migration 000008).
	rows, err := tx.Query(ctx, `
		SELECT invoice_date, receivable_cents
		FROM invoices
		WHERE tenant_id = $1 AND status IN ('ISSUED','PARTIALLY_PAID')
		  AND receivable_cents > 0
	`, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var totalProvision int64
	for _, b := range defaults {
		b.BalanceCents = 0
		b.ProvisionCents = 0
	}

	bucketMap := make(map[string]*ECLBucket, len(defaults))
	for i := range defaults {
		bucketMap[defaults[i].Label] = &defaults[i]
	}

	for rows.Next() {
		var invoiceDate time.Time
		var payable int64
		if err := rows.Scan(&invoiceDate, &payable); err != nil {
			return nil, 0, err
		}
		ageDays := int(asOfDate.Sub(invoiceDate).Hours() / 24)
		if ageDays < 0 {
			ageDays = 0
		}
		var bucket *ECLBucket
		for i := range defaults {
			if ageDays >= defaults[i].MinDays && ageDays <= defaults[i].MaxDays {
				bucket = &defaults[i]
				break
			}
		}
		if bucket == nil {
			bucket = &defaults[len(defaults)-1] // >90 fallback
		}
		bucket.BalanceCents += payable
	}

	for i := range defaults {
		b := &defaults[i]
		b.ProvisionCents = percentageRound(b.BalanceCents, b.RatePct/100.0)
		totalProvision += b.ProvisionCents
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return defaults, totalProvision, nil
}

// fetchECLJournal rebuilds the result payload from a stored journal entry.
func fetchECLJournal(ctx context.Context, tx pgx.Tx, tenant, entryID int64) (CalculateECLResult, error) {
	var r CalculateECLResult
	var intentType, desc pgtype.Text
	var entryDate pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT id, number, intent_type, entry_date, description
		FROM journal_entries WHERE tenant_id = $1 AND id = $2
	`, tenant, entryID).Scan(&r.JournalEntryID, &r.Number, &intentType, &entryDate, &desc)
	if err != nil {
		return CalculateECLResult{}, err
	}
	r.IntentType = textValueTrimmed(intentType)
	r.EntryDate = dateValuePG(entryDate)
	r.Description = textValueTrimmed(desc)
	return r, nil
}

// ---------------------------------------------------------------------------
// ECL Write-off (US-082)
// ---------------------------------------------------------------------------

// WriteOffRequest is the POST /ecl/write-off body.
type WriteOffRequest struct {
	EntryDate   string `json:"entry_date"`
	InvoiceID   int64  `json:"invoice_id"`
	AmountCents int64  `json:"amount_cents"`
	Notes       string `json:"notes"`
}

// WriteOffResult is the response for POST /ecl/write-off.
type WriteOffResult struct {
	JournalEntryID int64  `json:"journal_entry_id"`
	Number         string `json:"number"`
	IntentType     string `json:"intent_type"`
	EntryDate      string `json:"entry_date"`
	Description    string `json:"description"`
	InvoiceID      int64  `json:"invoice_id"`
	AmountCents    int64  `json:"amount_cents"`
}

// WriteOffReceivable writes off a specific receivable against the allowance
// account: Dr 1202 (Allowance) / Cr 1201 (Accounts Receivable).
func (service *Service) WriteOffReceivable(writer http.ResponseWriter, request *http.Request) {
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
	var req WriteOffRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := validateWriteOff(req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := service.postWriteOff(request.Context(), tenant, idem, req)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ECL_WRITEOFF_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) postWriteOff(ctx context.Context, tenant int64, idem string, req WriteOffRequest) (WriteOffResult, error) {
	result := WriteOffResult{}
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if err := withTenant(ctx, tx, tenant); err != nil {
			return err
		}
		existing, err := db.New(tx).GetJournalByIdempotencyKey(ctx, db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			r, ferr := fetchWriteOffJournal(ctx, tx, tenant, existing.ID)
			if ferr != nil {
				return ferr
			}
			result = r
			return nil
		} else if !isNoRows(err) {
			return err
		}

		allowanceID, err := resolveAccountByCode(ctx, tx, tenant, allowanceAccountCode) // 1202
		if err != nil {
			return err
		}
		arID, err := resolveAccountByCode(ctx, tx, tenant, arAccountCode) // 1201
		if err != nil {
			return err
		}
		// Verify the invoice exists and belongs to the tenant (if invoice_id given).
		if req.InvoiceID > 0 {
			var exists int64
			err = tx.QueryRow(ctx, `SELECT id FROM invoices WHERE tenant_id = $1 AND id = $2`, tenant, req.InvoiceID).Scan(&exists)
			if err != nil {
				return fmt.Errorf("invoice %d not found: %w", req.InvoiceID, err)
			}
		}

		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   "ECL-WRITEOFF/" + req.EntryDate,
			IntentType:  accounting.IntentType("ECL_WRITEOFF"),
			EntryDate:   req.EntryDate,
			Description: fmt.Sprintf("Write-off of uncollectible receivable%s", optionalNote(req.Notes)),
			Lines: []accounting.Line{
				{AccountID: allowanceID, DebitCents: req.AmountCents, SourceLineRef: "writeoff-allowance"},
				{AccountID: arID, CreditCents: req.AmountCents, SourceLineRef: "writeoff-ar"},
			},
		}
		posted, err := postJournal(ctx, tx, tenant, idem, journal, userIDFromCtx(ctx))
		if err != nil {
			return err
		}
		result = WriteOffResult{
			JournalEntryID: posted.ID,
			Number:         posted.Number,
			IntentType:     string(journal.IntentType),
			EntryDate:      journal.EntryDate,
			Description:    journal.Description,
			InvoiceID:      req.InvoiceID,
			AmountCents:    req.AmountCents,
		}
		return nil
	})
	if err != nil {
		return WriteOffResult{}, err
	}
	return result, nil
}

func validateWriteOff(req WriteOffRequest) error {
	if !validDate(req.EntryDate) {
		return errors.New("entry_date is required (YYYY-MM-DD)")
	}
	if req.AmountCents <= 0 {
		return errors.New("amount_cents must be > 0")
	}
	return nil
}

func fetchWriteOffJournal(ctx context.Context, tx pgx.Tx, tenant, entryID int64) (WriteOffResult, error) {
	var r WriteOffResult
	var intentType, desc pgtype.Text
	var entryDate pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT id, number, intent_type, entry_date, description
		FROM journal_entries WHERE tenant_id = $1 AND id = $2
	`, tenant, entryID).Scan(&r.JournalEntryID, &r.Number, &intentType, &entryDate, &desc)
	if err != nil {
		return WriteOffResult{}, err
	}
	r.IntentType = textValueTrimmed(intentType)
	r.EntryDate = dateValuePG(entryDate)
	r.Description = textValueTrimmed(desc)
	return r, nil
}
