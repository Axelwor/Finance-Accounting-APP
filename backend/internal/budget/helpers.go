package budget

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

// Service exposes the report-framework, dimension, and budget endpoints.
// Tenant id and user id come from the auth middleware context (JWT claims).
type Service struct {
	pool *pgxpool.Pool
}

// NewHandler builds the budget Service backed by the given pool.
func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Routes registers the budget endpoints on the given sub-router. Mirrors the
// purchase/notes handler pattern.
func (service *Service) Routes(router chi.Router) {
	// Report frameworks (US-090A)
	router.Get("/report-frameworks", service.ListFrameworks)
	router.Post("/report-frameworks", service.SetFramework)

	// Dimensions (US-093)
	router.Post("/dimensions", service.CreateDimension)
	router.Get("/dimensions", service.ListDimensions)
	router.Post("/journal-lines/{id}/dimensions", service.TagJournalLine)

	// Budgets (US-093)
	router.Post("/budgets", service.CreateBudget)
	router.Get("/budgets", service.ListBudgets)
	router.Get("/budgets/{id}", service.GetBudget)
	router.Get("/budgets/{id}/vs-actual", service.BudgetVsActual)
}

// ---------------------------------------------------------------------------
// Shared helpers (mirror notes/helpers.go and purchase/helpers.go)
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

// optionalInt parses a query string into an int pointer; empty -> NULL.
func optionalInt(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return value
}

// optionalInt64 parses a query string into an int64 pointer; empty/invalid -> NULL.
func optionalInt64(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil
	}
	return value
}

// validFramework reports whether raw is a recognised report framework code.
func validFramework(raw string) bool {
	switch raw {
	case "EMKM", "ETAP", "SAK_UMUM":
		return true
	}
	return false
}

// validDimensionType reports whether raw is a recognised dimension type.
func validDimensionType(raw string) bool {
	switch raw {
	case "branch", "project", "department", "cost_center":
		return true
	}
	return false
}
