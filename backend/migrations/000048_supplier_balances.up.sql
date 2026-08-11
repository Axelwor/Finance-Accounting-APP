-- A-20: AP sub-ledger — materialized per-supplier outstanding balance.
-- Mirrors customer_balances (migration 000036) but for the purchase cycle.
-- The per-invoice payable_cents already exists on supplier_invoices, but
-- there was no per-supplier aggregate, so aging/statement/sub-ledger-vs-GL
-- reconciliation was impossible.

CREATE TABLE IF NOT EXISTS supplier_balances (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    supplier_id BIGINT NOT NULL,
    ap_cents BIGINT NOT NULL DEFAULT 0,
    overpayment_cents BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, supplier_id),
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS supplier_balances_tenant_ap_idx
    ON supplier_balances (tenant_id, ap_cents);

ALTER TABLE supplier_balances ENABLE ROW LEVEL SECURITY;
CREATE POLICY supplier_balances_tenant_isolation ON supplier_balances
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE supplier_balances FORCE ROW LEVEL SECURITY;

-- Backfill from existing supplier invoices. The payable_cents on each
-- invoice already reflects purchase returns (which increase it) and
-- supplier payments (which decrease it). Using DO NOTHING so re-runs
-- never clobber balances that the Go code has incrementally adjusted.
INSERT INTO supplier_balances (tenant_id, supplier_id, ap_cents, overpayment_cents, updated_at)
SELECT tenant_id, supplier_id, COALESCE(SUM(payable_cents), 0), 0, now()
FROM supplier_invoices
WHERE status IN ('ISSUED', 'PARTIALLY_PAID')
GROUP BY tenant_id, supplier_id
ON CONFLICT (tenant_id, supplier_id)
DO NOTHING;

-- Seed overpayment_cents from supplier payments. Only updates rows
-- whose overpayment_cents is still 0 (i.e., not yet touched by the Go
-- code), so re-runs are safe.
INSERT INTO supplier_balances (tenant_id, supplier_id, ap_cents, overpayment_cents, updated_at)
SELECT tenant_id, supplier_id, 0, COALESCE(SUM(overpayment_cents), 0), now()
FROM supplier_payments
WHERE overpayment_cents > 0
GROUP BY tenant_id, supplier_id
ON CONFLICT (tenant_id, supplier_id)
DO UPDATE SET overpayment_cents = EXCLUDED.overpayment_cents, updated_at = now()
WHERE supplier_balances.overpayment_cents = 0;
