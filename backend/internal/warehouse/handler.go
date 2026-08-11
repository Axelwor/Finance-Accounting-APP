package warehouse

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

// ---------------------------------------------------------------------------
// F-02: Multi-Warehouse Management
//   Master warehouse/gudang with code, name, address, is_active.
//   Stock balances become per-warehouse (item_id + warehouse_id).
//   Stock transfers move stock between warehouses with in-transit tracking.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type CreateWarehouseRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	City     string `json:"city"`
	IsActive bool   `json:"is_active"`
}

type WarehouseResponse struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	City      string    `json:"city"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) Routes(r chi.Router) {
	r.Get("/warehouses", s.List)
	r.Post("/warehouses", s.Create)
	r.Get("/warehouses/{id}", s.Get)
	r.Put("/warehouses/{id}", s.Update)
	r.Delete("/warehouses/{id}", s.Deactivate)
	r.Get("/warehouses/{id}/stock", s.ListStock)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	var req CreateWarehouseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateWarehouse(req); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	var resp WarehouseResponse
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tid, 10)); err != nil {
			return err
		}
		return tx.QueryRow(r.Context(), `
			INSERT INTO warehouses (tenant_id, code, name, address, city, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, code, name, address, city, is_active, created_at
		`, tid, strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name),
			strings.TrimSpace(req.Address), strings.TrimSpace(req.City), true).Scan(
			&resp.ID, &resp.Code, &resp.Name, &resp.Address, &resp.City, &resp.IsActive, &resp.CreatedAt)
	})
	if err != nil {
		writeErr(w, http.StatusConflict, "CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	var results []WarehouseResponse
	err := withTenant(r.Context(), s.pool, tid, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, code, name, address, city, is_active, created_at
			FROM warehouses WHERE tenant_id = $1 ORDER BY code
		`, tid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var wh WarehouseResponse
			if err := rows.Scan(&wh.ID, &wh.Code, &wh.Name, &wh.Address, &wh.City, &wh.IsActive, &wh.CreatedAt); err != nil {
				return err
			}
			results = append(results, wh)
		}
		return rows.Err()
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if results == nil {
		results = []WarehouseResponse{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var resp WarehouseResponse
	err := withTenant(r.Context(), s.pool, tid, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `
			SELECT id, code, name, address, city, is_active, created_at
			FROM warehouses WHERE tenant_id = $1 AND id = $2
		`, tid, id).Scan(&resp.ID, &resp.Code, &resp.Name, &resp.Address, &resp.City, &resp.IsActive, &resp.CreatedAt)
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "warehouse not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req CreateWarehouseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var resp WarehouseResponse
	err := withTenant(r.Context(), s.pool, tid, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `
			UPDATE warehouses SET name = $3, address = $4, city = $5, is_active = $6, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, code, name, address, city, is_active, created_at
		`, tid, id, strings.TrimSpace(req.Name), strings.TrimSpace(req.Address),
			strings.TrimSpace(req.City), req.IsActive).Scan(
			&resp.ID, &resp.Code, &resp.Name, &resp.Address, &resp.City, &resp.IsActive, &resp.CreatedAt)
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "warehouse not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Deactivate(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	err := withTenant(r.Context(), s.pool, tid, func(tx pgx.Tx) error {
		_, execErr := tx.Exec(r.Context(), `
			UPDATE warehouses SET is_active = false, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tid, id)
		return execErr
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DEACTIVATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "is_active": false})
}

func (s *Service) ListStock(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	type stockRow struct {
		ItemID           int64   `json:"item_id"`
		ItemCode         string  `json:"item_code"`
		ItemName         string  `json:"item_name"`
		QtyOnHand        float64 `json:"qty_on_hand"`
		AvgUnitCostCents int64   `json:"avg_unit_cost_cents"`
	}
	var results []stockRow
	err := withTenant(r.Context(), s.pool, tid, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT i.id, i.code, i.name, COALESCE(sb.qty_on_hand, 0), COALESCE(sb.avg_unit_cost_cents, 0)
			FROM items i
			LEFT JOIN stock_balances sb ON sb.tenant_id = i.tenant_id AND sb.item_id = i.id AND sb.warehouse_id = $2
			WHERE i.tenant_id = $1 AND i.is_active = true
			ORDER BY i.code
		`, tid, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row stockRow
			if err := rows.Scan(&row.ItemID, &row.ItemCode, &row.ItemName, &row.QtyOnHand, &row.AvgUnitCostCents); err != nil {
				return err
			}
			results = append(results, row)
		}
		return rows.Err()
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if results == nil {
		results = []stockRow{}
	}
	writeJSON(w, http.StatusOK, results)
}

// ---------------------------------------------------------------------------
// Validation & Helpers
// ---------------------------------------------------------------------------

// withTenant runs fn inside a transaction with the RLS tenant scope set, so
// row-level security acts as a second layer of defense alongside the explicit
// WHERE tenant_id filters.
func withTenant(ctx context.Context, pool *pgxpool.Pool, tenantID int64, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10)); err != nil {
			return err
		}
		return fn(tx)
	})
}

func validateWarehouse(req CreateWarehouseRequest) (string, string) {
	if strings.TrimSpace(req.Code) == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if strings.TrimSpace(req.Name) == "" {
		return "INVALID_REQUEST", "name is required"
	}
	return "", ""
}

func pathID(raw string) int64 {
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
