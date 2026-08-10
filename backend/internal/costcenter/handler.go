package costcenter

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

// ---------------------------------------------------------------------------
// F-09: Cost/Profit Center
//   Master cost/profit/investment centers with hierarchy, allocation rules,
//   and a P&L view that aggregates journal lines tagged with a center's
//   dimension.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CreateCostCenterRequest is the JSON body for POST /cost-centers.
type CreateCostCenterRequest struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	CenterType    string  `json:"center_type"`
	ParentID      *int64  `json:"parent_id"`
	ManagerUserID *int64  `json:"manager_user_id"`
	IsActive      *bool   `json:"is_active"`
	Description   *string `json:"description"`
}

// UpdateCostCenterRequest is the JSON body for PUT /cost-centers/{id}.
type UpdateCostCenterRequest struct {
	Name          *string `json:"name"`
	CenterType    *string `json:"center_type"`
	ParentID      *int64  `json:"parent_id"`
	ManagerUserID *int64  `json:"manager_user_id"`
	IsActive      *bool   `json:"is_active"`
	Description   *string `json:"description"`
}

// CreateAllocationRequest is the JSON body for POST /cost-centers/{id}/allocations.
type CreateAllocationRequest struct {
	SourceCostCenterID    int64   `json:"source_cost_center_id"`
	TargetCostCenterID    int64   `json:"target_cost_center_id"`
	AllocationPercentage  float64 `json:"allocation_percentage"`
	AllocationBasis       string  `json:"allocation_basis"`
	IsActive              *bool   `json:"is_active"`
}

type CostCenterResponse struct {
	ID            int64     `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	CenterType    string    `json:"center_type"`
	ParentID      *int64    `json:"parent_id"`
	ManagerUserID *int64    `json:"manager_user_id"`
	IsActive      bool      `json:"is_active"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AllocationResponse struct {
	ID                    int64     `json:"id"`
	SourceCostCenterID    int64     `json:"source_cost_center_id"`
	TargetCostCenterID    int64     `json:"target_cost_center_id"`
	AllocationPercentage  float64   `json:"allocation_percentage"`
	AllocationBasis       string    `json:"allocation_basis"`
	IsActive              bool      `json:"is_active"`
	CreatedAt             time.Time `json:"created_at"`
}

type PnLResponse struct {
	CostCenterID   int64 `json:"cost_center_id"`
	RevenueCents   int64 `json:"revenue_cents"`
	ExpenseCents   int64 `json:"expense_cents"`
	NetCents       int64 `json:"net_cents"`
	LineCount      int   `json:"line_count"`
}

func (s *Service) Routes(r chi.Router) {
	r.Post("/cost-centers", s.Create)
	r.Get("/cost-centers", s.List)
	r.Get("/cost-centers/{id}", s.Get)
	r.Put("/cost-centers/{id}", s.Update)
	r.Delete("/cost-centers/{id}", s.Deactivate)
	r.Post("/cost-centers/{id}/allocations", s.CreateAllocation)
	r.Get("/cost-centers/{id}/allocations", s.ListAllocations)
	r.Get("/cost-centers/{id}/pnl", s.PnL)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	var req CreateCostCenterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateCostCenter(req.Code, req.Name, req.CenterType); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	var resp CostCenterResponse
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO cost_centers (
			tenant_id, code, name, center_type, parent_id,
			manager_user_id, is_active, description
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, code, name, center_type, parent_id, manager_user_id,
			is_active, description, created_at, updated_at
	`,
		tid, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
		strings.ToUpper(strings.TrimSpace(req.CenterType)), req.ParentID,
		req.ManagerUserID, active, req.Description,
	).Scan(
		&resp.ID, &resp.Code, &resp.Name, &resp.CenterType, &resp.ParentID,
		&resp.ManagerUserID, &resp.IsActive, &resp.Description, &resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, code, name, center_type, parent_id, manager_user_id,
			is_active, description, created_at, updated_at
		FROM cost_centers
		WHERE tenant_id = $1
		ORDER BY code
	`, tid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()
	results := make([]CostCenterResponse, 0)
	for rows.Next() {
		var c CostCenterResponse
		if err := rows.Scan(
			&c.ID, &c.Code, &c.Name, &c.CenterType, &c.ParentID,
			&c.ManagerUserID, &c.IsActive, &c.Description, &c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var resp CostCenterResponse
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, code, name, center_type, parent_id, manager_user_id,
			is_active, description, created_at, updated_at
		FROM cost_centers
		WHERE tenant_id = $1 AND id = $2
	`, tid, id).Scan(
		&resp.ID, &resp.Code, &resp.Name, &resp.CenterType, &resp.ParentID,
		&resp.ManagerUserID, &resp.IsActive, &resp.Description, &resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "cost center not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req UpdateCostCenterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.CenterType != nil {
		ct := strings.ToUpper(strings.TrimSpace(*req.CenterType))
		if ct != "COST" && ct != "PROFIT" && ct != "INVESTMENT" {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "center_type must be COST, PROFIT, or INVESTMENT")
			return
		}
	}

	var sets []string
	var args []any
	args = append(args, tid, id)
	idx := 3
	if req.Name != nil {
		sets = append(sets, "name = $"+strconv.Itoa(idx))
		args = append(args, strings.TrimSpace(*req.Name))
		idx++
	}
	if req.CenterType != nil {
		sets = append(sets, "center_type = $"+strconv.Itoa(idx))
		args = append(args, strings.ToUpper(strings.TrimSpace(*req.CenterType)))
		idx++
	}
	if req.ParentID != nil {
		sets = append(sets, "parent_id = $"+strconv.Itoa(idx))
		args = append(args, req.ParentID)
		idx++
	}
	if req.ManagerUserID != nil {
		sets = append(sets, "manager_user_id = $"+strconv.Itoa(idx))
		args = append(args, req.ManagerUserID)
		idx++
	}
	if req.IsActive != nil {
		sets = append(sets, "is_active = $"+strconv.Itoa(idx))
		args = append(args, *req.IsActive)
		idx++
	}
	if req.Description != nil {
		sets = append(sets, "description = $"+strconv.Itoa(idx))
		args = append(args, req.Description)
		idx++
	}
	if len(sets) == 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "no fields to update")
		return
	}
	sets = append(sets, "updated_at = now()")

	var resp CostCenterResponse
	err := s.pool.QueryRow(r.Context(), `
		UPDATE cost_centers SET `+strings.Join(sets, ", ")+`
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, code, name, center_type, parent_id, manager_user_id,
			is_active, description, created_at, updated_at
	`, args...).Scan(
		&resp.ID, &resp.Code, &resp.Name, &resp.CenterType, &resp.ParentID,
		&resp.ManagerUserID, &resp.IsActive, &resp.Description, &resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "cost center not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Deactivate(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var resp CostCenterResponse
	err := s.pool.QueryRow(r.Context(), `
		UPDATE cost_centers
		SET is_active = false, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, code, name, center_type, parent_id, manager_user_id,
			is_active, description, created_at, updated_at
	`, tid, id).Scan(
		&resp.ID, &resp.Code, &resp.Name, &resp.CenterType, &resp.ParentID,
		&resp.ManagerUserID, &resp.IsActive, &resp.Description, &resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "cost center not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) CreateAllocation(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req CreateAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateAllocation(req.SourceCostCenterID, req.TargetCostCenterID, req.AllocationPercentage); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	basis := normalizeAllocationBasis(req.AllocationBasis)
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	var resp AllocationResponse
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO cost_center_allocations (
			tenant_id, source_cost_center_id, target_cost_center_id,
			allocation_percentage, allocation_basis, is_active
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, source_cost_center_id, target_cost_center_id,
			allocation_percentage, allocation_basis, is_active, created_at
	`, tid, req.SourceCostCenterID, req.TargetCostCenterID,
		req.AllocationPercentage, basis, active,
	).Scan(
		&resp.ID, &resp.SourceCostCenterID, &resp.TargetCostCenterID,
		&resp.AllocationPercentage, &resp.AllocationBasis, &resp.IsActive,
		&resp.CreatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) ListAllocations(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, source_cost_center_id, target_cost_center_id,
			allocation_percentage, allocation_basis, is_active, created_at
		FROM cost_center_allocations
		WHERE tenant_id = $1
		  AND (source_cost_center_id = $2 OR target_cost_center_id = $2)
		ORDER BY created_at
	`, tid, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()
	results := make([]AllocationResponse, 0)
	for rows.Next() {
		var a AllocationResponse
		if err := rows.Scan(
			&a.ID, &a.SourceCostCenterID, &a.TargetCostCenterID,
			&a.AllocationPercentage, &a.AllocationBasis, &a.IsActive,
			&a.CreatedAt,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// PnL returns the profit & loss aggregated for a cost center by summing the
// journal lines tagged (via journal_line_dimensions) with the dimension whose
// code matches the cost center's code (dimension_type = 'cost_center').
// Revenue accounts contribute credit - debit; expense accounts contribute
// debit - credit — both expressed as positive amounts for a typical period.
func (s *Service) PnL(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}

	// Ensure the cost center exists for this tenant before computing P&L.
	var exists bool
	err := s.pool.QueryRow(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM cost_centers WHERE tenant_id = $1 AND id = $2)
	`, tid, id).Scan(&exists)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if !exists {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "cost center not found")
		return
	}

	var resp PnLResponse
	resp.CostCenterID = id
	// Join journal_lines → journal_line_dimensions → dimensions → accounts,
	// scoping to the cost center's matching dimension and only POSTED entries.
	err = s.pool.QueryRow(r.Context(), `
		SELECT
			COALESCE(SUM(CASE WHEN a.report_group = 'revenue'
				THEN jl.credit_cents - jl.debit_cents ELSE 0 END), 0) AS revenue_cents,
			COALESCE(SUM(CASE WHEN a.report_group = 'expense'
				THEN jl.debit_cents - jl.credit_cents ELSE 0 END), 0) AS expense_cents,
			COUNT(DISTINCT jl.id) AS line_count
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN journal_line_dimensions jld
			ON jld.tenant_id = jl.tenant_id AND jld.journal_line_id = jl.id
		JOIN dimensions d
			ON d.tenant_id = jld.tenant_id AND d.id = jld.dimension_id
		JOIN cost_centers cc
			ON cc.tenant_id = d.tenant_id AND cc.code = d.code
		JOIN accounts a
			ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1
		  AND cc.id = $2
		  AND d.dimension_type = 'cost_center'
		  AND je.status = 'POSTED'
	`, tid, id).Scan(&resp.RevenueCents, &resp.ExpenseCents, &resp.LineCount)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	resp.NetCents = resp.RevenueCents - resp.ExpenseCents
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Validation & Helpers
// ---------------------------------------------------------------------------

func validateCostCenter(code, name, centerType string) (string, string) {
	if strings.TrimSpace(code) == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if strings.TrimSpace(name) == "" {
		return "INVALID_REQUEST", "name is required"
	}
	ct := strings.ToUpper(strings.TrimSpace(centerType))
	if ct != "COST" && ct != "PROFIT" && ct != "INVESTMENT" {
		return "INVALID_REQUEST", "center_type must be COST, PROFIT, or INVESTMENT"
	}
	return "", ""
}

// validateAllocation validates the core fields of an allocation request. Both
// source and target cost center IDs must be positive, and the allocation
// percentage must fall in the open-closed range (0, 100].
func validateAllocation(sourceID, targetID int64, percentage float64) (string, string) {
	if sourceID <= 0 || targetID <= 0 {
		return "INVALID_REQUEST", "source_cost_center_id and target_cost_center_id are required"
	}
	if percentage <= 0 || percentage > 100 {
		return "INVALID_REQUEST", "allocation_percentage must be between 0 and 100"
	}
	return "", ""
}

// normalizeAllocationBasis upper-cases and trims the allocation basis string,
// defaulting to "REVENUE" when blank.
func normalizeAllocationBasis(basis string) string {
	b := strings.ToUpper(strings.TrimSpace(basis))
	if b == "" {
		return "REVENUE"
	}
	return b
}

// validCenterTypes returns the set of recognized center type codes.
func validCenterTypes() map[string]bool {
	return map[string]bool{
		"COST":      true,
		"PROFIT":    true,
		"INVESTMENT": true,
	}
}

// isValidCenterType reports whether a (trimmed, upper-cased) code is one of the
// recognized center types.
func isValidCenterType(centerType string) bool {
	ct := strings.ToUpper(strings.TrimSpace(centerType))
	return validCenterTypes()[ct]
}

// pathID(raw string) int64 — defined below.

func pathID(raw string) int64 {
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func tenantIDFromContext(r *http.Request) (int64, bool) {
	return auth.TenantIDFromContext(r.Context())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
