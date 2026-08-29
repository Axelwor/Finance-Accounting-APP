// SET-001: tax master data (nama pajak, rate, akun COA penjualan/pembelian).
// The posting engines resolve VAT accounts through this master with the
// legacy hardcoded codes (2202/1203) as final fallback.
package tax

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/db"
) // TaxResponse is the /taxes payload.
type TaxResponse struct {
	ID             int64   `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Rate           float64 `json:"rate"`
	SalesAccountID *int64  `json:"sales_account_id"`
	PurchaseAccID  *int64  `json:"purchase_account_id"`
	IsActive       bool    `json:"is_active"`
}

type TaxRequest struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Rate           float64 `json:"rate"`
	SalesAccountID *int64  `json:"sales_account_id"`
	PurchaseAccID  *int64  `json:"purchase_account_id"`
}

func (service *Service) registerMasterDataRoutes(router chi.Router) {
	router.Get("/taxes", service.ListTaxes)
	router.Post("/taxes", service.CreateTax)
	router.Put("/taxes/{id}", service.UpdateTax)
	router.Delete("/taxes/{id}", service.DeactivateTax)
}

func (service *Service) ListTaxes(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	results := []TaxResponse{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, code, name, rate::float8, sales_account_id, purchase_account_id, is_active
			FROM taxes WHERE tenant_id = $1 ORDER BY code
		`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t TaxResponse
			if err := rows.Scan(&t.ID, &t.Code, &t.Name, &t.Rate, &t.SalesAccountID, &t.PurchaseAccID, &t.IsActive); err != nil {
				return err
			}
			results = append(results, t)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TAX_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

func (service *Service) CreateTax(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req TaxRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, message := validateTaxRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	var resp TaxResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			INSERT INTO taxes (tenant_id, code, name, rate, sales_account_id, purchase_account_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, code) DO UPDATE
			SET name = EXCLUDED.name, rate = EXCLUDED.rate,
			    sales_account_id = EXCLUDED.sales_account_id,
			    purchase_account_id = EXCLUDED.purchase_account_id,
			    is_active = true, updated_at = now()
			RETURNING id, code, name, rate::float8, sales_account_id, purchase_account_id, is_active
		`, tenant, req.Code, req.Name, req.Rate, req.SalesAccountID, req.PurchaseAccID).Scan(
			&resp.ID, &resp.Code, &resp.Name, &resp.Rate, &resp.SalesAccountID, &resp.PurchaseAccID, &resp.IsActive)
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TAX_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, resp)
}

func (service *Service) UpdateTax(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(request, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req TaxRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, message := validateTaxRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	var resp TaxResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			UPDATE taxes SET code = $3, name = $4, rate = $5,
			    sales_account_id = $6, purchase_account_id = $7, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, code, name, rate::float8, sales_account_id, purchase_account_id, is_active
		`, tenant, id, req.Code, req.Name, req.Rate, req.SalesAccountID, req.PurchaseAccID).Scan(
			&resp.ID, &resp.Code, &resp.Name, &resp.Rate, &resp.SalesAccountID, &resp.PurchaseAccID, &resp.IsActive)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "TAX_NOT_FOUND", "tax not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TAX_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, resp)
}

func (service *Service) DeactivateTax(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(request, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		tag, err := tx.Exec(request.Context(),
			`UPDATE taxes SET is_active = false, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
			tenant, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "TAX_NOT_FOUND", "tax not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TAX_DEACTIVATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "is_active": false})
}

func validateTaxRequest(req TaxRequest) (string, string) {
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		return "INVALID_REQUEST", "code and name are required"
	}
	if req.Rate < 0 || req.Rate > 100 {
		return "INVALID_REQUEST", "rate must be between 0 and 100"
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// Posting-time resolution (used by the invoice/supplier-invoice engines)
// ---------------------------------------------------------------------------

// ResolveVATAccounts returns the output (sales) and input (purchase) VAT
// account ids for a tax master row, or for the seeded PPN when taxID is nil.
// Resolution order: tax master -> tenant_settings default accounts is not
// applicable for VAT (it lives in the taxes master) -> legacy hardcoded codes.
func ResolveVATAccounts(ctx context.Context, tx pgx.Tx, tenant int64, taxID *int64, isSales bool) (int64, error) {
	if taxID != nil && *taxID > 0 {
		var accountID *int64
		var isActive bool
		query := `SELECT sales_account_id, is_active FROM taxes WHERE tenant_id = $1 AND id = $2`
		if !isSales {
			query = `SELECT purchase_account_id, is_active FROM taxes WHERE tenant_id = $1 AND id = $2`
		}
		if err := tx.QueryRow(ctx, query, tenant, *taxID).Scan(&accountID, &isActive); err != nil {
			return 0, err
		}
		if isActive && accountID != nil && *accountID > 0 {
			return *accountID, nil
		}
	}
	// Fall back to the active PPN master row, then the legacy codes.
	var accountID *int64
	query := `SELECT sales_account_id FROM taxes WHERE tenant_id = $1 AND code = 'PPN' AND is_active = true LIMIT 1`
	if !isSales {
		query = `SELECT purchase_account_id FROM taxes WHERE tenant_id = $1 AND code = 'PPN' AND is_active = true LIMIT 1`
	}
	if err := tx.QueryRow(ctx, query, tenant).Scan(&accountID); err != nil && err != pgx.ErrNoRows {
		return 0, err
	}
	if accountID != nil && *accountID > 0 {
		return *accountID, nil
	}
	code := ppnKeluaranCode
	if !isSales {
		code = ppnMasukanCode
	}
	return resolveAccountByCode(ctx, tx, tenant, code)
}
