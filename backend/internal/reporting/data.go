package reporting

import (
	"context"
	"fmt"
	"net/http"
)

// fetchReportData computes the raw payload for a report, identical to what the
// corresponding GET handler returns. Both the JSON endpoint and the export
// endpoint call this, so their output stays byte-for-byte consistent.
func (service *Service) fetchReportData(r *http.Request, rtype reportType, dr dateRange) (any, error) {
	tenantID := tenantFrom(r)
	ctx := r.Context()
	switch rtype {
	case reportTrialBalance:
		return service.fetchTrialBalance(ctx, tenantID, dr)
	case reportProfitLoss:
		return service.fetchProfitLoss(ctx, tenantID, dr)
	case reportBalanceSheet:
		return service.fetchBalanceSheet(ctx, tenantID, dr)
	case reportCashFlow:
		return service.fetchCashFlow(ctx, tenantID, dr)
	default:
		return nil, fmt.Errorf("unknown report type: %s", rtype)
	}
}

// fetchTrialBalance returns per-account debit/credit totals across all posted
// journals. With to_date supplied the balance is cumulative from inception to
// to_date; from_date is ignored (a trial balance is a snapshot, not a movement).
func (service *Service) fetchTrialBalance(ctx context.Context, tenantID int64, dr dateRange) (TrialBalanceResult, error) {
	args := []any{tenantID}
	rows, err := service.pool.Query(ctx, `
		SELECT a.id, a.code, a.name,
		       COALESCE(SUM(jl.debit_cents), 0), COALESCE(SUM(jl.credit_cents), 0)
		FROM accounts a
		LEFT JOIN journal_lines jl
		       ON jl.tenant_id = $1 AND jl.account_id = a.id
		LEFT JOIN journal_entries je
		       ON je.tenant_id = $1 AND je.id = jl.entry_id AND je.status = 'POSTED'
		WHERE a.tenant_id = $1 AND a.is_group = false`+dateFilter(dr, 2, &args)+`
		GROUP BY a.id, a.code, a.name
		ORDER BY a.code
	`, args...)
	if err != nil {
		return TrialBalanceResult{}, err
	}
	defer rows.Close()

	result := TrialBalanceResult{Rows: []TrialBalanceRow{}}
	for rows.Next() {
		var r TrialBalanceRow
		if err := rows.Scan(&r.AccountID, &r.AccountCode, &r.AccountName, &r.DebitCents, &r.CreditCents); err != nil {
			return TrialBalanceResult{}, err
		}
		// Skip accounts with no movement — keeps the report tight.
		if r.DebitCents == 0 && r.CreditCents == 0 {
			continue
		}
		result.Rows = append(result.Rows, r)
		result.TotalDebitCents += r.DebitCents
		result.TotalCreditCents += r.CreditCents
	}
	result.Balanced = result.TotalDebitCents == result.TotalCreditCents
	return result, nil
}

// fetchProfitLoss aggregates revenue and expense groups for the requested range.
func (service *Service) fetchProfitLoss(ctx context.Context, tenantID int64, dr dateRange) (ProfitLossResult, error) {
	args := []any{tenantID}
	rows, err := service.pool.Query(ctx, `
		SELECT a.report_group, COALESCE(SUM(CASE
			WHEN a.report_group IN ('revenue') THEN jl.credit_cents - jl.debit_cents
			ELSE jl.debit_cents - jl.credit_cents END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')`+dateFilter(dr, 2, &args)+`
		GROUP BY a.report_group
	`, args...)
	if err != nil {
		return ProfitLossResult{}, err
	}
	defer rows.Close()

	result := ProfitLossResult{}
	for rows.Next() {
		var group string
		var amount int64
		if err := rows.Scan(&group, &amount); err != nil {
			return ProfitLossResult{}, err
		}
		switch group {
		case "revenue":
			result.RevenueCents = amount
		case "expense":
			result.ExpenseCents = amount
		}
	}
	result.ProfitCents = result.RevenueCents - result.ExpenseCents
	return result, nil
}

// fetchBalanceSheet aggregates asset, liability, and equity groups. Current-
// period profit (revenue − expense) is added to equity so the balance sheet
// balances before the period is closed (engine §21.2: current earnings
// real-time). With to_date supplied the snapshot is taken as of that date.
func (service *Service) fetchBalanceSheet(ctx context.Context, tenantID int64, dr dateRange) (BalanceSheetResult, error) {
	args := []any{tenantID}
	rows, err := service.pool.Query(ctx, `
		SELECT a.report_group, COALESCE(SUM(CASE
			WHEN a.report_group = 'asset' THEN jl.debit_cents - jl.credit_cents
			ELSE jl.credit_cents - jl.debit_cents END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('asset', 'liability', 'equity')`+dateFilter(dr, 2, &args)+`
		GROUP BY a.report_group
	`, args...)
	if err != nil {
		return BalanceSheetResult{}, err
	}
	defer rows.Close()

	result := BalanceSheetResult{}
	for rows.Next() {
		var group string
		var amount int64
		if err := rows.Scan(&group, &amount); err != nil {
			return BalanceSheetResult{}, err
		}
		switch group {
		case "asset":
			result.AssetCents = amount
		case "liability":
			result.LiabilityCents = amount
		case "equity":
			result.EquityCents = amount
		}
	}

	// Current-period profit: revenue − expense from posted journals in range.
	// Revenue normally credits, expense normally debits, so (credit − debit)
	// summed across both groups yields revenue − expense.
	profitArgs := []any{tenantID}
	err = service.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')`+dateFilter(dr, 2, &profitArgs)+`
	`, profitArgs...).Scan(&result.ProfitCents)
	if err != nil {
		return BalanceSheetResult{}, err
	}
	result.Balanced = result.AssetCents == result.LiabilityCents+result.EquityCents+result.ProfitCents
	return result, nil
}

// fetchCashFlow aggregates movements across CASH/BANK accounts within the range.
func (service *Service) fetchCashFlow(ctx context.Context, tenantID int64, dr dateRange) (CashFlowResult, error) {
	args := []any{tenantID}
	var r CashFlowResult
	err := service.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN jl.credit_cents > 0 THEN jl.credit_cents ELSE 0 END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.account_type IN ('CASH', 'BANK')`+dateFilter(dr, 2, &args)+`
	`, args...).Scan(&r.InflowCents, &r.OutflowCents)
	if err != nil {
		return CashFlowResult{}, err
	}
	r.NetCashFlowCents = r.InflowCents - r.OutflowCents
	return r, nil
}
