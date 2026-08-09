-- US-070..072: Production / Job Order Costing.
--
-- Production jobs accumulate material/labor/overhead costs into Work in
-- Progress (WIP, 1303) as costs are added, then move the accumulated cost
-- into Finished Goods (1304) on completion. Any difference between the
-- accumulated WIP cost and the (optionally) expected cost is booked as a
-- production variance:
--   loss (variance > 0): Dr 5901 Production Variance Loss / Cr 1303 WIP
--   gain (variance < 0): Dr 1303 WIP / Cr 4902 Production Variance Gain
--
-- New accounts seeded for every existing tenant:
--   1303 Work in Progress     (asset / INVENTORY)
--   1304 Finished Goods       (asset / INVENTORY)
--   4902 Production Variance Gain (revenue / OTHER_INCOME)
--   5901 Production Variance Loss (expense / OTHER_EXPENSE)

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, code, name, report_group, account_type
FROM tenants t
CROSS JOIN (VALUES
  ('1303', 'Work in Progress', 'asset', 'INVENTORY'),
  ('1304', 'Finished Goods', 'asset', 'INVENTORY'),
  ('4902', 'Production Variance Gain', 'revenue', 'OTHER_INCOME'),
  ('5901', 'Production Variance Loss', 'expense', 'OTHER_EXPENSE')
) AS v(code, name, report_group, account_type)
WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code);

-- Bill of Materials (BOM): the recipe for one finished good.
-- Each BOM points at a finished-good item and an output quantity, and owns
-- a set of bom_lines (material / labor / overhead) consumed to produce it.
CREATE TABLE bill_of_materials (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    finished_good_item_id BIGINT NOT NULL,
    output_qty NUMERIC(18,3) NOT NULL CHECK (output_qty > 0),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','VOID')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, finished_good_item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE bom_lines (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    bom_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    line_no INT NOT NULL DEFAULT 1,
    qty NUMERIC(18,3) NOT NULL CHECK (qty > 0),
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    cost_type TEXT NOT NULL DEFAULT 'material' CHECK (cost_type IN ('material','labor','overhead')),
    description TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, bom_id) REFERENCES bill_of_materials(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT
);

-- Production job (job order): one production run for a finished good.
-- Costs accumulate in total_*_cents as material/labor/overhead are added;
-- on completion the accumulated total_cost_cents moves to Finished Goods
-- and any variance is booked against 4902 / 5901.
CREATE TABLE production_jobs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    bom_id BIGINT,
    finished_good_item_id BIGINT NOT NULL,
    target_qty NUMERIC(18,3) NOT NULL CHECK (target_qty > 0),
    completed_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    start_date DATE NOT NULL,
    completion_date DATE,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','IN_PROGRESS','COMPLETED','CANCELLED')),
    wip_account_id BIGINT NOT NULL,
    finished_good_account_id BIGINT NOT NULL,
    total_material_cents BIGINT NOT NULL DEFAULT 0,
    total_labor_cents BIGINT NOT NULL DEFAULT 0,
    total_overhead_cents BIGINT NOT NULL DEFAULT 0,
    total_cost_cents BIGINT NOT NULL DEFAULT 0,
    variance_cents BIGINT NOT NULL DEFAULT 0,
    journal_entry_id BIGINT,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, bom_id) REFERENCES bill_of_materials(tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, finished_good_item_id) REFERENCES items(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, wip_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, finished_good_account_id) REFERENCES accounts(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

-- Production job cost lines: one row per material/labor/overhead addition.
-- Each cost line is posted through its own journal entry (PRODUCTION_COST)
-- so the WIP accumulation is auditable line-by-line.
CREATE TABLE production_job_costs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    job_id BIGINT NOT NULL,
    line_no INT NOT NULL DEFAULT 1,
    cost_type TEXT NOT NULL CHECK (cost_type IN ('material','labor','overhead')),
    item_id BIGINT,
    description TEXT,
    qty NUMERIC(18,3),
    unit_cost_cents BIGINT NOT NULL DEFAULT 0,
    total_cents BIGINT NOT NULL CHECK (total_cents > 0),
    journal_entry_id BIGINT,
    posted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, job_id) REFERENCES production_jobs(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, item_id) REFERENCES items(tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX bill_of_materials_tenant_idx ON bill_of_materials (tenant_id, code);
CREATE INDEX bom_lines_bom_idx ON bom_lines (tenant_id, bom_id, line_no);
CREATE INDEX production_jobs_tenant_idx ON production_jobs (tenant_id, start_date, number);
CREATE INDEX production_job_costs_job_idx ON production_job_costs (tenant_id, job_id, line_no);

ALTER TABLE bill_of_materials ENABLE ROW LEVEL SECURITY;
ALTER TABLE bom_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE production_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE production_job_costs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_bill_of_materials ON bill_of_materials
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_bom_lines ON bom_lines
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_production_jobs ON production_jobs
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_production_job_costs ON production_job_costs
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE bill_of_materials FORCE ROW LEVEL SECURITY;
ALTER TABLE bom_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE production_jobs FORCE ROW LEVEL SECURITY;
ALTER TABLE production_job_costs FORCE ROW LEVEL SECURITY;
