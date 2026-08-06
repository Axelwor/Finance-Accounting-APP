package coa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"finance-accounting-app/backend/internal/auth"
)

// errorResponse is the JSON error envelope used by every coa handler.
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

// pathID parses a positive integer path parameter (chi URLParam).
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

// isForeignKeyViolation reports whether err is a PostgreSQL foreign_key_violation.
func isForeignKeyViolation(err error) bool {
	const pgErrCodeForeignKeyViolation = "23503"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeForeignKeyViolation
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// normalizeCode strips surrounding whitespace from an account code.
func normalizeCode(code string) string {
	return strings.TrimSpace(code)
}

// validAccountCode rejects codes that could collide after whitespace
// normalization or that are empty.
func validAccountCode(code string) bool {
	normalized := normalizeCode(code)
	if normalized == "" {
		return false
	}
	return normalized == code && utf8.RuneCountInString(normalized) <= 64
}

// validReportGroups lists the report groups allowed by the schema CHECK
// constraint on accounts.report_group.
var validReportGroups = map[string]bool{
	"asset":     true,
	"liability": true,
	"equity":    true,
	"revenue":   true,
	"expense":   true,
}

func validReportGroup(group string) bool {
	return validReportGroups[group]
}

// validReportTypes lists the report types allowed by the schema CHECK
// constraint on report_mappings.report_type.
var validReportTypes = map[string]bool{
	"balance_sheet": true,
	"profit_loss":   true,
	"cash_flow":     true,
}

func validReportType(reportType string) bool {
	return validReportTypes[reportType]
}

// validDirections lists the directions allowed by the schema CHECK constraint
// on categories.direction.
var validDirections = map[string]bool{
	"IN":  true,
	"OUT": true,
}

func validDirection(direction string) bool {
	return validDirections[direction]
}

// validateAccountInput runs the pure validation for account creation and
// returns a stable error code plus a human readable message.
func validateAccountInput(req createAccountRequest) (string, string) {
	if req.Code == "" || req.Name == "" {
		return "INVALID_REQUEST", "code and name are required"
	}
	if !validAccountCode(req.Code) {
		return "INVALID_REQUEST", "code must be non-empty, trimmed, and at most 64 characters"
	}
	if !validReportGroup(req.ReportGroup) {
		return "INVALID_REQUEST", fmt.Sprintf("report_group must be one of asset, liability, equity, revenue, expense (got %q)", req.ReportGroup)
	}
	// Only detail (postable) accounts carry an account type; group accounts
	// must not. This mirrors the engine rule "akun grup tidak boleh diposting".
	if req.IsGroup {
		if req.AccountType != "" {
			return "INVALID_REQUEST", "group accounts must not set account_type"
		}
	} else if req.AccountType == "" {
		return "INVALID_REQUEST", "account_type is required for detail accounts"
	}
	if req.ValidFrom != nil && req.ValidTo != nil && req.ValidTo.Before(*req.ValidFrom) {
		return "INVALID_REQUEST", "valid_to must not be before valid_from"
	}
	return "", ""
}

// validateReportMappingInput runs the pure validation for report mapping
// creation and returns a stable error code plus a human readable message.
func validateReportMappingInput(req createReportMappingRequest) (string, string) {
	if req.AccountID <= 0 {
		return "INVALID_REQUEST", "account_id is required"
	}
	if req.ReportLine == "" {
		return "INVALID_REQUEST", "report_line is required"
	}
	if !validReportType(req.ReportType) {
		return "INVALID_REQUEST", fmt.Sprintf("report_type must be one of balance_sheet, profit_loss, cash_flow (got %q)", req.ReportType)
	}
	if req.Priority < 0 {
		return "INVALID_REQUEST", "priority must be a non-negative integer"
	}
	return "", ""
}

// validateCategoryInput runs the pure validation for category creation and
// returns a stable error code plus a human readable message.
func validateCategoryInput(req createCategoryRequest) (string, string) {
	if req.Name == "" {
		return "INVALID_REQUEST", "name is required"
	}
	if !validDirection(req.Direction) {
		return "INVALID_REQUEST", fmt.Sprintf("direction must be IN or OUT (got %q)", req.Direction)
	}
	if req.DefaultDebitAccountID == 0 && req.DefaultCreditAccountID == 0 {
		return "INVALID_REQUEST", "at least one of default_debit_account_id or default_credit_account_id is required"
	}
	if req.DefaultDebitAccountID < 0 || req.DefaultCreditAccountID < 0 {
		return "INVALID_REQUEST", "default account ids must be positive integers"
	}
	return "", ""
}
