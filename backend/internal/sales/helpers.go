package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// parseDate parses a YYYY-MM-DD string into a pgtype.Date (invalid when empty).
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

// errorResponse is the JSON error envelope used by every sales handler.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeJSON(request *http.Request, target any) error {
	// Buffer the body so it can be restored for downstream consumers (e.g.
	// the idempotency request-hash computed after decoding). Without this the
	// body is consumed here and the hash is always SHA-256("") — see M-023.
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return json.Unmarshal(bodyBytes, target)
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

// tenantID reads the tenant id injected by the auth middleware.
func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

// userID reads the acting user id injected by the auth middleware (0 if absent).
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

// withTenant scopes ROW LEVEL SECURITY to the tenant for the whole
// transaction. All of sales_quotations and sales_quotations_lines are FORCE RLS,
// so the app.tenant_id setting must be present before any SELECT/INSERT/UPDATE
// or the rows simply will not be visible.
func withTenant(ctx context.Context, tx pgx.Tx, tenantIDValue int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantIDValue, 10))
	return err
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation.
func isUniqueViolation(err error) bool {
	const pgErrCodeUniqueViolation = "23505"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation
}

// isForeignKeyViolation reports whether err is a PostgreSQL foreign_key_violation.
func isForeignKeyViolation(err error) bool {
	const pgErrCodeForeignKeyViolation = "23503"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeForeignKeyViolation
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// dateString renders a pgtype.Date as YYYY-MM-DD, or "" when not set.
func dateString(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

// textValue returns the string of a pgtype.Text ("" when not set).
func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

// pgtypeFloat converts a float64 into a pgtype.Numeric for NUMERIC columns.
// Uses string conversion (like inventory.pgtypeFloat) because pgtype.Numeric.Scan
// does not accept float64 directly.
func pgtypeFloat(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strings.TrimSpace(fmt.Sprintf("%g", v)))
	return n
}
