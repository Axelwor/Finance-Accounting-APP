-- B-04: Seed accounts 1702, 4902, 4908, 5908 for existing tenants.
-- These were seeded only in migrations 000026/000028/000037; tenants created
-- before those migrations or in degraded states may lack them. seed.go now
-- includes all four for new tenants.

INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active)
SELECT t.id, v.code, v.name, v.report_group, v.account_type, false, true
FROM tenants t
CROSS JOIN (VALUES
  ('1702', 'Accumulated RoU Depreciation', 'asset', 'CONTRA_ASSET'),
  ('4902', 'Applied Overhead', 'expense', 'EXPENSE'),
  ('4908', 'Production Variance Gain', 'revenue', 'OTHER_INCOME'),
  ('5908', 'Production Variance Loss', 'expense', 'OTHER_EXPENSE')
) AS v(code, name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);
