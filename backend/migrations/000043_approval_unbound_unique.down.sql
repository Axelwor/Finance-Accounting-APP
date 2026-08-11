-- Rollback 000043
DROP INDEX IF EXISTS approval_requests_unbound_active_unique;
DROP INDEX IF EXISTS approval_requests_bound_active_unique;
CREATE UNIQUE INDEX IF NOT EXISTS approval_requests_active_unique
    ON approval_requests (tenant_id, entity_type, entity_id)
    WHERE status IN ('PENDING', 'APPROVED') AND consumed_at IS NULL;
