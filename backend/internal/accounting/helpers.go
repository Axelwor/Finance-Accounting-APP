package accounting

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

	"finance-accounting-app/backend/internal/approval"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
)

// Service exposes the accountant-mode endpoints (manual journals, general
// ledger, journal register). Tenant id and user id come from the auth
// middleware context (JWT claims).
type Service struct {
	pool *pgxpool.Pool
	gate *approval.Gate
}

// NewHandler builds the accounting Service backed by the given pool.
func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, gate: approval.NewGate(pool)}
}

// ---------------------------------------------------------------------------
// Shared helpers (mirror cash/helpers.go and purchase/helpers.go)
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
	message = httperr.SanitizeMessage(status, code, message)
	writeJSON(writer, status, errorResponse{Code: code, Message: message})
}

// tenantID reads the tenant id from the authenticated request context.
func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

// pathID parses a positive integer path parameter.
func pathID(request *http.Request, key string) (int64, error) {
	raw := chi.URLParam(request, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return id, nil
}

// idempotencyKey validates the required Idempotency-Key header.
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

// entryDate normalizes a date string to YYYY-MM-DD.
func entryDate(raw string) (string, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("entry_date must be YYYY-MM-DD")
	}
	return parsed.Format("2006-01-02"), nil
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func text(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func int8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func uuidValue(raw string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(raw)
	return value
}

func parseDate(raw string) pgtype.Date {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

// accountRow is the minimal account shape the accountant endpoints need.
type accountRow struct {
	ID          int64
	Code        string
	Name        string
	ReportGroup string
	AccountType string
	IsGroup     bool
	IsActive    bool
}

// loadAccount reads one account for the tenant inside the transaction.
func loadAccount(ctx context.Context, tx pgx.Tx, tenantID, accountID int64) (accountRow, error) {
	var row accountRow
	err := tx.QueryRow(ctx, `
		SELECT id, code, name, report_group, account_type, is_group, is_active
		FROM accounts
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, accountID).Scan(&row.ID, &row.Code, &row.Name, &row.ReportGroup, &row.AccountType, &row.IsGroup, &row.IsActive)
	if err != nil {
		return accountRow{}, err
	}
	return row, nil
}

// nextJournalNumber allocates the next JRN-{year}-{seq} number for the
// tenant inside the posting transaction. All journal kinds share the
// single 'JRN' sequence so numbers never collide.
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
	return prefix + "-" + strconv.FormatInt(int64(year), 10) + "-" + leftPad6(seq), nil
}

// leftPad6 zero-pads a sequence number to 6 digits (e.g. 42 -> "000042").
func leftPad6(seq int64) string {
	s := strconv.FormatInt(seq, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

// resolvePeriod finds the OPEN/REOPENED accounting period containing the
// entry date. Failures wrap ErrEntryDateOutsideOpenPeriod so handlers can
// return 422 ENTRY_DATE_OUTSIDE_OPEN_PERIOD (QA-08) instead of a generic 500.
func resolvePeriod(ctx context.Context, tx pgx.Tx, tenantID int64, date string) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounting_periods
		WHERE tenant_id = $1 AND status IN ('OPEN','REOPENED')
		  AND period_start <= $2::date AND period_end >= $2::date
		ORDER BY period_start DESC
		LIMIT 1
	`, tenantID, date).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("entry date is outside an open period: %w", ErrEntryDateOutsideOpenPeriod)
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

// insertOutbox writes a journal.posted event in the same transaction as
// the journal (outbox pattern).
func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID int64, topic string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, topic, payload)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, topic, payload)
	return err
}

// computeHash reproduces the engine's canonical hash for a journal so the
// chain is tamper-evident. Mirrors accounting.hashJournal but allows the
// real previous hash from ledger_chain_heads.
func computeHash(journal Journal) string {
	return hashJournal(journal)
}

// scopeTenant sets the RLS tenant for the whole transaction.
func scopeTenant(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10))
	return err
}

// lockChainHead returns the tenant ledger chain head, seeding a genesis
// row on the first posting.
func lockChainHead(ctx context.Context, tx pgx.Tx, tenantID int64) (db.LedgerChainHead, error) {
	head, err := db.New(tx).LockLedgerChainHead(ctx, tenantID)
	if err == nil {
		return head, nil
	}
	if !isNoRows(err) {
		return db.LedgerChainHead{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, NULL, 'genesis')
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID); err != nil {
		return db.LedgerChainHead{}, err
	}
	return db.New(tx).LockLedgerChainHead(ctx, tenantID)
}
