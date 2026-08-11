-- Rollback 000045
ALTER TABLE petty_cash_vouchers DROP COLUMN IF EXISTS replenished_at;
DELETE FROM accounts WHERE code IN ('1305', '2106');
