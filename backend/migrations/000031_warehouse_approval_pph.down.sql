-- Rollback 000031
DELETE FROM accounts WHERE code IN ('2107', '2108', '2109', '2110', '2111', '5203');
DROP TABLE IF EXISTS pph_calculations;
DROP TABLE IF EXISTS approval_requests;
DROP TABLE IF EXISTS approval_workflows;
ALTER TABLE stock_balances DROP COLUMN IF EXISTS warehouse_id;
DROP TABLE IF EXISTS warehouses;
