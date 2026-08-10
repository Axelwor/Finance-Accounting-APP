package lease

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// DepreciateLeaseContract handles POST /lease-contracts/{id}/depreciate.
//
// It posts the monthly RoU depreciation journal:
//   Dr 5209 RoU Depreciation Expense / Cr 1702 Accumulated RoU Depreciation
//
// The depreciation amount is: rou_cost_cents / total_months
// (straight-line over the lease term).
//
// Idempotent per (contract_id, period_year, period_month) via the
// intent_type=LEASE_DEPRECIATION + source_ref=DEP-{contract_id}-{year}-{month}
// unique constraint.
func (service *Service) DepreciateLeaseContract(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	uid := userID(request)

	contractID, err := strconv.ParseInt(chi.URLParam(request, "id"), 10, 64)
	if err != nil || contractID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}

	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// The caller may override the depreciation month/year via query params
	// ?year=2026&month=8. Defaults to the current month.
	now := time.Now()
	year := now.Year()
	month := int(now.Month())
	if raw := request.URL.Query().Get("year"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			year = parsed
		}
	}
	if raw := request.URL.Query().Get("month"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 12 {
			month = parsed
		}
	}

	result, err := service.postRoUDepreciation(request.Context(), tenantID, contractID, uid, idem, year, month)
	if err != nil {
		status := http.StatusInternalServerError
		code := "DEPRECIATION_FAILED"
		msg := err.Error()
		if isUniqueViolation(err) {
			status = http.StatusConflict
			code = "DEPRECIATION_ALREADY_POSTED"
			msg = fmt.Sprintf("RoU depreciation for contract %d in %d-%02d has already been posted", contractID, year, month)
		}
		writeError(writer, status, code, msg)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// postRoUDepreciation posts the RoU depreciation journal inside a transaction.
func (service *Service) postRoUDepreciation(ctx context.Context, tenantID, contractID, userID int64, idem string, year, month int) (map[string]any, error) {
	var result map[string]any
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if err := withTenant(ctx, tx, tenantID); err != nil {
			return err
		}

		// Load the lease contract and verify it is ACTIVE.
		var rouCostCents int64
		var totalMonths int
		var startDate, endDate time.Time
		var statusStr string
		var contractNumber string
		err := tx.QueryRow(ctx, `
			SELECT rou_cost_cents, total_months, start_date, end_date, status, number
			FROM lease_contracts
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, contractID).Scan(&rouCostCents, &totalMonths, &startDate, &endDate, &statusStr, &contractNumber)
		if err != nil {
			return fmt.Errorf("lease contract not found: %w", err)
		}
		if statusStr != statusActive {
			return fmt.Errorf("lease contract is %s, must be ACTIVE to depreciate", statusStr)
		}
		if rouCostCents <= 0 {
			return fmt.Errorf("lease contract has zero RoU cost; nothing to depreciate")
		}
		if totalMonths <= 0 {
			return fmt.Errorf("lease contract has zero total months; cannot compute depreciation")
		}

		// Check the depreciation month is within the lease term.
		depDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		if depDate.Before(startDate) || depDate.After(endDate) {
			return fmt.Errorf("depreciation date %04d-%02d is outside lease term (%s to %s)",
				year, month, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		}

		// Straight-line: monthly depreciation = rou_cost / total_months.
		monthlyDepreciation := rouCostCents / int64(totalMonths)
		if monthlyDepreciation <= 0 {
			monthlyDepreciation = 1 // at least 1 cent to avoid zero-amount journals
		}

		// Check how much has already been depreciated.
		var accumDepAcctID int64
		_ = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = '1702'`, tenantID).Scan(&accumDepAcctID)
		var alreadyDepreciated int64
		if accumDepAcctID > 0 {
			_ = tx.QueryRow(ctx, `
				SELECT COALESCE(SUM(jl.credit_cents), 0)
				FROM journal_lines jl
				JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
				WHERE jl.tenant_id = $1 AND jl.account_id = $2
				  AND je.intent_type = $3 AND je.source_ref LIKE $4
			`, tenantID, accumDepAcctID, intentLeaseDepreciation, fmt.Sprintf("DEP-%d-%%", contractID)).Scan(&alreadyDepreciated)
		}

		// Don't depreciate beyond cost.
		if alreadyDepreciated >= rouCostCents {
			return fmt.Errorf("lease contract %d is fully depreciated", contractID)
		}

		amount := monthlyDepreciation
		remaining := rouCostCents - alreadyDepreciated
		// Last month: absorb residual.
		if remaining < amount {
			amount = remaining
		}

		// Resolve the 1702 and 5209 accounts.
		depExpAcctID, err := resolveAccountByCode(ctx, tx, tenantID, rouDepreciationExpenseCode)
		if err != nil {
			return err
		}

		// Build the journal: Dr 5209 / Cr 1702.
		sourceRef := fmt.Sprintf("DEP-%d-%04d-%02d", contractID, year, month)
		entryDate := fmt.Sprintf("%04d-%02d-01", year, month)
		journal := accounting.Journal{
			TenantID:    tenantID,
			SourceRef:   sourceRef,
			IntentType:  intentLeaseDepreciation,
			EntryDate:   entryDate,
			Description: fmt.Sprintf("RoU depreciation: %s (%04d-%02d)", contractNumber, year, month),
			Lines: []accounting.Line{
				{AccountID: depExpAcctID, DebitCents: amount, SourceLineRef: "dep-exp"},
				{AccountID: accumDepAcctID, CreditCents: amount, SourceLineRef: "accum-dep"},
			},
		}

		if err := accounting.BalanceCheck(journal.Lines); err != nil {
			return fmt.Errorf("depreciation journal not balanced: %w", err)
		}

		entryID, err := postJournal(ctx, tx, tenantID, journal, idem, userID)
		if err != nil {
			return err
		}

		// Update the lease contract's accumulated depreciation.
		if _, err := tx.Exec(ctx, `
			UPDATE lease_contracts
			SET accum_dep_cents = COALESCE(accum_dep_cents, 0) + $3,
			    updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, contractID, amount); err != nil {
			return err
		}

		result = map[string]any{
			"journal_entry_id":   entryID,
			"contract_id":        contractID,
			"contract_number":    contractNumber,
			"depreciation_cents": amount,
			"period":             fmt.Sprintf("%04d-%02d", year, month),
			"accum_dep_cents":    alreadyDepreciated + amount,
			"rou_cost_cents":     rouCostCents,
		}
		return nil
	})
	return result, err
}

// ListDepreciationLog handles GET /lease-contracts/{id}/depreciation-log.
// Returns the history of RoU depreciation postings for the contract.
func (service *Service) ListDepreciationLog(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	contractID, err := strconv.ParseInt(chi.URLParam(request, "id"), 10, 64)
	if err != nil || contractID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}

	rows, err := service.pool.Query(request.Context(), `
		SELECT period_year, period_month, depreciation_cents, journal_entry_id, posted_at
		FROM lease_depreciation_log
		WHERE tenant_id = $1 AND lease_id = $2
		ORDER BY period_year DESC, period_month DESC
	`, tenantID, contractID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()

	type depLogEntry struct {
		PeriodYear         int    `json:"period_year"`
		PeriodMonth        int    `json:"period_month"`
		DepreciationCents  int64  `json:"depreciation_cents"`
		JournalEntryID     int64  `json:"journal_entry_id,omitempty"`
		PostedAt           string `json:"posted_at"`
	}
	var log []depLogEntry
	for rows.Next() {
		var e depLogEntry
		if err := rows.Scan(&e.PeriodYear, &e.PeriodMonth, &e.DepreciationCents, &e.JournalEntryID, &e.PostedAt); err != nil {
			writeError(writer, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
			return
		}
		log = append(log, e)
	}
	if log == nil {
		log = []depLogEntry{}
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"contract_id": contractID,
		"entries":     log,
	})
}
