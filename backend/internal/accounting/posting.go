package accounting

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// post runs one posting command inside a single pgx transaction: scope RLS,
// idempotent replay, lock the ledger chain head, build the journal via the
// pure engine, recompute the hash against the real previous hash, insert the
// entry + lines, advance the chain head, write the outbox event, commit.
// Mirrors cash.post but lives in the accounting package so manual journals
// share the same chain as cash/opening entries.
func (service *Service) post(request *http.Request, tenant int64, idem string, build func(ctx context.Context, tx pgx.Tx) (Journal, error)) (postingResult, error) {
	var result postingResult
	userID, _ := auth.UserIDFromContext(request.Context())
	err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := scopeTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		// Idempotent replay: an identical retry returns the stored journal.
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		}); err == nil {
			result = postingResult{
				ID:       existing.ID,
				Number:   existing.Number,
				Status:   existing.Status,
				Hash:     existing.Hash,
				PrevHash: existing.PrevHash,
				Intent:   existing.IntentType.String,
			}
			return nil
		}
		// Lock the chain head so concurrent postings serialize on one row.
		head, err := lockChainHead(request.Context(), tx, tenant)
		if err != nil {
			return err
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
			IdempotencyKey: uuidValue(idem),
			Hash:           journal.Hash,
			PrevHash:       journal.PreviousHash,
			CreatedBy:      int8Value(userID),
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
				Description:   pgtype.Text{String: line.SourceLineRef, Valid: line.SourceLineRef != ""},
				SourceLineRef: text(line.SourceLineRef),
				DimensionIds:  []byte("[]"),
			}); err != nil {
				return err
			}
		}
		if err := upsertChainHead(request.Context(), tx, tenant, entry.ID, journal.Hash); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "journal.posted", journalPayload(journal, entry.ID, number)); err != nil {
			return err
		}
		result = postingResult{
			ID:       entry.ID,
			Number:   number,
			Status:   "POSTED",
			Hash:     journal.Hash,
			PrevHash: journal.PreviousHash,
			Intent:   string(journal.IntentType),
		}
		return nil
	})
	if err != nil {
		return postingResult{}, err
	}
	return result, nil
}

// postingResult is what the manual-journal handler returns.
type postingResult struct {
	ID       int64  `json:"id"`
	Number   string `json:"number"`
	Status   string `json:"status"`
	Hash     string `json:"hash"`
	PrevHash string `json:"prev_hash"`
	Intent   string `json:"intent_type"`
}

// journalPayload is the JSON payload stored on the outbox event.
func journalPayload(journal Journal, entryID int64, number string) []byte {
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
		return []byte("{}")
	}
	return data
}

// pgtypeText wraps a string into a nullable pgtype.Text.
func pgtypeTextValue(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

// accountForEngine converts a loaded accountRow into the pure engine shape.
func accountForEngine(row accountRow) Account {
	return Account{
		ID:       row.ID,
		IsGroup:  row.IsGroup,
		IsActive: row.IsActive,
	}
}

// parseInt64 parses a base-10 int64 from a string.
func parseInt64(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
