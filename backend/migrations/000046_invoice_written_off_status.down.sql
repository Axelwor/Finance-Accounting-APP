-- Revert A-07: remove 'WRITTEN_OFF' from the invoices status CHECK.
-- Any invoices left in WRITTEN_OFF status must be moved to VOID first.

UPDATE invoices SET status = 'VOID' WHERE status = 'WRITTEN_OFF';

ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('DRAFT', 'ISSUED', 'PARTIALLY_PAID', 'PAID', 'VOID'));
