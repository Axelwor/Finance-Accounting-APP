package production

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

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
)

// Service exposes the production/job-order endpoints (BOM, production jobs,
// job costs, job completion). Tenant id and user id come from the auth
// middleware context (JWT claims).
type Service struct {
	pool *pgxpool.Pool
}

// Shared account codes (seeded by migration 000020 / auth seed).
const (
	wipAccountCode           = "1303" // Work in Progress
	finishedGoodsAccountCode = "1304" // Finished Goods
	inventoryAccountCode     = "1301" // Inventory (raw materials)
	appliedOverheadCode      = "4902" // Applied Overhead (M-010)
	varianceGainAccountCode  = "4908" // Production Variance Gain (M-012)
	varianceLossAccountCode  = "5908" // Production Variance Loss (M-012)
)

// NewHandler builds the production Service backed by the given pool.
func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Routes registers the production endpoints on the given router.
func (service *Service) Routes(router chi.Router) {
	// Bill of Materials (US-070).
	router.Post("/bill-of-materials", service.CreateBOM)
	router.Get("/bill-of-materials", service.ListBOMs)
	router.Get("/bill-of-materials/{id}", service.GetBOM)

	// Production jobs (US-070..072).
	router.Post("/production-jobs", service.CreateProductionJob)
	router.Get("/production-jobs", service.ListProductionJobs)
	router.Get("/production-jobs/{id}", service.GetProductionJob)
	router.Post("/production-jobs/{id}/costs", service.AddProductionJobCost)
	router.Post("/production-jobs/{id}/complete", service.CompleteProductionJob)

	// M-012: Overhead variance recognition at period close.
	router.Post("/overhead-variance", service.PostOverheadVariance)
}

// ---------------------------------------------------------------------------
// Helpers (shared across production handlers — mirrors purchase/helpers.go)
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

// userID reads the user id from the authenticated request context.
func userID(request *http.Request) int64 {
	value, _ := auth.UserIDFromContext(request.Context())
	return value
}

// pathID parses a positive integer path parameter.
func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

// withTenant sets the RLS tenant for the whole transaction.
func withTenant(ctx context.Context, tx pgx.Tx, tenantIDValue int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantIDValue, 10))
	return err
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

func optionalInt8(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func textValueOptional(raw string) pgtype.Text {
	return pgtype.Text{String: raw, Valid: strings.TrimSpace(raw) != ""}
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
	return strings.TrimSpace(value.String)
}

// numericToFloat converts a pgtype.Numeric to float64 (0 when not set).
func numericToFloat(value pgtype.Numeric) float64 {
	if !value.Valid {
		return 0
	}
	f, err := value.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}

// pgtypeFloat converts a float64 into a pgtype.Numeric for NUMERIC columns.
func pgtypeFloat(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strings.TrimSpace(fmt.Sprintf("%g", v)))
	return n
}

// resolveAccountByCode loads a single account id by its code (tenant-scoped
// via RLS). Returns an error when the account does not exist.
func resolveAccountByCode(ctx context.Context, tx pgx.Tx, tenantID int64, code string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2
	`, tenantID, code).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("account %s not found: %w", code, err)
	}
	return id, nil
}

// nextDocNumber allocates the next document number for a tenant/doc type:
// {PREFIX}-{YYYY}-{6-digit seq}. It upserts a doc_numbering row so the
// sequence is monotonic per tenant/doc/year.
func nextDocNumber(ctx context.Context, tx pgx.Tx, tenantID int64, docType, prefix string) (string, error) {
	year := time.Now().UTC().Year()
	var p string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year)
		DO UPDATE SET last_seq = document_numbering.last_seq + 1
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

// idempotencyKey validates the required Idempotency-Key header (UUID).
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

// ---------------------------------------------------------------------------
// Journal posting helpers (mirror purchase/grn.go).
// ---------------------------------------------------------------------------

// lockOrSeedHead returns the tenant ledger chain head, seeding a genesis row
// on the first posting.
func lockOrSeedHead(ctx context.Context, tx pgx.Tx, tenantID int64) (db.LedgerChainHead, error) {
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

// upsertHead advances the ledger chain head to the new entry.
func upsertHead(ctx context.Context, tx pgx.Tx, tenantID, entryID int64, hash string) error {
	_, err := tx.Exec(ctx, `
		UPDATE ledger_chain_heads
		SET last_journal_id = $1, last_hash = $2
		WHERE tenant_id = $3
	`, entryID, hash, tenantID)
	return err
}

// resolvePeriod finds the OPEN accounting period containing entryDate.
func resolvePeriod(ctx context.Context, tx pgx.Tx, tenantID int64, entryDate string) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounting_periods
		WHERE tenant_id = $1 AND status = 'OPEN'
		  AND $2::date BETWEEN period_start AND period_end
		ORDER BY period_start DESC
		LIMIT 1
	`, tenantID, entryDate).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("no open accounting period for %s: %w", entryDate, err)
	}
	return periodID, nil
}

// nextJournalNumber allocates the next JE-{YYYY}-{6-digit seq} number.
func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	return nextDocNumber(ctx, tx, tenantID, "JE", "JE")
}

// insertOutbox writes an outbox event for downstream projection.
func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID int64, topic string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, topic, payload)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, topic, payload)
	return err
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}
