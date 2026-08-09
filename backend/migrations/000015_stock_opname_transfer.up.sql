-- P3 Phase 3D: Stock Opname (physical count adjustment) + Stock Transfer.
-- Stock opname posts an adjustment journal when approved:
--   surplus  (diff > 0): Dr 1301 Inventory / Cr 4907 Inventory Adjustment Gain
--   shortage (diff < 0): Dr 5907 Inventory Adjustment Loss / Cr 1301 Inventory
-- Stock transfer posts no journal (same inventory account, no value change);
-- it only records TRANSFER_OUT / TRANSFER_IN inventory movements.

-- Seed adjustment accounts for existing tenants.
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '4907', 'Inventory Adjustment Gain', 'revenue', 'OTHER_INCOME'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '4907');

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '5907', 'Inventory Adjustment Loss', 'expense', 'OTHER_EXPENSE'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '5907');

-- Stock opname (physical count adjustment).
CREATE TABLE stock_opnames (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,          -- OPN-{YYYY}-{seq}
    opname_date DATE NOT NULL,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','COUNTED','APPROVED','VOID')),
    total_adjustment_cents BIGINT NOT NULL DEFAULT 0,
    journal_entry_id BIGINT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE stock_opname_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    opname_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    line_no INT NOT NULL DEFAULT 1,
    system_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    counted_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    diff_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    adjustment_cents BIGINT NOT NULL DEFAULT 0,
    inventory_account_id BIGINT NOT NULL,
    reason TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, opname_id) REFERENCES stock_opnames(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

-- Stock transfer between warehouses (single warehouse for now).
CREATE TABLE stock_transfers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,          -- TRF-{YYYY}-{seq}
    transfer_date DATE NOT NULL,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'COMPLETED' CHECK (status IN ('COMPLETED','VOID')),
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number)
);

CREATE TABLE stock_transfer_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    transfer_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    inventory_account_id BIGINT NOT NULL,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, transfer_id) REFERENCES stock_transfers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX stock_opnames_tenant_date_idx ON stock_opnames (tenant_id, opname_date);
CREATE INDEX stock_opname_lines_opname_idx ON stock_opname_lines (tenant_id, opname_id);
CREATE INDEX stock_transfers_tenant_date_idx ON stock_transfers (tenant_id, transfer_date);
CREATE INDEX stock_transfer_lines_transfer_idx ON stock_transfer_lines (tenant_id, transfer_id);

ALTER TABLE stock_opnames ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_opname_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_transfer_lines ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_stock_opnames ON stock_opnames
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_stock_opname_lines ON stock_opname_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_stock_transfers ON stock_transfers
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_stock_transfer_lines ON stock_transfer_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE stock_opnames FORCE ROW LEVEL SECURITY;
ALTER TABLE stock_opname_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE stock_transfers FORCE ROW LEVEL SECURITY;
ALTER TABLE stock_transfer_lines FORCE ROW LEVEL SECURITY;
