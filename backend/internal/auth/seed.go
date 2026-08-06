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
		{"1101", "Kas", "asset", "CASH"},
		{"1102", "Bank", "asset", "BANK"},
		{"1201", "Piutang Usaha", "asset", "AR"},
		{"1301", "Persediaan", "asset", "INVENTORY"},
		{"1401", "Aset Tetap", "asset", "FIXED_ASSET"},
		{"2101", "Hutang Usaha", "liability", "AP"},
		{"2202", "Utang PPN", "liability", "TAX_PAYABLE"},
		{"3101", "Modal", "equity", "EQUITY"},
		{"3201", "Laba Ditahan", "equity", "EQUITY"},
		{"3301", "Laba Berjalan", "equity", "EQUITY"},
		{"4101", "Pendapatan Penjualan", "revenue", "REVENUE"},
		{"5101", "HPP", "expense", "COGS"},
		{"5201", "Beban Gaji", "expense", "EXPENSE"},
		{"5202", "Beban Sewa", "expense", "EXPENSE"},
		{"5203", "Beban Transport", "expense", "EXPENSE"},
		{"5204", "Beban Listrik & Air", "expense", "EXPENSE"},
		{"5205", "Beban Lain-lain", "expense", "EXPENSE"},
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
		{"Penjualan", "IN", "1101", "4101"},
		{"Terima piutang", "IN", "1101", "1201"},
		{"Modal tambahan", "IN", "1101", "3101"},
		{"Belanja barang dagang", "OUT", "1301", "1101"},
		{"Gaji karyawan", "OUT", "5201", "1101"},
		{"Sewa tempat", "OUT", "5202", "1101"},
		{"Transport", "OUT", "5203", "1101"},
		{"Listrik dan air", "OUT", "5204", "1101"},
		{"Pengeluaran lain", "OUT", "5205", "1101"},
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
	return nil
}
