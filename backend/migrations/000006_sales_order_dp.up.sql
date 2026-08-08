-- P2 Phase 2B: Sales Order (SO) + Down Payment (DP).
-- SO is a commitment — no journal (like SQ).
-- DP posts a journal: Dr Cash/Bank / Cr 2201 Customer Deposit.

-- Seed 2201 Customer Deposit for existing tenants (seed.go adds it for new ones).
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '2201', 'Customer Deposit', 'liability', 'CUSTOMER_DEPOSIT'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '2201'
);

CREATE TABLE sales_orders (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    quotation_id BIGINT,
    customer_id BIGINT NOT NULL,
    order_date DATE NOT NULL,
    payment_term_id BIGINT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'CONFIRMED' CHECK (status IN ('CONFIRMED', 'CLOSED', 'CANCELLED')),
    total_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    dp_received_cents BIGINT NOT NULL DEFAULT 0 CHECK (dp_received_cents >= 0),
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, quotation_id) REFERENCES sales_quotations(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, payment_term_id) REFERENCES payment_terms(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE sales_orders_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id BIGINT NOT NULL,
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
    FOREIGN KEY (tenant_id, order_id) REFERENCES sales_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, revenue_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cogs_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE sales_down_payments (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    order_id BIGINT NOT NULL,
    journal_entry_id BIGINT,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    cash_account_id BIGINT NOT NULL,
    deposit_account_id BIGINT NOT NULL,
    dp_date DATE NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'RECEIVED' CHECK (status IN ('RECEIVED', 'REFUNDED')),
    idempotency_key UUID,
    source_ref TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, order_id) REFERENCES sales_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cash_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, deposit_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX sales_down_payments_idem ON sales_down_payments (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE INDEX sales_orders_tenant_date_idx ON sales_orders (tenant_id, order_date);
CREATE INDEX sales_orders_lines_order_idx ON sales_orders_lines (tenant_id, order_id);
CREATE INDEX sales_down_payments_order_idx ON sales_down_payments (tenant_id, order_id);

ALTER TABLE sales_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales_orders_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales_down_payments ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_sales_orders ON sales_orders
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_sales_orders_lines ON sales_orders_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_sales_down_payments ON sales_down_payments
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE sales_orders FORCE ROW LEVEL SECURITY;
ALTER TABLE sales_orders_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE sales_down_payments FORCE ROW LEVEL SECURITY;
