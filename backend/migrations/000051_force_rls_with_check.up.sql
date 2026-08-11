-- B-06: Add FORCE ROW LEVEL SECURITY and WITH CHECK to 6 tables.
-- 
-- The following tables had RLS ENABLEd but not FORCEd, and policies had USING only
-- (no WITH CHECK). This makes RLS mandatory for all users and enforces tenant_id
-- validation on INSERT/UPDATE as well as SELECT.
--
-- Tables affected: cheques, cost_centers, cost_center_allocations, budget_variance_reports,
-- email_templates, email_queue

-- Drop existing policies (without WITH CHECK) and recreate with WITH CHECK

-- cheques
DROP POLICY IF EXISTS cheques_tenant_isolation ON cheques;
CREATE POLICY cheques_tenant_isolation ON cheques
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE cheques FORCE ROW LEVEL SECURITY;
ALTER TABLE cheques ENABLE ROW LEVEL SECURITY;

-- cost_centers
DROP POLICY IF EXISTS cost_centers_tenant_isolation ON cost_centers;
CREATE POLICY cost_centers_tenant_isolation ON cost_centers
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE cost_centers FORCE ROW LEVEL SECURITY;
ALTER TABLE cost_centers ENABLE ROW LEVEL SECURITY;

-- cost_center_allocations
DROP POLICY IF EXISTS cost_center_allocations_tenant_isolation ON cost_center_allocations;
CREATE POLICY cost_center_allocations_tenant_isolation ON cost_center_allocations
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE cost_center_allocations FORCE ROW LEVEL SECURITY;
ALTER TABLE cost_center_allocations ENABLE ROW LEVEL SECURITY;

-- budget_variance_reports
DROP POLICY IF EXISTS budget_variance_reports_tenant_isolation ON budget_variance_reports;
CREATE POLICY budget_variance_reports_tenant_isolation ON budget_variance_reports
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE budget_variance_reports FORCE ROW LEVEL SECURITY;
ALTER TABLE budget_variance_reports ENABLE ROW LEVEL SECURITY;

-- email_templates
DROP POLICY IF EXISTS email_templates_tenant_isolation ON email_templates;
CREATE POLICY email_templates_tenant_isolation ON email_templates
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE email_templates FORCE ROW LEVEL SECURITY;
ALTER TABLE email_templates ENABLE ROW LEVEL SECURITY;

-- email_queue
DROP POLICY IF EXISTS email_queue_tenant_isolation ON email_queue;
CREATE POLICY email_queue_tenant_isolation ON email_queue
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE email_queue FORCE ROW LEVEL SECURITY;
ALTER TABLE email_queue ENABLE ROW LEVEL SECURITY;
