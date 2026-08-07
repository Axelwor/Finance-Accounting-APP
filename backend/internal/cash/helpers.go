package cash

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// errorResponse is the JSON error envelope used by every cash handler.
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

// tenantID reads the tenant id from the authenticated request context. The
// auth middleware injects it from JWT claims.
func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

// idempotencyKey validates the required Idempotency-Key header and returns it
// as a pgtype.UUID so it round-trips into the idempotency_key column. Invalid
// keys are rejected with a stable error code.
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

// pathID parses a positive integer path parameter.
func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

// entryDate normalizes the date string to YYYY-MM-DD.
func entryDate(raw string) (string, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("entry_date must be a valid date in YYYY-MM-DD format")
	}
	return parsed.Format("2006-01-02"), nil
}

// validateAmount rejects non-positive amounts (cents are integer per engine).
func validateAmount(amount int64) error {
	if amount <= 0 {
		return errors.New("amount_cents must be a positive integer")
	}
	return nil
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

// accountForEngine converts a loaded account row into the pure engine shape.
func accountForEngine(row db.Account) accounting.Account {
	return accounting.Account{
		ID:          row.ID,
		ReportGroup: row.ReportGroup,
		Type:        accounting.AccountType(row.AccountType),
		IsGroup:     row.IsGroup,
		IsActive:    row.IsActive,
	}
}

// equityAccountCode is the seeded "Modal" account used as the default equity
// plug target for opening balances. The seed provisions it on registration.
const equityAccountCode = "3101"

// resolveEquityAccount returns the tenant's seeded equity account id, so
// opening balances can be posted without the client knowing its own ids.
func resolveEquityAccount(ctx context.Context, tx pgx.Tx, tenantID int64) (int64, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM accounts
		WHERE tenant_id = $1 AND code = $2
	`, tenantID, equityAccountCode).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("seeded equity account %s not found: %w", equityAccountCode, err)
	}
	return accountID, nil
}

// errorFor maps a posting error to the HTTP status/error code/message.
func errorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "ACCOUNT_NOT_FOUND", "account does not exist for this tenant"
	}
	if isUniqueViolation(err) {
		if isIdempotencyViolation(err) {
			return http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "this Idempotency-Key was already used"
		}
		if isIntentViolation(err) {
			return http.StatusConflict, "DUPLICATE_INTENT", "a journal with this source_ref and intent_type already exists"
		}
		return http.StatusConflict, "JOURNAL_POST_FAILED", "a conflicting journal already exists"
	}
	if isForeignKeyViolation(err) {
		return http.StatusConflict, "JOURNAL_POST_FAILED", "journal references a resource that does not exist for this tenant"
	}
	if errors.Is(err, pgx.ErrTxClosed) {
		return http.StatusInternalServerError, "JOURNAL_POST_FAILED", "transaction failed"
	}
	return http.StatusInternalServerError, "JOURNAL_POST_FAILED", err.Error()
}

// isIdempotencyViolation reports whether a unique violation came from the
// journal_entries_idempotency_unique index.
func isIdempotencyViolation(err error) bool {
	return violationConstraint(err, "journal_entries_idempotency_unique")
}

// isIntentViolation reports whether a unique violation came from the
// journal_entries_intent_unique index (tenant_id, source_ref, intent_type).
func isIntentViolation(err error) bool {
	return violationConstraint(err, "journal_entries_intent_unique")
}

func violationConstraint(err error, name string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.ConstraintName == name || strings.Contains(pgErr.Message, name)
}
