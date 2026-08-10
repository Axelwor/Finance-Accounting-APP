-- Reverse the reconciliation: restore before_json/after_json names and
-- drop the user_agent column added by 000034.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'audit_logs' AND column_name = 'before_data'
    ) THEN
        ALTER TABLE audit_logs RENAME COLUMN before_data TO before_json;
        ALTER TABLE audit_logs RENAME COLUMN after_data TO after_json;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'audit_logs' AND column_name = 'user_agent'
    ) THEN
        ALTER TABLE audit_logs DROP COLUMN user_agent;
    END IF;
END $$;
