package coa

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// category is the JSON representation of a categories row.
type category struct {
	ID                     int64  `json:"id"`
	Name                   string `json:"name"`
	Direction              string `json:"direction"`
	DefaultDebitAccountID  *int64 `json:"default_debit_account_id"`
	DefaultCreditAccountID *int64 `json:"default_credit_account_id"`
	IsActive               bool   `json:"is_active"`
}

// List returns all categories belonging to the tenant.
func (service *Service) ListCategories(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, name, direction, default_debit_account_id, default_credit_account_id, is_active
		FROM categories
		WHERE tenant_id = $1
		ORDER BY name, direction
	`, tenantID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "CATEGORIES_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	categories := make([]category, 0)
	for rows.Next() {
		var item category
		var debitID, creditID pgtype.Int8
		if err := rows.Scan(&item.ID, &item.Name, &item.Direction, &debitID, &creditID, &item.IsActive); err != nil {
			writeError(writer, http.StatusInternalServerError, "CATEGORIES_LIST_FAILED", err.Error())
			return
		}
		item.DefaultDebitAccountID = nullableInt64(debitID)
		item.DefaultCreditAccountID = nullableInt64(creditID)
		categories = append(categories, item)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "CATEGORIES_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, categories)
}

type createCategoryRequest struct {
	Name                   string `json:"name"`
	Direction              string `json:"direction"`
	DefaultDebitAccountID  int64  `json:"default_debit_account_id"`
	DefaultCreditAccountID int64  `json:"default_credit_account_id"`
}

// Create validates the payload and inserts a new category inside one
// transaction with the tenant id set transaction-locally so RLS passes.
func (service *Service) CreateCategory(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req createCategoryRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCategoryInput(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}

	var created category
	err = pgx.BeginFunc(request.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprintf("%d", tenantID)); err != nil {
			return err
		}
		if err := validateCategoryAccounts(request.Context(), tx, tenantID, req); err != nil {
			return err
		}
		return tx.QueryRow(request.Context(), `
			INSERT INTO categories (
				tenant_id, name, direction, default_debit_account_id, default_credit_account_id
			) VALUES ($1, $2, $3, $4, $5)
			RETURNING id, name, direction, default_debit_account_id, default_credit_account_id, is_active`,
			tenantID, req.Name, req.Direction,
			nullableInt8(nonZeroPtr(req.DefaultDebitAccountID)),
			nullableInt8(nonZeroPtr(req.DefaultCreditAccountID)),
		).Scan(&created.ID, &created.Name, &created.Direction, &created.DefaultDebitAccountID,
			&created.DefaultCreditAccountID, &created.IsActive)
	})
	if err != nil {
		writeCreateCategoryError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, created)
}

// validateCategoryAccounts ensures the default debit/credit accounts exist and
// belong to the same tenant. RLS is active inside the transaction, so the
// lookups are already tenant-scoped; the tenant_id filter is kept as an
// explicit guard.
func validateCategoryAccounts(ctx context.Context, tx pgx.Tx, tenantID int64, req createCategoryRequest) error {
	if req.DefaultDebitAccountID > 0 {
		if err := accountExists(ctx, tx, tenantID, req.DefaultDebitAccountID); err != nil {
			return err
		}
	}
	if req.DefaultCreditAccountID > 0 {
		if err := accountExists(ctx, tx, tenantID, req.DefaultCreditAccountID); err != nil {
			return err
		}
	}
	return nil
}

func accountExists(ctx context.Context, tx pgx.Tx, tenantID, accountID int64) error {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM accounts WHERE tenant_id = $1 AND id = $2)
	`, tenantID, accountID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return &parentValidationError{message: fmt.Sprintf("account %d does not belong to this tenant", accountID)}
	}
	return nil
}

func writeCreateCategoryError(writer http.ResponseWriter, err error) {
	var validationErr *parentValidationError
	if errors.As(err, &validationErr) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", validationErr.Error())
		return
	}
	if isUniqueViolation(err) {
		writeError(writer, http.StatusConflict, "CATEGORY_EXISTS", "a category with this name and direction already exists for the tenant")
		return
	}
	writeError(writer, http.StatusInternalServerError, "CATEGORY_CREATE_FAILED", err.Error())
}

// nonZeroPtr returns nil for zero values so empty JSON fields become NULL.
func nonZeroPtr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}
