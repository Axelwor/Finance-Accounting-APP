-- US-080..083: Tax (PPN, PPh, ECL).
-- Seeds the accounts required by the tax package:
--   2203 Income Tax Payable       (liability)  — PPh 21/23/26/final payable
--   5208 Income Tax Expense        (expense)   — PPh final UMKM expense
--   1202 Allowance for Doubtful Accts (asset, contra) — ECL provision (PSAK 48)
--   5209 Bad Debt Expense          (expense)   — ECL charge to P&L
--   4906 Bad Debt Recovery         (revenue)   — written-off receivable recovered
--   1206 Deferred Tax Asset        (asset)     — PSAK 46 deferred tax (minimal)
--   5904 Deferred Tax Expense      (expense)   — PSAK 46 deferred tax (minimal)
-- Existing seeded accounts used by the tax package:
--   1203 Input VAT (PPN masukan), 2202 VAT Payable (PPN keluaran), 4101 Sales Revenue,
--   1201 Accounts Receivable, 1101 Cash / 1102 Bank.

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('2203', 'Income Tax Payable',         'liability', 'TAX_PAYABLE'),
  ('5208', 'Income Tax Expense',         'expense',  'TAX_EXPENSE'),
  ('1202', 'Allowance for Doubtful Accounts', 'asset', 'CONTRA_ASSET'),
  ('5205', 'Bad Debt Expense',           'expense',  'BAD_DEBT'),
  ('4906', 'Bad Debt Recovery',          'revenue',  'OTHER_INCOME'),
  ('1206', 'Deferred Tax Asset',         'asset',    'DEFERRED_TAX'),
  ('5904', 'Deferred Tax Expense',       'expense',  'DEFERRED_TAX')
) AS v(code, name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a
    WHERE a.tenant_id = t.id AND a.code = v.code
);

-- Tax rates configuration. One row per (tenant, tax_type, effective_from).
-- rate is a percentage (0..100) stored with 6 decimals so 0.5 (PPh Final UMKM)
-- and 11 (PPN) are both exact. effective_to is NULL = open-ended.
CREATE TABLE tax_rates (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    tax_type TEXT NOT NULL CHECK (tax_type IN ('PPN','PPH_FINAL_UMKM','PPH_21','PPH_23','PPH_26')),
    rate NUMERIC(9,6) NOT NULL CHECK (rate >= 0 AND rate <= 100),
    effective_from DATE NOT NULL,
    effective_to DATE,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);

CREATE INDEX tax_rates_tenant_type_idx ON tax_rates (tenant_id, tax_type, effective_from);

-- Seed default rates for every existing tenant: PPN 11% and PPh Final UMKM 0.5%,
-- effective from the start of the current calendar year. Tenants can add more
-- rows (PPh 21/23/26, historical rates) via the API later.
INSERT INTO tax_rates (tenant_id, tax_type, rate, effective_from, description)
SELECT t.id, v.tax_type, v.rate, DATE_TRUNC('year', CURRENT_DATE)::date, v.description
FROM tenants t
CROSS JOIN (VALUES
  ('PPN',            11.0, 'PPN 11% (default)'),
  ('PPH_FINAL_UMKM',  0.5, 'PPh Final UMKM 0.5% (PP 23/2018)')
) AS v(tax_type, rate, description)
WHERE NOT EXISTS (
    SELECT 1 FROM tax_rates tr
    WHERE tr.tenant_id = t.id AND tr.tax_type = v.tax_type
);

-- PPN reconciliation per month. ppn_keluaran_cents = output VAT (2202 credit),
-- ppn_masukan_cents = input VAT (1203 debit), net_ppn_cents = keluaran - masukan
-- (positive = payable to the tax office, negative = excess to carry forward).
CREATE TABLE ppn_reconciliations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    period_year INT NOT NULL,
    period_month INT NOT NULL,
    ppn_keluaran_cents BIGINT NOT NULL DEFAULT 0,
    ppn_masukan_cents BIGINT NOT NULL DEFAULT 0,
    net_ppn_cents BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','FILED','PAID')),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, period_year, period_month)
);

CREATE INDEX ppn_reconciliations_tenant_period_idx
    ON ppn_reconciliations (tenant_id, period_year, period_month);

-- Row Level Security: both tables are tenant-scoped. The application role sets
-- app.tenant_id at the start of each transaction (see scopeTenant helpers).
ALTER TABLE tax_rates ENABLE ROW LEVEL SECURITY;
ALTER TABLE ppn_reconciliations ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_tax_rates ON tax_rates
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

CREATE POLICY tenant_isolation_ppn_reconciliations ON ppn_reconciliations
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE tax_rates FORCE ROW LEVEL SECURITY;
ALTER TABLE ppn_reconciliations FORCE ROW LEVEL SECURITY;
