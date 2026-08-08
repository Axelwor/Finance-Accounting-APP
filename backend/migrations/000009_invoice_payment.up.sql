-- P2 Phase 2E: Invoice payment (Pelunasan).
-- Payment posts a journal: Dr Cash/Bank / Cr 1201 AR.
-- Overpayment: Dr Cash / Cr AR (receivable) + Cr 2402 Customer Deposit (excess).

-- Seed 2402 Customer Overpayment for existing tenants (seed.go adds it for new ones).
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '2402', 'Customer Overpayment', 'liability', 'CUSTOMER_DEPOSIT'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '2402'
);

CREATE TABLE invoice_payments (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    invoice_id BIGINT NOT NULL,
    customer_id BIGINT NOT NULL,
    journal_entry_id BIGINT,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    ar_applied_cents BIGINT NOT NULL DEFAULT 0,
    overpayment_cents BIGINT NOT NULL DEFAULT 0,
    cash_account_id BIGINT NOT NULL,
    payment_date DATE NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'RECEIVED' CHECK (status IN ('RECEIVED', 'REVERSED')),
    idempotency_key UUID,
    source_ref TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cash_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX invoice_payments_idem ON invoice_payments (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE INDEX invoice_payments_invoice_idx ON invoice_payments (tenant_id, invoice_id);

ALTER TABLE invoice_payments ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_invoice_payments ON invoice_payments
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE invoice_payments FORCE ROW LEVEL SECURITY;
