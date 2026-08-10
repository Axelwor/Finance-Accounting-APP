-- M-025: Seed missing COA accounts per ACCOUNTING_ENGINE §3.0.2.
-- 3105 = Suspense (Equity) — for opening balance with imbalance (§5)
-- 4902 = Applied Overhead (Expense) — for production overhead (§11.2)
-- 1302 = Raw Material (Asset) — for raw material tracking (§3.0.1)

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.val_name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('3105', 'Suspense / Modal Setoran', 'equity', 'EQUITY'),
  ('4902', 'Applied Overhead', 'expense', 'APPLIED_OVERHEAD'),
  ('1302', 'Raw Material', 'asset', 'INVENTORY')
) AS v(code, val_name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);
