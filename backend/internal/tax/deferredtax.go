package tax

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// Deferred Tax (US-083, PSAK 46) — minimal.
// ---------------------------------------------------------------------------
//
// Deferred tax arises from temporary differences between the carrying amount
// of an asset/liability and its tax base. This minimal endpoint takes the
// signed temporary difference (in cents) and the tax rate (percentage), then
// posts the deferred tax movement:
//
//	temporary_differences_cents > 0  →  Dr 1206 Deferred Tax Asset / Cr 5904
//	temporary_differences_cents < 0  →  Dr 5904 / Cr 1206  (reversal)
//
// The rate is supplied by the caller (the period-close workflow that already
// knows the statutory rate). No tax_rates row is read here.

// CalculateDeferredTaxRequest is the POST /deferred-tax/calculate body.
type CalculateDeferredTaxRequest struct {
	TemporaryDifferencesCents int64   `json:"temporary_differences_cents"`
	TaxRate                   float64 `json:"tax_rate"` // percentage, e.g. 22 for 22%
	EntryDate                 string  `json:"entry_date"`
	Notes                     string  `json:"notes"`
}

// CalculateDeferredTaxResult is the response for POST /deferred-tax/calculate.
type CalculateDeferredTaxResult struct {
	JournalEntryID            int64  `json:"journal_entry_id"`
	Number                    string `json:"number"`
	IntentType                string `json:"intent_type"`
	EntryDate                 string `json:"entry_date"`
	Description               string `json:"description"`
	TemporaryDifferencesCents int64  `json:"temporary_differences_cents"`
	TaxRate                   string `json:"tax_rate"`
	DeferredTaxCents          int64  `json:"deferred_tax_cents"`
	Direction                 string `json:"direction"` // ASSET or REVERSAL
}

// CalculateDeferredTax posts the deferred tax movement for a set of temporary
// differences. Idempotent via the Idempotency-Key header.
func (service *Service) CalculateDeferredTax(writer http.ResponseWriter, request *http.Request) {
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
	var req CalculateDeferredTaxRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := validateDeferredTax(req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := service.postDeferredTax(request.Context(), tenant, idem, req)
	if err != nil {
		if errors.Is(err, errNoDeferredTax) {
			writeError(writer, http.StatusUnprocessableEntity, "NO_DEFERRED_TAX", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "DEFERRED_TAX_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) postDeferredTax(ctx context.Context, tenant int64, idem string, req CalculateDeferredTaxRequest) (CalculateDeferredTaxResult, error) {
	result := CalculateDeferredTaxResult{}
	uid := userIDFromCtx(ctx)
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if err := withTenant(ctx, tx, tenant); err != nil {
			return err
		}

		deferredCents := percentageRound(abs64(req.TemporaryDifferencesCents), req.TaxRate/100.0)
		if deferredCents <= 0 {
			return errNoDeferredTax
		}

		assetID, err := resolveAccountByCode(ctx, tx, tenant, deferredTaxAssetCode) // 1206
		if err != nil {
			return err
		}
		expenseID, err := resolveAccountByCode(ctx, tx, tenant, deferredTaxExpenseCode) // 5904
		if err != nil {
			return err
		}

		var lines []accounting.Line
		direction := "ASSET"
		if req.TemporaryDifferencesCents > 0 {
			// Dr 1206 Deferred Tax Asset / Cr 5904 Deferred Tax Expense.
			lines = []accounting.Line{
				{AccountID: assetID, DebitCents: deferredCents, SourceLineRef: "deferred-tax-asset"},
				{AccountID: expenseID, CreditCents: deferredCents, SourceLineRef: "deferred-tax-expense"},
			}
		} else {
			// Reversal: Dr 5904 / Cr 1206.
			direction = "REVERSAL"
			lines = []accounting.Line{
				{AccountID: expenseID, DebitCents: deferredCents, SourceLineRef: "deferred-tax-reversal-expense"},
				{AccountID: assetID, CreditCents: deferredCents, SourceLineRef: "deferred-tax-reversal-asset"},
			}
		}

		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   "DEFERRED-TAX/" + req.EntryDate,
			IntentType:  accounting.IntentType("DEFERRED_TAX"),
			EntryDate:   req.EntryDate,
			Description: fmt.Sprintf("Deferred tax (%s) at %s%%%s", direction, formatPercent(req.TaxRate), optionalNote(req.Notes)),
			Lines:       lines,
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
		result = CalculateDeferredTaxResult{
			JournalEntryID:            posted.ID,
			Number:                    posted.Number,
			IntentType:                intentType,
			EntryDate:                 journal.EntryDate,
			Description:               description,
			TemporaryDifferencesCents: req.TemporaryDifferencesCents,
			TaxRate:                   formatPercent(req.TaxRate),
			DeferredTaxCents:          deferredCents,
			Direction:                 direction,
		}
		return nil
	})
	if err != nil {
		return CalculateDeferredTaxResult{}, err
	}
	return result, nil
}

func validateDeferredTax(req CalculateDeferredTaxRequest) error {
	if req.TemporaryDifferencesCents == 0 {
		return errors.New("temporary_differences_cents must be non-zero")
	}
	if req.TaxRate <= 0 || req.TaxRate > 100 {
		return errors.New("tax_rate must be between 0 and 100")
	}
	if !validDate(req.EntryDate) {
		return errors.New("entry_date is required (YYYY-MM-DD)")
	}
	return nil
}

// errNoDeferredTax signals that the temporary differences produce no movement.
var errNoDeferredTax = errors.New("deferred tax movement is zero; nothing to post")

// abs64 returns the absolute value of an int64.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
