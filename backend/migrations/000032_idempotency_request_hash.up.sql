-- M-023: Add request_hash column for idempotency payload matching.
-- When an idempotency key is reused with a DIFFERENT payload, the API should
-- return 409 IDEMPOTENCY_KEY_REUSE instead of silently returning the old result.
ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS request_hash TEXT;
CREATE INDEX IF NOT EXISTS journal_entries_request_hash_idx
    ON journal_entries (tenant_id, idempotency_key, request_hash)
    WHERE idempotency_key IS NOT NULL;
