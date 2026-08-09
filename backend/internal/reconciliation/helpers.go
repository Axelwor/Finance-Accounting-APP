package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
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
