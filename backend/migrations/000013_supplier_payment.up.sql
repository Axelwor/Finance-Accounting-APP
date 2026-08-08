-- US-034: Bayar Supplier (Supplier Payment).
-- Payment posts a journal: Dr 2101 Accounts Payable / Cr Cash-Bank.
-- Overpayment (amount > payable): Dr 2101 (payable) + Dr 1204 (excess) / Cr Cash-Bank (total).
-- Overpayment lands in 1204 Other Receivables (NOT 2402 — that's for customers only).

-- Seed 1204 Other Receivables for existing tenants (seed.go adds it for new ones).
-- Kept idempotent so re-running the migration is safe.
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '1204', 'Other Receivables', 'asset', 'OTHER_RECEIVABLE'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '1204'
);

CREATE TABLE supplier_payments (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,                       -- PAY-{YYYY}-{seq}
    supplier_id BIGINT NOT NULL,
    invoice_id BIGINT NOT NULL,
    journal_entry_id BIGINT,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    ap_applied_cents BIGINT NOT NULL DEFAULT 0,
    overpayment_cents BIGINT NOT NULL DEFAULT 0,
    cash_account_id BIGINT NOT NULL,
    payment_date DATE NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'PAID' CHECK (status IN ('PAID', 'REVERSED')),
    idempotency_key UUID,
    source_ref TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES supplier_invoices(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cash_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

-- Idempotency key is only unique when set; NULLs are allowed for multiple rows.
CREATE UNIQUE INDEX supplier_payments_idem ON supplier_payments (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE INDEX supplier_payments_invoice_idx ON supplier_payments (tenant_id, invoice_id);

ALTER TABLE supplier_payments ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_supplier_payments ON supplier_payments
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE supplier_payments FORCE ROW LEVEL SECURITY;
