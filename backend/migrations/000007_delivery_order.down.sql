DROP TABLE IF EXISTS inventory_movements;
DROP TABLE IF EXISTS delivery_orders_lines;
DROP TABLE IF EXISTS delivery_orders;
ALTER TABLE sales_orders_lines DROP COLUMN IF EXISTS delivered_qty;
