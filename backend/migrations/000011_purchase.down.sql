DROP TABLE IF EXISTS grn_lines;
DROP TABLE IF EXISTS goods_received_notes;
DROP TABLE IF EXISTS purchase_orders_lines;
DROP TABLE IF EXISTS purchase_orders;
DROP TABLE IF EXISTS suppliers;
DELETE FROM accounts WHERE code IN ('1205', '2105', '1203')
  AND account_type IN ('ADVANCE_TO_SUPPLIER', 'ACCRUED_LIABILITY', 'TAX_RECEIVABLE');
