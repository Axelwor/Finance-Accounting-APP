-- M-022: Seed RoU depreciation accounts for PSAK 73 compliance.
-- 1702 = Accumulated Right-of-Use Depreciation (contra-asset)
-- 5209 = RoU Depreciation Expense (expense)

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.val_name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('1702', 'Accumulated RoU Depreciation', 'asset', 'CONTRA_ASSET'),
  ('5209', 'RoU Depreciation Expense', 'expense', 'DEPRECIATION')
) AS v(code, val_name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);

-- Track which periods have been depreciated per lease contract.
-- Prevents double-posting depreciation for the same period.
CREATE TABLE lease_depreciation_log (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    lease_id BIGINT NOT NULL,
    period_year INT NOT NULL,
    period_month INT NOT NULL,
    depreciation_cents BIGINT NOT NULL,
    journal_entry_id BIGINT,
    posted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, lease_id, period_year, period_month),
    FOREIGN KEY (tenant_id, lease_id) REFERENCES lease_contracts(tenant_id, id) ON DELETE RESTRICT
);

ALTER TABLE lease_depreciation_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_lease_depreciation_log ON lease_depreciation_log
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE lease_depreciation_log FORCE ROW LEVEL SECURITY;
