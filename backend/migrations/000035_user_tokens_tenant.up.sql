-- Multi-tenant sessions: store the tenant + role a token was issued for so
-- refresh rotation preserves the ACTIVE tenant after a tenant switch. Without
-- this, refreshing a token re-derives the tenant from user_tenants (first
-- membership), silently sending switched users back to their first tenant.
ALTER TABLE user_tokens ADD COLUMN IF NOT EXISTS tenant_id BIGINT;
ALTER TABLE user_tokens ADD COLUMN IF NOT EXISTS role TEXT;
