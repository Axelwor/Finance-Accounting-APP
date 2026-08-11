-- F-03: Strengthen approval_requests for enforcement in posting flows.
-- amount_cents: the planned amount of the document being approved (so the
--   gate can verify the approved amount covers the invoice total).
-- consumed_at: set when the approval is used by a posting, preventing reuse.

ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS amount_cents BIGINT;
ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ;
