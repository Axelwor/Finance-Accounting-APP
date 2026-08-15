-- F-02: Multi-Warehouse master
CREATE TABLE IF NOT EXISTS warehouses (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    address TEXT,
    city TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code)
);
ALTER TABLE warehouses ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS warehouses_tenant ON warehouses;
CREATE POLICY warehouses_tenant ON warehouses
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);
ALTER TABLE warehouses FORCE ROW LEVEL SECURITY;

-- Add warehouse_id to stock_balances (nullable for backward compat)
ALTER TABLE stock_balances ADD COLUMN IF NOT EXISTS warehouse_id BIGINT;
ALTER TABLE stock_balances DROP CONSTRAINT IF EXISTS stock_balances_tenant_item_key;
-- Unique index (not a table constraint) so the COALESCE expression is allowed;
-- CREATE UNIQUE INDEX IF NOT EXISTS is idempotent.
CREATE UNIQUE INDEX IF NOT EXISTS stock_balances_tenant_item_warehouse_uidx
    ON stock_balances (tenant_id, item_id, COALESCE(warehouse_id, 0));

-- F-03: Approval Workflow Engine
CREATE TABLE IF NOT EXISTS approval_workflows (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    entity_type TEXT NOT NULL, -- invoice, purchase_order, credit_note, journal_entry
    min_amount_cents BIGINT NOT NULL DEFAULT 0,
    approver_role TEXT NOT NULL DEFAULT 'admin', -- admin, accountant, manager
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, entity_type)
);
ALTER TABLE approval_workflows ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS approval_workflows_tenant ON approval_workflows;
CREATE POLICY approval_workflows_tenant ON approval_workflows
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);
ALTER TABLE approval_workflows FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS approval_requests (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    entity_type TEXT NOT NULL,
    entity_id BIGINT NOT NULL,
    entity_number TEXT,
    requested_by BIGINT NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'CANCELLED')),
    approved_by BIGINT,
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    UNIQUE (tenant_id, entity_type, entity_id)
);
ALTER TABLE approval_requests ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS approval_requests_tenant ON approval_requests;
CREATE POLICY approval_requests_tenant ON approval_requests
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);
ALTER TABLE approval_requests FORCE ROW LEVEL SECURITY;

-- F-12: PPh (Withholding Tax) tables
CREATE TABLE IF NOT EXISTS pph_calculations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    pph_type TEXT NOT NULL CHECK (pph_type IN ('PPH21', 'PPH22', 'PPH23', 'PPH26', 'PPH_FINAL_UMKM')),
    calculation_date DATE NOT NULL,
    dpp_cents BIGINT NOT NULL, -- Dasar Pengenaan Pajak (taxable base)
    rate_percent NUMERIC(5,2) NOT NULL,
    pph_cents BIGINT NOT NULL,
    entity_name TEXT,
    entity_npwp TEXT,
    description TEXT,
    journal_entry_id BIGINT,
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'POSTED', 'FILED')),
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
ALTER TABLE pph_calculations ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS pph_calculations_tenant ON pph_calculations;
CREATE POLICY pph_calculations_tenant ON pph_calculations
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);
ALTER TABLE pph_calculations FORCE ROW LEVEL SECURITY;

-- Seed PPh-related accounts
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.val_name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('2107', 'PPh 21 Payable', 'liability', 'TAX_PAYABLE'),
  ('2108', 'PPh 22 Payable', 'liability', 'TAX_PAYABLE'),
  ('2109', 'PPh 23 Payable', 'liability', 'TAX_PAYABLE'),
  ('2110', 'PPh 26 Payable', 'liability', 'TAX_PAYABLE'),
  ('2111', 'PPh Final UMKM Payable', 'liability', 'TAX_PAYABLE'),
  ('5203', 'Income Tax Expense', 'expense', 'TAX_EXPENSE')
) AS v(code, val_name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);

-- Seed default warehouse per tenant
INSERT INTO warehouses (tenant_id, code, name, is_default)
SELECT t.id, 'WH-MAIN', 'Main Warehouse', true
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM warehouses w WHERE w.tenant_id = t.id AND w.code = 'WH-MAIN'
);
