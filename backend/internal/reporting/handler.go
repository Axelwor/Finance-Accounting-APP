package reporting

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// TrialBalance returns total debit and credit across all posted journals for a tenant.
func (service *Service) TrialBalance(writer http.ResponseWriter, request *http.Request) {
	tenantID := tenantFrom(request)
	var debitTotal, creditTotal int64
	err := service.pool.QueryRow(request.Context(), `
		SELECT COALESCE(SUM(jl.debit_cents), 0), COALESCE(SUM(jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
	`, tenantID).Scan(&debitTotal, &creditTotal)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"total_debit_cents":  debitTotal,
		"total_credit_cents": creditTotal,
		"balanced":           debitTotal == creditTotal,
	})
}

// ProfitLoss aggregates revenue and expense groups for the current open period.
func (service *Service) ProfitLoss(writer http.ResponseWriter, request *http.Request) {
	tenantID := tenantFrom(request)
	rows, err := service.pool.Query(request.Context(), `
		SELECT a.report_group, COALESCE(SUM(CASE
			WHEN a.report_group IN ('revenue') THEN jl.credit_cents - jl.debit_cents
			ELSE jl.debit_cents - jl.credit_cents END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')
		GROUP BY a.report_group
	`, tenantID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	defer rows.Close()

	result := map[string]int64{"revenue_cents": 0, "expense_cents": 0}
	for rows.Next() {
		var group string
		var amount int64
		if err := rows.Scan(&group, &amount); err != nil {
			writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
			return
		}
		result[group+"_cents"] = amount
	}
	result["profit_cents"] = result["revenue_cents"] - result["expense_cents"]
	writeJSON(writer, http.StatusOK, result)
}

// BalanceSheet aggregates asset, liability, and equity groups. Current-period
// profit (revenue − expense) is added to equity so the balance sheet balances
// before the period is closed (engine §21.2: laba berjalan real-time).
func (service *Service) BalanceSheet(writer http.ResponseWriter, request *http.Request) {
	tenantID := tenantFrom(request)
	rows, err := service.pool.Query(request.Context(), `
		SELECT a.report_group, COALESCE(SUM(CASE
			WHEN a.report_group = 'asset' THEN jl.debit_cents - jl.credit_cents
			ELSE jl.credit_cents - jl.debit_cents END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('asset', 'liability', 'equity')
		GROUP BY a.report_group
	`, tenantID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	defer rows.Close()

	result := map[string]any{
		"asset_cents":     int64(0),
		"liability_cents": int64(0),
		"equity_cents":    int64(0),
		"profit_cents":    int64(0),
	}
	for rows.Next() {
		var group string
		var amount int64
		if err := rows.Scan(&group, &amount); err != nil {
			writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
			return
		}
		result[group+"_cents"] = amount
	}

	// Current-period profit: revenue − expense from posted journals.
	// Revenue normally credits, expense normally debits, so (credit − debit)
	// summed across both groups yields revenue − expense.
	var profit int64
	err = service.pool.QueryRow(request.Context(), `
		SELECT COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')
	`, tenantID).Scan(&profit)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	result["profit_cents"] = profit

	assets := result["asset_cents"].(int64)
	liabilities := result["liability_cents"].(int64)
	equity := result["equity_cents"].(int64)
	result["balanced"] = assets == liabilities+equity+profit
	writeJSON(writer, http.StatusOK, result)
}

// CashFlow aggregates movements across CASH/BANK accounts.
func (service *Service) CashFlow(writer http.ResponseWriter, request *http.Request) {
	tenantID := tenantFrom(request)
	var inflow, outflow int64
	err := service.pool.QueryRow(request.Context(), `
		SELECT COALESCE(SUM(CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN jl.credit_cents > 0 THEN jl.credit_cents ELSE 0 END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.account_type IN ('CASH', 'BANK')
	`, tenantID).Scan(&inflow, &outflow)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"inflow_cents":        inflow,
		"outflow_cents":       outflow,
		"net_cash_flow_cents": inflow - outflow,
	})
}
