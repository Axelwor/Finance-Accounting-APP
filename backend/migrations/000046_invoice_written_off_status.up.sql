-- A-07: Add 'WRITTEN_OFF' status to invoices for ECL write-offs.
-- The ECL write-off handler (tax/ecl.go) now sets an invoice to WRITTEN_OFF
-- when its receivable is fully consumed by a write-off. This is semantically
-- distinct from VOID (cancelled before issuance) and PAID (settled):
-- WRITTEN_OFF means the revenue was earned but the receivable was
-- uncollectible and removed from the books via the allowance / bad debt.
-- The write-off handler also falls back to 'VOID' at runtime if this
-- migration has not been applied yet (CHECK violation catch).

ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('DRAFT', 'ISSUED', 'PARTIALLY_PAID', 'PAID', 'VOID', 'WRITTEN_OFF'));
