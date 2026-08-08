-- P2 Phase 2C: Delivery Order (DO) + inventory movements.
-- DO posts a journal: Dr 5101 COGS / Cr 1301 Inventory (per item delivered).
-- Stock reduces per qty delivered; negative stock rejected by default.

-- Track how much has been delivered per SO line (for partial delivery).
ALTER TABLE sales_orders_lines ADD COLUMN delivered_qty NUMERIC(18,3) NOT NULL DEFAULT 0;

CREATE TABLE delivery_orders (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    sales_order_id BIGINT NOT NULL,
    customer_id BIGINT NOT NULL,
    delivery_date DATE NOT NULL,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'SHIPPED' CHECK (status IN ('SHIPPED', 'RETURNED', 'CANCELLED')),
    journal_entry_id BIGINT,
    total_cogs_cents BIGINT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, sales_order_id) REFERENCES sales_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE delivery_orders_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    delivery_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    source_order_line_id BIGINT,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    cogs_cents BIGINT NOT NULL DEFAULT 0,
    inventory_account_id BIGINT NOT NULL,
    cogs_account_id BIGINT NOT NULL,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, delivery_id) REFERENCES delivery_orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cogs_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE inventory_movements (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    item_id BIGINT NOT NULL,
    movement_type TEXT NOT NULL CHECK (movement_type IN ('GRN', 'SALES_RETURN', 'DO', 'PRODUCTION_OUT', 'PRODUCTION_IN', 'TRANSFER_IN', 'TRANSFER_OUT', 'OPNAME_IN', 'OPNAME_OUT', 'ADJUSTMENT')),
    qty NUMERIC(18,3) NOT NULL,
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    source_ref TEXT,
    source_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX delivery_orders_tenant_date_idx ON delivery_orders (tenant_id, delivery_date);
CREATE INDEX delivery_orders_lines_delivery_idx ON delivery_orders_lines (tenant_id, delivery_id);
CREATE INDEX inventory_movements_item_idx ON inventory_movements (tenant_id, item_id, created_at);

ALTER TABLE delivery_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE delivery_orders_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_movements ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_delivery_orders ON delivery_orders
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_delivery_orders_lines ON delivery_orders_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_inventory_movements ON inventory_movements
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE delivery_orders FORCE ROW LEVEL SECURITY;
ALTER TABLE delivery_orders_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory_movements FORCE ROW LEVEL SECURITY;
