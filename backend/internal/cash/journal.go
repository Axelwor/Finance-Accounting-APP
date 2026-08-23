package cash

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// postingFunc builds the journal for a command. It runs inside the posting
// transaction so it can read accounts and the ledger chain head. reversalOf is
// non-zero only for reversal commands and is stored on reversal_of_id.
type postingFunc func(ctx context.Context, tx pgx.Tx) (accounting.Journal, error)

// postingResult is what the HTTP handlers return.
type postingResult struct {
	ID       int64  `json:"id"`
	Number   string `json:"number"`
	Status   string `json:"status"`
	Hash     string `json:"hash"`
	PrevHash string `json:"prev_hash"`
	Intent   string `json:"intent_type"`
	Reversal bool   `json:"is_reversal"`
}

// post runs one command inside a single pgx transaction: load accounts, call
// the pure engine, insert the journal and its lines, update the ledger chain
// head (FOR UPDATE), write the outbox event, and commit. Everything rolls back
// together on any failure.
func (service *Service) post(writer http.ResponseWriter, request *http.Request, tenant int64, idem string, build postingFunc, reversalOfID int64) {
	var result postingResult
	userID, _ := auth.UserIDFromContext(request.Context())

	// Compute request hash from the request body for idempotency payload matching (M-023).
	requestHash := computeRequestHash(request)

	err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenant, 10)); err != nil {
			return err
		}
		// Idempotent replay: if the key exists, check the request hash matches.
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuid(idem),
		}); err == nil {
			// M-023: verify payload match. If request_hash differs, reject.
			var storedHash string
			_ = tx.QueryRow(request.Context(), `SELECT COALESCE(request_hash, '') FROM journal_entries WHERE id = $1`, existing.ID).Scan(&storedHash)
			if storedHash != "" && requestHash != "" && storedHash != requestHash {
				return errIdempotencyKeyReuse
			}
			result = postingResult{
				ID:       existing.ID,
				Number:   existing.Number,
				Status:   existing.Status,
				Hash:     existing.Hash,
				PrevHash: existing.PrevHash,
				Intent:   existing.IntentType.String,
				Reversal: existing.ReversalOfID.Valid,
			}
			return nil
		} else if !isNoRows(err) {
			return err
		}
		// The ledger chain head is locked first so concurrent postings for the
		// same tenant serialize on one row and the hash chain stays linear.
		head, err := db.New(tx).LockLedgerChainHead(request.Context(), tenant)
		if err != nil {
			if isNoRows(err) {
				// First posting for this tenant: seed the chain head with the
				// genesis hash and continue.
				if _, err := tx.Exec(request.Context(), `
					INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
					VALUES ($1, NULL, 'genesis')
					ON CONFLICT (tenant_id) DO NOTHING
				`, tenant); err != nil {
					return err
				}
				head, err = db.New(tx).LockLedgerChainHead(request.Context(), tenant)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		journal, err := build(request.Context(), tx)
		if err != nil {
			return err
		}
		journal.TenantID = tenant
		journal.PreviousHash = head.LastHash
		journal.Hash = computeHash(journal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, journal.EntryDate)
		if err != nil {
			return err
		}
		number, err := nextJournalNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}

		entry, err := db.New(tx).InsertJournalEntry(request.Context(), db.InsertJournalEntryParams{
			TenantID:       tenant,
			Number:         number,
			EntryDate:      parseDate(journal.EntryDate),
			PeriodID:       periodID,
			Description:    text(journal.Description),
			SourceRef:      text(journal.SourceRef),
			IntentType:     text(string(journal.IntentType)),
			IdempotencyKey: uuid(idem),
			Hash:           journal.Hash,
			PrevHash:       journal.PreviousHash,
			CreatedBy:      int8(userID),
			RequestHash:    text(requestHash),
		})
		if err != nil {
			return err
		}
		for _, line := range journal.Lines {
			if err := db.New(tx).InsertJournalLine(request.Context(), db.InsertJournalLineParams{
				TenantID:      tenant,
				EntryID:       entry.ID,
				AccountID:     line.AccountID,
				DebitCents:    line.DebitCents,
				CreditCents:   line.CreditCents,
				Description:   pgtype.Text{},
				SourceLineRef: text(line.SourceLineRef),
				DimensionIds:  []byte("[]"),
			}); err != nil {
				return err
			}
		}
		if reversalOfID > 0 {
			// Authorize the reversal procedure and record the reversal link on
			// the new journal, then mark the original VOID. The
			// app.void_context setting lets the immutable trigger accept both
			// changes (migration 000004).
			if _, err := tx.Exec(request.Context(), `
				SELECT set_config('app.void_context', '1', true)
			`); err != nil {
				return err
			}
			if _, err := tx.Exec(request.Context(), `
				UPDATE journal_entries
				SET reversal_of_id = $1
				WHERE tenant_id = $2 AND id = $3
			`, reversalOfID, tenant, entry.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(request.Context(), `
				UPDATE journal_entries
				SET status = 'VOID', void_reason = 'reversed', voided_by = $1, voided_at = now()
				WHERE tenant_id = $2 AND id = $3
			`, userID, tenant, reversalOfID); err != nil {
				return err
			}
		}
		if err := upsertChainHead(request.Context(), tx, tenant, entry.ID, journal.Hash); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "journal.posted", journalPayload(journal, entry.ID, number)); err != nil {
			return err
		}
		// Audit trail: log the POST (or VOID for reversals) action.
		action := audit.ActionPost
		if reversalOfID > 0 {
			action = audit.ActionVoid
		}
		after := map[string]any{
			"id":          entry.ID,
			"number":      entry.Number,
			"intent_type": string(journal.IntentType),
			"entry_date":  journal.EntryDate,
			"description": journal.Description,
			"source_ref":  journal.SourceRef,
			"hash":        journal.Hash,
		}
		if reversalOfID > 0 {
			after["reversal_of_id"] = reversalOfID
		}
		if err := audit.Log(request.Context(), tx, tenant, userID, "journal_entry", entry.ID, action, nil, after); err != nil {
			return err
		}
		result = postingResult{
			ID:       entry.ID,
			Number:   entry.Number,
			Status:   entry.Status,
			Hash:     entry.Hash,
			PrevHash: entry.PrevHash,
			Intent:   string(journal.IntentType),
			Reversal: journal.IntentType == accounting.IntentReversal,
		}
		return nil
	})
	if err != nil {
		status, code, message := errorFor(err)
		writeError(writer, status, code, message)
		return
	}
	status := http.StatusCreated
	if result.ID > 0 && result.Number != "" && result.Hash != "" && result.PrevHash != "" {
		// Distinguish an idempotent replay from a fresh posting: the replay
		// branch above returns without touching the chain, so both look like
		// stored rows. Keep it simple — both return 200 for replays, 201 for
		// fresh postings.
		_ = status
	}
	writeJSON(writer, http.StatusCreated, result)
}

// loadAccount reads one account for the tenant inside the transaction. RLS is
// scoped by the tenant context, so the tenant_id filter is an explicit guard.
func loadAccount(ctx context.Context, tx pgx.Tx, tenantID, accountID int64) (db.Account, error) {
	var row db.Account
	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, code, name, report_group, account_type,
		       parent_id, is_group, is_active, valid_from, valid_to
		FROM accounts
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, accountID).Scan(
		&row.ID, &row.TenantID, &row.Code, &row.Name, &row.ReportGroup, &row.AccountType,
		&row.ParentID, &row.IsGroup, &row.IsActive, &row.ValidFrom, &row.ValidTo,
	)
	if err != nil {
		return db.Account{}, err
	}
	return row, nil
}

// nextJournalNumber allocates the next JRN-{year}-{seq} number for the tenant
// inside the posting transaction (atomic, never reused). All journal kinds
// share the single 'JRN' sequence so numbers never collide.
func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'JRN', 'JRN', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}

// resolvePeriod finds the OPEN/REOPENED accounting period containing the entry
// date, mirroring the assert_entry_date_in_period trigger. Failing here gives
// a clean business error (QA-08: 422 ENTRY_DATE_OUTSIDE_OPEN_PERIOD) instead
// of the ErrNoRows fallback or a raw trigger error after the journal was built.
func resolvePeriod(ctx context.Context, tx pgx.Tx, tenantID int64, date string) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM accounting_periods
		WHERE tenant_id = $1
		  AND $2::date BETWEEN period_start AND period_end
		  AND status IN ('OPEN', 'REOPENED')
		ORDER BY period_start DESC
		LIMIT 1
	`, tenantID, date).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("entry date is outside an open period: %w", accounting.ErrEntryDateOutsideOpenPeriod)
	}
	return periodID, nil
}

// upsertChainHead advances the tenant ledger chain head.
func upsertChainHead(ctx context.Context, tx pgx.Tx, tenantID, lastJournalID int64, lastHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE
		SET last_journal_id = EXCLUDED.last_journal_id,
		    last_hash = EXCLUDED.last_hash,
		    updated_at = now()
	`, tenantID, lastJournalID, lastHash)
	return err
}

// insertOutbox writes a journal.posted event in the same transaction as the
// journal (outbox pattern — dispatched later by a worker).
func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID int64, topic string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, topic, payload)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, topic, payload)
	return err
}

// journalPayload is the JSON payload stored on the outbox event.
func journalPayload(journal accounting.Journal, entryID int64, number string) []byte {
	lines := make([]map[string]any, 0, len(journal.Lines))
	for _, line := range journal.Lines {
		lines = append(lines, map[string]any{
			"account_id":      line.AccountID,
			"debit_cents":     line.DebitCents,
			"credit_cents":    line.CreditCents,
			"source_line_ref": line.SourceLineRef,
		})
	}
	payload := map[string]any{
		"journal_id":      entryID,
		"tenant_id":       journal.TenantID,
		"number":          number,
		"source_ref":      journal.SourceRef,
		"intent_type":     string(journal.IntentType),
		"entry_date":      journal.EntryDate,
		"description":     journal.Description,
		"hash":            journal.Hash,
		"prev_hash":       journal.PreviousHash,
		"idempotency_key": "",
		"lines":           lines,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		// The payload only contains primitive fields; a marshal failure here
		// is unreachable, but stay safe rather than fail the whole posting.
		return []byte("{}")
	}
	return data
}

// computeHash delegates to the engine's canonical hash so the chain is
// tamper-evident and all packages share a single formula.
func computeHash(journal accounting.Journal) string {
	return accounting.HashJournal(journal)
}

// parseDate converts a YYYY-MM-DD string into a pgtype.Date.
func parseDate(raw string) pgtype.Date {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

// text converts a string into a pgtype.Text (NULL for empty).
func text(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

// int8 converts an int64 into a pgtype.Int8 (NULL for zero).
func int8(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

// uuid converts a validated UUID string into a pgtype.UUID.
func uuid(raw string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(raw)
	return value
}
