DROP INDEX IF EXISTS journal_entries_request_hash_idx;
ALTER TABLE journal_entries DROP COLUMN IF EXISTS request_hash;
