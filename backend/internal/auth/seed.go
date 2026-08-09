package auth

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// seedDefaultCOA provisions the core chart of accounts and UI categories for a
// freshly created tenant. It runs inside the registration transaction so a
// failure rolls the whole registration back.
func seedDefaultCOA(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	accounts := []struct {
		code        string
		name        string
		reportGroup string
		accountType string
	}{
		{"1101", "Cash", "asset", "CASH"},
		{"1102", "Bank", "asset", "BANK"},
		{"1201", "Accounts Receivable", "asset", "AR"},
		{"1203", "Input VAT", "asset", "INPUT_VAT"},
		{"1204", "Other Receivables", "asset", "OTHER_RECEIVABLE"},
		{"1205", "Prepayment to Suppliers", "asset", "PREPAYMENT"},
		{"1301", "Inventory", "asset", "INVENTORY"},
		{"1303", "Work in Progress", "asset", "INVENTORY"},
		{"1304", "Finished Goods", "asset", "INVENTORY"},
		{"1401", "Fixed Assets", "asset", "FIXED_ASSET"},
		{"2101", "Accounts Payable", "liability", "AP"},
		{"2105", "Uninvoiced Payables", "liability", "ACCRUED_LIABILITY"},
		{"2201", "Customer Deposit", "liability", "CUSTOMER_DEPOSIT"},
		{"2202", "VAT Payable", "liability", "TAX_PAYABLE"},
		{"2402", "Customer Overpayment", "liability", "CUSTOMER_DEPOSIT"},
		{"3101", "Capital", "equity", "EQUITY"},
		{"3201", "Retained Earnings", "equity", "EQUITY"},
		{"3301", "Current Earnings", "equity", "EQUITY"},
		{"4101", "Sales Revenue", "revenue", "REVENUE"},
		{"4201", "Sales Returns", "revenue", "CONTRA_REVENUE"},
		{"4907", "Inventory Adjustment Gain", "revenue", "OTHER_INCOME"},
		{"5101", "COGS", "expense", "COGS"},
		{"5201", "Salary Expense", "expense", "EXPENSE"},
		{"5202", "Rent Expense", "expense", "EXPENSE"},
		{"5203", "Transportation Expense", "expense", "EXPENSE"},
		{"5204", "Utilities Expense", "expense", "EXPENSE"},
		{"5205", "Other Expenses", "expense", "EXPENSE"},
		{"5907", "Inventory Adjustment Loss", "expense", "OTHER_EXPENSE"},
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
			if err == pgx.ErrNoRows {
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
