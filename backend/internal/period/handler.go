package period

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// idempotencyKey validates the required Idempotency-Key header (must be a
// UUID). m-021: period close/unlock post journals and therefore require it.
func idempotencyKey(request *http.Request) (string, error) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", errors.New("Idempotency-Key header is required")
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(key); err != nil {
		return "", errors.New("Idempotency-Key must be a UUID")
	}
	return key, nil
}

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Post("/periods/close", service.Close)
	router.Post("/periods/unlock", service.Unlock)
}

// Unlock reopens a CLOSED period: it reverses the closing entry journal and
// sets the period back to OPEN. Idempotent via the PERIOD_CLOSE source ref.
func (service *Service) Unlock(writer http.ResponseWriter, request *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenantID <= 0 {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	userID, _ := auth.UserIDFromContext(request.Context())
	// m-021: unlock posts a reversal journal, so it requires a valid
	// Idempotency-Key just like close does.
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := service.unlockPeriod(request.Context(), tenantID, userID, idem)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "UNLOCK_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) unlockPeriod(ctx context.Context, tenantID, userID int64, idem string) (map[string]any, error) {
	var result map[string]any
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10)); err != nil {
			return err
		}

		var periodID int64
		if err := tx.QueryRow(ctx, `
			SELECT id FROM accounting_periods
			WHERE tenant_id = $1 AND status = 'CLOSED'
			ORDER BY period_start DESC LIMIT 1
		`, tenantID).Scan(&periodID); err != nil {
			return fmt.Errorf("no closed period: %w", err)
		}

		// Locate the closing journal for this period.
		var closingJournalID int64
		err := tx.QueryRow(ctx, `
			SELECT id FROM journal_entries
			WHERE tenant_id = $1 AND intent_type = 'PERIOD_CLOSE' AND source_ref = $2
			ORDER BY id DESC LIMIT 1
		`, tenantID, fmt.Sprintf("CLOSE-%d", periodID)).Scan(&closingJournalID)
		if err != nil {
			return fmt.Errorf("closing journal not found: %w", err)
		}

		// Build a reversal of the closing journal.
		lines, err := loadJournalLines(ctx, tx, tenantID, closingJournalID)
		if err != nil {
			return err
		}
		var reversed []accounting.Line
		for _, line := range lines {
			reversed = append(reversed, accounting.Line{
				AccountID:     line.AccountID,
				DebitCents:    line.CreditCents,
				CreditCents:   line.DebitCents,
				SourceLineRef: "rev-" + line.SourceLineRef,
			})
		}
		if err := accounting.BalanceCheck(reversed); err != nil {
			return err
		}

		journal := accounting.Journal{
			TenantID:    tenantID,
			SourceRef:   fmt.Sprintf("UNLOCK-%d", periodID),
			IntentType:  accounting.IntentType("PERIOD_REOPEN"),
			EntryDate:   time.Now().Format("2006-01-02"),
			Description: "Closing journal reversal (period reopen)",
			Lines:       reversed,
		}

		head, err := lockOrSeedChainHead(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = computeHash(journal)

		number, err := nextJournalNumber(ctx, tx, tenantID)
		if err != nil {
			return err
		}

		// Reopen the period first so the entry-date trigger accepts the
		// reversal journal (status REOPENED is accepted), then insert.
		if _, err := tx.Exec(ctx, `
			UPDATE accounting_periods SET status = 'REOPENED'
			WHERE id = $1
		`, periodID); err != nil {
			return err
		}

		// Set void context so the immutable trigger accepts linking the
		// original closing journal as reversed.
		if _, err := tx.Exec(ctx, `SELECT set_config('app.void_context', '1', true)`); err != nil {
			return err
		}
		var entryID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by, reversal_of_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id
		`, tenantID, number, journal.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem, journal.Hash, journal.PreviousHash, userID, closingJournalID).Scan(&entryID)
		if err != nil {
			return err
		}
		for _, line := range journal.Lines {
			if _, err := tx.Exec(ctx, `
				INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, tenantID, entryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
				return err
			}
		}

		// Finalize the reopen: status OPEN and clear close metadata.
		if _, err := tx.Exec(ctx, `
			UPDATE accounting_periods SET status = 'OPEN', closed_at = NULL, closed_by = NULL
			WHERE id = $1
		`, periodID); err != nil {
			return err
		}
		// Audit trail: log the period UNLOCK action.
		after := map[string]any{
			"period_id":   periodID,
			"journal_id":  entryID,
			"number":      number,
			"status":      "OPEN",
			"unlocked_by": userID,
		}
		if err := audit.Log(ctx, tx, tenantID, userID, "accounting_period", periodID, audit.ActionUnlock, nil, after); err != nil {
			return err
		}
		if err := upsertChainHead(ctx, tx, tenantID, entryID, journal.Hash); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (tenant_id, topic, payload) VALUES ($1, 'period.reopened', $2::jsonb)`,
			tenantID, mustJSON(map[string]any{"period_id": periodID, "journal_id": entryID, "number": number})); err != nil {
			return err
		}

		result = map[string]any{
			"period_id":  periodID,
			"status":     "OPEN",
			"journal_id": entryID,
			"number":     number,
			"hash":       journal.Hash,
		}
		return nil
	})
	return result, err
}

func loadJournalLines(ctx context.Context, tx pgx.Tx, tenantID, entryID int64) ([]accounting.Line, error) {
	rows, err := tx.Query(ctx, `
		SELECT account_id, debit_cents, credit_cents, source_line_ref
		FROM journal_lines
		WHERE tenant_id = $1 AND entry_id = $2
	`, tenantID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []accounting.Line
	for rows.Next() {
		var line accounting.Line
		if err := rows.Scan(&line.AccountID, &line.DebitCents, &line.CreditCents, &line.SourceLineRef); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (service *Service) Close(writer http.ResponseWriter, request *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenantID <= 0 {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	userID, _ := auth.UserIDFromContext(request.Context())
	idem := request.Header.Get("Idempotency-Key")

	result, err := service.closePeriod(request.Context(), tenantID, userID, idem)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "CLOSE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) closePeriod(ctx context.Context, tenantID, userID int64, idem string) (map[string]any, error) {
	var result map[string]any
	err := db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10)); err != nil {
			return err
		}

		periodID, err := findOpenPeriod(ctx, tx, tenantID)
		if err != nil {
			return err
		}

		balances, err := loadPLBalances(ctx, tx, tenantID, periodID)
		if err != nil {
			return err
		}

		retainedID, runningID, err := resolveEquityAccounts(ctx, tx, tenantID)
		if err != nil {
			return err
		}

		lines := buildClosingLines(balances, retainedID, runningID)
		journal := accounting.Journal{
			TenantID:    tenantID,
			SourceRef:   fmt.Sprintf("CLOSE-%d", periodID),
			IntentType:  accounting.IntentType("PERIOD_CLOSE"),
			EntryDate:   time.Now().Format("2006-01-02"),
			Description: "Period closing journal",
			Lines:       lines,
		}
		if err := accounting.BalanceCheck(journal.Lines); err != nil {
			return err
		}

		head, err := lockOrSeedChainHead(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = computeHash(journal)

		number, err := nextJournalNumber(ctx, tx, tenantID)
		if err != nil {
			return err
		}

		entryID, err := insertClosingEntry(ctx, tx, tenantID, userID, idem, periodID, number, journal)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `UPDATE accounting_periods SET status = 'CLOSED', closed_at = now(), closed_by = $1 WHERE id = $2`, userID, periodID); err != nil {
			return err
		}
		// Audit trail: log the period CLOSE action.
		after := map[string]any{
			"period_id":  periodID,
			"journal_id": entryID,
			"number":     number,
			"status":     "CLOSED",
			"closed_by":  userID,
		}
		if err := audit.Log(ctx, tx, tenantID, userID, "accounting_period", periodID, audit.ActionClose, nil, after); err != nil {
			return err
		}
		if err := upsertChainHead(ctx, tx, tenantID, entryID, journal.Hash); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (tenant_id, topic, payload) VALUES ($1, 'period.closed', $2::jsonb)`,
			tenantID, mustJSON(map[string]any{"period_id": periodID, "journal_id": entryID, "number": number})); err != nil {
			return err
		}

		result = map[string]any{
			"period_id":  periodID,
			"status":     "CLOSED",
			"journal_id": entryID,
			"number":     number,
			"hash":       journal.Hash,
		}
		return nil
	})
	return result, err
}

func findOpenPeriod(ctx context.Context, tx pgx.Tx, tenantID int64) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounting_periods
		WHERE tenant_id = $1 AND status = 'OPEN'
		ORDER BY period_start DESC LIMIT 1
	`, tenantID).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("no open period: %w", err)
	}
	var closingCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM journal_entries
		WHERE tenant_id = $1 AND intent_type = 'PERIOD_CLOSE' AND source_ref = $2
	`, tenantID, fmt.Sprintf("CLOSE-%d", periodID)).Scan(&closingCount)
	if err != nil {
		return 0, err
	}
	if closingCount > 0 {
		return 0, fmt.Errorf("period already closed")
	}
	return periodID, nil
}

type plBalance struct {
	accountID int64
	amount    int64
}

func loadPLBalances(ctx context.Context, tx pgx.Tx, tenantID, periodID int64) ([]plBalance, error) {
	rows, err := tx.Query(ctx, `
		SELECT jl.account_id, COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		JOIN accounting_periods p ON p.tenant_id = $1 AND p.id = $2
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')
		  AND je.entry_date >= p.period_start AND je.entry_date <= p.period_end
		GROUP BY jl.account_id
	`, tenantID, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var balances []plBalance
	for rows.Next() {
		var b plBalance
		if err := rows.Scan(&b.accountID, &b.amount); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}
	return balances, rows.Err()
}

func resolveEquityAccounts(ctx context.Context, tx pgx.Tx, tenantID int64) (retainedID, runningID int64, err error) {
	err = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = '3201'`, tenantID).Scan(&retainedID)
	if err != nil {
		return 0, 0, fmt.Errorf("retained earnings account not found: %w", err)
	}
	err = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = '3301'`, tenantID).Scan(&runningID)
	if err != nil {
		return 0, 0, fmt.Errorf("current earnings account not found: %w", err)
	}
	return retainedID, runningID, nil
}

func buildClosingLines(balances []plBalance, retainedID, runningID int64) []accounting.Line {
	var lines []accounting.Line
	var revenueTotal, expenseTotal int64
	for _, b := range balances {
		if b.amount == 0 {
			continue
		}
		if b.amount > 0 {
			lines = append(lines,
				accounting.Line{AccountID: b.accountID, DebitCents: b.amount, SourceLineRef: fmt.Sprintf("rev-%d", b.accountID)},
				accounting.Line{AccountID: runningID, CreditCents: b.amount, SourceLineRef: "to-running"},
			)
			revenueTotal += b.amount
		} else {
			amount := -b.amount
			lines = append(lines,
				accounting.Line{AccountID: runningID, DebitCents: amount, SourceLineRef: "from-running"},
				accounting.Line{AccountID: b.accountID, CreditCents: amount, SourceLineRef: fmt.Sprintf("exp-%d", b.accountID)},
			)
			expenseTotal += amount
		}
	}
	netProfit := revenueTotal - expenseTotal
	if netProfit != 0 {
		if netProfit > 0 {
			lines = append(lines,
				accounting.Line{AccountID: runningID, DebitCents: netProfit, SourceLineRef: "close-running"},
				accounting.Line{AccountID: retainedID, CreditCents: netProfit, SourceLineRef: "to-retained"},
			)
		} else {
			amount := -netProfit
			lines = append(lines,
				accounting.Line{AccountID: retainedID, DebitCents: amount, SourceLineRef: "from-retained"},
				accounting.Line{AccountID: runningID, CreditCents: amount, SourceLineRef: "close-running"},
			)
		}
	}
	return lines
}

func insertClosingEntry(ctx context.Context, tx pgx.Tx, tenantID, userID int64, idem string, periodID int64, number string, journal accounting.Journal) (int64, error) {
	var entryID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, tenantID, number, journal.EntryDate, periodID, journal.Description,
		journal.SourceRef, string(journal.IntentType), idem, journal.Hash, journal.PreviousHash, userID).Scan(&entryID)
	if err != nil {
		return 0, err
	}
	for _, line := range journal.Lines {
		_, err := tx.Exec(ctx, `
			INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, tenantID, entryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef)
		if err != nil {
			return 0, err
		}
	}
	return entryID, nil
}

func lockOrSeedChainHead(ctx context.Context, tx pgx.Tx, tenantID int64) (db.LedgerChainHead, error) {
	head, err := db.New(tx).LockLedgerChainHead(ctx, tenantID)
	if err == nil {
		return head, nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, NULL, 'genesis') ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID)
	if err != nil {
		return db.LedgerChainHead{}, err
	}
	return db.New(tx).LockLedgerChainHead(ctx, tenantID)
}

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

func upsertChainHead(ctx context.Context, tx pgx.Tx, tenantID, lastJournalID int64, lastHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE
		SET last_journal_id = EXCLUDED.last_journal_id, last_hash = EXCLUDED.last_hash, updated_at = now()
	`, tenantID, lastJournalID, lastHash)
	return err
}

func computeHash(journal accounting.Journal) string {
	return accounting.HashJournal(journal)
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}
