package tax

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// PPN (US-080): PPN summary, reconciliation report, and filing.
//
// PPN keluaran (output VAT) accrues as a credit to 2202 (VAT Payable) when a
// sales invoice is posted. PPN masukan (input VAT) accrues as a debit to 1203
// (Input VAT) when a supplier invoice is posted. The net PPN payable for a
// period is keluaran - masukan (positive = payable to the tax office).
// ---------------------------------------------------------------------------

// PPNSummary is the response for GET /ppn/summary.
type PPNSummary struct {
	FromDate         string `json:"from_date"`
	ToDate           string `json:"to_date"`
	PPNKeluaranCents int64  `json:"ppn_keluaran_cents"`
	PPNMasukanCents  int64  `json:"ppn_masukan_cents"`
	NetPPNCents      int64  `json:"net_ppn_cents"`
}

// PPNReconciliationLine is one transaction in the detailed reconciliation.
type PPNReconciliationLine struct {
	EntryID     int64  `json:"entry_id"`
	EntryNumber string `json:"entry_number"`
	EntryDate   string `json:"entry_date"`
	Description string `json:"description"`
	IntentType  string `json:"intent_type"`
	SourceRef   string `json:"source_ref"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Direction   string `json:"direction"` // KELUARAN or MASUKAN
	DebitCents  int64  `json:"debit_cents"`
	CreditCents int64  `json:"credit_cents"`
}

// PPNReconciliationResult is the response for GET /ppn/reconciliation.
type PPNReconciliationResult struct {
	PeriodYear       int64                   `json:"period_year"`
	PeriodMonth      int64                   `json:"period_month"`
	PPNKeluaranCents int64                   `json:"ppn_keluaran_cents"`
	PPNMasukanCents  int64                   `json:"ppn_masukan_cents"`
	NetPPNCents      int64                   `json:"net_ppn_cents"`
	Lines            []PPNReconciliationLine `json:"lines"`
}

// PPNReconciliationRecord is one filed reconciliation (response for POST /ppn/reconcile).
type PPNReconciliationRecord struct {
	ID               int64  `json:"id"`
	PeriodYear       int64  `json:"period_year"`
	PeriodMonth      int64  `json:"period_month"`
	PPNKeluaranCents int64  `json:"ppn_keluaran_cents"`
	PPNMasukanCents  int64  `json:"ppn_masukan_cents"`
	NetPPNCents      int64  `json:"net_ppn_cents"`
	Status           string `json:"status"`
	Notes            string `json:"notes"`
	CreatedAt        string `json:"created_at"`
}

// PPNSummary computes output VAT (2202 credits) minus input VAT (1203 debits)
// across the requested date range. Both from_date and to_date are optional
// YYYY-MM-DD bounds; when omitted the range is open-ended.
//
// GET /ppn/summary?from_date=&to_date=
func (service *Service) PPNSummary(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	fromDate := normalizeDate(request.URL.Query().Get("from_date"))
	toDate := normalizeDate(request.URL.Query().Get("to_date"))

	var keluaran, masukan int64
	// Sum credits to the output VAT account (keluaran) and debits to the
	// input VAT account (masukan). SET-001: the accounts resolve through the
	// tax master with the legacy 2202/1203 codes as fallback.
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		keluaranID, masukanID, err := vatAccountIDs(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		args := []any{tenant, keluaranID, masukanID}
		where := "WHERE jl.tenant_id = $1 AND (a.id = $2 OR a.id = $3)"
		idx := 4
		if fromDate != "" {
			where += fmt.Sprintf(" AND je.entry_date >= $%d", idx)
			args = append(args, fromDate)
			idx++
		}
		if toDate != "" {
			where += fmt.Sprintf(" AND je.entry_date <= $%d", idx)
			args = append(args, toDate)
			idx++
		}
		return tx.QueryRow(request.Context(), `
			SELECT
			  COALESCE(SUM(CASE WHEN a.id = $2 THEN jl.credit_cents ELSE 0 END), 0) AS keluaran,
			  COALESCE(SUM(CASE WHEN a.id = $3 THEN jl.debit_cents  ELSE 0 END), 0) AS masukan
			FROM journal_lines jl
			JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
			JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		`+where+`
		  AND je.status = 'POSTED'
		`, args...).Scan(&keluaran, &masukan)
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PPN_SUMMARY_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, PPNSummary{
		FromDate:         fromDate,
		ToDate:           toDate,
		PPNKeluaranCents: keluaran,
		PPNMasukanCents:  masukan,
		NetPPNCents:      keluaran - masukan,
	})
}

// PPNReconciliation lists every posted VAT movement in the requested calendar
// month, grouped by keluaran (2202 credit) vs masukan (1203 debit), with the
// period totals. period_year and period_month are required (1-12).
//
// GET /ppn/reconciliation?period_year=&period_month=
func (service *Service) PPNReconciliation(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	periodYear, periodMonth, err := parsePeriod(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	fromDate, toDate := monthBounds(periodYear, periodMonth)

	lines := make([]PPNReconciliationLine, 0)
	var keluaran, masukan int64
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		keluaranID, masukanID, err := vatAccountIDs(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		rows, err := tx.Query(request.Context(), `
			SELECT je.id, je.number, je.entry_date, COALESCE(je.description, ''),
			       COALESCE(je.intent_type, ''), COALESCE(je.source_ref, ''),
			       a.code, a.name,
			       CASE WHEN a.id = $2 THEN 'KELUARAN' ELSE 'MASUKAN' END,
			       jl.debit_cents, jl.credit_cents
			FROM journal_lines jl
			JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
			JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
			WHERE jl.tenant_id = $1 AND (a.id = $2 OR a.id = $3)
			  AND je.status = 'POSTED'
			  AND je.entry_date >= $4 AND je.entry_date <= $5
			ORDER BY je.entry_date, je.number
		`, tenant, keluaranID, masukanID, fromDate, toDate)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var l PPNReconciliationLine
			var entryDate time.Time
			if err := rows.Scan(&l.EntryID, &l.EntryNumber, &entryDate, &l.Description,
				&l.IntentType, &l.SourceRef, &l.AccountCode, &l.AccountName, &l.Direction,
				&l.DebitCents, &l.CreditCents); err != nil {
				return err
			}
			l.EntryDate = entryDate.Format("2006-01-02")
			if l.Direction == "KELUARAN" {
				keluaran += l.CreditCents
			} else {
				masukan += l.DebitCents
			}
			lines = append(lines, l)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PPN_RECON_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, PPNReconciliationResult{
		PeriodYear:       periodYear,
		PeriodMonth:      periodMonth,
		PPNKeluaranCents: keluaran,
		PPNMasukanCents:  masukan,
		NetPPNCents:      keluaran - masukan,
		Lines:            lines,
	})
}

// CreatePPNReconciliationRequest is the body of POST /ppn/reconcile.
type CreatePPNReconciliationRequest struct {
	PeriodYear  int64  `json:"period_year"`
	PeriodMonth int64  `json:"period_month"`
	Notes       string `json:"notes"`
}

// CreatePPNReconciliation files (or re-files) the PPN reconciliation for a
// calendar month. It computes the keluaran/masukan totals from the posted
// journal entries and upserts a ppn_reconciliations row marked FILED.
//
// POST /ppn/reconcile  { period_year, period_month, notes }
func (service *Service) CreatePPNReconciliation(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreatePPNReconciliationRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.PeriodYear <= 0 || req.PeriodMonth < 1 || req.PeriodMonth > 12 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "period_year and period_month (1-12) are required")
		return
	}

	fromDate, toDate := monthBounds(req.PeriodYear, req.PeriodMonth)
	var rec PPNReconciliationRecord
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var keluaran, masukan int64
		keluaranID, masukanID, err := vatAccountIDs(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(request.Context(), `
			SELECT
			  COALESCE(SUM(CASE WHEN a.id = $2 THEN jl.credit_cents ELSE 0 END), 0),
			  COALESCE(SUM(CASE WHEN a.id = $3 THEN jl.debit_cents  ELSE 0 END), 0)
			FROM journal_lines jl
			JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
			JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
			WHERE jl.tenant_id = $1 AND (a.id = $2 OR a.id = $3)
			  AND je.status = 'POSTED'
			  AND je.entry_date >= $4 AND je.entry_date <= $5
		`, tenant, keluaranID, masukanID, fromDate, toDate).Scan(&keluaran, &masukan); err != nil {
			return err
		}
		net := keluaran - masukan

		rec.PeriodYear = req.PeriodYear
		rec.PeriodMonth = req.PeriodMonth
		rec.PPNKeluaranCents = keluaran
		rec.PPNMasukanCents = masukan
		rec.NetPPNCents = net
		rec.Status = "FILED"
		rec.Notes = req.Notes

		var createdAt time.Time
		if err := tx.QueryRow(request.Context(), `
			INSERT INTO ppn_reconciliations
			  (tenant_id, period_year, period_month, ppn_keluaran_cents, ppn_masukan_cents, net_ppn_cents, status, notes)
			VALUES ($1, $2, $3, $4, $5, $6, 'FILED', $7)
			ON CONFLICT (tenant_id, period_year, period_month) DO UPDATE
			SET ppn_keluaran_cents = EXCLUDED.ppn_keluaran_cents,
			    ppn_masukan_cents = EXCLUDED.ppn_masukan_cents,
			    net_ppn_cents = EXCLUDED.net_ppn_cents,
			    status = 'FILED',
			    notes = EXCLUDED.notes,
			    created_at = now()
			RETURNING id, created_at
		`, tenant, req.PeriodYear, req.PeriodMonth, keluaran, masukan, net, req.Notes).Scan(&rec.ID, &createdAt); err != nil {
			return err
		}
		rec.CreatedAt = createdAt.Format(time.RFC3339)

		if err := audit.Log(request.Context(), tx, tenant, userIDFromCtx(request.Context()), "ppn_reconciliation", rec.ID, audit.ActionCreate, nil, map[string]any{
			"period_year":        req.PeriodYear,
			"period_month":       req.PeriodMonth,
			"ppn_keluaran_cents": keluaran,
			"ppn_masukan_cents":  masukan,
			"net_ppn_cents":      net,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PPN_RECON_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, rec)
}

// ---------------------------------------------------------------------------
// PPN helpers
// ---------------------------------------------------------------------------

// parsePeriod reads period_year and period_month (required) from the query.
// vatAccountIDs resolves the output (keluaran) and input (masukan) VAT
// account ids through the tax master (SET-001) with the legacy hardcoded
// codes as fallback, so unmapped tenants post exactly as before.
func vatAccountIDs(ctx context.Context, tx pgx.Tx, tenant int64) (int64, int64, error) {
	keluaranID, err := ResolveVATAccounts(ctx, tx, tenant, nil, true)
	if err != nil {
		return 0, 0, err
	}
	masukanID, err := ResolveVATAccounts(ctx, tx, tenant, nil, false)
	if err != nil {
		return 0, 0, err
	}
	return keluaranID, masukanID, nil
}

func parsePeriod(request *http.Request) (int64, int64, error) {
	yearStr := request.URL.Query().Get("period_year")
	monthStr := request.URL.Query().Get("period_month")
	if yearStr == "" || monthStr == "" {
		return 0, 0, fmt.Errorf("period_year and period_month are required")
	}
	year, err := parseInt(yearStr)
	if err != nil || year <= 0 {
		return 0, 0, fmt.Errorf("period_year must be a positive integer")
	}
	month, err := parseInt(monthStr)
	if err != nil || month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("period_month must be an integer 1-12")
	}
	return year, month, nil
}

func parseInt(raw string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(raw, "%d", &v)
	return v, err
}

// monthBounds returns the inclusive YYYY-MM-DD bounds for a calendar month.
func monthBounds(year, month int64) (string, string) {
	start := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

// ensure ctx/tx imports stay referenced as this file grows.
var _ = (*pgx.Tx)(nil)

// ValidatePPNRate enforces that a caller-supplied PPN rate (percentage) on a
// document line matches the tenant's configured active PPN rate in tax_rates
// for the document date. This closes the "client-supplied rate not enforced"
// gap: previously any rate 0..100 was accepted.
//
// Rules:
//   - When the tenant has no active PPN row covering the date, the supplied
//     rate is accepted as-is (backward compatible; the tenant has not
//     configured PPN).
//   - When a configured rate exists:
//     rate == configured  → accepted
//     rate == 0           → accepted (explicitly untaxed line)
//     otherwise           → rejected with a descriptive error
//
// The comparison uses a small epsilon so a NUMERIC(9,6) 11.0 and a JSON 11
// compare equal.
func ValidatePPNRate(ctx context.Context, tx pgx.Tx, tenantID int64, entryDate string, rate float64) error {
	var configured float64
	err := tx.QueryRow(ctx, `
		SELECT rate::float8
		FROM tax_rates
		WHERE tenant_id = $1 AND tax_type = 'PPN' AND is_active = true
		  AND effective_from <= $2::date
		  AND (effective_to IS NULL OR effective_to >= $2::date)
		ORDER BY effective_from DESC
		LIMIT 1
	`, tenantID, entryDate).Scan(&configured)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No PPN configuration for this tenant/date — accept the supplied rate.
			return nil
		}
		return err
	}
	return checkPPNRateMatches(rate, configured)
}

// checkPPNRateMatches compares a caller-supplied rate against the configured
// active PPN rate. Pure function — unit-testable without a database.
func checkPPNRateMatches(rate, configured float64) error {
	if rate == 0 {
		// Explicitly untaxed line.
		return nil
	}
	diff := rate - configured
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.0001 {
		return fmt.Errorf("tax_rate %.4f does not match the active PPN rate %.4f configured for this tenant", rate, configured)
	}
	return nil
}
