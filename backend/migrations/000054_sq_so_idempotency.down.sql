-- 000054 down: remove idempotency keys from quotations and sales orders.

DROP INDEX IF EXISTS sales_orders_idem;
DROP INDEX IF EXISTS sales_quotations_idem;

ALTER TABLE sales_orders
    DROP COLUMN IF EXISTS idempotency_key;

ALTER TABLE sales_quotations
    DROP COLUMN IF EXISTS idempotency_key;
