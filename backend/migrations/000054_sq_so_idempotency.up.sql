-- 000054: Idempotency keys for quotations and sales orders (double-submit
-- protection) — the two sales-cycle documents that post no journal and thus
-- cannot reuse journal_entries.idempotency_key.

ALTER TABLE sales_quotations
    ADD COLUMN IF NOT EXISTS idempotency_key UUID;

ALTER TABLE sales_orders
    ADD COLUMN IF NOT EXISTS idempotency_key UUID;

-- A tenant cannot submit the same idempotency key twice; NULL keys (older
-- clients or programmatic imports) are unrestricted.
CREATE UNIQUE INDEX IF NOT EXISTS sales_quotations_idem
    ON sales_quotations (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS sales_orders_idem
    ON sales_orders (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
