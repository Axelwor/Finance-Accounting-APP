-- Rollback 000041
ALTER TABLE approval_requests DROP COLUMN IF EXISTS consumed_at;
ALTER TABLE approval_requests DROP COLUMN IF EXISTS amount_cents;
