package approval

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/httperr"
)

// ---------------------------------------------------------------------------
// F-03: Approval Workflow Engine
//   Generic approval system: any entity (invoice, PO, CN, journal) can
//   require approval before posting. Configurable per entity_type and
//   min_amount. Approvers are role-based.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type CreateWorkflowRequest struct {
	EntityType     string `json:"entity_type"`
	MinAmountCents int64  `json:"min_amount_cents"`
	ApproverRole   string `json:"approver_role"`
}

type ApprovalRequestResponse struct {
	ID              int64     `json:"id"`
	EntityType      string    `json:"entity_type"`
	EntityID        int64     `json:"entity_id"`
	EntityNumber    string    `json:"entity_number"`
	RequestedBy     int64     `json:"requested_by"`
	RequestedAt     time.Time `json:"requested_at"`
	Status          string    `json:"status"`
	ApprovedBy      int64     `json:"approved_by,omitempty"`
	ApprovedAt      time.Time `json:"approved_at,omitempty"`
	RejectionReason string    `json:"rejection_reason,omitempty"`
}

func (s *Service) Routes(r chi.Router) {
	// Workflow configuration (admin only)
	r.Post("/approval-workflows", s.CreateWorkflow)
	r.Get("/approval-workflows", s.ListWorkflows)
	r.Delete("/approval-workflows/{id}", s.DeleteWorkflow)

	// Approval requests
	r.Post("/approval-requests", s.SubmitRequest)
	r.Get("/approval-requests", s.ListRequests)
	r.Get("/approval-requests/{id}", s.GetRequest)
	r.Post("/approval-requests/{id}/approve", s.Approve)
	r.Post("/approval-requests/{id}/reject", s.Reject)
}

// =====================================================================
// WORKFLOW CONFIGURATION
// =====================================================================

func (s *Service) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateWorkflow(req); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	var id int64
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO approval_workflows (tenant_id, entity_type, min_amount_cents, approver_role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, entity_type) DO UPDATE
		SET min_amount_cents = EXCLUDED.min_amount_cents,
		    approver_role = EXCLUDED.approver_role,
		    is_active = true
		RETURNING id
	`, tid, strings.ToLower(req.EntityType), req.MinAmountCents, req.ApproverRole).Scan(&id)
	if err != nil {
		writeErr(w, http.StatusConflict, "CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": id, "entity_type": req.EntityType,
		"min_amount_cents": req.MinAmountCents, "approver_role": req.ApproverRole,
	})
}

func (s *Service) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, entity_type, min_amount_cents, approver_role, is_active
		FROM approval_workflows WHERE tenant_id = $1 ORDER BY entity_type
	`, tid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type wf struct {
		ID             int64  `json:"id"`
		EntityType     string `json:"entity_type"`
		MinAmountCents int64  `json:"min_amount_cents"`
		ApproverRole   string `json:"approver_role"`
		IsActive       bool   `json:"is_active"`
	}
	var results []wf
	for rows.Next() {
		var w wf
		if err := rows.Scan(&w.ID, &w.EntityType, &w.MinAmountCents, &w.ApproverRole, &w.IsActive); err != nil {
			continue
		}
		results = append(results, w)
	}
	if results == nil {
		results = []wf{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Service) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	_, err := s.pool.Exec(r.Context(),
		`UPDATE approval_workflows SET is_active = false WHERE tenant_id = $1 AND id = $2`, tid, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "is_active": false})
}

// =====================================================================
// APPROVAL REQUESTS
// =====================================================================

type SubmitApprovalRequest struct {
	EntityType   string `json:"entity_type"`
	EntityID     int64  `json:"entity_id"`
	EntityNumber string `json:"entity_number"`
	AmountCents  int64  `json:"amount_cents"`
}

func (s *Service) SubmitRequest(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	var req SubmitApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.EntityType == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "entity_type is required")
		return
	}
	// F-03: support both entity-bound approvals (entity_id set) and
	// amount-based approvals (amount_cents set, entity not yet created).
	if req.EntityID <= 0 && req.AmountCents <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "entity_id or amount_cents is required")
		return
	}

	// Check if approval is required for this entity type and amount
	var requiresApproval bool
	_ = s.pool.QueryRow(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM approval_workflows
			WHERE tenant_id = $1 AND entity_type = $2 AND is_active = true
			  AND min_amount_cents <= $3
		)
	`, tid, strings.ToLower(req.EntityType), req.AmountCents).Scan(&requiresApproval)

	if !requiresApproval {
		writeJSON(w, http.StatusOK, map[string]any{
			"entity_type":       req.EntityType,
			"entity_id":         req.EntityID,
			"approval_required": false,
			"message":           "no approval required for this amount",
		})
		return
	}

	// Create approval request. Dedup targets the right partial unique index:
	// entity-bound requests dedupe on (type, entity_id); amount-based
	// (entity_id=0) requests dedupe on (type, entity_number).
	conflictClause := "ON CONFLICT DO NOTHING"
	if req.EntityID > 0 {
		conflictClause = "ON CONFLICT (tenant_id, entity_type, entity_id) WHERE entity_id > 0 AND status IN ('PENDING','APPROVED') AND consumed_at IS NULL DO NOTHING"
	} else {
		conflictClause = "ON CONFLICT (tenant_id, entity_type, entity_number) WHERE entity_id = 0 AND status IN ('PENDING','APPROVED') AND consumed_at IS NULL DO NOTHING"
	}
	var resp ApprovalRequestResponse
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO approval_requests (tenant_id, entity_type, entity_id, entity_number, requested_by, amount_cents)
		VALUES ($1, $2, $3, $4, $5, $6)
		`+conflictClause+`
		RETURNING id, entity_type, entity_id, entity_number, requested_by, requested_at, status
	`, tid, strings.ToLower(req.EntityType), req.EntityID, req.EntityNumber, uid, req.AmountCents).Scan(
		&resp.ID, &resp.EntityType, &resp.EntityID, &resp.EntityNumber,
		&resp.RequestedBy, &resp.RequestedAt, &resp.Status)
	if err != nil {
		writeErr(w, http.StatusConflict, "ALREADY_SUBMITTED", "approval request already exists for this entity")
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) ListRequests(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	statusFilter := r.URL.Query().Get("status")
	args := []any{tid}
	query := `
		SELECT id, entity_type, entity_id, entity_number, requested_by, requested_at, status,
		       approved_by, approved_at, rejection_reason
		FROM approval_requests WHERE tenant_id = $1`
	if statusFilter != "" {
		query += ` AND status = $2`
		args = append(args, strings.ToUpper(statusFilter))
	}
	query += ` ORDER BY requested_at DESC LIMIT 50`

	rows, err := s.pool.Query(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	var results []ApprovalRequestResponse
	for rows.Next() {
		var resp ApprovalRequestResponse
		var approvedBy, rejectionReason *string
		var approvedAt *time.Time
		if err := rows.Scan(&resp.ID, &resp.EntityType, &resp.EntityID, &resp.EntityNumber,
			&resp.RequestedBy, &resp.RequestedAt, &resp.Status,
			&approvedBy, &approvedAt, &rejectionReason); err != nil {
			continue
		}
		if approvedBy != nil {
			resp.ApprovedBy, _ = strconv.ParseInt(*approvedBy, 10, 64)
		}
		if approvedAt != nil {
			resp.ApprovedAt = *approvedAt
		}
		if rejectionReason != nil {
			resp.RejectionReason = *rejectionReason
		}
		results = append(results, resp)
	}
	if results == nil {
		results = []ApprovalRequestResponse{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Service) GetRequest(w http.ResponseWriter, r *http.Request) {
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
	var resp ApprovalRequestResponse
	var approvedBy, rejectionReason *string
	var approvedAt *time.Time
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, entity_type, entity_id, entity_number, requested_by, requested_at, status,
		       approved_by, approved_at, rejection_reason
		FROM approval_requests WHERE tenant_id = $1 AND id = $2
	`, tid, id).Scan(&resp.ID, &resp.EntityType, &resp.EntityID, &resp.EntityNumber,
		&resp.RequestedBy, &resp.RequestedAt, &resp.Status,
		&approvedBy, &approvedAt, &rejectionReason)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "approval request not found")
		return
	}
	if approvedBy != nil {
		resp.ApprovedBy, _ = strconv.ParseInt(*approvedBy, 10, 64)
	}
	if approvedAt != nil {
		resp.ApprovedAt = *approvedAt
	}
	if rejectionReason != nil {
		resp.RejectionReason = *rejectionReason
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Approve(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	_, err := s.pool.Exec(r.Context(), `
		UPDATE approval_requests
		SET status = 'APPROVED', approved_by = $3, approved_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status = 'PENDING'
	`, tid, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "APPROVE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          id,
		"status":      "APPROVED",
		"approved_by": uid,
		"message":     "entity approved — you may now post the journal entry",
	})
}

func (s *Service) Reject(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.Reason) == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "rejection reason is required")
		return
	}
	_, err := s.pool.Exec(r.Context(), `
		UPDATE approval_requests
		SET status = 'REJECTED', approved_by = $3, approved_at = now(), rejection_reason = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'PENDING'
	`, tid, id, uid, strings.TrimSpace(req.Reason))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "REJECT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      id,
		"status":  "REJECTED",
		"reason":  req.Reason,
		"message": "entity rejected",
	})
}

// =====================================================================
// Validation & Helpers
// =====================================================================

func validateWorkflow(req CreateWorkflowRequest) (string, string) {
	if strings.TrimSpace(req.EntityType) == "" {
		return "INVALID_REQUEST", "entity_type is required"
	}
	switch strings.ToLower(req.ApproverRole) {
	case "admin", "accountant", "manager":
	default:
		return "INVALID_REQUEST", "approver_role must be one of: admin, accountant, manager"
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
	message = httperr.SanitizeMessage(status, code, message)
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
