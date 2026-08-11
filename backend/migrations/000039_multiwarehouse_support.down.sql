-- Rollback 000039
DROP INDEX IF EXISTS stock_transfers_warehouse_idx;
DROP INDEX IF EXISTS stock_balances_warehouse_idx;
ALTER TABLE inventory_movements DROP COLUMN IF EXISTS warehouse_id;
ALTER TABLE stock_transfer_lines DROP COLUMN IF EXISTS warehouse_id;
ALTER TABLE stock_transfers DROP COLUMN IF EXISTS to_warehouse_id;
ALTER TABLE stock_transfers DROP COLUMN IF EXISTS from_warehouse_id;
ALTER TABLE stock_balances DROP CONSTRAINT IF EXISTS stock_balances_tenant_item_warehouse_unique;
ALTER TABLE stock_balances ADD CONSTRAINT stock_balances_tenant_id_item_id_key UNIQUE (tenant_id, item_id);
ALTER TABLE stock_balances ALTER COLUMN warehouse_id DROP NOT NULL;
ALTER TABLE stock_balances ALTER COLUMN warehouse_id DROP DEFAULT;
CREATE INDEX IF NOT EXISTS inventory_cost_layers_warehouse_idx ON inventory_cost_layers (tenant_id, warehouse_id);
