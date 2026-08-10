-- M-012: Seed production variance accounts and Applied Overhead for existing
-- tenants. These support the overhead variance recognition at period close
-- and the job-completion variance posting introduced in M-010/M-012.

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('4902', 'Applied Overhead', 'expense', 'EXPENSE'),
  ('4908', 'Production Variance Gain', 'revenue', 'OTHER_INCOME'),
  ('5908', 'Production Variance Loss', 'expense', 'OTHER_EXPENSE')
) AS v(code, name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);
