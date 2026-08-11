package tax

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// PPh Final UMKM (US-081)
// ---------------------------------------------------------------------------
//
// PPh Final UMKM is 0,5% of monthly sales turnover, calculated each month and
// remitted to the treasury. The provision posts:
//
//	Dr 5208 Income Tax Expense   (the periodic tax charge)
//	   Cr 2203 Income Tax Payable (the liability settled on remittance)
//
// The payment settles the payable:
//
//	Dr 2203 Income Tax Payable
//	   Cr <cash/bank account>
//
// The monthly revenue base is the credit turnover of the sales account (4101)
// inside the requested month, net of credit notes (4201). The rate is read
// from the tax_rates table (PPH_FINAL_UMKM) effective on the period's month.

// CalculatePPhFinalRequest is the POST /pph-final/calculate body.
type CalculatePPhFinalRequest struct {
	PeriodYear  int    `json:"period_year"`
	PeriodMonth int    `json:"period_month"`
	EntryDate   string `json:"entry_date"`
	Notes       string `json:"notes"`
}

// PPhFinalResult is the response for both calculate and pay endpoints.
type PPhFinalResult struct {
	JournalEntryID      int64  `json:"journal_entry_id"`
	Number              string `json:"number"`
	IntentType          string `json:"intent_type"`
	EntryDate           string `json:"entry_date"`
	Description         string `json:"description"`
	RevenueCents        int64  `json:"revenue_cents"`
	TaxRate             string `json:"tax_rate"`
	TaxCents            int64  `json:"tax_cents"`
	PayableBalanceCents int64  `json:"payable_balance_cents"`
}

// CalculatePPhFinal computes the monthly PPh Final UMKM from the sales
// turnover and posts the provision journal (Dr 5208 / Cr 2203). Idempotent
// via the Idempotency-Key header.
func (service *Service) CalculatePPhFinal(writer http.ResponseWriter, request *http.Request) {
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
	var req CalculatePPhFinalRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := validatePPhFinalRequest(req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := service.postPPhFinalProvision(request.Context(), tenant, idem, req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errNoOpenPeriod) {
			status = http.StatusBadRequest
		}
		writeError(writer, status, "PPH_FINAL_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) postPPhFinalProvision(ctx context.Context, tenant int64, idem string, req CalculatePPhFinalRequest) (PPhFinalResult, error) {
	result := PPhFinalResult{}
	uid := userIDFromCtx(ctx)
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if err := withTenant(ctx, tx, tenant); err != nil {
			return err
		}
		// Resolve rate for the requested month.
		rate, rateStr, err := resolveTaxRate(ctx, tx, tenant, "PPH_FINAL_UMKM", req.PeriodYear, req.PeriodMonth)
		if err != nil {
			return err
		}

		// Compute revenue base: credit turnover of 4101 minus debit turnover of
		// 4201 (sales returns) within the month. Both are credit-normal so we
		// net the debit movement on returns against the credit movement on sales.
		revenueCents, err := monthlyRevenue(ctx, tx, tenant, req.PeriodYear, req.PeriodMonth)
		if err != nil {
			return err
		}
		taxCents := percentageRound(revenueCents, rate)
		if taxCents <= 0 {
			return fmt.Errorf("no PPh Final provision: revenue for %04d-%02d is zero", req.PeriodYear, req.PeriodMonth)
		}

		// Resolve accounts.
		expenseID, err := resolveAccountByCode(ctx, tx, tenant, pphExpenseAccountCode) // 5208
		if err != nil {
			return err
		}
		payableID, err := resolveAccountByCode(ctx, tx, tenant, pphPayableAccountCode) // 2203
		if err != nil {
			return err
		}

		// Post journal. postJournal handles idempotent replay internally.
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   "PPH-FINAL/" + fmt.Sprintf("%04d-%02d", req.PeriodYear, req.PeriodMonth),
			IntentType:  accounting.IntentType("PPH_FINAL"),
			EntryDate:   req.EntryDate,
			Description: fmt.Sprintf("PPh Final UMKM %04d-%02d (%s%% of turnover)", req.PeriodYear, req.PeriodMonth, rateStr),
			Lines: []accounting.Line{
				{AccountID: expenseID, DebitCents: taxCents, SourceLineRef: "pph-final-expense"},
				{AccountID: payableID, CreditCents: taxCents, SourceLineRef: "pph-final-payable"},
			},
		}
		posted, err := postJournal(ctx, tx, tenant, idem, journal, uid)
		if err != nil {
			return err
		}

		// If this was a replay, fetch the stored description for the response.
		description := journal.Description
		intentType := string(journal.IntentType)
		if posted.Number != "" {
			stored, serr := fetchJournalHeader(ctx, tx, tenant, posted.ID)
			if serr == nil {
				description = stored.description
				intentType = stored.intentType
			}
		}

		payableBalance, _ := accountBalance(ctx, tx, tenant, payableID)

		result = PPhFinalResult{
			JournalEntryID:      posted.ID,
			Number:              posted.Number,
			IntentType:          intentType,
			EntryDate:           journal.EntryDate,
			Description:         description,
			RevenueCents:        revenueCents,
			TaxRate:             rateStr,
			TaxCents:            taxCents,
			PayableBalanceCents: payableBalance,
		}

		if err := audit.Log(ctx, tx, tenant, uid, "pph_final", posted.ID, audit.ActionPost, nil, map[string]any{
			"period_year":      req.PeriodYear,
			"period_month":     req.PeriodMonth,
			"revenue_cents":    revenueCents,
			"tax_rate":         rateStr,
			"tax_cents":        taxCents,
			"journal_entry_id": posted.ID,
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return PPhFinalResult{}, err
	}
	return result, nil
}

func validatePPhFinalRequest(req CalculatePPhFinalRequest) error {
	if req.PeriodYear < 2000 || req.PeriodYear > 2100 {
		return errors.New("period_year must be a valid 4-digit year")
	}
	if req.PeriodMonth < 1 || req.PeriodMonth > 12 {
		return errors.New("period_month must be between 1 and 12")
	}
	if !validDate(req.EntryDate) {
		return errors.New("entry_date is required (YYYY-MM-DD)")
	}
	return nil
}

// PayPPhFinalRequest is the POST /pph-final/pay body.
type PayPPhFinalRequest struct {
	EntryDate     string `json:"entry_date"`
	CashAccountID int64  `json:"cash_account_id"`
	AmountCents   int64  `json:"amount_cents"`
	Notes         string `json:"notes"`
}

// PayPPhFinal records the settlement of the PPh payable (Dr 2203 / Cr Cash).
func (service *Service) PayPPhFinal(writer http.ResponseWriter, request *http.Request) {
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
	var req PayPPhFinalRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := validatePayPPhFinal(req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := service.postPPhFinalPayment(request.Context(), tenant, idem, req)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TAX_PAYMENT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) postPPhFinalPayment(ctx context.Context, tenant int64, idem string, req PayPPhFinalRequest) (PPhFinalResult, error) {
	result := PPhFinalResult{}
	uid := userIDFromCtx(ctx)
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if err := withTenant(ctx, tx, tenant); err != nil {
			return err
		}

		payableID, err := resolveAccountByCode(ctx, tx, tenant, pphPayableAccountCode) // 2203
		if err != nil {
			return err
		}
		// Verify cash account belongs to tenant and is a cash/bank account.
		var accountType string
		err = tx.QueryRow(ctx, `
			SELECT account_type FROM accounts WHERE tenant_id = $1 AND id = $2 AND is_active = true AND is_group = false
		`, tenant, req.CashAccountID).Scan(&accountType)
		if err != nil {
			return fmt.Errorf("cash account not found: %w", err)
		}
		if accountType != "CASH" && accountType != "BANK" {
			return fmt.Errorf("cash_account_id must be a CASH or BANK account")
		}

		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   "TAX-PAYMENT/" + req.EntryDate,
			IntentType:  accounting.IntentType("TAX_PAYMENT"),
			EntryDate:   req.EntryDate,
			Description: "PPh Final tax payment" + optionalNote(req.Notes),
			Lines: []accounting.Line{
				{AccountID: payableID, DebitCents: req.AmountCents, SourceLineRef: "tax-payment-payable"},
				{AccountID: req.CashAccountID, CreditCents: req.AmountCents, SourceLineRef: "tax-payment-cash"},
			},
		}
		posted, err := postJournal(ctx, tx, tenant, idem, journal, uid)
		if err != nil {
			return err
		}
		description := journal.Description
		intentType := string(journal.IntentType)
		if stored, serr := fetchJournalHeader(ctx, tx, tenant, posted.ID); serr == nil {
			description = stored.description
			intentType = stored.intentType
		}
		payableBalance, _ := accountBalance(ctx, tx, tenant, payableID)
		result = PPhFinalResult{
			JournalEntryID:      posted.ID,
			Number:              posted.Number,
			IntentType:          intentType,
			EntryDate:           journal.EntryDate,
			Description:         description,
			TaxCents:            req.AmountCents,
			PayableBalanceCents: payableBalance,
		}

		if err := audit.Log(ctx, tx, tenant, uid, "pph_final", posted.ID, audit.ActionPost, nil, map[string]any{
			"action":           "payment",
			"amount_cents":     req.AmountCents,
			"journal_entry_id": posted.ID,
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return PPhFinalResult{}, err
	}
	return result, nil
}

func validatePayPPhFinal(req PayPPhFinalRequest) error {
	if !validDate(req.EntryDate) {
		return errors.New("entry_date is required (YYYY-MM-DD)")
	}
	if req.CashAccountID <= 0 {
		return errors.New("cash_account_id is required")
	}
	if req.AmountCents <= 0 {
		return errors.New("amount_cents must be > 0")
	}
	return nil
}

func optionalNote(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return " — " + note
}

// fetchPPhFinalProvision rebuilds the result payload from a stored journal entry.
func fetchPPhFinalProvision(ctx context.Context, tx pgx.Tx, tenant, entryID int64) (PPhFinalResult, error) {
	var r PPhFinalResult
	var intentType, desc pgtype.Text
	var entryDate pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT id, number, intent_type, entry_date, description
		FROM journal_entries WHERE tenant_id = $1 AND id = $2
	`, tenant, entryID).Scan(&r.JournalEntryID, &r.Number, &intentType, &entryDate, &desc)
	if err != nil {
		return PPhFinalResult{}, err
	}
	r.IntentType = textValueTrimmed(intentType)
	r.EntryDate = dateValuePG(entryDate)
	r.Description = textValueTrimmed(desc)
	payableID, _ := resolveAccountByCode(ctx, tx, tenant, pphPayableAccountCode)
	if payableID > 0 {
		r.PayableBalanceCents, _ = accountBalance(ctx, tx, tenant, payableID)
	}
	return r, nil
}

// monthlyRevenue returns the net sales turnover (credit movement on 4101 minus
// debit movement on 4201) for the given month.
func monthlyRevenue(ctx context.Context, tx pgx.Tx, tenantID int64, year, month int) (int64, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	var sales, returns int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id AND je.status = 'POSTED'
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND a.code = '4101'
		  AND je.entry_date BETWEEN $2 AND $3
	`, tenantID, start.Format("2006-01-02"), end.Format("2006-01-02")).Scan(&sales)
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id AND je.status = 'POSTED'
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND a.code = '4201'
		  AND je.entry_date BETWEEN $2 AND $3
	`, tenantID, start.Format("2006-01-02"), end.Format("2006-01-02")).Scan(&returns)
	if err != nil {
		return 0, err
	}
	net := sales - returns
	if net < 0 {
		net = 0
	}
	return net, nil
}

// resolveTaxRate returns the decimal rate (e.g. 0.5) and a display string for
// the given tax type active on the 1st of the requested month.
func resolveTaxRate(ctx context.Context, tx pgx.Tx, tenantID int64, taxType string, year, month int) (float64, string, error) {
	periodStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	var rate float64
	err := tx.QueryRow(ctx, `
		SELECT rate::float8 FROM tax_rates
		WHERE tenant_id = $1 AND tax_type = $2 AND is_active = true
		  AND effective_from <= $3
		  AND (effective_to IS NULL OR effective_to >= $3)
		ORDER BY effective_from DESC LIMIT 1
	`, tenantID, taxType, periodStart.Format("2006-01-02")).Scan(&rate)
	if err != nil {
		return 0, "", fmt.Errorf("no active %s tax rate for %04d-%02d: %w", taxType, year, month, err)
	}
	return rate / 100.0, formatPercent(rate), nil
}

// accountBalance returns the signed debit-credit balance on an account.
func accountBalance(ctx context.Context, tx pgx.Tx, tenantID, accountID int64) (int64, error) {
	var balance int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id AND je.status = 'POSTED'
		WHERE jl.tenant_id = $1 AND jl.account_id = $2
	`, tenantID, accountID).Scan(&balance)
	return balance, err
}

func formatPercent(rate float64) string {
	// rate is stored as a percentage value (e.g. 0.5, 11).
	if rate == float64(int64(rate)) {
		return strconv.FormatFloat(rate, 'f', 1, 64)
	}
	return strconv.FormatFloat(rate, 'f', 2, 64)
}

// percentageRound applies rate to base, rounding half-up to the nearest rupiah.
func percentageRound(base int64, rate float64) int64 {
	if base <= 0 || rate <= 0 {
		return 0
	}
	// base * rate (rate is decimal, e.g. 0.005).
	amount := float64(base) * rate
	rounded := int64(amount + 0.5)
	return rounded
}

// ensure unused imports stay used when the file evolves.
var _ = pgx.ErrNoRows
