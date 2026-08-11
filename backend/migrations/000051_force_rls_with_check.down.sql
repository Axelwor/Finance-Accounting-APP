-- B-06: No-op down migration. Dropping FORCE ROW LEVEL SECURITY is not reversible
-- safely in production environments. Downgrade requires restoring original policies.
SELECT 1;
