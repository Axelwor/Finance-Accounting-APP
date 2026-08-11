-- G-01: Resolve account code collisions from 4-way conflicts between seed.go and migrations.
-- 
-- Collision resolutions (per instructions):
-- - 5209: Keep Bad Debt Expense in seed.go. RoU Depreciation moves to 5210.
--         Update lease/depreciation.go and lease/helpers.go to use 5210.
-- - 5904: Keep Deferred Tax Expense in seed.go. FX Loss moves to 5905 (migration 000029).
-- - 5203: Keep Transportation in seed.go. Income Tax moves to 5208 or 5211.
--         Check if 5208 is used; if taken, use 5211.
-- - 1304: Keep Finished Goods in seed.go. Cheques in Transit moves to 1305 (migrations/cheque handler).
--
-- This migration seeds the corrected account codes for existing tenants that need them.
-- Existing accounts are NOT renamed (would break journals); new accounts are seeded with new codes.

-- 5210 = RoU Depreciation (was in conflict with 5209)
INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active)
SELECT t.id, '5210', 'RoU Depreciation', 'expense', 'DEPRECIATION', false, true
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '5210'
);

-- 5905 = Loss on Foreign Exchange (was in conflict with 5904)
INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active)
SELECT t.id, '5905', 'Loss on Foreign Exchange', 'expense', 'FX_LOSS', false, true
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '5905'
);

-- 5208 = Income Tax Expense (if not already taken by PPh Final UMKM check needed)
-- Using 5211 instead to avoid potential conflict
INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active)
SELECT t.id, '5211', 'Income Tax Expense', 'expense', 'TAX_EXPENSE', false, true
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '5211'
);

-- 1305 = Cheques in Transit (was in conflict with 1304)
INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active)
SELECT t.id, '1305', 'Cheques in Transit', 'asset', 'CHEQUES_IN_TRANSIT', false, true
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '1305'
);
