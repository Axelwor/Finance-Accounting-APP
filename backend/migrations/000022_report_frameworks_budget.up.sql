-- US-090A + US-093: Report framework selection (EMKM/ETAP/SAK Umum) and
-- dimensions + budget vs actual reporting.
--
-- Tables added:
--   report_frameworks      — per-tenant selected reporting framework (same data,
--                            different presentation).
--   dimensions             — cabang / proyek / departemen / cost center master.
--   journal_line_dimensions— many-to-many tags on posted journal lines.
--   budgets                — budget header (fiscal_year + optional dimension).
--   budget_lines           — monthly planned amount per account (+ dimension).
--
-- All tables are tenant-scoped with RLS (app.tenant_id set per transaction).

-- Report framework config per tenant. A tenant picks one framework; the same
-- posted journals are re-presented by the reporting layer.
CREATE TABLE report_frameworks (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    framework TEXT NOT NULL CHECK (framework IN ('EMKM','ETAP','SAK_UMUM')),
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, framework)
);

-- One default framework per tenant. NULLs the previous default on update.
CREATE UNIQUE INDEX report_frameworks_one_default
    ON report_frameworks (tenant_id)
    WHERE is_default = true;

-- Dimensions (cabang, proyek, departemen, cost center). Used to tag journal
-- lines and to scope budgets.
CREATE TABLE dimensions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    dimension_type TEXT NOT NULL CHECK (dimension_type IN ('branch','project','department','cost_center')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code)
);

CREATE INDEX dimensions_tenant_type_idx ON dimensions (tenant_id, dimension_type, is_active);

-- Dimension tags on journal lines (many-to-many). A line can carry multiple
-- dimensions (e.g. a branch + a project). The composite FK keeps tags within
-- the same tenant as their line / dimension.
CREATE TABLE journal_line_dimensions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    journal_line_id BIGINT NOT NULL,
    dimension_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, journal_line_id, dimension_id),
    FOREIGN KEY (tenant_id, journal_line_id) REFERENCES journal_lines(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, dimension_id) REFERENCES dimensions(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX journal_line_dimensions_line_idx
    ON journal_line_dimensions (tenant_id, journal_line_id);
CREATE INDEX journal_line_dimensions_dim_idx
    ON journal_line_dimensions (tenant_id, dimension_id);

-- Budgets: planned amounts per account / month for a fiscal year. A budget
-- may be scoped to a dimension (e.g. a branch budget) or cover the whole
-- tenant when dimension_id is NULL.
CREATE TABLE budgets (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    fiscal_year INT NOT NULL,
    dimension_id BIGINT,
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','APPROVED','CLOSED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, name, fiscal_year),
    FOREIGN KEY (tenant_id, dimension_id) REFERENCES dimensions(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX budgets_tenant_year_idx ON budgets (tenant_id, fiscal_year, status);

-- Monthly planned amount per account. month is 1..12. dimension_id is
-- optional and, when set, must match the budget header dimension (enforced in
-- application code) — kept here to support finer-grained line-level scoping.
CREATE TABLE budget_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    budget_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    dimension_id BIGINT,
    month INT NOT NULL CHECK (month >= 1 AND month <= 12),
    amount_cents BIGINT NOT NULL DEFAULT 0,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, budget_id) REFERENCES budgets(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, dimension_id) REFERENCES dimensions(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX budget_lines_budget_idx ON budget_lines (tenant_id, budget_id, month);
CREATE INDEX budget_lines_account_idx ON budget_lines (tenant_id, account_id, month);

-- Row Level Security: every table is tenant-scoped. The application role sets
-- app.tenant_id at the start of each transaction (see scopeTenant helpers).
ALTER TABLE report_frameworks ENABLE ROW LEVEL SECURITY;
ALTER TABLE dimensions ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal_line_dimensions ENABLE ROW LEVEL SECURITY;
ALTER TABLE budgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE budget_lines ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_report_frameworks ON report_frameworks
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_dimensions ON dimensions
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_journal_line_dimensions ON journal_line_dimensions
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_budgets ON budgets
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_budget_lines ON budget_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE report_frameworks FORCE ROW LEVEL SECURITY;
ALTER TABLE dimensions FORCE ROW LEVEL SECURITY;
ALTER TABLE journal_line_dimensions FORCE ROW LEVEL SECURITY;
ALTER TABLE budgets FORCE ROW LEVEL SECURITY;
ALTER TABLE budget_lines FORCE ROW LEVEL SECURITY;
