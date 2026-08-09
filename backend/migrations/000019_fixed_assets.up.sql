-- US-060..063: Fixed Assets (Aset Tetap) — registrasi, penyusutan multi-metode,
-- revaluasi (ke ekuitas/OCI), disposisi & penjualan, serta impairment (PSAK 16).

-- Seed accounts 1402, 5206, 3401, 4903, 5903, 5207 for existing tenants.
-- 1401 Fixed Assets already exists (seeded in 000001 / seed.go).
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('1402', 'Accumulated Depreciation', 'asset', 'CONTRA_ASSET'),
  ('5206', 'Depreciation Expense', 'expense', 'DEPRECIATION'),
  ('3401', 'Revaluation Surplus (OCI)', 'equity', 'OCI'),
  ('4903', 'Gain on Asset Disposal', 'revenue', 'OTHER_INCOME'),
  ('5903', 'Loss on Asset Disposal', 'expense', 'OTHER_EXPENSE'),
  ('5207', 'Impairment Loss', 'expense', 'IMPAIRMENT')
) AS v(code, name, report_group, account_type)
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code);

CREATE TABLE fixed_assets (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    asset_account_id BIGINT NOT NULL,
    accum_dep_account_id BIGINT NOT NULL,
    dep_expense_account_id BIGINT NOT NULL,
    impairment_account_id BIGINT,
    acquisition_date DATE NOT NULL,
    acquisition_cost_cents BIGINT NOT NULL CHECK (acquisition_cost_cents > 0),
    salvage_value_cents BIGINT NOT NULL DEFAULT 0,
    useful_life_months INT NOT NULL CHECK (useful_life_months > 0),
    depreciation_method TEXT NOT NULL DEFAULT 'straight_line' CHECK (depreciation_method IN ('straight_line','declining_balance','units_of_production')),
    rate NUMERIC(9,6) NOT NULL DEFAULT 0,
    units_total BIGINT,
    units_used BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISPOSED','IMPAIRED')),
    book_value_cents BIGINT NOT NULL DEFAULT 0,
    accum_dep_cents BIGINT NOT NULL DEFAULT 0,
    journal_entry_id BIGINT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, asset_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, accum_dep_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, dep_expense_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, impairment_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE asset_depreciation_schedule (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    asset_id BIGINT NOT NULL,
    period_year INT NOT NULL,
    period_month INT NOT NULL,
    depreciation_cents BIGINT NOT NULL,
    journal_entry_id BIGINT,
    posted BOOLEAN NOT NULL DEFAULT false,
    posted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, asset_id, period_year, period_month),
    FOREIGN KEY (tenant_id, asset_id) REFERENCES fixed_assets(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE asset_transactions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    asset_id BIGINT NOT NULL,
    tx_type TEXT NOT NULL CHECK (tx_type IN ('ACQUISITION','DEPRECIATION','REVALUATION','DISPOSAL','IMPAIRMENT')),
    tx_date DATE NOT NULL,
    amount_cents BIGINT NOT NULL,
    journal_entry_id BIGINT,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, asset_id) REFERENCES fixed_assets(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_fixed_assets_tenant_status ON fixed_assets (tenant_id, status);
CREATE INDEX idx_asset_depreciation_schedule_asset ON asset_depreciation_schedule (tenant_id, asset_id, period_year, period_month);
CREATE INDEX idx_asset_transactions_asset ON asset_transactions (tenant_id, asset_id, tx_date);

ALTER TABLE fixed_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_depreciation_schedule ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_transactions ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_fixed_assets ON fixed_assets
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_asset_depreciation_schedule ON asset_depreciation_schedule
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_asset_transactions ON asset_transactions
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE fixed_assets FORCE ROW LEVEL SECURITY;
ALTER TABLE asset_depreciation_schedule FORCE ROW LEVEL SECURITY;
ALTER TABLE asset_transactions FORCE ROW LEVEL SECURITY;
