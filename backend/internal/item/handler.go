package item

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

// Service exposes the item master-data endpoints. Tenant id comes from the
// authenticated request context (JWT), never from a client-supplied header.
type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Routes registers the item endpoints on the chi router.
func (service *Service) Routes(router chiRouter) {
	router.Get("/items", service.List)
	router.Post("/items", service.Create)
	router.Post("/items/{id}/deactivate", service.Deactivate)
	router.Get("/items/{id}/prices", service.ListPrices)
	router.Post("/items/{id}/prices", service.CreatePrice)
}

type chiRouter interface {
	Get(pattern string, handlerFn http.HandlerFunc)
	Post(pattern string, handlerFn http.HandlerFunc)
}

// ---------------------------------------------------------------------------
// Requests / responses
// ---------------------------------------------------------------------------

type ItemRequest struct {
	Code                     string  `json:"code"`
	Name                     string  `json:"name"`
	ItemType                 string  `json:"item_type"`
	UOM                      string  `json:"uom"`
	CostingMethod            *string `json:"costing_method"`
	SaleAccountID            *int64  `json:"sale_account_id"`
	CogsAccountID            *int64  `json:"cogs_account_id"`
	InventoryAccountID       *int64  `json:"inventory_account_id"`
	RevenueRecognitionMethod *string `json:"revenue_recognition_method"`
	IsTrackedStock           bool    `json:"is_tracked_stock"`
	MinStockQty              *string `json:"min_stock_qty"`
}

type PriceRequest struct {
	PriceListName  string  `json:"price_list_name"`
	CustomerGroup  *string `json:"customer_group"`
	CustomerID     *int64  `json:"customer_id"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	CurrencyCode   string  `json:"currency_code"`
	EffectiveFrom  *string `json:"effective_from"`
	EffectiveTo    *string `json:"effective_to"`
}

type itemRow struct {
	ID                       int64   `json:"id"`
	Code                     string  `json:"code"`
	Name                     string  `json:"name"`
	ItemType                 string  `json:"item_type"`
	UOM                      string  `json:"uom"`
	CostingMethod            *string `json:"costing_method"`
	SaleAccountID            *int64  `json:"sale_account_id"`
	CogsAccountID            *int64  `json:"cogs_account_id"`
	InventoryAccountID       *int64  `json:"inventory_account_id"`
	RevenueRecognitionMethod *string `json:"revenue_recognition_method"`
	IsTrackedStock           bool    `json:"is_tracked_stock"`
	MinStockQty              *string `json:"min_stock_qty"`
	IsActive                 bool    `json:"is_active"`
}

type priceRow struct {
	ID             int64   `json:"id"`
	ItemID         int64   `json:"item_id"`
	PriceListName  string  `json:"price_list_name"`
	CustomerGroup  *string `json:"customer_group"`
	CustomerID     *int64  `json:"customer_id"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	CurrencyCode   string  `json:"currency_code"`
	EffectiveFrom  *string `json:"effective_from"`
	EffectiveTo    *string `json:"effective_to"`
	IsActive       bool    `json:"is_active"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (service *Service) List(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}
	itemType := strings.TrimSpace(request.URL.Query().Get("type"))
	includeInactive := request.URL.Query().Get("include_inactive") == "true"

	query := `SELECT id, code, name, item_type, uom, costing_method, sale_account_id,
		cogs_account_id, inventory_account_id, revenue_recognition_method,
		is_tracked_stock, min_stock_qty::text, is_active
		FROM items WHERE tenant_id = $1`
	args := []any{tenant}
	if itemType != "" {
		query += ` AND item_type = $2`
		args = append(args, itemType)
	}
	if !includeInactive {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY code`

	rows, err := service.pool.Query(request.Context(), query, args...)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	items := []itemRow{}
	for rows.Next() {
		var it itemRow
		if err := rows.Scan(&it.ID, &it.Code, &it.Name, &it.ItemType, &it.UOM,
			&it.CostingMethod, &it.SaleAccountID, &it.CogsAccountID, &it.InventoryAccountID,
			&it.RevenueRecognitionMethod, &it.IsTrackedStock, &it.MinStockQty, &it.IsActive); err != nil {
			writeError(writer, http.StatusInternalServerError, "ITEM_LIST_FAILED", err.Error())
			return
		}
		items = append(items, it)
	}
	writeJSON(writer, http.StatusOK, items)
}

func (service *Service) Create(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}
	var req ItemRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if verr := validateCreate(&req); verr != "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", verr)
		return
	}

	tx, err := service.pool.Begin(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_CREATE_FAILED", err.Error())
		return
	}
	defer tx.Rollback(request.Context())
	if err := withTenant(request.Context(), tx, tenant); err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_CREATE_FAILED", err.Error())
		return
	}

	if err := validateAccountRefs(request.Context(), tx, tenant, req); err != nil {
		writeError(writer, http.StatusBadRequest, "ITEM_INVALID_REFERENCE", err.Error())
		return
	}

	var id int64
	err = tx.QueryRow(request.Context(), `
		INSERT INTO items
			(tenant_id, code, name, item_type, uom, costing_method, sale_account_id,
			 cogs_account_id, inventory_account_id, revenue_recognition_method,
			 is_tracked_stock, min_stock_qty)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id`,
		tenant, req.Code, req.Name, req.ItemType, orDefault(req.UOM, "pcs"),
		req.CostingMethod, req.SaleAccountID, req.CogsAccountID, req.InventoryAccountID,
		req.RevenueRecognitionMethod, req.IsTrackedStock, req.MinStockQty,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "ITEM_CODE_EXISTS", "an item with this code already exists")
			return
		}
		writeError(writer, http.StatusInternalServerError, "ITEM_CREATE_FAILED", err.Error())
		return
	}
	if err := tx.Commit(request.Context()); err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_CREATE_FAILED", err.Error())
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(map[string]any{"id": id, "code": req.Code, "name": req.Name})
}

func (service *Service) Deactivate(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}
	id, err := pathID(request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	tag, err := service.pool.Exec(request.Context(),
		`UPDATE items SET is_active = false WHERE tenant_id = $1 AND id = $2`, tenant, id)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_DEACTIVATE_FAILED", err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(writer, http.StatusNotFound, "ITEM_NOT_FOUND", "item does not exist for this tenant")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "is_active": false})
}

func (service *Service) ListPrices(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}
	itemID, err := pathID(request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, item_id, price_list_name, customer_group, customer_id,
		       unit_price_cents, currency_code, effective_from::text, effective_to::text, is_active
		FROM item_price_lists
		WHERE tenant_id = $1 AND item_id = $2
		ORDER BY price_list_name, effective_from NULLS LAST`, tenant, itemID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_PRICE_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	prices := []priceRow{}
	for rows.Next() {
		var p priceRow
		if err := rows.Scan(&p.ID, &p.ItemID, &p.PriceListName, &p.CustomerGroup,
			&p.CustomerID, &p.UnitPriceCents, &p.CurrencyCode, &p.EffectiveFrom,
			&p.EffectiveTo, &p.IsActive); err != nil {
			writeError(writer, http.StatusInternalServerError, "ITEM_PRICE_LIST_FAILED", err.Error())
			return
		}
		prices = append(prices, p)
	}
	writeJSON(writer, http.StatusOK, prices)
}

func (service *Service) CreatePrice(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}
	itemID, err := pathID(request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req PriceRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.UnitPriceCents < 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "unit_price_cents must be >= 0")
		return
	}
	if req.PriceListName == "" {
		req.PriceListName = "Umum"
	}
	if req.CurrencyCode == "" {
		req.CurrencyCode = "IDR"
	}

	tx, err := service.pool.Begin(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_PRICE_CREATE_FAILED", err.Error())
		return
	}
	defer tx.Rollback(request.Context())
	if err := withTenant(request.Context(), tx, tenant); err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_PRICE_CREATE_FAILED", err.Error())
		return
	}

	var exists bool
	err = tx.QueryRow(request.Context(),
		`SELECT EXISTS(SELECT 1 FROM items WHERE tenant_id = $1 AND id = $2)`, tenant, itemID).Scan(&exists)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_PRICE_CREATE_FAILED", err.Error())
		return
	}
	if !exists {
		writeError(writer, http.StatusNotFound, "ITEM_NOT_FOUND", "item does not exist for this tenant")
		return
	}

	var id int64
	err = tx.QueryRow(request.Context(), `
		INSERT INTO item_price_lists
			(tenant_id, item_id, price_list_name, customer_group, customer_id,
			 unit_price_cents, currency_code, effective_from, effective_to)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id`,
		tenant, itemID, req.PriceListName, req.CustomerGroup, req.CustomerID,
		req.UnitPriceCents, req.CurrencyCode, req.EffectiveFrom, req.EffectiveTo,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "ITEM_PRICE_EXISTS", "this price list entry already exists")
			return
		}
		writeError(writer, http.StatusInternalServerError, "ITEM_PRICE_CREATE_FAILED", err.Error())
		return
	}
	if err := tx.Commit(request.Context()); err != nil {
		writeError(writer, http.StatusInternalServerError, "ITEM_PRICE_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{"id": id, "item_id": itemID})
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// validateCreate returns an error message string, or "" if valid.
func validateCreate(req *ItemRequest) string {
	if strings.TrimSpace(req.Code) == "" {
		return "code is required"
	}
	if strings.TrimSpace(req.Name) == "" {
		return "name is required"
	}
	switch req.ItemType {
	case "goods":
		if req.CostingMethod == nil || *req.CostingMethod == "" {
			return "goods requires a costing_method"
		}
		if req.InventoryAccountID == nil {
			return "goods requires an inventory_account_id"
		}
		if req.CogsAccountID == nil {
			return "goods requires a cogs_account_id"
		}
		if req.IsTrackedStock == false {
			return "goods must set is_tracked_stock = true"
		}
	case "service":
		if req.InventoryAccountID != nil {
			return "service cannot have an inventory_account_id"
		}
		if req.CogsAccountID != nil {
			return "service cannot have a cogs_account_id"
		}
		if req.IsTrackedStock {
			return "service cannot be tracked stock"
		}
	default:
		return "item_type must be 'goods' or 'service'"
	}
	return ""
}

// validateAccountRefs checks that any provided account ids belong to the tenant.
func validateAccountRefs(ctx context.Context, tx pgx.Tx, tenant int64, req ItemRequest) error {
	ids := []int64{}
	if req.SaleAccountID != nil {
		ids = append(ids, *req.SaleAccountID)
	}
	if req.CogsAccountID != nil {
		ids = append(ids, *req.CogsAccountID)
	}
	if req.InventoryAccountID != nil {
		ids = append(ids, *req.InventoryAccountID)
	}
	for _, id := range ids {
		var exists bool
		err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM accounts WHERE tenant_id = $1 AND id = $2)`, tenant, id).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("account %d does not belong to this tenant", id)
		}
	}
	return nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// ---------------------------------------------------------------------------
// Helpers
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

func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

func isUniqueViolation(err error) bool {
	const pgErrCodeUniqueViolation = "23505"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation
}

// withTenant sets the RLS tenant context on the transaction so FORCE RLS
// tables are visible. Mirrors the pattern used across the codebase.
func withTenant(ctx context.Context, tx pgx.Tx, tenant int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenant, 10))
	return err
}
