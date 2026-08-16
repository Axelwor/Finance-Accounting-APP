package costcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
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
	SourceCostCenterID   int64   `json:"source_cost_center_id"`
	TargetCostCenterID   int64   `json:"target_cost_center_id"`
	AllocationPercentage float64 `json:"allocation_percentage"`
	AllocationBasis      string  `json:"allocation_basis"`
	IsActive             *bool   `json:"is_active"`
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
	ID                   int64     `json:"id"`
	SourceCostCenterID   int64     `json:"source_cost_center_id"`
	TargetCostCenterID   int64     `json:"target_cost_center_id"`
	AllocationPercentage float64   `json:"allocation_percentage"`
	AllocationBasis      string    `json:"allocation_basis"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
}

type PnLResponse struct {
	CostCenterID int64 `json:"cost_center_id"`
	RevenueCents int64 `json:"revenue_cents"`
	ExpenseCents int64 `json:"expense_cents"`
	NetCents     int64 `json:"net_cents"`
	LineCount    int   `json:"line_count"`
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
	r.Post("/cost-centers/{id}/execute-allocations", s.ExecuteAllocations)
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
// Execute Allocations (A-25)
// ---------------------------------------------------------------------------

// ExecuteAllocations posts proportional journal entries that reallocate a
// share of the source cost center's P&L to each target cost center based on
// the configured allocation percentages. Uses the hash-chain, idempotency,
// and outbox pattern shared by all journal-posting modules.
//
// The allocation is bounded to a date window: use the from/to query params
// (YYYY-MM-DD) to scope the source P&L aggregation; when omitted, the
// tenant's current open accounting period is used.
//
// POST /cost-centers/{id}/execute-allocations?from=YYYY-MM-DD&to=YYYY-MM-DD
func (s *Service) ExecuteAllocations(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	sourceID := pathID(chi.URLParam(r, "id"))
	if sourceID <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key header is required")
		return
	}
	if _, err := parseUUID(idem); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key must be a UUID")
		return
	}
	fromDate := r.URL.Query().Get("from")
	toDate := r.URL.Query().Get("to")
	for _, d := range []string{fromDate, toDate} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "from/to must be YYYY-MM-DD dates")
				return
			}
		}
	}

	var result map[string]any
	err := db.WithTransaction(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := scopeTenant(r.Context(), tx, tid); err != nil {
			return err
		}
		// Idempotent replay.
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(r.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID: tid, IdempotencyKey: uuidValue(idem),
		}); err == nil {
			result = map[string]any{"journal_entry_id": existing.ID, "number": existing.Number, "replayed": true}
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Verify source cost center exists and is active.
		var sourceCode string
		var sourceActive bool
		err := tx.QueryRow(r.Context(), `
			SELECT code, is_active FROM cost_centers WHERE tenant_id = $1 AND id = $2
		`, tid, sourceID).Scan(&sourceCode, &sourceActive)
		if err != nil {
			return fmt.Errorf("source cost center not found: %w", err)
		}
		if !sourceActive {
			return fmt.Errorf("source cost center %s is not active", sourceCode)
		}

		// Resolve the allocation date window. Default to the tenant's current
		// open accounting period so the aggregation is bounded to one period.
		var periodStart, periodEnd string
		if fromDate != "" && toDate != "" {
			periodStart, periodEnd = fromDate, toDate
		} else if fromDate != "" || toDate != "" {
			return fmt.Errorf("both from and to are required to scope the allocation period")
		} else {
			err := tx.QueryRow(r.Context(), `
				SELECT period_start::text, period_end::text
				FROM accounting_periods
				WHERE tenant_id = $1 AND status IN ('OPEN','REOPENED')
				ORDER BY period_start DESC LIMIT 1
			`, tid).Scan(&periodStart, &periodEnd)
			if err != nil {
				return fmt.Errorf("no open accounting period found and no from/to dates supplied: %w", err)
			}
		}

		// Read source cost center's P&L totals for the window only.
		var revenueCents, expenseCents int64
		err = tx.QueryRow(r.Context(), `
			SELECT
				COALESCE(SUM(CASE WHEN a.report_group = 'revenue'
					THEN jl.credit_cents - jl.debit_cents ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN a.report_group = 'expense'
					THEN jl.debit_cents - jl.credit_cents ELSE 0 END), 0)
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
			WHERE jl.tenant_id = $1 AND cc.id = $2
			  AND d.dimension_type = 'cost_center' AND je.status = 'POSTED'
			  AND je.entry_date >= $3::date AND je.entry_date <= $4::date
		`, tid, sourceID, periodStart, periodEnd).Scan(&revenueCents, &expenseCents)
		if err != nil {
			return fmt.Errorf("failed to read source P&L: %w", err)
		}

		// Load active allocations for this source cost center.
		rows, err := tx.Query(r.Context(), `
			SELECT target_cost_center_id, allocation_percentage, allocation_basis
			FROM cost_center_allocations
			WHERE tenant_id = $1 AND source_cost_center_id = $2 AND is_active = true
			ORDER BY id
		`, tid, sourceID)
		if err != nil {
			return fmt.Errorf("failed to load allocations: %w", err)
		}
		type allocationRow struct {
			targetID   int64
			percentage float64
			basis      string
		}
		var allocations []allocationRow
		for rows.Next() {
			var a allocationRow
			if err := rows.Scan(&a.targetID, &a.percentage, &a.basis); err != nil {
				rows.Close()
				return err
			}
			allocations = append(allocations, a)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(allocations) == 0 {
			return fmt.Errorf("no active allocations found for cost center %s", sourceCode)
		}

		// Find a revenue account and an expense account for the tenant.
		var revenueAcctID, expenseAcctID int64
		err = tx.QueryRow(r.Context(), `
			SELECT id FROM accounts WHERE tenant_id = $1 AND report_group = 'revenue' AND is_group = false
			ORDER BY code LIMIT 1
		`, tid).Scan(&revenueAcctID)
		if err != nil {
			return fmt.Errorf("no revenue account found for tenant: %w", err)
		}
		err = tx.QueryRow(r.Context(), `
			SELECT id FROM accounts WHERE tenant_id = $1 AND report_group = 'expense' AND is_group = false
			ORDER BY code LIMIT 1
		`, tid).Scan(&expenseAcctID)
		if err != nil {
			return fmt.Errorf("no expense account found for tenant: %w", err)
		}

		// Find dimension IDs for source and all target cost centers.
		dimCache := make(map[int64]int64) // cost_center_id → dimension_id
		resolveDim := func(ccID int64) (int64, error) {
			if d, ok := dimCache[ccID]; ok {
				return d, nil
			}
			var ccCode string
			err := tx.QueryRow(r.Context(), `
				SELECT code FROM cost_centers WHERE tenant_id = $1 AND id = $2
			`, tid, ccID).Scan(&ccCode)
			if err != nil {
				return 0, fmt.Errorf("target cost center %d not found: %w", ccID, err)
			}
			var dimID int64
			err = tx.QueryRow(r.Context(), `
				SELECT id FROM dimensions
				WHERE tenant_id = $1 AND code = $2 AND dimension_type = 'cost_center'
			`, tid, ccCode).Scan(&dimID)
			if err != nil {
				return 0, fmt.Errorf("dimension not found for cost center %s: %w", ccCode, err)
			}
			dimCache[ccID] = dimID
			return dimID, nil
		}
		sourceDimID, err := resolveDim(sourceID)
		if err != nil {
			return err
		}

		// Build journal lines: for each allocation, post revenue and expense
		// reallocation lines tagged with the appropriate cost center dimensions.
		var lines []accounting.Line
		var postedAllocations []map[string]any
		for _, alloc := range allocations {
			if _, err := resolveDim(alloc.targetID); err != nil {
				return err
			}
			revenueShare := int64(float64(revenueCents) * alloc.percentage / 100)
			expenseShare := int64(float64(expenseCents) * alloc.percentage / 100)
			if revenueShare > 0 {
				// Revenue is credit-normal. Move the share from source to
				// target: Dr revenue (source CC dim, decreases source P&L) /
				// Cr revenue (target CC dim, increases target P&L).
				lines = append(lines,
					accounting.Line{AccountID: revenueAcctID, DebitCents: revenueShare, SourceLineRef: fmt.Sprintf("alloc-rev-dr-src-%d", sourceID)},
					accounting.Line{AccountID: revenueAcctID, CreditCents: revenueShare, SourceLineRef: fmt.Sprintf("alloc-rev-cr-tgt-%d", alloc.targetID)},
				)
			}
			if expenseShare > 0 {
				// Expense is debit-normal. Move the share from source to
				// target: Dr expense (target CC dim, increases target P&L) /
				// Cr expense (source CC dim, decreases source P&L).
				lines = append(lines,
					accounting.Line{AccountID: expenseAcctID, DebitCents: expenseShare, SourceLineRef: fmt.Sprintf("alloc-exp-dr-tgt-%d", alloc.targetID)},
					accounting.Line{AccountID: expenseAcctID, CreditCents: expenseShare, SourceLineRef: fmt.Sprintf("alloc-exp-cr-src-%d", sourceID)},
				)
			}
			postedAllocations = append(postedAllocations, map[string]any{
				"target_cost_center_id": alloc.targetID,
				"percentage":            alloc.percentage,
				"revenue_cents":         revenueShare,
				"expense_cents":         expenseShare,
			})
		}
		if len(lines) == 0 {
			return fmt.Errorf("no allocatable amounts (source revenue and expense are both zero)")
		}
		if err := accounting.BalanceCheck(lines); err != nil {
			return fmt.Errorf("allocation journal not balanced: %w", err)
		}

		journal := accounting.Journal{
			TenantID:    tid,
			SourceRef:   fmt.Sprintf("CC-ALLOC-%d", sourceID),
			IntentType:  accounting.IntentType("COST_CENTER_ALLOCATION"),
			EntryDate:   periodEnd,
			Description: fmt.Sprintf("Cost center allocation: %s (%s..%s)", sourceCode, periodStart, periodEnd),
			Lines:       lines,
		}

		entryID, number, lineIDs, err := postAllocationJournal(r.Context(), tx, tid, uid, idem, journal)
		if err != nil {
			return err
		}

		// Tag each journal line with its cost center dimension. The dimension
		// ids are appended in the exact order the journal lines were built:
		// Dr revenue (source) / Cr revenue (target), then Dr expense (target)
		// / Cr expense (source).
		lineDimIDs := make([]int64, 0, len(lines))
		for _, alloc := range allocations {
			revenueShare := int64(float64(revenueCents) * alloc.percentage / 100)
			expenseShare := int64(float64(expenseCents) * alloc.percentage / 100)
			targetDimID := dimCache[alloc.targetID]
			if revenueShare > 0 {
				lineDimIDs = append(lineDimIDs, sourceDimID, targetDimID)
			}
			if expenseShare > 0 {
				lineDimIDs = append(lineDimIDs, targetDimID, sourceDimID)
			}
		}
		if err := tagLinesDimensions(r.Context(), tx, tid, lineIDs, lineDimIDs); err != nil {
			return fmt.Errorf("failed to tag journal lines with cost center dimensions: %w", err)
		}

		result = map[string]any{
			"journal_entry_id":      entryID,
			"number":                number,
			"source_cost_center_id": sourceID,
			"period_from":           periodStart,
			"period_to":             periodEnd,
			"allocations":           postedAllocations,
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "ALLOCATION_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Journal posting helpers (hash-chain, idempotency, outbox)
// ---------------------------------------------------------------------------

func scopeTenant(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10))
	return err
}

func uuidValue(raw string) pgtype.UUID {
	var v pgtype.UUID
	_ = v.Scan(raw)
	return v
}

// parseUUID validates that raw is a UUID, matching the idempotency-key
// contract used by every other posting module.
func parseUUID(raw string) (pgtype.UUID, error) {
	var v pgtype.UUID
	if err := v.Scan(raw); err != nil {
		return v, fmt.Errorf("must be a UUID")
	}
	return v, nil
}

func isNoRows(err error) bool {
	return err == pgx.ErrNoRows
}

// int8Value mirrors the helper in the other posting modules: zero maps to
// NULL rather than a literal 0, keeping created_by semantics consistent.
func int8Value(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v != 0}
}

func lockOrSeedHead(ctx context.Context, tx pgx.Tx, tenantID int64) (db.LedgerChainHead, error) {
	head, err := db.New(tx).LockLedgerChainHead(ctx, tenantID)
	if err == nil {
		return head, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, NULL, 'genesis') ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID); err != nil {
		return db.LedgerChainHead{}, err
	}
	return db.New(tx).LockLedgerChainHead(ctx, tenantID)
}

func resolvePeriod(ctx context.Context, tx pgx.Tx, tenantID int64, date string) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounting_periods
		WHERE tenant_id = $1 AND $2::date BETWEEN period_start AND period_end
		  AND status IN ('OPEN', 'REOPENED')
		ORDER BY period_start DESC LIMIT 1
	`, tenantID, date).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("entry date is outside an open period: %w", err)
	}
	return periodID, nil
}

func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var p string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'JRN', 'JRN', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&p, &seq)
	if err != nil {
		return "", err
	}
	s := strconv.FormatInt(seq, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return p + "-" + strconv.FormatInt(int64(year), 10) + "-" + s, nil
}

func upsertHead(ctx context.Context, tx pgx.Tx, tenantID, lastJournalID int64, lastHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE
		SET last_journal_id = EXCLUDED.last_journal_id, last_hash = EXCLUDED.last_hash, updated_at = now()
	`, tenantID, lastJournalID, lastHash)
	return err
}

func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID int64, topic string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, topic, payload)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, topic, payload)
	return err
}

// postAllocationJournal builds, hashes, and inserts a cost-center allocation
// journal entry + lines, advancing the hash chain and writing the outbox
// event. It returns the entry id, journal number, and the inserted journal
// line ids in insertion order so callers can tag dimensions without
// re-scanning the table.
func postAllocationJournal(ctx context.Context, tx pgx.Tx, tenantID, uid int64, idem string, journal accounting.Journal) (int64, string, []int64, error) {
	head, err := lockOrSeedHead(ctx, tx, tenantID)
	if err != nil {
		return 0, "", nil, err
	}
	journal.PreviousHash = head.LastHash
	journal.Hash = accounting.HashJournal(journal)

	periodID, err := resolvePeriod(ctx, tx, tenantID, journal.EntryDate)
	if err != nil {
		return 0, "", nil, err
	}
	number, err := nextJournalNumber(ctx, tx, tenantID)
	if err != nil {
		return 0, "", nil, err
	}
	var entryID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, tenantID, number, journal.EntryDate, periodID, journal.Description,
		journal.SourceRef, string(journal.IntentType), idem,
		journal.Hash, journal.PreviousHash, int8Value(uid)).Scan(&entryID)
	if err != nil {
		return 0, "", nil, err
	}
	lineIDs := make([]int64, 0, len(journal.Lines))
	for _, line := range journal.Lines {
		var lineID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, tenantID, entryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef).Scan(&lineID); err != nil {
			return 0, "", nil, err
		}
		lineIDs = append(lineIDs, lineID)
	}
	if err := upsertHead(ctx, tx, tenantID, entryID, journal.Hash); err != nil {
		return 0, "", nil, err
	}
	outboxPayload, _ := json.Marshal(map[string]any{
		"journal_entry_id": entryID, "number": number, "intent": string(journal.IntentType),
	})
	if err := insertOutbox(ctx, tx, tenantID, "journal.posted", outboxPayload); err != nil {
		return 0, "", nil, err
	}
	return entryID, number, lineIDs, nil
}

// tagLinesDimensions links each journal line (by id) to its dimension in a
// single round-trip. lineIDs and dimensionIDs must be the same length.
func tagLinesDimensions(ctx context.Context, tx pgx.Tx, tenantID int64, lineIDs []int64, dimensionIDs []int64) error {
	if len(lineIDs) != len(dimensionIDs) {
		return fmt.Errorf("line/dimension id count mismatch: %d vs %d", len(lineIDs), len(dimensionIDs))
	}
	if len(lineIDs) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO journal_line_dimensions (tenant_id, journal_line_id, dimension_id) VALUES `)
	args := make([]any, 0, 1+2*len(lineIDs))
	args = append(args, tenantID)
	for i, lineID := range lineIDs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("($1, $%d, $%d)", 2+2*i, 3+2*i))
		args = append(args, lineID, dimensionIDs[i])
	}
	b.WriteString(" ON CONFLICT (tenant_id, journal_line_id, dimension_id) DO NOTHING")
	_, err := tx.Exec(ctx, b.String(), args...)
	return err
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
		"COST":       true,
		"PROFIT":     true,
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
	message = httperr.SanitizeMessage(status, code, message)
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
