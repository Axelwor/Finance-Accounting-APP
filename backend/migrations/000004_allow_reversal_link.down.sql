-- Restore the original immutable trigger (no reversal-link allowance).
-- Kept as a rollback reference; the app.void_context flow is the only path.
CREATE OR REPLACE FUNCTION reject_posted_journal_mutation() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'POSTED' AND NOT (
        TG_OP = 'UPDATE'
        AND NEW.status = 'VOID'
        AND NEW.reversal_of_id IS NULL
        AND NEW.void_reason IS NOT NULL
        AND NEW.voided_by IS NOT NULL
        AND NEW.voided_at IS NOT NULL
        AND current_setting('app.void_context', true) = '1'
    ) THEN
        RAISE EXCEPTION 'posted journal is immutable; use authorized reversal procedure';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
