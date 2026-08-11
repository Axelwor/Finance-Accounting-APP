-- F-02: Multi-warehouse support for stock_balances and stock_transfers.
-- The old unique constraint on (tenant_id, item_id) prevents having multiple
-- rows per item per warehouse. We need (tenant_id, item_id, warehouse_id).

-- Step 1: Backfill existing NULL warehouse_id to 0 (unknown/unspecified)
-- so the new constraint can be created without NULL ambiguity.
UPDATE stock_balances SET warehouse_id = 0 WHERE warehouse_id IS NULL;

-- Step 2: Make warehouse_id NOT NULL with default 0 and replace the unique
-- constraint to include warehouse_id.
ALTER TABLE stock_balances ALTER COLUMN warehouse_id SET DEFAULT 0;
ALTER TABLE stock_balances ALTER COLUMN warehouse_id SET NOT NULL;
ALTER TABLE stock_balances DROP CONSTRAINT IF EXISTS stock_balances_tenant_id_item_id_key;
ALTER TABLE stock_balances ADD CONSTRAINT stock_balances_tenant_item_warehouse_unique
    UNIQUE (tenant_id, item_id, warehouse_id);

-- Step 3: Add warehouse_id to inventory_cost_layers for FIFO per-warehouse.
ALTER TABLE inventory_cost_layers ADD COLUMN IF NOT EXISTS warehouse_id BIGINT DEFAULT 0;
UPDATE inventory_cost_layers SET warehouse_id = 0 WHERE warehouse_id IS NULL;
ALTER TABLE inventory_cost_layers ALTER COLUMN warehouse_id SET NOT NULL;

-- Step 4: Add from/to warehouse columns to stock_transfers.
ALTER TABLE stock_transfers ADD COLUMN IF NOT EXISTS from_warehouse_id BIGINT;
ALTER TABLE stock_transfers ADD COLUMN IF NOT EXISTS to_warehouse_id BIGINT;

-- Step 4a: Add warehouse_id to inventory_movements for per-movement warehouse tracking.
ALTER TABLE inventory_movements ADD COLUMN IF NOT EXISTS warehouse_id BIGINT;

-- Step 4b: Add warehouse_id to stock_transfer_lines for per-line warehouse tracking.
ALTER TABLE stock_transfer_lines ADD COLUMN IF NOT EXISTS warehouse_id BIGINT;

-- Step 5: Create indexes for warehouse queries.
CREATE INDEX IF NOT EXISTS stock_balances_warehouse_idx ON stock_balances (tenant_id, warehouse_id);
CREATE INDEX IF NOT EXISTS stock_transfers_warehouse_idx ON stock_transfers (tenant_id, from_warehouse_id, to_warehouse_id);
CREATE INDEX IF NOT EXISTS inventory_cost_layers_warehouse_idx ON inventory_cost_layers (tenant_id, warehouse_id);
