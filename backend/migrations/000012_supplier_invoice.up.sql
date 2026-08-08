-- US-033: Supplier Invoice (Tagihan).
-- Adds supplier_invoices + supplier_invoice_lines and wires them into the
-- existing purchase flow (PO → GRN → Tagihan). The Tagihan step posts a
-- single journal that records PPN masukan, reclassifies 2105 → 2101, and
-- (later) realizes purchase DP.

CREATE TABLE supplier_invoices (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    number TEXT NOT NULL,          -- BIL-{YYYY}-{seq}
    supplier_id BIGINT NOT NULL,
    grn_id BIGINT,                 -- optional link to GRN
    invoice_date DATE NOT NULL,
    due_date DATE,
    supplier_invoice_number TEXT,  -- supplier's own invoice number
    dpp_cents BIGINT NOT NULL DEFAULT 0,    -- dasar pengenaan pajak
    vat_cents BIGINT NOT NULL DEFAULT 0,
    total_cents BIGINT NOT NULL DEFAULT 0,
    dp_applied_cents BIGINT NOT NULL DEFAULT 0,
    payable_cents BIGINT NOT NULL DEFAULT 0,  -- total - dp_applied
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'ISSUED' CHECK (status IN ('ISSUED','PARTIALLY_PAID','PAID','VOID')),
    journal_entry_id BIGINT,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number)
);

CREATE TABLE supplier_invoice_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    invoice_id BIGINT NOT NULL,
    item_id BIGINT,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    discount_cents BIGINT NOT NULL DEFAULT 0,
    tax_rate NUMERIC(9,6) NOT NULL DEFAULT 0,
    line_total_cents BIGINT NOT NULL DEFAULT 0,
    description TEXT,
    UNIQUE (tenant_id, id)
);

ALTER TABLE supplier_invoices
    ADD CONSTRAINT supplier_invoices_supplier_fk
        FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT supplier_invoices_grn_fk
        FOREIGN KEY (tenant_id, grn_id) REFERENCES goods_received_notes(tenant_id, id) ON DELETE SET NULL,
    ADD CONSTRAINT supplier_invoices_journal_fk
        FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE SET NULL;

ALTER TABLE supplier_invoice_lines
    ADD CONSTRAINT supplier_invoice_lines_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id) REFERENCES supplier_invoices(tenant_id, id) ON DELETE CASCADE;

CREATE POLICY tenant_isolation_supplier_invoices ON supplier_invoices
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_supplier_invoice_lines ON supplier_invoice_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE supplier_invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE supplier_invoice_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE supplier_invoices FORCE ROW LEVEL SECURITY;
ALTER TABLE supplier_invoice_lines FORCE ROW LEVEL SECURITY;
