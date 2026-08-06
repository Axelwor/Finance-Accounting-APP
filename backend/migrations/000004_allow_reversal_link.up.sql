-- Allow the authorized reversal procedure to (a) mark an original journal
-- VOID and (b) set reversal_of_id on the new reversal journal, both only
-- while app.void_context is set transaction-locally.

CREATE OR REPLACE FUNCTION reject_posted_journal_mutation() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'POSTED' AND NOT (
        TG_OP = 'UPDATE'
        AND current_setting('app.void_context', true) = '1'
        AND (
            -- Authorized transition to VOID with full audit metadata.
            (NEW.status = 'VOID'
             AND NEW.reversal_of_id IS NULL
             AND NEW.void_reason IS NOT NULL
             AND NEW.voided_by IS NOT NULL
             AND NEW.voided_at IS NOT NULL)
            OR
            -- Recording the reversal link on the new reversal journal.
            (NEW.status = 'POSTED'
             AND NEW.reversal_of_id IS NOT NULL
             AND NEW.reversal_of_id <> OLD.id)
        )
    ) THEN
        RAISE EXCEPTION 'posted journal is immutable; use authorized reversal procedure';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
