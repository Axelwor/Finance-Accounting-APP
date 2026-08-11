-- Rollback 000042
DROP INDEX IF EXISTS approval_requests_active_unique;
ALTER TABLE approval_requests ADD CONSTRAINT approval_requests_tenant_id_entity_type_entity_id_key
    UNIQUE (tenant_id, entity_type, entity_id);
