-- P2-015: Inventory costing (PSAK 14). Persists on-hand balances and average
-- cost on stock_balances, and records each GRN receipt as a FIFO cost layer in
-- inventory_cost_layers. Delivery orders (and other stock-out movements)
-- consume the oldest open layers first (FIFO) or value stock at the running
-- moving average. Reversals (sales returns, purchase returns) restore layers
-- or adjust the average cost.

-- Stock balances (persisted on-hand qty + average cost). One row per item.
-- qty_on_hand is the authoritative running balance; avg_unit_cost_cents is the
-- moving-average unit cost (also used as the fallback valuation for FIFO when
-- no layers remain, e.g. legacy data).
CREATE TABLE stock_balances (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    item_id BIGINT NOT NULL,
    qty_on_hand NUMERIC(18,3) NOT NULL DEFAULT 0,
    avg_unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, item_id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT
);

-- FIFO cost layers. Each GRN creates a layer (qty_original = qty received,
-- unit_cost_cents = GRN unit cost). Each stock-out movement consumes the
-- oldest open layer first (ORDER BY created_at). A layer is "closed" when
-- qty_remaining = 0 (closed_at set).
CREATE TABLE inventory_cost_layers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    item_id BIGINT NOT NULL,
    grn_line_id BIGINT,
    qty_original NUMERIC(18,3) NOT NULL CHECK (qty_original > 0),
    qty_remaining NUMERIC(18,3) NOT NULL DEFAULT 0,
    unit_cost_cents BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX stock_balances_tenant_item_idx ON stock_balances (tenant_id, item_id);
CREATE INDEX inventory_cost_layers_item_open_idx
    ON inventory_cost_layers (tenant_id, item_id, created_at)
    WHERE closed_at IS NULL;
CREATE INDEX inventory_cost_layers_grn_line_idx
    ON inventory_cost_layers (tenant_id, grn_line_id)
    WHERE grn_line_id IS NOT NULL;

ALTER TABLE stock_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_cost_layers ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_stock_balances ON stock_balances
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_inventory_cost_layers ON inventory_cost_layers
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE stock_balances FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory_cost_layers FORCE ROW LEVEL SECURITY;
