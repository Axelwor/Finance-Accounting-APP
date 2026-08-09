-- US-050: Bank Reconciliation (Rekonsiliasi Kas & Bank).
-- Bank statements are imported line-by-line (CSV parsed client-side), then
-- reconciled against the recorded cash/bank journal lines. A reconciliation
-- session auto-matches statement lines to journal lines by amount + date
-- proximity, allows manual match/unmatch, and finalises when the adjusted
-- book balance equals the adjusted statement balance (diff = 0).

CREATE TABLE bank_statements (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    bank_account_id BIGINT NOT NULL,
    statement_date DATE NOT NULL,
    opening_balance_cents BIGINT NOT NULL DEFAULT 0,
    closing_balance_cents BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'IMPORTED' CHECK (status IN ('IMPORTED','RECONCILING','RECONCILED','VOID')),
    notes TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, bank_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE bank_statement_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    statement_id BIGINT NOT NULL,
    line_no INT NOT NULL DEFAULT 1,
    tx_date DATE NOT NULL,
    description TEXT,
    reference TEXT,
    amount_cents BIGINT NOT NULL,
    matched_journal_line_id BIGINT,
    match_status TEXT NOT NULL DEFAULT 'UNMATCHED' CHECK (match_status IN ('UNMATCHED','MATCHED','MANUAL','ADJUSTMENT')),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, statement_id) REFERENCES bank_statements(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE bank_reconciliations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    statement_id BIGINT NOT NULL,
    bank_account_id BIGINT NOT NULL,
    recon_date DATE NOT NULL,
    book_balance_cents BIGINT NOT NULL DEFAULT 0,
    statement_balance_cents BIGINT NOT NULL DEFAULT 0,
    adjusted_book_cents BIGINT NOT NULL DEFAULT 0,
    adjusted_statement_cents BIGINT NOT NULL DEFAULT 0,
    diff_cents BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','RECONCILED','VOID')),
    notes TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, statement_id) REFERENCES bank_statements(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, bank_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX bank_statements_tenant_date_idx ON bank_statements (tenant_id, statement_date);
CREATE INDEX bank_statements_tenant_account_idx ON bank_statements (tenant_id, bank_account_id);
CREATE INDEX bank_statement_lines_statement_idx ON bank_statement_lines (tenant_id, statement_id);
CREATE INDEX bank_statement_lines_match_idx ON bank_statement_lines (tenant_id, match_status);
CREATE INDEX bank_reconciliations_statement_idx ON bank_reconciliations (tenant_id, statement_id);
CREATE INDEX bank_reconciliations_tenant_date_idx ON bank_reconciliations (tenant_id, recon_date);

ALTER TABLE bank_statements ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_statement_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_reconciliations ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_bank_statements ON bank_statements
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_bank_statement_lines ON bank_statement_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_bank_reconciliations ON bank_reconciliations
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE bank_statements FORCE ROW LEVEL SECURITY;
ALTER TABLE bank_statement_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE bank_reconciliations FORCE ROW LEVEL SECURITY;
