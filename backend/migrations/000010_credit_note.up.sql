-- P2 Phase 2F: Credit Note (CN / Sales Return).
-- CN posts two journal pairs:
-- 1. Revenue reversal: Dr 4201 Retur Penjualan / Cr 1201 AR (or Cash)
-- 2. COGS reversal: Dr 1301 Inventory / Cr 5101 COGS (stock returns)

-- Seed 4201 Sales Returns for existing tenants (seed.go adds it for new ones).
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '4201', 'Sales Returns', 'revenue', 'CONTRA_REVENUE'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '4201'
);

CREATE TABLE credit_notes (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    invoice_id BIGINT NOT NULL,
    customer_id BIGINT NOT NULL,
    cn_date DATE NOT NULL,
    refund_method TEXT NOT NULL DEFAULT 'deduct' CHECK (refund_method IN ('deduct', 'refund', 'credit_balance')),
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'APPLIED' CHECK (status IN ('DRAFT', 'APPLIED', 'VOID')),
    total_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    ar_deducted_cents BIGINT NOT NULL DEFAULT 0,
    cogs_reversed_cents BIGINT NOT NULL DEFAULT 0,
    revenue_journal_entry_id BIGINT,
    cogs_journal_entry_id BIGINT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, revenue_journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cogs_journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE credit_note_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    credit_note_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    invoice_line_id BIGINT,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    line_total_cents BIGINT NOT NULL DEFAULT 0,
    cogs_reversed_cents BIGINT NOT NULL DEFAULT 0,
    inventory_account_id BIGINT NOT NULL,
    cogs_account_id BIGINT NOT NULL,
    revenue_account_id BIGINT,
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, credit_note_id) REFERENCES credit_notes(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, invoice_line_id) REFERENCES invoice_lines(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, inventory_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cogs_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, revenue_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX credit_notes_tenant_date_idx ON credit_notes (tenant_id, cn_date);
CREATE INDEX credit_note_lines_cn_idx ON credit_note_lines (tenant_id, credit_note_id);

ALTER TABLE credit_notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE credit_note_lines ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_credit_notes ON credit_notes
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_credit_note_lines ON credit_note_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE credit_notes FORCE ROW LEVEL SECURITY;
ALTER TABLE credit_note_lines FORCE ROW LEVEL SECURITY;
