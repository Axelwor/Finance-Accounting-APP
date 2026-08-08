-- P3 Phase 3A: Purchase flow — Suppliers, Purchase Orders (PO), Goods Received Notes (GRN).
-- PO posts no journal (commitment). GRN posts: Dr 1301 Inventory / Cr 2105 Utang Belum Ditagih.
-- New accounts: 1205 (Advance to Supplier), 2105 (Accrued Payables), 1203 (Input VAT).

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '1205', 'Advance to Supplier', 'asset', 'ADVANCE_TO_SUPPLIER'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '1205');

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '2105', 'Accrued Payables', 'liability', 'ACCRUED_LIABILITY'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '2105');

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '1203', 'Input VAT', 'asset', 'TAX_RECEIVABLE'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '1203');

CREATE TABLE suppliers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    npwp TEXT,
    contact_person TEXT,
    phone TEXT,
    email TEXT,
    address TEXT,
    city TEXT,
    province TEXT,
    postal_code TEXT,
    payment_term_id BIGINT,
    credit_limit_cents BIGINT,
    default_ap_account_id BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, payment_term_id) REFERENCES payment_terms(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, default_ap_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE purchase_orders (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    supplier_id BIGINT NOT NULL,
    order_date DATE NOT NULL,
    expected_date DATE,
    payment_term_id BIGINT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'CONFIRMED' CHECK (status IN ('CONFIRMED', 'PARTIALLY_RECEIVED', 'RECEIVED', 'CANCELLED')),
    total_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    received_cents BIGINT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, payment_term_id) REFERENCES payment_terms(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE purchase_orders_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    discount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    tax_rate NUMERIC(9,6) NOT NULL DEFAULT 0,
    line_total_cents BIGINT NOT NULL DEFAULT 0,
    received_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, order_id) REFERENCES purchase_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE goods_received_notes (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    purchase_order_id BIGINT NOT NULL,
    supplier_id BIGINT NOT NULL,
    grn_date DATE NOT NULL,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'RECEIVED' CHECK (status IN ('RECEIVED', 'RETURNED', 'CANCELLED')),
    journal_entry_id BIGINT,
    total_cents BIGINT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, purchase_order_id) REFERENCES purchase_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE grn_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    grn_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    po_line_id BIGINT,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    line_total_cents BIGINT NOT NULL DEFAULT 0,
    inventory_account_id BIGINT NOT NULL,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, grn_id) REFERENCES goods_received_notes(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, po_line_id) REFERENCES purchase_orders_lines(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX purchase_orders_tenant_date_idx ON purchase_orders (tenant_id, order_date);
CREATE INDEX purchase_orders_lines_order_idx ON purchase_orders_lines (tenant_id, order_id);
CREATE INDEX grn_tenant_date_idx ON goods_received_notes (tenant_id, grn_date);
CREATE INDEX grn_lines_grn_idx ON grn_lines (tenant_id, grn_id);

ALTER TABLE suppliers ENABLE ROW LEVEL SECURITY;
ALTER TABLE purchase_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE purchase_orders_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE goods_received_notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE grn_lines ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_suppliers ON suppliers
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_purchase_orders ON purchase_orders
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_purchase_orders_lines ON purchase_orders_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_goods_received_notes ON goods_received_notes
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_grn_lines ON grn_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE suppliers FORCE ROW LEVEL SECURITY;
ALTER TABLE purchase_orders FORCE ROW LEVEL SECURITY;
ALTER TABLE purchase_orders_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE goods_received_notes FORCE ROW LEVEL SECURITY;
ALTER TABLE grn_lines FORCE ROW LEVEL SECURITY;
