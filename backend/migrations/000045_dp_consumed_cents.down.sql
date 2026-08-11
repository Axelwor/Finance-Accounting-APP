-- Down migration for 000045 (column can be dropped, data is derivable from SO - DPs - invoices).
ALTER TABLE sales_orders DROP COLUMN IF EXISTS dp_consumed_cents;
