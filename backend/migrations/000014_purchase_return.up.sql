-- P2 Phase 2G: Purchase Return (Retur Pembelian).
-- Purchase return posts a single balanced journal:
--   Dr 2101 Accounts Payable (total + vat_reversed)  -- AP goes back up
--   Cr 1301 Inventory (total)                        -- reduce inventory
--   Cr 1203 Input VAT (vat_reversed)                 -- reverse input VAT (if any)
-- The supplier invoice's payable_cents is increased (AP owed to supplier rises).

CREATE TABLE purchase_returns (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,          -- PRET-{YYYY}-{seq}
    supplier_id BIGINT NOT NULL,
    invoice_id BIGINT NOT NULL,
    return_date DATE NOT NULL,
    refund_method TEXT NOT NULL DEFAULT 'deduct' CHECK (refund_method IN ('deduct','refund','credit_balance')),
    reason TEXT,
    total_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    vat_reversed_cents BIGINT NOT NULL DEFAULT 0,
    ap_deducted_cents BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'APPLIED' CHECK (status IN ('APPLIED','VOID')),
    journal_entry_id BIGINT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES supplier_invoices(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE purchase_return_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    return_id BIGINT NOT NULL,
    item_id BIGINT,
    invoice_line_id BIGINT,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    line_total_cents BIGINT NOT NULL DEFAULT 0,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, return_id) REFERENCES purchase_returns(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX purchase_returns_tenant_date_idx ON purchase_returns (tenant_id, return_date);
CREATE INDEX purchase_return_lines_return_idx ON purchase_return_lines (tenant_id, return_id);

ALTER TABLE purchase_returns ENABLE ROW LEVEL SECURITY;
ALTER TABLE purchase_return_lines ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_purchase_returns ON purchase_returns
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_purchase_return_lines ON purchase_return_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE purchase_returns FORCE ROW LEVEL SECURITY;
ALTER TABLE purchase_return_lines FORCE ROW LEVEL SECURITY;
