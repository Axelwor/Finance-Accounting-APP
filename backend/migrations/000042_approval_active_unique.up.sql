-- F-03: Allow multiple amount-based approval requests over time.
--
-- Problem: the blanket UNIQUE (tenant_id, entity_type, entity_id) blocks a
-- second amount-based submission because every unbound request uses
-- entity_id = 0. After the first approval is consumed, no new amount-based
-- approval could be submitted.
--
-- Fix: replace the blanket constraint with a PARTIAL unique index that only
-- applies to requests that are still usable (status IN PENDING/APPROVED and
-- not yet consumed). Consumed/rejected/cancelled requests no longer block new
-- submissions, while still preventing duplicate active requests for the same
-- bound entity.

ALTER TABLE approval_requests DROP CONSTRAINT IF EXISTS approval_requests_tenant_id_entity_type_entity_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS approval_requests_active_unique
    ON approval_requests (tenant_id, entity_type, entity_id)
    WHERE status IN ('PENDING', 'APPROVED') AND consumed_at IS NULL;
