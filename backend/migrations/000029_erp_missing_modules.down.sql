-- Rollback all tables and columns added in 000029.

-- Remove FX gain/loss accounts.
DELETE FROM accounts WHERE code IN ('4904', '5904');

-- Drop currency columns from journal_entries.
ALTER TABLE journal_entries DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE journal_entries DROP COLUMN IF EXISTS currency_code;

-- Drop tables.
DROP TABLE IF EXISTS petty_cash_vouchers;
DROP TABLE IF EXISTS petty_cash_funds;
DROP TABLE IF EXISTS recurring_transactions;
DROP TABLE IF EXISTS ap_aging_snapshots;
DROP TABLE IF EXISTS ar_aging_snapshots;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS currencies;
