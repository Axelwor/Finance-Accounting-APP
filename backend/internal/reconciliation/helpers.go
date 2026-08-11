package reconciliation

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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// Service wires the bank reconciliation HTTP handlers over a pgx pool.
type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Routes registers the bank-statement and reconciliation endpoints.
// Mounted under /api/v1 (inside the authenticated route group).
func (service *Service) Routes(router chi.Router) {
	router.Post("/bank-statements", service.CreateStatement)
	router.Get("/bank-statements", service.ListStatements)
	router.Get("/bank-statements/{id}", service.GetStatement)

	router.Post("/bank-statements/{id}/reconcile", service.StartReconciliation)
	router.Post("/bank-reconciliations/{id}/match", service.MatchLine)
	router.Post("/bank-reconciliations/{id}/unmatch", service.UnmatchLine)
	router.Post("/bank-reconciliations/{id}/complete", service.CompleteReconciliation)
	router.Get("/bank-reconciliations/{id}", service.GetReconciliation)
}

// ---------------------------------------------------------------------------
// Shared helpers (mirrors backend/internal/purchase/helpers.go).
// ---------------------------------------------------------------------------

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeJSON(request *http.Request, target any) error {
	return json.NewDecoder(request.Body).Decode(target)
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, errorResponse{Code: code, Message: message})
}

func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

func userID(request *http.Request) int64 {
	value, _ := auth.UserIDFromContext(request.Context())
	return value
}

func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

// withTenant scopes the transaction to the tenant for RLS (matches the
// purchase / cash handler pattern).
func withTenant(ctx context.Context, tx pgx.Tx, tenantIDValue int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantIDValue, 10))
	return err
}

// runTx is a thin wrapper over db.WithTransaction that also sets the tenant
// RLS context, so handlers stay short.
func (service *Service) runTx(request *http.Request, tenant int64, fn func(pgx.Tx) error) error {
	return db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		return fn(tx)
	})
}

func isUniqueViolation(err error) bool {
	const pgErrCodeUniqueViolation = "23505"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation
}

func isForeignKeyViolation(err error) bool {
	const pgErrCodeForeignKeyViolation = "23503"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeForeignKeyViolation
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func parseDate(raw string) (pgtype.Date, error) {
	if strings.TrimSpace(raw) == "" {
		return pgtype.Date{}, errors.New("date is required")
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: parsed, Valid: true}, nil
}

func optionalDate(raw string) (pgtype.Date, error) {
	if strings.TrimSpace(raw) == "" {
		return pgtype.Date{}, nil
	}
	return parseDate(raw)
}

func dateString(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func textValueOptional(raw string) pgtype.Text {
	return pgtype.Text{String: raw, Valid: strings.TrimSpace(raw) != ""}
}

func int8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

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

func uuidValue(raw string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(raw)
	return value
}

// ---------------------------------------------------------------------------
// Journal posting helpers (hash-chain, idempotency, outbox)
// ---------------------------------------------------------------------------

func lockOrSeedHead(ctx context.Context, tx pgx.Tx, tenantID int64) (db.LedgerChainHead, error) {
	head, err := db.New(tx).LockLedgerChainHead(ctx, tenantID)
	if err == nil {
		return head, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, NULL, 'genesis') ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID); err != nil {
		return db.LedgerChainHead{}, err
	}
	return db.New(tx).LockLedgerChainHead(ctx, tenantID)
}

func resolvePeriod(ctx context.Context, tx pgx.Tx, tenantID int64, date string) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounting_periods
		WHERE tenant_id = $1 AND $2::date BETWEEN period_start AND period_end
		  AND status IN ('OPEN', 'REOPENED')
		ORDER BY period_start DESC LIMIT 1
	`, tenantID, date).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("entry date is outside an open period: %w", err)
	}
	return periodID, nil
}

func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var p string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'JRN', 'JRN', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&p, &seq)
	if err != nil {
		return "", err
	}
	s := strconv.FormatInt(seq, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return p + "-" + strconv.FormatInt(int64(year), 10) + "-" + s, nil
}

func upsertHead(ctx context.Context, tx pgx.Tx, tenantID, lastJournalID int64, lastHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE
		SET last_journal_id = EXCLUDED.last_journal_id, last_hash = EXCLUDED.last_hash, updated_at = now()
	`, tenantID, lastJournalID, lastHash)
	return err
}

func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID int64, topic string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, topic, payload)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, topic, payload)
	return err
}

// postReconJournal builds, hashes, and inserts a reconciliation adjustment
// journal entry + lines, advancing the hash chain and writing the outbox event.
func postReconJournal(ctx context.Context, tx pgx.Tx, tenantID, uid int64, idem string, journal accounting.Journal) (int64, string, error) {
	head, err := lockOrSeedHead(ctx, tx, tenantID)
	if err != nil {
		return 0, "", err
	}
	journal.PreviousHash = head.LastHash
	journal.Hash = accounting.HashJournal(journal)

	periodID, err := resolvePeriod(ctx, tx, tenantID, journal.EntryDate)
	if err != nil {
		return 0, "", err
	}
	number, err := nextJournalNumber(ctx, tx, tenantID)
	if err != nil {
		return 0, "", err
	}
	var entryID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, tenantID, number, journal.EntryDate, periodID, journal.Description,
		journal.SourceRef, string(journal.IntentType), idem,
		journal.Hash, journal.PreviousHash, int8Value(uid)).Scan(&entryID)
	if err != nil {
		return 0, "", err
	}
	for _, line := range journal.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, tenantID, entryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
			return 0, "", err
		}
	}
	if err := upsertHead(ctx, tx, tenantID, entryID, journal.Hash); err != nil {
		return 0, "", err
	}
	outboxPayload, _ := json.Marshal(map[string]any{
		"journal_entry_id": entryID, "number": number, "intent": string(journal.IntentType),
	})
	if err := insertOutbox(ctx, tx, tenantID, "journal.posted", outboxPayload); err != nil {
		return 0, "", err
	}
	return entryID, number, nil
}

// resolveAccountByCode loads a postable account id by its COA code.
func resolveAccountByCode(ctx context.Context, tx pgx.Tx, tenantID int64, code string) (int64, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2`,
		tenantID, code).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("account %s not found: %w", code, err)
	}
	return accountID, nil
}
