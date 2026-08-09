-- Fix: seed 5209 (Bad Debt Expense) for existing tenants. The original migration
-- 000021 seeded 5205 which collides with "Other Expenses" already in seed.go.
-- New tenants get 5209 via seed.go; this migration adds it for existing tenants.
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '5209', 'Bad Debt Expense', 'expense', 'BAD_DEBT'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '5209'
);
