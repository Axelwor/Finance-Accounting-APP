-- US-110: Konsolidasi Multi-Entitas (PSAK 65) + US-111: Sewa (PSAK 73)

-- Seed accounts 1701 (Right-of-Use Asset), 2301 (Lease Liability), 5906 (Interest Expense)
-- for existing tenants. 1701/2301 are new; 5906 may not exist in older COA seeds.
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.val_name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('1701', 'Right-of-Use Asset', 'asset', 'ROU_ASSET'),
  ('2301', 'Lease Liability', 'liability', 'LEASE_LIABILITY'),
  ('5906', 'Interest Expense', 'expense', 'INTEREST_EXPENSE')
) AS v(code, val_name, report_group, account_type)
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code);

-- ---------------------------------------------------------------------------
-- US-110: Entity hierarchy (parent-child tenants for consolidation)
-- ---------------------------------------------------------------------------
CREATE TABLE entity_hierarchy (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    parent_tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    relationship TEXT NOT NULL DEFAULT 'CHILD' CHECK (relationship IN ('PARENT','CHILD')),
    consolidation_pct NUMERIC(9,6) NOT NULL DEFAULT 1.0 CHECK (consolidation_pct > 0 AND consolidation_pct <= 1.0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, parent_tenant_id),
    CHECK (tenant_id <> parent_tenant_id)
);

-- ---------------------------------------------------------------------------
-- US-110: Inter-company transactions (for elimination during consolidation)
-- ---------------------------------------------------------------------------
CREATE TABLE inter_company_transactions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    counterparty_tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    tx_type TEXT NOT NULL CHECK (tx_type IN ('SALE','PURCHASE','LOAN','INTEREST','DIVIDEND','MANAGEMENT_FEE')),
    journal_entry_id BIGINT,
    amount_cents BIGINT NOT NULL,
    tx_date DATE NOT NULL,
    description TEXT,
    eliminated BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE SET NULL
);

-- ---------------------------------------------------------------------------
-- US-111: Lease contracts (PSAK 73)
-- ---------------------------------------------------------------------------
CREATE TABLE lease_contracts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    lessee_name TEXT NOT NULL,
    lessor_name TEXT,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    payment_amount_cents BIGINT NOT NULL CHECK (payment_amount_cents > 0),
    payment_frequency TEXT NOT NULL DEFAULT 'MONTHLY' CHECK (payment_frequency IN ('MONTHLY','QUARTERLY','ANNUALLY')),
    total_payments INT NOT NULL CHECK (total_payments > 0),
    discount_rate NUMERIC(9,6) NOT NULL CHECK (discount_rate > 0 AND discount_rate <= 1),
    rou_asset_account_id BIGINT NOT NULL,
    lease_liability_account_id BIGINT NOT NULL,
    interest_expense_account_id BIGINT NOT NULL,
    short_term_lease_account_id BIGINT,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','TERMINATED','EXPIRED')),
    initial_rou_cents BIGINT NOT NULL DEFAULT 0,
    initial_liability_cents BIGINT NOT NULL DEFAULT 0,
    journal_entry_id BIGINT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, rou_asset_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, lease_liability_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, interest_expense_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, short_term_lease_account_id) REFERENCES accounts(tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE SET NULL,
    CHECK (end_date >= start_date)
);

CREATE TABLE lease_payments (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    lease_id BIGINT NOT NULL,
    payment_no INT NOT NULL,
    payment_date DATE NOT NULL,
    payment_amount_cents BIGINT NOT NULL,
    principal_cents BIGINT NOT NULL DEFAULT 0,
    interest_cents BIGINT NOT NULL DEFAULT 0,
    remaining_liability_cents BIGINT NOT NULL DEFAULT 0,
    journal_entry_id BIGINT,
    posted BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, lease_id, payment_no),
    FOREIGN KEY (tenant_id, lease_id) REFERENCES lease_contracts(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE SET NULL
);

-- ---------------------------------------------------------------------------
-- Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX idx_entity_hierarchy_parent ON entity_hierarchy (parent_tenant_id);
CREATE INDEX idx_inter_company_tx_tenant ON inter_company_transactions (tenant_id, tx_date);
CREATE INDEX idx_inter_company_tx_counterparty ON inter_company_transactions (counterparty_tenant_id, tx_date);
CREATE INDEX idx_lease_contracts_tenant_status ON lease_contracts (tenant_id, status);
CREATE INDEX idx_lease_payments_lease ON lease_payments (tenant_id, lease_id, payment_no);

-- ---------------------------------------------------------------------------
-- Row Level Security
-- ---------------------------------------------------------------------------
ALTER TABLE entity_hierarchy ENABLE ROW LEVEL SECURITY;
ALTER TABLE inter_company_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE lease_contracts ENABLE ROW LEVEL SECURITY;
ALTER TABLE lease_payments ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_entity_hierarchy ON entity_hierarchy
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_inter_company_transactions ON inter_company_transactions
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_lease_contracts ON lease_contracts
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_lease_payments ON lease_payments
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE entity_hierarchy FORCE ROW LEVEL SECURITY;
ALTER TABLE inter_company_transactions FORCE ROW LEVEL SECURITY;
ALTER TABLE lease_contracts FORCE ROW LEVEL SECURITY;
ALTER TABLE lease_payments FORCE ROW LEVEL SECURITY;
