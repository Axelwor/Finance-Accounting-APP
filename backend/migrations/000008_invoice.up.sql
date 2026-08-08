-- P2 Phase 2D: Invoice (INV).
-- INV posts a revenue journal: Dr 1201 AR / Cr 4101 Revenue.
-- DP realization: Dr 2201 Customer Deposit / Cr 1201 AR (reduces receivable).

CREATE TABLE invoices (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    sales_order_id BIGINT,
    customer_id BIGINT NOT NULL,
    invoice_date DATE NOT NULL,
    due_date DATE,
    payment_term_id BIGINT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'ISSUED' CHECK (status IN ('DRAFT', 'ISSUED', 'PARTIALLY_PAID', 'PAID', 'VOID')),
    total_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    dp_applied_cents BIGINT NOT NULL DEFAULT 0 CHECK (dp_applied_cents >= 0),
    receivable_cents BIGINT NOT NULL DEFAULT 0,
    revenue_journal_entry_id BIGINT,
    dp_journal_entry_id BIGINT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, sales_order_id) REFERENCES sales_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, payment_term_id) REFERENCES payment_terms(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, revenue_journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, dp_journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE invoice_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    invoice_id BIGINT NOT NULL,
    item_id BIGINT,
    delivery_id BIGINT,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    discount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    tax_rate NUMERIC(9,6) NOT NULL DEFAULT 0,
    line_total_cents BIGINT NOT NULL DEFAULT 0,
    revenue_account_id BIGINT,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, delivery_id) REFERENCES delivery_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, revenue_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX invoices_tenant_date_idx ON invoices (tenant_id, invoice_date);
CREATE INDEX invoice_lines_invoice_idx ON invoice_lines (tenant_id, invoice_id);

ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_lines ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_invoices ON invoices
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_invoice_lines ON invoice_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE invoices FORCE ROW LEVEL SECURITY;
ALTER TABLE invoice_lines FORCE ROW LEVEL SECURITY;
