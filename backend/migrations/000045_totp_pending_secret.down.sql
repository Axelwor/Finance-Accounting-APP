-- Rollback 000045
ALTER TABLE users DROP COLUMN IF EXISTS totp_pending_secret;
