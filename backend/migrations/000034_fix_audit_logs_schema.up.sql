-- Reconcile audit_logs schema with the application code.
--
-- Migration 000001 originally created audit_logs with before_json/after_json.
-- Migration 000023 redefined it with before_data/after_data (the names the
-- application code uses), but its CREATE TABLE failed on databases where
-- 000001 had already created the table. Reconcile idempotently:
--   * rename before_json/after_json -> before_data/after_data when present
--   * add before_data/after_data when neither pair exists
--   * add missing user_agent column

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'audit_logs' AND column_name = 'before_json'
    ) THEN
        ALTER TABLE audit_logs RENAME COLUMN before_json TO before_data;
        ALTER TABLE audit_logs RENAME COLUMN after_json TO after_data;
    ELSE
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_name = 'audit_logs' AND column_name = 'before_data'
        ) THEN
            ALTER TABLE audit_logs ADD COLUMN before_data JSONB;
        END IF;
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_name = 'audit_logs' AND column_name = 'after_data'
        ) THEN
            ALTER TABLE audit_logs ADD COLUMN after_data JSONB;
        END IF;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'audit_logs' AND column_name = 'user_agent'
    ) THEN
        ALTER TABLE audit_logs ADD COLUMN user_agent TEXT;
    END IF;
END $$;
