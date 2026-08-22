-- Restore the original blanket policy on report_templates.

DROP POLICY IF EXISTS report_templates_select ON report_templates;
DROP POLICY IF EXISTS report_templates_insert ON report_templates;
DROP POLICY IF EXISTS report_templates_update ON report_templates;
DROP POLICY IF EXISTS report_templates_delete ON report_templates;

CREATE POLICY report_templates_tenant ON report_templates
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);
