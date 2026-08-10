-- Migration 000038: Remove default report templates seed
DELETE FROM tenants WHERE id = 0 AND slug = 'global-defaults';
DELETE FROM report_templates WHERE tenant_id = 0;
