package tax

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
	"finance-accounting-app/backend/internal/httperr"
)

// Service exposes the tax endpoints: PPN reconciliation, PPh Final UMKM,
// ECL (penyisihan piutang), and deferred tax. Tenant id and user id come
// from the auth middleware context (JWT claims).
type Service struct {
	pool *pgxpool.Pool
}

// NewHandler builds the tax Service backed by the given pool.
func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Routes registers the tax endpoints on the given sub-router. Mirrors the
// purchase/reconciliation handler pattern.
func (service *Service) Routes(router chi.Router) {
	// PPN (US-080)
	router.Get("/ppn/summary", service.PPNSummary)
	router.Get("/ppn/reconciliation", service.PPNReconciliation)
	router.Post("/ppn/reconcile", service.CreatePPNReconciliation)

	// PPh Final UMKM (US-081)
	router.Post("/pph-final/calculate", service.CalculatePPhFinal)
	router.Post("/pph-final/pay", service.PayPPhFinal)

	// ECL / Penyisihan Piutang (US-082)
	router.Post("/ecl/calculate", service.CalculateECL)
	router.Post("/ecl/write-off", service.WriteOffReceivable)

	// Deferred Tax (US-083)
	router.Post("/deferred-tax/calculate", service.CalculateDeferredTax)
}

// ---------------------------------------------------------------------------
// Shared account codes (seeded by migration 000021 / seed.go).
// ---------------------------------------------------------------------------

const (
	// Existing VAT accounts (seeded earlier by migrations 000008 / 000011).
	ppnKeluaranCode = "2202" // VAT Payable (PPN keluaran — output VAT)
	ppnMasukanCode  = "1203" // Input VAT (PPN masukan)
	salesCode       = "4101" // Sales Revenue (PPh Final base)
	cashCode        = "1101" // Cash (default tax settlement account)
	arAccountCode   = "1201" // Accounts Receivable (ECL base / write-off credit)

	// New accounts seeded by migration 000021.
	pphPayableAccountCode  = "2203" // Income Tax Payable
	pphExpenseAccountCode  = "5208" // Income Tax Expense
	allowanceAccountCode   = "1202" // Allowance for Doubtful Accounts
	badDebtExpenseCode     = "5209" // Bad Debt Expense
	badDebtRecoveryCode    = "4906" // Bad Debt Recovery
	deferredTaxAssetCode   = "1206" // Deferred Tax Asset
	deferredTaxExpenseCode = "5904" // Deferred Tax Expense
)

// Sentinel errors used by the ECL / PPh handlers.
var (
	errNoAdjustment = errors.New("ECL target equals current allowance; no journal posted")
	errNoOpenPeriod = errors.New("entry date is outside an open accounting period")
)

// ---------------------------------------------------------------------------
// Shared helpers (mirror purchase/helpers.go and accounting/helpers.go)
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

// userID reads the user id from the authenticated request context (0 if absent).
func userID(request *http.Request) int64 {
	uid, _ := auth.UserIDFromContext(request.Context())
	return uid
}

// userIDFromCtx reads the user id from a plain context (0 if absent). Used by
// the posting helpers that receive a context rather than a *http.Request.
func userIDFromCtx(ctx context.Context) int64 {
	uid, _ := auth.UserIDFromContext(ctx)
	return uid
}

// journalHeader is the lightweight projection of a stored journal entry used
// to rebuild response payloads on idempotent replay.
type journalHeader struct {
	id          int64
	number      string
	intentType  string
	entryDate   string
	description string
}

// fetchJournalHeader loads the header fields of a stored journal entry. Used
// after postJournal returns to surface the persisted description/intent when
// the call was an idempotent replay.
func fetchJournalHeader(ctx context.Context, tx pgx.Tx, tenant, entryID int64) (journalHeader, error) {
	var h journalHeader
	var intentType, desc pgtype.Text
	var entryDate pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT id, number, COALESCE(intent_type,''), entry_date, COALESCE(description,'')
		FROM journal_entries WHERE tenant_id = $1 AND id = $2
	`, tenant, entryID).Scan(&h.id, &h.number, &intentType, &entryDate, &desc)
	if err != nil {
		return journalHeader{}, err
	}
	h.intentType = textValueTrimmed(intentType)
	h.entryDate = dateValuePG(entryDate)
	h.description = textValueTrimmed(desc)
	return h, nil
}

// idempotencyKey validates the required Idempotency-Key header (must be a UUID).
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

func int8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

// textValuePG wraps a string into a nullable pgtype.Text (NULL for empty).
func textValuePG(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

// withTenant sets the RLS tenant for the whole transaction.
func withTenant(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10))
	return err
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// isCheckViolation reports whether err is a PostgreSQL CHECK constraint
// violation (SQLSTATE 23514). Used by the ECL write-off to fall back to the
// 'VOID' status when the 'WRITTEN_OFF' status has not been migrated yet.
func isCheckViolation(err error) bool {
	const pgErrCodeCheckViolation = "23514"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeCheckViolation
}

// normalizeDate trims and validates a YYYY-MM-DD date; returns "" when the
// input is blank or unparseable (treated as an open bound by callers).
func normalizeDate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", trimmed); err != nil {
		return ""
	}
	return trimmed
}

// validDate checks that raw is a parseable YYYY-MM-DD.
func validDate(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	_, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	return err == nil
}

// ---------------------------------------------------------------------------
// Journal posting helpers (mirror purchase/grn.go and cash/journal.go)
// ---------------------------------------------------------------------------

// postedEntry carries the inserted journal entry id and its human number.
type postedEntry struct {
	ID     int64
	Number string
}

// postJournal inserts a journal entry + its lines inside an existing
// transaction. The caller has already scoped RLS, handled idempotent replay,
// and locked the chain head is NOT done here — this helper performs the
// idempotency check, head lock, journal insert, chain-head advance, and outbox
// write. Mirrors accounting/posting.go but accepts an already-built journal.
//
// Returns the inserted entry id and number. On an idempotent replay (the
// Idempotency-Key was already used) it returns the original entry.
func postJournal(ctx context.Context, tx pgx.Tx, tenant int64, idem string, journal accounting.Journal, uid int64) (postedEntry, error) {
	// Idempotent replay: an identical retry returns the stored journal.
	existing, err := db.New(tx).GetJournalByIdempotencyKey(ctx, db.GetJournalByIdempotencyKeyParams{
		TenantID:       tenant,
		IdempotencyKey: uuidValue(idem),
	})
	if err == nil {
		return postedEntry{ID: existing.ID, Number: existing.Number}, nil
	} else if !isNoRows(err) {
		return postedEntry{}, err
	}

	// Lock the chain head so concurrent postings serialize on one row.
	head, err := lockOrSeedHead(ctx, tx, tenant)
	if err != nil {
		return postedEntry{}, err
	}
	journal.TenantID = tenant
	journal.PreviousHash = head.LastHash
	journal.Hash = hashJournal(journal)

	periodID, err := resolvePeriod(ctx, tx, tenant, journal.EntryDate)
	if err != nil {
		return postedEntry{}, err
	}
	number, err := nextJournalNumber(ctx, tx, tenant)
	if err != nil {
		return postedEntry{}, err
	}

	entry, err := db.New(tx).InsertJournalEntry(ctx, db.InsertJournalEntryParams{
		TenantID:       tenant,
		Number:         number,
		EntryDate:      parseDatePG(journal.EntryDate),
		PeriodID:       periodID,
		Description:    textValuePG(journal.Description),
		SourceRef:      textValuePG(journal.SourceRef),
		IntentType:     textValuePG(string(journal.IntentType)),
		IdempotencyKey: uuidValue(idem),
		Hash:           journal.Hash,
		PrevHash:       journal.PreviousHash,
		CreatedBy:      int8Value(uid),
	})
	if err != nil {
		return postedEntry{}, err
	}
	for _, line := range journal.Lines {
		if err := db.New(tx).InsertJournalLine(ctx, db.InsertJournalLineParams{
			TenantID:      tenant,
			EntryID:       entry.ID,
			AccountID:     line.AccountID,
			DebitCents:    line.DebitCents,
			CreditCents:   line.CreditCents,
			Description:   textValuePG(line.SourceLineRef),
			SourceLineRef: textValuePG(line.SourceLineRef),
			DimensionIds:  []byte("[]"),
		}); err != nil {
			return postedEntry{}, err
		}
	}
	if err := upsertHead(ctx, tx, tenant, entry.ID, journal.Hash); err != nil {
		return postedEntry{}, err
	}
	if err := insertOutbox(ctx, tx, tenant, "tax.posted", mustJSON(map[string]any{
		"journal_id": entry.ID, "number": number, "intent": string(journal.IntentType),
	})); err != nil {
		return postedEntry{}, err
	}
	return postedEntry{ID: entry.ID, Number: number}, nil
}

// parseDatePG converts a YYYY-MM-DD string into a pgtype.Date (zero when blank).
func parseDatePG(raw string) pgtype.Date {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

// dateValuePG converts a pgtype.Date to a YYYY-MM-DD string (empty if invalid).
func dateValuePG(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// textValueTrimmed returns the trimmed string of a pgtype.Text (empty if invalid).
func textValueTrimmed(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return strings.TrimSpace(t.String)
}

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
		return 0, fmt.Errorf("%w: %v", errNoOpenPeriod, err)
	}
	return periodID, nil
}

func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	return nextDocNumber(ctx, tx, tenantID, "JRN", "JRN")
}

func nextDocNumber(ctx context.Context, tx pgx.Tx, tenantID int64, docType, prefix string) (string, error) {
	year := time.Now().Year()
	var p string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, docType, prefix, year).Scan(&p, &seq)
	if err != nil {
		return "", err
	}
	return p + "-" + strconv.FormatInt(int64(year), 10) + "-" + leftPad6(seq), nil
}

func leftPad6(seq int64) string {
	s := strconv.FormatInt(seq, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
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

func hashJournal(journal accounting.Journal) string {
	return accounting.HashJournal(journal)
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// resolveAccountByCode loads the account id for a tenant by code. Used to
// resolve the seeded tax accounts at posting time.
func resolveAccountByCode(ctx context.Context, tx pgx.Tx, tenantID int64, code string) (int64, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2`,
		tenantID, code).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("account %s not found: %w", code, err)
	}
	return accountID, nil
}
