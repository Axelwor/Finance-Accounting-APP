package settings

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AccountSetting enumerates the tenant_settings default-account columns that
// the posting engines may resolve (SET-001). Callers fall back to the legacy
// hardcoded account code when the mapping is empty, which keeps behaviour
// identical for tenants that never opened the settings screen.
type AccountSetting string

const (
	SettingDefaultSales         AccountSetting = "default_sales_account_id"
	SettingDefaultPurchase      AccountSetting = "default_purchase_account_id"
	SettingDefaultCOGS          AccountSetting = "default_cogs_account_id"
	SettingDefaultAR            AccountSetting = "default_ar_account_id"
	SettingDefaultAP            AccountSetting = "default_ap_account_id"
	SettingDefaultCash          AccountSetting = "default_cash_account_id"
	SettingDefaultCapital       AccountSetting = "default_capital_account_id"
	SettingRetainedEarnings     AccountSetting = "retained_earnings_account_id"
	SettingOpeningBalanceEquity AccountSetting = "opening_balance_equity_account_id"
	SettingFxGain               AccountSetting = "fx_gain_account_id"
	SettingFxLoss               AccountSetting = "fx_loss_account_id"
)

// ResolveAccount returns the account id for a settings column, falling back
// to the legacy seeded account code when the mapping is NULL. The fallback
// code is the same constant the engines used before settings existed, so an
// unmapped tenant posts exactly as before.
func ResolveAccount(ctx context.Context, tx pgx.Tx, tenant int64, setting AccountSetting, fallbackCode string) (int64, error) {
	var id *int64
	if err := tx.QueryRow(ctx,
		`SELECT `+string(setting)+` FROM tenant_settings WHERE tenant_id = $1`, tenant).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			id = nil
		} else {
			return 0, err
		}
	}
	if id != nil && *id > 0 {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM accounts WHERE tenant_id = $1 AND id = $2)`, tenant, *id).Scan(&exists); err != nil {
			return 0, err
		}
		if exists {
			return *id, nil
		}
	}
	// Legacy fallback: resolve by the hardcoded code.
	var accountID int64
	err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2`,
		tenant, fallbackCode).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("account %s not found: %w", fallbackCode, err)
	}
	return accountID, nil
}

// LatestRate is a shared helper for the engines: the most recent manual rate
// for a currency pair with effective_date <= the given date (YYYY-MM-DD).
// Returns 0 when no rate exists (the caller decides whether to default to 1).
func LatestRate(ctx context.Context, tx pgx.Tx, tenant int64, from, to, date string) (float64, error) {
	var rate *float64
	err := tx.QueryRow(ctx, `
		SELECT rate::float8 FROM exchange_rates
		WHERE tenant_id = $1 AND from_currency = $2 AND to_currency = $3
		  AND effective_date <= $4::date
		ORDER BY effective_date DESC, id DESC
		LIMIT 1
	`, tenant, from, to, date).Scan(&rate)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if rate == nil {
		return 0, nil
	}
	return *rate, nil
}
