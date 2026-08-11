-- A-27: Journal posting support for cheque, petty cash modules.
--
-- Cheque clearing accounts: codes 1304/2105 collide with seed.go
-- (Finished Goods / Uninvoiced Payables), so we use non-colliding codes.
-- 1305 Cheques in Transit, 2106 Cheques Issued Outstanding.
INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active, valid_from)
SELECT t.id, v.code, v.name, v.report_group, v.account_type, false, true, CURRENT_DATE
FROM tenants t
CROSS JOIN (VALUES
  ('1305', 'Cheques in Transit',         'asset',    'OTHER_CURRENT_ASSET'),
  ('2106', 'Cheques Issued Outstanding', 'liability', 'ACCRUED_LIABILITY')
) AS v(code, name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);

-- Petty cash: track which vouchers have been replenished so Replenish
-- only sums un-replenished vouchers (prevents double-posting).
ALTER TABLE petty_cash_vouchers ADD COLUMN IF NOT EXISTS replenished_at TIMESTAMPTZ;
