-- M-007: AR sub-ledger — materialized per-customer outstanding balance.
-- The per-invoice receivable_cents already exists on invoices, but there was
-- no per-customer aggregate, so aging/statement/ECL had to re-derive it from
-- the GL every time and GL-vs-sub-ledger reconciliation was impossible.

CREATE TABLE IF NOT EXISTS customer_balances (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    customer_id BIGINT NOT NULL,
    ar_cents BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, customer_id),
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS customer_balances_tenant_ar_idx
    ON customer_balances (tenant_id, ar_cents);

ALTER TABLE customer_balances ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS customer_balances_tenant_isolation ON customer_balances;
CREATE POLICY customer_balances_tenant_isolation ON customer_balances
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE customer_balances FORCE ROW LEVEL SECURITY;

-- Backfill from existing invoices.
INSERT INTO customer_balances (tenant_id, customer_id, ar_cents, updated_at)
SELECT tenant_id, customer_id, COALESCE(SUM(receivable_cents), 0), now()
FROM invoices
WHERE status IN ('ISSUED', 'PARTIALLY_PAID')
GROUP BY tenant_id, customer_id
ON CONFLICT (tenant_id, customer_id)
DO UPDATE SET ar_cents = EXCLUDED.ar_cents, updated_at = now();
