package reporting

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// fetchReportData computes the raw payload for a report, identical to what the
// corresponding GET handler returns. Both the JSON endpoint and the export
// endpoint call this, so their output stays byte-for-byte consistent.
func (service *Service) fetchReportData(r *http.Request, rtype reportType, f reportFilter) (any, error) {
	tenantID := tenantFrom(r)
	ctx := r.Context()
	switch rtype {
	case reportTrialBalance:
		return service.fetchTrialBalance(ctx, tenantID, f)
	case reportProfitLoss:
		return service.fetchProfitLoss(ctx, tenantID, f)
	case reportBalanceSheet:
		return service.fetchBalanceSheet(ctx, tenantID, f)
	case reportCashFlow:
		return service.fetchCashFlow(ctx, tenantID, f)
	default:
		return nil, fmt.Errorf("unknown report type: %s", rtype)
	}
}

// dimensionJoin returns a JOIN fragment that restricts journal_lines to lines
// tagged with any of f.dimensionIDs (cabang / proyek). The id slice is
// appended to args as a single array parameter and the new base arg offset
// (for subsequent date args) is returned. When no dimension is selected, the
// fragment is empty and the offset is unchanged.
//
// The join targets a DISTINCT subquery rather than journal_line_dimensions
// directly: a journal line tagged with two of the selected dimensions would
// otherwise match twice and double-count in the SUM aggregates.
func dimensionJoin(f reportFilter, baseArg int, args *[]any) (string, int) {
	if len(f.dimensionIDs) == 0 {
		return "", baseArg
	}
	*args = append(*args, f.dimensionIDs)
	return fmt.Sprintf(
		" JOIN (SELECT DISTINCT tenant_id, journal_line_id FROM journal_line_dimensions WHERE dimension_id = ANY($%d)) jld"+
			" ON jld.tenant_id = jl.tenant_id AND jld.journal_line_id = jl.id",
		baseArg), baseArg + 1
}

// fetchTrialBalance returns per-account debit/credit totals across all posted
// journals. With to_date supplied the balance is cumulative from inception to
// to_date; from_date is ignored (a trial balance is a snapshot, not a movement).
// When dimension ids are supplied only journal lines tagged with one of those
// dimensions are aggregated.
func (service *Service) fetchTrialBalance(ctx context.Context, tenantID int64, f reportFilter) (TrialBalanceResult, error) {
	args := []any{tenantID}
	join, dateBase := dimensionJoin(f, 2, &args)
	rows, err := service.pool.Query(ctx, `
		SELECT a.id, a.code, a.name,
		       COALESCE(SUM(jl.debit_cents), 0), COALESCE(SUM(jl.credit_cents), 0)
		FROM accounts a
		LEFT JOIN journal_lines jl
		       ON jl.tenant_id = $1 AND jl.account_id = a.id
		LEFT JOIN journal_entries je
		       ON je.tenant_id = $1 AND je.id = jl.entry_id AND je.status = 'POSTED'
		`+join+`
		WHERE a.tenant_id = $1 AND a.is_group = false`+dateFilter(f.dateRange, dateBase, &args)+`
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
// When a framework is supplied (EMKM / ETAP / SAK_UMUM) the same totals are also
// returned as framework-grouped Sections (same data, different presentation).
func (service *Service) fetchProfitLoss(ctx context.Context, tenantID int64, f reportFilter) (ProfitLossResult, error) {
	args := []any{tenantID}
	join, dateBase := dimensionJoin(f, 2, &args)
	rows, err := service.pool.Query(ctx, `
		SELECT a.report_group, COALESCE(SUM(CASE
			WHEN a.report_group IN ('revenue') THEN jl.credit_cents - jl.debit_cents
			ELSE jl.debit_cents - jl.credit_cents END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		`+join+`
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')`+dateFilter(f.dateRange, dateBase, &args)+`
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

	// Framework presentation: re-group the same posted movements by account_type
	// and relabel per the selected framework. Same data, different breakdown.
	if f.framework != "" {
		sections, err := service.fetchFrameworkSections(ctx, tenantID, f)
		if err != nil {
			return ProfitLossResult{}, err
		}
		result.Framework = f.framework
		result.Sections = sections
	}
	return result, nil
}

// fetchFrameworkSections returns the per-account_type net amounts rolled up
// into framework-specific sections. Revenue accounts are credit-led (net =
// credit − debit); expense accounts are debit-led (net = debit − credit).
func (service *Service) fetchFrameworkSections(ctx context.Context, tenantID int64, f reportFilter) ([]ProfitLossSection, error) {
	args := []any{tenantID}
	join, dateBase := dimensionJoin(f, 2, &args)
	rows, err := service.pool.Query(ctx, `
		SELECT a.account_type, a.report_group,
		       COALESCE(SUM(CASE
			WHEN a.report_group = 'revenue' THEN jl.credit_cents - jl.debit_cents
			ELSE jl.debit_cents - jl.credit_cents END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		`+join+`
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')`+dateFilter(f.dateRange, dateBase, &args)+`
		GROUP BY a.account_type, a.report_group
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Net amount and report_group per account_type.
	byType := make(map[string]int64)
	groupByType := make(map[string]string)
	for rows.Next() {
		var accountType, reportGroup string
		var amount int64
		if err := rows.Scan(&accountType, &reportGroup, &amount); err != nil {
			return nil, err
		}
		byType[accountType] += amount
		groupByType[accountType] = reportGroup
	}
	return buildFrameworkSections(f.framework, byType, groupByType), nil
}

// sectionSpec maps a framework section to the account_types it rolls up.
type sectionSpec struct {
	code         string
	label        string
	accountTypes []string
}

// frameworkSections defines the per-framework section layout. EMKM collapses
// to two sections (Pendapatan / Beban); ETAP and SAK_UMUM break out operating
// revenue, COGS, operating expenses, and other income/expense. SAK_UMUM uses
// the same groupings as ETAP — the frontend renders subtotals (laba kotor,
// laba operasional, laba bersih) on top of these sections.
var frameworkSections = map[string][]sectionSpec{
	"EMKM": {
		{code: "revenue", label: "Pendapatan"},
		{code: "expense", label: "Beban"},
	},
	"ETAP": {
		{code: "operating_revenue", label: "Pendapatan Usaha", accountTypes: []string{"REVENUE", "CONTRA_REVENUE"}},
		{code: "cogs", label: "Beban Pokok Penjualan", accountTypes: []string{"COGS"}},
		{code: "operating_expense", label: "Beban Operasional", accountTypes: []string{"EXPENSE", "DEPRECIATION"}},
		{code: "other_income", label: "Pendapatan Lain-lain", accountTypes: []string{"OTHER_INCOME"}},
		{code: "other_expense", label: "Beban Lain-lain", accountTypes: []string{"OTHER_EXPENSE", "IMPAIRMENT", "BAD_DEBT", "TAX_EXPENSE", "DEFERRED_TAX"}},
	},
	"SAK_UMUM": {
		{code: "operating_revenue", label: "Pendapatan", accountTypes: []string{"REVENUE", "CONTRA_REVENUE"}},
		{code: "cogs", label: "Beban Pokok Penjualan", accountTypes: []string{"COGS"}},
		{code: "operating_expense", label: "Beban Usaha", accountTypes: []string{"EXPENSE", "DEPRECIATION"}},
		{code: "other_income", label: "Pendapatan Lain-lain", accountTypes: []string{"OTHER_INCOME"}},
		{code: "other_expense", label: "Beban Lain-lain", accountTypes: []string{"OTHER_EXPENSE", "IMPAIRMENT", "BAD_DEBT", "TAX_EXPENSE", "DEFERRED_TAX"}},
	},
}

// buildFrameworkSections rolls the per-account_type net amounts into the
// framework's section layout. For EMKM (no explicit account_types listed) each
// section absorbs every account_type on its report_group side. For ETAP /
// SAK_UMUM the explicit account_type lists are summed, and any custom (unmapped)
// account types are swept into the matching revenue/expense section by
// report_group so nothing is silently dropped.
func buildFrameworkSections(framework string, byType map[string]int64, groupByType map[string]string) []ProfitLossSection {
	specs, ok := frameworkSections[framework]
	if !ok {
		return nil
	}
	sections := make([]ProfitLossSection, 0, len(specs))
	used := make(map[string]bool)
	for _, spec := range specs {
		var amount int64
		if len(spec.accountTypes) == 0 {
			// EMKM: roll up by report_group side.
			for t, amt := range byType {
				if groupByType[t] == spec.code {
					amount += amt
					used[t] = true
				}
			}
		} else {
			for _, t := range spec.accountTypes {
				amount += byType[t]
				used[t] = true
			}
		}
		sections = append(sections, ProfitLossSection{
			Code:        spec.code,
			Label:       spec.label,
			AmountCents: amount,
		})
	}
	// Sweep custom (unmapped) account types into the matching revenue/expense
	// section by report_group.
	for t, amt := range byType {
		if used[t] || amt == 0 {
			continue
		}
		group := groupByType[t]
		target := "other_expense"
		if group == "revenue" {
			target = "operating_revenue"
		}
		for i := range sections {
			if sections[i].Code == target {
				sections[i].AmountCents += amt
				break
			}
		}
	}
	return sections
}

// fetchBalanceSheet aggregates asset, liability, and equity groups. Current-
// period profit (revenue − expense) is added to equity so the balance sheet
// balances before the period is closed (engine §21.2: current earnings
// real-time). With to_date supplied the snapshot is taken as of that date.
func (service *Service) fetchBalanceSheet(ctx context.Context, tenantID int64, f reportFilter) (BalanceSheetResult, error) {
	args := []any{tenantID}
	join, dateBase := dimensionJoin(f, 2, &args)
	rows, err := service.pool.Query(ctx, `
		SELECT a.report_group, COALESCE(SUM(CASE
			WHEN a.report_group = 'asset' THEN jl.debit_cents - jl.credit_cents
			ELSE jl.credit_cents - jl.debit_cents END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		`+join+`
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('asset', 'liability', 'equity')`+dateFilter(f.dateRange, dateBase, &args)+`
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
	profitJoin, profitDateBase := dimensionJoin(f, 2, &profitArgs)
	err = service.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		`+profitJoin+`
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')`+dateFilter(f.dateRange, profitDateBase, &profitArgs)+`
	`, profitArgs...).Scan(&result.ProfitCents)
	if err != nil {
		return BalanceSheetResult{}, err
	}
	result.Balanced = result.AssetCents == result.LiabilityCents+result.EquityCents+result.ProfitCents
	return result, nil
}

// fetchCashFlow classifies cash movements into operating, investing, and
// financing activities per ACCOUNTING_ENGINE §17.
//
// Classification rules (based on the offsetting account — the non-cash leg):
//   - Revenue/Expense accounts → Operating
//   - AR/AP accounts            → Operating
//   - Inventory/WIP accounts    → Operating
//   - Asset (fixed/intangible)  → Investing
//   - Liability (loans/leases)  → Financing
//   - Equity accounts           → Financing
//   - Cash-to-Cash (transfers)  → Not classified (internal movement)
func (service *Service) fetchCashFlow(ctx context.Context, tenantID int64, f reportFilter) (CashFlowResult, error) {
	args := []any{tenantID}
	join, dateBase := dimensionJoin(f, 2, &args)
	query := `
		SELECT
			-- Operating: revenue, expense, AR, AP, inventory, WIP
			COALESCE(SUM(CASE
				WHEN ca.account_type IN ('REVENUE','EXPENSE','COGS','DEPRECIATION_EXPENSE',
				                          'OTHER_EXPENSE','OTHER_REVENUE')
			      OR ca.code LIKE '1201%'  -- AR
			      OR ca.code LIKE '2101%'  -- AP
			      OR ca.code LIKE '1301%'  -- Inventory
			      OR ca.code LIKE '1302%'  -- WIP
			      OR ca.code LIKE '1303%'  -- WIP Production
			      OR ca.code LIKE '2105%'  -- Uninvoiced Payables
			      OR ca.code LIKE '2202%'  -- Output VAT
			      OR ca.code LIKE '1103%'  -- VAT Input
				THEN CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE 0 END
			END), 0),
			COALESCE(SUM(CASE
				WHEN ca.account_type IN ('REVENUE','EXPENSE','COGS','DEPRECIATION_EXPENSE',
				                          'OTHER_EXPENSE','OTHER_REVENUE')
			      OR ca.code LIKE '1201%'
			      OR ca.code LIKE '2101%'
			      OR ca.code LIKE '1301%'
			      OR ca.code LIKE '1302%'
			      OR ca.code LIKE '1303%'
			      OR ca.code LIKE '2105%'
			      OR ca.code LIKE '2202%'
			      OR ca.code LIKE '1103%'
				THEN CASE WHEN jl.credit_cents > 0 THEN jl.credit_cents ELSE 0 END
			END), 0),
			-- Investing: fixed assets, intangible assets, RoU
			COALESCE(SUM(CASE
				WHEN ca.account_type IN ('FIXED_ASSET','INTANGIBLE_ASSET','ACCUMULATED_DEPRECIATION')
			      OR ca.code LIKE '1701%'  -- RoU Asset
			      OR ca.code LIKE '1702%'  -- Accumulated RoU
			      OR ca.code LIKE '1501%'  -- Fixed Assets
			      OR ca.code LIKE '1502%'  -- Accumulated Depreciation
			      OR ca.code LIKE '1601%'  -- Intangible
				THEN CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE 0 END
			END), 0),
			COALESCE(SUM(CASE
				WHEN ca.account_type IN ('FIXED_ASSET','INTANGIBLE_ASSET','ACCUMULATED_DEPRECIATION')
			      OR ca.code LIKE '1701%'
			      OR ca.code LIKE '1702%'
			      OR ca.code LIKE '1501%'
			      OR ca.code LIKE '1502%'
			      OR ca.code LIKE '1601%'
				THEN CASE WHEN jl.credit_cents > 0 THEN jl.credit_cents ELSE 0 END
			END), 0),
			-- Financing: loans, leases, equity, dividends
			COALESCE(SUM(CASE
				WHEN ca.account_type IN ('EQUITY','LOAN_PAYABLE')
			      OR ca.code LIKE '2301%'  -- Lease Liability
			      OR ca.code LIKE '2401%'  -- Long-term Loan
			      OR ca.code LIKE '3101%'  -- Capital
			      OR ca.code LIKE '3201%'  -- Retained Earnings
			      OR ca.code LIKE '3301%'  -- Current Earnings
			      OR ca.code LIKE '3302%'  -- Dividends Payable
				THEN CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE 0 END
			END), 0),
			COALESCE(SUM(CASE
				WHEN ca.account_type IN ('EQUITY','LOAN_PAYABLE')
			      OR ca.code LIKE '2301%'
			      OR ca.code LIKE '2401%'
			      OR ca.code LIKE '3101%'
			      OR ca.code LIKE '3201%'
			      OR ca.code LIKE '3301%'
			      OR ca.code LIKE '3302%'
				THEN CASE WHEN jl.credit_cents > 0 THEN jl.credit_cents ELSE 0 END
			END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		-- Join the offsetting account (the non-cash leg) to classify the activity.
		JOIN journal_lines ol ON ol.tenant_id = jl.tenant_id AND ol.entry_id = jl.entry_id
		                       AND ol.account_id != jl.account_id
		JOIN accounts ca ON ca.tenant_id = jl.tenant_id AND ca.id = ol.account_id
		` + join + `
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.account_type IN ('CASH', 'BANK')` + dateFilter(f.dateRange, dateBase, &args) + `
	`
	var r CashFlowResult
	err := service.pool.QueryRow(ctx, query, args...).Scan(
		&r.OperatingInflowCents, &r.OperatingOutflowCents,
		&r.InvestingInflowCents, &r.InvestingOutflowCents,
		&r.FinancingInflowCents, &r.FinancingOutflowCents,
	)
	if err != nil {
		return CashFlowResult{}, err
	}
	r.NetOperatingCents = r.OperatingInflowCents - r.OperatingOutflowCents
	r.NetInvestingCents = r.InvestingInflowCents - r.InvestingOutflowCents
	r.NetFinancingCents = r.FinancingInflowCents - r.FinancingOutflowCents
	r.InflowCents = r.OperatingInflowCents + r.InvestingInflowCents + r.FinancingInflowCents
	r.OutflowCents = r.OperatingOutflowCents + r.InvestingOutflowCents + r.FinancingOutflowCents
	r.NetCashFlowCents = r.InflowCents - r.OutflowCents
	return r, nil
}

// strconv is imported for the dateFilter placeholder offsets used above.
var _ = strconv.Itoa
