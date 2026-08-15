-- F-13: Asset maintenance log. Records maintenance activities per fixed asset
-- (cost, date, type, notes). Read-only reporting + write endpoint; maintenance
-- cost is expensed (not capitalized) so no journal is auto-posted here — the
-- caller posts via cash-out/journal if desired.

CREATE TABLE IF NOT EXISTS asset_maintenance (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    asset_id BIGINT NOT NULL,
    maintenance_date DATE NOT NULL,
    maintenance_type TEXT NOT NULL CHECK (maintenance_type IN ('ROUTINE','REPAIR','INSPECTION','OVERHAUL','OTHER')),
    description TEXT NOT NULL DEFAULT '',
    cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (cost_cents >= 0),
    performed_by TEXT,
    next_due_date DATE,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, asset_id) REFERENCES fixed_assets(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS asset_maintenance_asset_idx ON asset_maintenance (tenant_id, asset_id, maintenance_date);
CREATE INDEX IF NOT EXISTS asset_maintenance_due_idx ON asset_maintenance (tenant_id, next_due_date);

ALTER TABLE asset_maintenance ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_asset_maintenance ON asset_maintenance;
CREATE POLICY tenant_isolation_asset_maintenance ON asset_maintenance
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE asset_maintenance FORCE ROW LEVEL SECURITY;
