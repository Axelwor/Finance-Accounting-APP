package customer

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/httperr"
)

// errorResponse is the JSON error envelope used by every customer handler.
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

// tenantID reads the tenant id from the authenticated request context. The
// auth middleware injects it from JWT claims.
func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

// pathID parses a positive integer path parameter.
func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation.
func isUniqueViolation(err error) bool {
	const pgErrCodeUniqueViolation = "23505"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// textOrNil converts a nullable pgtype.Text into a *string (nil when NULL).
func textOrNil(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	out := value.String
	return &out
}

// int8OrNil converts a nullable pgtype.Int8 into a *int64 (nil when NULL).
func int8OrNil(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}

// int4OrNil converts a nullable pgtype.Int4 into a *int32 (nil when NULL).
func int4OrNil(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	out := value.Int32
	return &out
}

// boolOrFalse dereferences a *bool, defaulting to false when nil.
func boolOrFalse(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

// int64OrZero dereferences a *int64, defaulting to 0 when nil.
func int64OrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// optionalDate parses an optional YYYY-MM-DD string into a pgtype.Date.
func optionalDate(raw *string) (pgtype.Date, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.Date{}, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*raw))
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: parsed, Valid: true}, nil
}

// isCheckViolation reports whether err is a PostgreSQL check_violation.
func isCheckViolation(err error) bool {
	const pgErrCodeCheckViolation = "23514"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeCheckViolation
}
