-- Align dashboard_widgets with the dashboard handler's schema:
-- the handler queries user_id / position / col_span / row_span, but
-- 000030 created the table with grid_x/grid_y/grid_w/grid_h/sort_order and
-- no per-widget user_id (owner is implicit via dashboard_layouts).
--
-- Mapping applied to existing rows:
--   user_id  <- dashboard_layouts.user_id
--   position <- sort_order
--   col_span <- grid_w
--   row_span <- grid_h

ALTER TABLE dashboard_widgets
    ADD COLUMN IF NOT EXISTS user_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS col_span INT NOT NULL DEFAULT 4,
    ADD COLUMN IF NOT EXISTS row_span INT NOT NULL DEFAULT 3;

UPDATE dashboard_widgets w
SET user_id = l.user_id,
    position = w.sort_order,
    col_span = w.grid_w,
    row_span = w.grid_h
FROM dashboard_layouts l
WHERE w.layout_id = l.id AND w.user_id = 0;

CREATE INDEX IF NOT EXISTS idx_dashboard_widgets_tenant_user
    ON dashboard_widgets (tenant_id, user_id);

-- Backfill the default widget set for tenants that only have layouts
-- (no widgets). The handler seeds defaults lazily too; this covers rows
-- created before the seed existed.
INSERT INTO dashboard_widgets (tenant_id, user_id, layout_id, widget_type, title, position, col_span, row_span)
SELECT l.tenant_id, l.user_id, l.id, 'kpi_cash', 'Cash Balance', 0, 3, 1
FROM dashboard_layouts l
WHERE NOT EXISTS (
    SELECT 1 FROM dashboard_widgets w WHERE w.layout_id = l.id
);
