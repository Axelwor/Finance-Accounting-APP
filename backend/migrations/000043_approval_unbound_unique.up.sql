-- F-03 (v2): Fix amount-based approval uniqueness properly.
--
-- Migration 000042's partial unique index on (tenant_id, entity_type,
-- entity_id) STILL blocks multiple concurrent amount-based approvals because
-- every unbound request uses entity_id = 0. Two legitimate amount-based
-- approvals (different amounts / entity_numbers) collided.
--
-- Fix:
--   * entity-bound active requests dedupe on (type, entity_id)
--   * amount-based (entity_id = 0) active requests dedupe on (type,
--     entity_number) so multiple distinct amount-based approvals coexist.
-- Consumed/rejected/cancelled requests never block new submissions.

DROP INDEX IF EXISTS approval_requests_active_unique;

CREATE UNIQUE INDEX IF NOT EXISTS approval_requests_bound_active_unique
    ON approval_requests (tenant_id, entity_type, entity_id)
    WHERE entity_id > 0 AND status IN ('PENDING', 'APPROVED') AND consumed_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS approval_requests_unbound_active_unique
    ON approval_requests (tenant_id, entity_type, entity_number)
    WHERE entity_id = 0 AND status IN ('PENDING', 'APPROVED') AND consumed_at IS NULL;
