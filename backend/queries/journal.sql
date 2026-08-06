-- name: GetJournalEntry :one
SELECT id, tenant_id, number, entry_date, period_id, status, description,
       source_ref, intent_type, idempotency_key, reversal_of_id,
       void_reason, voided_by, voided_at, hash, prev_hash,
       created_by, created_at, updated_at
FROM journal_entries
WHERE tenant_id = $1 AND id = $2;

-- name: GetJournalByIdempotencyKey :one
SELECT id, tenant_id, number, entry_date, period_id, status, description,
       source_ref, intent_type, idempotency_key, reversal_of_id,
       void_reason, voided_by, voided_at, hash, prev_hash,
       created_by, created_at, updated_at
FROM journal_entries
WHERE tenant_id = $1 AND idempotency_key = $2;

-- name: InsertJournalEntry :one
INSERT INTO journal_entries (
    tenant_id, number, entry_date, period_id, description,
    source_ref, intent_type, idempotency_key, hash, prev_hash, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, tenant_id, number, entry_date, period_id, status, description,
          source_ref, intent_type, idempotency_key, reversal_of_id,
          void_reason, voided_by, voided_at, hash, prev_hash,
          created_by, created_at, updated_at;

-- name: InsertJournalLine :exec
INSERT INTO journal_lines (
    tenant_id, entry_id, account_id, debit_cents, credit_cents,
    description, source_line_ref, dimension_ids
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetAccountBalance :one
SELECT COALESCE(SUM(debit_cents - credit_cents), 0)::bigint AS balance_cents
FROM journal_lines
WHERE tenant_id = $1 AND account_id = $2;

-- name: LockLedgerChainHead :one
SELECT tenant_id, last_journal_id, last_hash, updated_at
FROM ledger_chain_heads
WHERE tenant_id = $1
FOR UPDATE;
