// Package itemmaster exposes the item master data endpoints (SET-001):
// units (satuan), item categories (kategori barang), and item brands
// (merek barang). All three follow the warehouse CRUD handler pattern.
package itemmaster

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Post("/units", service.CreateUnit)
	router.Put("/units/{id}", service.UpdateUnit)
	router.Delete("/units/{id}", service.DeactivateUnit)

	router.Post("/item-categories", service.CreateCategory)
	router.Put("/item-categories/{id}", service.UpdateCategory)
	router.Delete("/item-categories/{id}", service.DeactivateCategory)

	router.Post("/item-brands", service.CreateBrand)
	router.Put("/item-brands/{id}", service.UpdateBrand)
	router.Delete("/item-brands/{id}", service.DeactivateBrand)
}

// RegisterReadRoutes mounts the list endpoints every authenticated user needs
// (item pickers resolve unit/category/brand names for all roles).
func (service *Service) RegisterReadRoutes(router chi.Router) {
	router.Get("/units", service.ListUnits)
	router.Get("/item-categories", service.ListCategories)
	router.Get("/item-brands", service.ListBrands)
}

// ---------------------------------------------------------------------------
// Payloads
// ---------------------------------------------------------------------------

type UnitRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	DecimalPlaces int    `json:"decimal_places"`
}

type UnitResponse struct {
	ID            int64  `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	DecimalPlaces int    `json:"decimal_places"`
	IsActive      bool   `json:"is_active"`
}

type NameRequest struct {
	Name string `json:"name"`
}

type NameResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// ---------------------------------------------------------------------------
// Units
// ---------------------------------------------------------------------------

func (service *Service) ListUnits(writer http.ResponseWriter, request *http.Request) {
	listMaster(writer, request, service.pool, `
		SELECT id, code, name, decimal_places, is_active FROM units
		WHERE tenant_id = $1 AND (is_active = true OR $2)
		ORDER BY name
	`, scanUnit)
}

func scanUnit(rows pgx.Rows) (any, error) {
	var u UnitResponse
	if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.DecimalPlaces, &u.IsActive); err != nil {
		return nil, err
	}
	return u, nil
}

func (service *Service) CreateUnit(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var req UnitRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "code and name are required")
		return
	}
	if req.DecimalPlaces < 0 || req.DecimalPlaces > 4 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "decimal_places must be between 0 and 4")
		return
	}
	var resp UnitResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			INSERT INTO units (tenant_id, code, name, decimal_places)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, name) DO UPDATE
			SET code = EXCLUDED.code, decimal_places = EXCLUDED.decimal_places,
			    is_active = true, updated_at = now()
			RETURNING id, code, name, decimal_places, is_active
		`, tenant, req.Code, req.Name, req.DecimalPlaces).Scan(
			&resp.ID, &resp.Code, &resp.Name, &resp.DecimalPlaces, &resp.IsActive)
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "UNIT_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, resp)
}

func (service *Service) UpdateUnit(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req UnitRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "code and name are required")
		return
	}
	if req.DecimalPlaces < 0 || req.DecimalPlaces > 4 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "decimal_places must be between 0 and 4")
		return
	}
	var resp UnitResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			UPDATE units SET code = $3, name = $4, decimal_places = $5, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, code, name, decimal_places, is_active
		`, tenant, id, req.Code, req.Name, req.DecimalPlaces).Scan(
			&resp.ID, &resp.Code, &resp.Name, &resp.DecimalPlaces, &resp.IsActive)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "UNIT_NOT_FOUND", "unit not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "UNIT_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, resp)
}

func (service *Service) DeactivateUnit(writer http.ResponseWriter, request *http.Request) {
	deactivateMaster(writer, request, service.pool, "units", "UNIT", "unit_id")
}

// ---------------------------------------------------------------------------
// Categories / Brands
// ---------------------------------------------------------------------------

func (service *Service) ListCategories(writer http.ResponseWriter, request *http.Request) {
	listMaster(writer, request, service.pool, `
		SELECT id, name, is_active FROM item_categories
		WHERE tenant_id = $1 AND (is_active = true OR $2)
		ORDER BY name
	`, scanName)
}

func (service *Service) ListBrands(writer http.ResponseWriter, request *http.Request) {
	listMaster(writer, request, service.pool, `
		SELECT id, name, is_active FROM item_brands
		WHERE tenant_id = $1 AND (is_active = true OR $2)
		ORDER BY name
	`, scanName)
}

func scanName(rows pgx.Rows) (any, error) {
	var n NameResponse
	if err := rows.Scan(&n.ID, &n.Name, &n.IsActive); err != nil {
		return nil, err
	}
	return n, nil
}

func (service *Service) CreateCategory(writer http.ResponseWriter, request *http.Request) {
	createNameMaster(writer, request, service.pool, "item_categories", "CATEGORY")
}

func (service *Service) CreateBrand(writer http.ResponseWriter, request *http.Request) {
	createNameMaster(writer, request, service.pool, "item_brands", "BRAND")
}

func (service *Service) UpdateCategory(writer http.ResponseWriter, request *http.Request) {
	updateNameMaster(writer, request, service.pool, "item_categories", "CATEGORY")
}

func (service *Service) UpdateBrand(writer http.ResponseWriter, request *http.Request) {
	updateNameMaster(writer, request, service.pool, "item_brands", "BRAND")
}

func (service *Service) DeactivateCategory(writer http.ResponseWriter, request *http.Request) {
	deactivateMaster(writer, request, service.pool, "item_categories", "CATEGORY", "category_id")
}

func (service *Service) DeactivateBrand(writer http.ResponseWriter, request *http.Request) {
	deactivateMaster(writer, request, service.pool, "item_brands", "BRAND", "brand_id")
}

// ---------------------------------------------------------------------------
// Shared generic helpers
// ---------------------------------------------------------------------------

func listMaster(writer http.ResponseWriter, request *http.Request, pool *pgxpool.Pool, query string, scan func(pgx.Rows) (any, error)) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	includeInactive := request.URL.Query().Get("include_inactive") == "true"
	results := []any{}
	err = db.WithTenantData(request.Context(), pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), query, tenant, includeInactive)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scan(rows)
			if err != nil {
				return err
			}
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "MASTER_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

func createNameMaster(writer http.ResponseWriter, request *http.Request, pool *pgxpool.Pool, table, errPrefix string) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var req NameRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}
	var resp NameResponse
	err = db.WithTenantData(request.Context(), pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			INSERT INTO `+table+` (tenant_id, name)
			VALUES ($1, $2)
			ON CONFLICT (tenant_id, name) DO UPDATE
			SET is_active = true, updated_at = now()
			RETURNING id, name, is_active
		`, tenant, name).Scan(&resp.ID, &resp.Name, &resp.IsActive)
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, errPrefix+"_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, resp)
}

func updateNameMaster(writer http.ResponseWriter, request *http.Request, pool *pgxpool.Pool, table, errPrefix string) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req NameRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}
	var resp NameResponse
	err = db.WithTenantData(request.Context(), pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			UPDATE `+table+` SET name = $3, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, name, is_active
		`, tenant, id, name).Scan(&resp.ID, &resp.Name, &resp.IsActive)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusNotFound, errPrefix+"_NOT_FOUND", errPrefix+" not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, errPrefix+"_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, resp)
}

// deactivateMaster flips is_active off but refuses while active items still
// reference the master row (referential integrity of history).
func deactivateMaster(writer http.ResponseWriter, request *http.Request, pool *pgxpool.Pool, table, errPrefix, refColumn string) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var inUse bool
	err = db.WithTenantData(request.Context(), pool, tenant, func(tx pgx.Tx) error {
		if err := tx.QueryRow(request.Context(), `
			SELECT EXISTS(SELECT 1 FROM items WHERE tenant_id = $1 AND is_active = true AND `+refColumn+` = $2)
		`, tenant, id).Scan(&inUse); err != nil {
			return err
		}
		if inUse {
			return errMasterInUse
		}
		tag, err := tx.Exec(request.Context(),
			`UPDATE `+table+` SET is_active = false, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
			tenant, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, errMasterInUse) {
		writeError(writer, http.StatusConflict, errPrefix+"_IN_USE",
			"cannot deactivate: still referenced by active items")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusNotFound, errPrefix+"_NOT_FOUND", errPrefix+" not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, errPrefix+"_DEACTIVATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "is_active": false})
}

var errMasterInUse = errors.New("master row still referenced by active items")

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
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
	writeJSON(writer, status, map[string]string{"code": code, "message": message})
}

func pathID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

// pgtype import guard: referenced by scan helpers that may grow date fields.
var _ = pgtype.Date{}
