package coa

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// account is the JSON representation of an accounts row.
type account struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	ReportGroup string     `json:"report_group"`
	AccountType string     `json:"account_type"`
	ParentID    *int64     `json:"parent_id"`
	IsGroup     bool       `json:"is_group"`
	IsActive    bool       `json:"is_active"`
	ValidFrom   *time.Time `json:"valid_from"`
	ValidTo     *time.Time `json:"valid_to"`
}

// accountRow is the raw row shape used when scanning pgx results.
type accountRow struct {
	ID          int64
	Code        string
	Name        string
	ReportGroup string
	AccountType string
	ParentID    pgtype.Int8
	IsGroup     bool
	IsActive    bool
	ValidFrom   pgtype.Date
	ValidTo     pgtype.Date
}

func (row accountRow) toJSON() account {
	parentID := nullableInt64(row.ParentID)
	return account{
		ID:          row.ID,
		Code:        row.Code,
		Name:        row.Name,
		ReportGroup: row.ReportGroup,
		AccountType: row.AccountType,
		ParentID:    parentID,
		IsGroup:     row.IsGroup,
		IsActive:    row.IsActive,
		ValidFrom:   nullableTime(row.ValidFrom),
		ValidTo:     nullableTime(row.ValidTo),
	}
}

const accountColumns = `id, code, name, report_group, account_type, parent_id, is_group, is_active, valid_from, valid_to`

// List returns all accounts belonging to the tenant.
func (service *Service) List(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT `+accountColumns+`
		FROM accounts
		WHERE tenant_id = $1
		ORDER BY code
	`, tenantID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ACCOUNTS_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	accounts := make([]account, 0)
	for rows.Next() {
		var row accountRow
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.ReportGroup, &row.AccountType,
			&row.ParentID, &row.IsGroup, &row.IsActive, &row.ValidFrom, &row.ValidTo); err != nil {
			writeError(writer, http.StatusInternalServerError, "ACCOUNTS_LIST_FAILED", err.Error())
			return
		}
		accounts = append(accounts, row.toJSON())
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "ACCOUNTS_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, accounts)
}

type createAccountRequest struct {
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	ReportGroup string     `json:"report_group"`
	AccountType string     `json:"account_type"`
	ParentID    *int64     `json:"parent_id"`
	IsGroup     bool       `json:"is_group"`
	ValidFrom   *time.Time `json:"valid_from"`
	ValidTo     *time.Time `json:"valid_to"`
}

// Create validates the payload and inserts a new account inside one
// transaction with the tenant id set transaction-locally so RLS passes.
func (service *Service) Create(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req createAccountRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateAccountInput(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	req.Code = normalizeCode(req.Code)

	var created accountRow
	err = pgx.BeginFunc(request.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprintf("%d", tenantID)); err != nil {
			return err
		}
		if err := validateParent(request.Context(), tx, tenantID, req.ParentID); err != nil {
			return err
		}
		return tx.QueryRow(request.Context(), `
			INSERT INTO accounts (
				tenant_id, code, name, report_group, account_type, parent_id, is_group, valid_from, valid_to
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING `+accountColumns,
			tenantID, req.Code, req.Name, req.ReportGroup, req.AccountType, nullableInt8(req.ParentID),
			req.IsGroup, nullableDate(req.ValidFrom), nullableDate(req.ValidTo),
		).Scan(&created.ID, &created.Code, &created.Name, &created.ReportGroup, &created.AccountType,
			&created.ParentID, &created.IsGroup, &created.IsActive, &created.ValidFrom, &created.ValidTo)
	})
	if err != nil {
		writeCreateAccountError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, created.toJSON())
}

// validateParent ensures the parent account exists and belongs to the same
// tenant. RLS is active inside the transaction, so the lookup is already
// tenant-scoped; the tenant_id filter is kept as an explicit guard.
func validateParent(ctx context.Context, tx pgx.Tx, tenantID int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM accounts WHERE tenant_id = $1 AND id = $2
		)
	`, tenantID, *parentID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("parent account %d does not belong to this tenant", *parentID)
	}
	return nil
}

func writeCreateAccountError(writer http.ResponseWriter, err error) {
	var validationErr *parentValidationError
	if errors.As(err, &validationErr) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", validationErr.Error())
		return
	}
	if isUniqueViolation(err) {
		writeError(writer, http.StatusConflict, "ACCOUNT_CODE_EXISTS", "an account with this code already exists for the tenant")
		return
	}
	writeError(writer, http.StatusInternalServerError, "ACCOUNT_CREATE_FAILED", err.Error())
}

type parentValidationError struct {
	message string
}

func (err *parentValidationError) Error() string { return err.message }

// Deactivate sets is_active = false unless the account has journal history.
func (service *Service) Deactivate(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	accountID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	err = pgx.BeginFunc(request.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprintf("%d", tenantID)); err != nil {
			return err
		}
		// Touch the row inside the RLS-scoped transaction so it is visible to
		// the reference checks below, and reject non-existent accounts.
		var touched int64
		if err := tx.QueryRow(request.Context(), `
			SELECT id FROM accounts WHERE id = $1 FOR UPDATE
		`, accountID).Scan(&touched); err != nil {
			return err
		}
		if err := ensureNoJournalHistory(request.Context(), tx, accountID); err != nil {
			return err
		}
		_, err := tx.Exec(request.Context(), `
			UPDATE accounts SET is_active = false WHERE id = $1
		`, accountID)
		return err
	})
	if err != nil {
		writeDeactivateError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": accountID, "is_active": false})
}

// ensureNoJournalHistory blocks deactivation when the account is referenced by
// any journal line, per US-005 (accounts with transaction history can only be
// deactivated, not deleted).
func ensureNoJournalHistory(ctx context.Context, tx pgx.Tx, accountID int64) error {
	var referenced bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM journal_lines WHERE account_id = $1
		)
	`, accountID).Scan(&referenced)
	if err != nil {
		return err
	}
	if referenced {
		return &parentValidationError{message: "account has journal history and cannot be deactivated"}
	}
	return nil
}

func writeDeactivateError(writer http.ResponseWriter, err error) {
	if isNoRows(err) {
		writeError(writer, http.StatusNotFound, "ACCOUNT_NOT_FOUND", "account does not exist")
		return
	}
	var validationErr *parentValidationError
	if errors.As(err, &validationErr) {
		writeError(writer, http.StatusConflict, "ACCOUNT_HAS_JOURNAL_HISTORY", validationErr.Error())
		return
	}
	writeError(writer, http.StatusInternalServerError, "ACCOUNT_DEACTIVATE_FAILED", err.Error())
}

// nullableInt8 converts a *int64 to a pgtype.Int8.
func nullableInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

// nullableInt64 converts a pgtype.Int8 to a *int64 (nil when NULL).
func nullableInt64(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

// nullableDate converts a *time.Time to a pgtype.Date.
func nullableDate(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *value, Valid: true}
}

// nullableTime converts a pgtype.Date to a *time.Time (nil when NULL).
func nullableTime(value pgtype.Date) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
