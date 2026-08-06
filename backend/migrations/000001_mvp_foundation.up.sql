CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE tenants (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    fiscal_year_start DATE,
    period_type TEXT NOT NULL DEFAULT 'monthly' CHECK (period_type IN ('monthly', 'quarterly', 'yearly')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    full_name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_tenants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    role TEXT NOT NULL CHECK (role IN ('owner', 'accountant', 'admin', 'staff', 'consultant')),
    UNIQUE (user_id, tenant_id)
);

CREATE TABLE accounting_periods (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('FUTURE', 'OPEN', 'CLOSED', 'REOPENED')),
    closed_at TIMESTAMPTZ,
    closed_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    unlock_requested BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, period_start, period_end),
    CHECK (period_end >= period_start)
);

CREATE UNIQUE INDEX accounting_periods_one_open_per_tenant
    ON accounting_periods (tenant_id)
    WHERE status = 'OPEN';

ALTER TABLE accounting_periods
    ADD CONSTRAINT accounting_periods_no_overlap
    EXCLUDE USING gist (
        tenant_id WITH =,
        daterange(period_start, period_end, '[]') WITH &&
    );

CREATE TABLE accounts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    report_group TEXT NOT NULL CHECK (report_group IN ('asset', 'liability', 'equity', 'revenue', 'expense')),
    account_type TEXT NOT NULL,
    parent_id BIGINT,
    is_group BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    valid_from DATE,
    valid_to DATE,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, parent_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE report_mappings (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    account_id BIGINT NOT NULL,
    report_type TEXT NOT NULL CHECK (report_type IN ('balance_sheet', 'profit_loss', 'cash_flow')),
    report_line TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 100,
    UNIQUE (tenant_id, account_id, report_type),
    FOREIGN KEY (tenant_id, account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE categories (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('IN', 'OUT')),
    default_debit_account_id BIGINT,
    default_credit_account_id BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE (tenant_id, name, direction),
    FOREIGN KEY (tenant_id, default_debit_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, default_credit_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE journal_entries (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    entry_date DATE NOT NULL,
    period_id BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'POSTED' CHECK (status IN ('POSTED', 'VOID')),
    description TEXT,
    source_ref TEXT,
    intent_type TEXT,
    idempotency_key UUID,
    reversal_of_id BIGINT REFERENCES journal_entries(id) ON DELETE RESTRICT,
    void_reason TEXT,
    voided_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    voided_at TIMESTAMPTZ,
    hash TEXT NOT NULL,
    prev_hash TEXT NOT NULL,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, period_id) REFERENCES accounting_periods(tenant_id, id) ON DELETE RESTRICT,
    CHECK ((source_ref IS NOT NULL AND intent_type IS NOT NULL) OR created_by IS NOT NULL),
    CHECK (status <> 'VOID' OR (void_reason IS NOT NULL AND voided_by IS NOT NULL AND voided_at IS NOT NULL)),
    CHECK (reversal_of_id IS NULL OR reversal_of_id <> id)
);

CREATE UNIQUE INDEX journal_entries_intent_unique
    ON journal_entries (tenant_id, source_ref, intent_type)
    WHERE source_ref IS NOT NULL AND intent_type IS NOT NULL;

CREATE UNIQUE INDEX journal_entries_idempotency_unique
    ON journal_entries (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX journal_entries_one_reversal
    ON journal_entries (tenant_id, reversal_of_id)
    WHERE reversal_of_id IS NOT NULL;

CREATE TABLE journal_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    entry_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    debit_cents BIGINT NOT NULL DEFAULT 0 CHECK (debit_cents >= 0),
    credit_cents BIGINT NOT NULL DEFAULT 0 CHECK (credit_cents >= 0),
    description TEXT,
    source_line_ref TEXT,
    dimension_ids JSONB NOT NULL DEFAULT '[]',
    CHECK (debit_cents = 0 OR credit_cents = 0),
    CHECK (debit_cents + credit_cents > 0),
    FOREIGN KEY (tenant_id, entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id BIGINT NOT NULL,
    before_json JSONB,
    after_json JSONB,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    topic TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'DISPATCHED', 'FAILED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    dispatched_at TIMESTAMPTZ
);

CREATE TABLE document_numbering (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    doc_type TEXT NOT NULL,
    prefix TEXT NOT NULL,
    last_seq BIGINT NOT NULL DEFAULT 0 CHECK (last_seq >= 0),
    fiscal_year INTEGER NOT NULL,
    UNIQUE (tenant_id, doc_type, prefix, fiscal_year)
);

CREATE INDEX journal_entries_tenant_date_idx ON journal_entries (tenant_id, entry_date);
CREATE INDEX journal_entries_tenant_status_idx ON journal_entries (tenant_id, status);
CREATE INDEX journal_entries_reversal_idx ON journal_entries (tenant_id, reversal_of_id);
CREATE INDEX journal_lines_account_idx ON journal_lines (tenant_id, account_id, entry_id);
CREATE INDEX audit_logs_entity_idx ON audit_logs (tenant_id, entity_type, entity_id);
CREATE INDEX outbox_events_status_idx ON outbox_events (status, created_at);

ALTER TABLE journal_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounting_periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_journal_entries ON journal_entries
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_journal_lines ON journal_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_accounts ON accounts
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_accounting_periods ON accounting_periods
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_categories ON categories
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_audit_logs ON audit_logs
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_outbox_events ON outbox_events
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

CREATE OR REPLACE FUNCTION reject_posted_journal_mutation() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'POSTED' AND NOT (
        TG_OP = 'UPDATE'
        AND NEW.status = 'VOID'
        AND NEW.reversal_of_id IS NULL
        AND NEW.void_reason IS NOT NULL
        AND NEW.voided_by IS NOT NULL
        AND NEW.voided_at IS NOT NULL
        AND current_setting('app.void_context', true) = '1'
    ) THEN
        RAISE EXCEPTION 'posted journal is immutable; use authorized reversal procedure';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER journal_entries_immutable
BEFORE UPDATE OR DELETE ON journal_entries
FOR EACH ROW EXECUTE FUNCTION reject_posted_journal_mutation();

CREATE OR REPLACE FUNCTION reject_posted_line_mutation() RETURNS trigger AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM journal_entries
        WHERE tenant_id = OLD.tenant_id AND id = OLD.entry_id AND status = 'POSTED'
    ) THEN
        RAISE EXCEPTION 'posted journal line is immutable; use reversal procedure';
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER journal_lines_immutable
BEFORE UPDATE OR DELETE ON journal_lines
FOR EACH ROW EXECUTE FUNCTION reject_posted_line_mutation();

CREATE OR REPLACE FUNCTION assert_journal_balanced() RETURNS trigger AS $$
DECLARE
    debit_total BIGINT;
    credit_total BIGINT;
    line_count BIGINT;
BEGIN
    SELECT COUNT(*), COALESCE(SUM(debit_cents), 0), COALESCE(SUM(credit_cents), 0)
      INTO line_count, debit_total, credit_total
      FROM journal_lines
     WHERE tenant_id = COALESCE(NEW.tenant_id, OLD.tenant_id)
       AND entry_id = COALESCE(NEW.entry_id, OLD.entry_id);
    IF line_count = 0 OR debit_total <> credit_total THEN
        RAISE EXCEPTION 'journal entry is not balanced';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER journal_lines_balance_deferred
AFTER INSERT OR UPDATE OR DELETE ON journal_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_journal_balanced();

CREATE OR REPLACE FUNCTION assert_entry_date_in_period() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM accounting_periods
         WHERE tenant_id = NEW.tenant_id
           AND id = NEW.period_id
           AND NEW.entry_date BETWEEN period_start AND period_end
           AND status IN ('OPEN', 'REOPENED')
    ) THEN
        RAISE EXCEPTION 'entry date is outside an open period';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER journal_entry_period_validation
BEFORE INSERT ON journal_entries
FOR EACH ROW EXECUTE FUNCTION assert_entry_date_in_period();

CREATE TABLE ledger_chain_heads (
    tenant_id BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE RESTRICT,
    last_journal_id BIGINT,
    last_hash TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE ledger_chain_heads
    ADD CONSTRAINT ledger_chain_heads_last_journal_fk
    FOREIGN KEY (tenant_id, last_journal_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT;

CREATE INDEX ledger_chain_heads_journal_idx ON ledger_chain_heads (tenant_id, last_journal_id);

ALTER TABLE journal_entries FORCE ROW LEVEL SECURITY;
ALTER TABLE journal_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;
ALTER TABLE accounting_periods FORCE ROW LEVEL SECURITY;
ALTER TABLE categories FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
ALTER TABLE outbox_events FORCE ROW LEVEL SECURITY;
-- Revoke direct mutation from the application role in deployment; the role name is environment-specific.
-- GRANT SELECT ON ...; GRANT EXECUTE ON posting procedures; REVOKE INSERT, UPDATE, DELETE ON journal_entries, journal_lines FROM app_user;
