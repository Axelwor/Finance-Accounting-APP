-- Reverse 000056: drop the columns added to align with the handler schema.
ALTER TABLE dashboard_widgets
    DROP COLUMN IF EXISTS user_id,
    DROP COLUMN IF EXISTS position,
    DROP COLUMN IF EXISTS col_span,
    DROP COLUMN IF EXISTS row_span;

DROP INDEX IF EXISTS idx_dashboard_widgets_tenant_user;
