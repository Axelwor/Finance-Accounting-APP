DROP TABLE IF EXISTS sales_down_payments;
DROP TABLE IF EXISTS sales_orders_lines;
DROP TABLE IF EXISTS sales_orders;
DELETE FROM accounts WHERE code = '2201' AND account_type = 'CUSTOMER_DEPOSIT';
