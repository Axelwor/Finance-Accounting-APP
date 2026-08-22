-- Phase C / F-01 companion: global report templates must survive RLS.
--
-- report_templates stores BOTH per-tenant templates and GLOBAL ones
-- (tenant_id = 0, seeded system defaults). The original policy was a strict
-- equality on the tenant GUC, which makes every tenant_id = 0 row invisible
-- to the restricted application role — default invoice/report templates
-- would disappear after the cutover.
--
-- Command-specific policies replace the single blanket policy:
--   SELECT : own rows + global rows (read-only consumption of defaults)
--   INSERT : own rows only (app cannot forge global rows)
--   UPDATE : own rows only (a global row is not even USING-visible)
--   DELETE : own rows only (global templates cannot be deleted by the app)

DROP POLICY IF EXISTS report_templates_tenant ON report_templates;

CREATE POLICY report_templates_select ON report_templates
    FOR SELECT
    USING (
        tenant_id = current_setting('app.tenant_id', true)::BIGINT
        OR tenant_id = 0
    );

CREATE POLICY report_templates_insert ON report_templates
    FOR INSERT
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

CREATE POLICY report_templates_update ON report_templates
    FOR UPDATE
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

CREATE POLICY report_templates_delete ON report_templates
    FOR DELETE
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
