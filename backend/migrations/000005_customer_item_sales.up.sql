-- P2 Phase 2A: Customer, Item, and Sales Quotation master data.
-- No journal posting here — SQ is a commitment only (see ACCOUNTING_ENGINE.md).

CREATE TABLE payment_terms (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    due_days INT NOT NULL DEFAULT 30,
    discount_days INT,
    discount_percent NUMERIC(9,6),
    cash_flow_category TEXT CHECK (cash_flow_category IN ('operating', 'investing', 'financing')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code)
);

CREATE TABLE customers (
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
    default_revenue_account_id BIGINT,
    default_receivable_account_id BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, payment_term_id) REFERENCES payment_terms(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, default_revenue_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, default_receivable_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE items (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    item_type TEXT NOT NULL CHECK (item_type IN ('goods', 'service')),
    uom TEXT NOT NULL DEFAULT 'pcs',
    costing_method TEXT CHECK (costing_method IN ('fifo', 'moving_average', 'specific')),
    sale_account_id BIGINT,
    cogs_account_id BIGINT,
    inventory_account_id BIGINT,
    revenue_recognition_method TEXT CHECK (revenue_recognition_method IN ('point_in_time', 'over_time', 'milestone', 'straight_line')),
    is_tracked_stock BOOLEAN NOT NULL DEFAULT false,
    min_stock_qty NUMERIC(18,3),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, sale_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cogs_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    CHECK (item_type <> 'goods' OR (costing_method IS NOT NULL AND inventory_account_id IS NOT NULL)),
    CHECK (item_type <> 'service' OR (inventory_account_id IS NULL AND cogs_account_id IS NULL))
);

CREATE TABLE item_price_lists (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    item_id BIGINT NOT NULL,
    price_list_name TEXT NOT NULL DEFAULT 'Umum',
    customer_group TEXT,
    customer_id BIGINT,
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    effective_from DATE,
    effective_to DATE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, item_id, price_list_name, customer_group, effective_from),
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE sales_quotations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    customer_id BIGINT NOT NULL,
    quotation_date DATE NOT NULL,
    valid_until DATE,
    payment_term_id BIGINT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'SENT', 'CONVERTED', 'EXPIRED', 'CANCELLED')),
    total_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, payment_term_id) REFERENCES payment_terms(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE sales_quotations_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    quotation_id BIGINT NOT NULL,
    item_id BIGINT,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    discount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    tax_rate NUMERIC(9,6) NOT NULL DEFAULT 0,
    line_total_cents BIGINT NOT NULL DEFAULT 0,
    revenue_account_id BIGINT,
    cogs_account_id BIGINT,
    inventory_account_id BIGINT,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, quotation_id) REFERENCES sales_quotations(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, revenue_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cogs_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX customers_tenant_idx ON customers (tenant_id, is_active);
CREATE INDEX items_tenant_idx ON items (tenant_id, is_active);
CREATE INDEX sales_quotations_tenant_date_idx ON sales_quotations (tenant_id, quotation_date);
CREATE INDEX sales_quotations_lines_quotation_idx ON sales_quotations_lines (tenant_id, quotation_id);

ALTER TABLE payment_terms ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE items ENABLE ROW LEVEL SECURITY;
ALTER TABLE item_price_lists ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales_quotations ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales_quotations_lines ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_payment_terms ON payment_terms
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_customers ON customers
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_items ON items
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_item_price_lists ON item_price_lists
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_sales_quotations ON sales_quotations
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_sales_quotations_lines ON sales_quotations_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE payment_terms FORCE ROW LEVEL SECURITY;
ALTER TABLE customers FORCE ROW LEVEL SECURITY;
ALTER TABLE items FORCE ROW LEVEL SECURITY;
ALTER TABLE item_price_lists FORCE ROW LEVEL SECURITY;
ALTER TABLE sales_quotations FORCE ROW LEVEL SECURITY;
ALTER TABLE sales_quotations_lines FORCE ROW LEVEL SECURITY;