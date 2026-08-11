package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// seedDefaultCOA provisions the core chart of accounts and UI categories for a
// freshly created tenant. It runs inside the registration transaction so a
// failure rolls the whole registration back.
func seedDefaultCOA(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	return SeedDefaultCOA(ctx, tx, tenantID)
}

// SeedDefaultCOA is the exported variant so other packages (e.g. tenant, which
// creates additional books for an existing user) can provision a new tenant
// with the same default chart of accounts, categories, and open period.
func SeedDefaultCOA(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	accounts := []struct {
		code        string
		name        string
		reportGroup string
		accountType string
	}{
		{"1101", "Cash", "asset", "CASH"},
		{"1102", "Bank", "asset", "BANK"},
		{"1201", "Accounts Receivable", "asset", "AR"},
		{"1202", "Allowance for Doubtful Accounts", "asset", "CONTRA_ASSET"},
		{"1203", "Input VAT", "asset", "INPUT_VAT"},
		{"1204", "Other Receivables", "asset", "OTHER_RECEIVABLE"},
		{"1205", "Prepayment to Suppliers", "asset", "PREPAYMENT"},
		{"1206", "Deferred Tax Asset", "asset", "DEFERRED_TAX"},
		{"1301", "Inventory", "asset", "INVENTORY"},
		{"1303", "Work in Progress", "asset", "INVENTORY"},
		{"1304", "Finished Goods", "asset", "INVENTORY"},
		{"1401", "Fixed Assets", "asset", "FIXED_ASSET"},
		{"1402", "Accumulated Depreciation", "asset", "CONTRA_ASSET"},
		{"1701", "Right-of-Use Asset", "asset", "ROU_ASSET"},
		{"1702", "Accumulated RoU Depreciation", "asset", "CONTRA_ASSET"},
		{"2101", "Accounts Payable", "liability", "AP"},
		{"2105", "Uninvoiced Payables", "liability", "ACCRUED_LIABILITY"},
		{"2201", "Customer Deposit", "liability", "CUSTOMER_DEPOSIT"},
		{"2202", "VAT Payable", "liability", "TAX_PAYABLE"},
		{"2203", "Income Tax Payable", "liability", "TAX_PAYABLE"},
		{"2301", "Lease Liability", "liability", "LEASE_LIABILITY"},
		{"2402", "Customer Overpayment", "liability", "CUSTOMER_DEPOSIT"},
		{"3101", "Capital", "equity", "EQUITY"},
		{"3201", "Retained Earnings", "equity", "EQUITY"},
		{"3301", "Current Earnings", "equity", "EQUITY"},
		{"3401", "Revaluation Surplus (OCI)", "equity", "OCI"},
		{"4101", "Sales Revenue", "revenue", "REVENUE"},
		{"4201", "Sales Returns", "revenue", "CONTRA_REVENUE"},
		{"4902", "Applied Overhead", "expense", "EXPENSE"},
		{"4903", "Gain on Asset Disposal", "revenue", "OTHER_INCOME"},
		{"4906", "Bad Debt Recovery", "revenue", "OTHER_INCOME"},
		{"4907", "Inventory Adjustment Gain", "revenue", "OTHER_INCOME"},
		{"4908", "Production Variance Gain", "revenue", "OTHER_INCOME"},
		{"5101", "COGS", "expense", "COGS"},
		{"5201", "Salary Expense", "expense", "EXPENSE"},
		{"5202", "Rent Expense", "expense", "EXPENSE"},
		{"5203", "Transportation Expense", "expense", "EXPENSE"},
		{"5204", "Utilities Expense", "expense", "EXPENSE"},
		{"5205", "Other Expenses", "expense", "EXPENSE"},
		{"5206", "Depreciation Expense", "expense", "DEPRECIATION"},
		{"5207", "Impairment Loss", "expense", "IMPAIRMENT"},
		{"5903", "Loss on Asset Disposal", "expense", "OTHER_EXPENSE"},
		{"5209", "Bad Debt Expense", "expense", "BAD_DEBT"},
		{"5208", "Income Tax Expense", "expense", "TAX_EXPENSE"},
		{"5904", "Deferred Tax Expense", "expense", "DEFERRED_TAX"},
		{"5906", "Interest Expense", "expense", "INTEREST_EXPENSE"},
		{"5907", "Inventory Adjustment Loss", "expense", "OTHER_EXPENSE"},
		{"5908", "Production Variance Loss", "expense", "OTHER_EXPENSE"},
	}
	accountIDs := make(map[string]int64, len(accounts))
	for _, account := range accounts {
		var id int64
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tenant_id, code) DO NOTHING
			RETURNING id
		`, tenantID, account.code, account.name, account.reportGroup, account.accountType).Scan(&id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Already existed; fetch it.
				if err := tx.QueryRow(ctx,
					`SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2`,
					tenantID, account.code).Scan(&id); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		accountIDs[account.code] = id
	}

	categories := []struct {
		name       string
		direction  string
		debitCode  string
		creditCode string
	}{
		{"Sales", "IN", "1101", "4101"},
		{"Receive receivables", "IN", "1101", "1201"},
		{"Additional capital", "IN", "1101", "3101"},
		{"Purchase of goods", "OUT", "1301", "1101"},
		{"Employee salaries", "OUT", "5201", "1101"},
		{"Rent", "OUT", "5202", "1101"},
		{"Transportation", "OUT", "5203", "1101"},
		{"Electricity and water", "OUT", "5204", "1101"},
		{"Other expenses", "OUT", "5205", "1101"},
	}
	for _, category := range categories {
		if _, err := tx.Exec(ctx, `
			INSERT INTO categories (tenant_id, name, direction, default_debit_account_id, default_credit_account_id)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tenant_id, name, direction) DO NOTHING
		`, tenantID, category.name, category.direction, accountIDs[category.debitCode], accountIDs[category.creditCode]); err != nil {
			return err
		}
	}

	// Open the current calendar year as the default accounting period so the
	// tenant can post immediately.
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting_periods (tenant_id, period_start, period_end, status)
		VALUES ($1, date_trunc('year', current_date)::date, (date_trunc('year', current_date) + interval '1 year - 1 day')::date, 'OPEN')
		ON CONFLICT (tenant_id, period_start, period_end) DO NOTHING
	`, tenantID); err != nil {
		return err
	}
	return nil
}
