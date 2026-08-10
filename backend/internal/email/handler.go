package email

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
// F-15: Email Notification
//   Manage reusable email templates and an email queue. Sending is deferred —
//   the /send endpoint marks an attempt and returns 202 Accepted; actual SMTP
//   delivery is handled out-of-band by a worker.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CreateTemplateRequest is the JSON body for POST /email/templates.
type CreateTemplateRequest struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Subject      string  `json:"subject"`
	BodyHTML     string  `json:"body_html"`
	BodyText     *string `json:"body_text"`
	TriggerEvent string  `json:"trigger_event"`
	IsActive     *bool   `json:"is_active"`
}

// UpdateTemplateRequest is the JSON body for PUT /email/templates/{id}.
type UpdateTemplateRequest struct {
	Name         *string `json:"name"`
	Subject      *string `json:"subject"`
	BodyHTML     *string `json:"body_html"`
	BodyText     *string `json:"body_text"`
	TriggerEvent *string `json:"trigger_event"`
	IsActive     *bool   `json:"is_active"`
}

// EnqueueRequest is the JSON body for POST /email/queue. Either TemplateID or
// (Subject + BodyHTML) must be supplied; the template path resolves subject &
// body from the template.
type EnqueueRequest struct {
	TemplateID  *int64  `json:"template_id"`
	ToEmail     string  `json:"to_email"`
	CCEmail     *string `json:"cc_email"`
	BCCEmail    *string `json:"bcc_email"`
	Subject     *string `json:"subject"`
	BodyHTML    *string `json:"body_html"`
	BodyText    *string `json:"body_text"`
	EntityType  *string `json:"entity_type"`
	EntityID    *int64  `json:"entity_id"`
}

type TemplateResponse struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Subject      string    `json:"subject"`
	BodyHTML     string    `json:"body_html"`
	BodyText     string    `json:"body_text"`
	TriggerEvent string    `json:"trigger_event"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type QueueResponse struct {
	ID          int64          `json:"id"`
	TemplateID  *int64         `json:"template_id"`
	ToEmail     string         `json:"to_email"`
	CCEmail     string         `json:"cc_email"`
	BCCEmail    string         `json:"bcc_email"`
	Subject     string         `json:"subject"`
	BodyHTML    string         `json:"body_html"`
	BodyText    string         `json:"body_text"`
	Status      string         `json:"status"`
	RetryCount  int            `json:"retry_count"`
	MaxRetries  int            `json:"max_retries"`
	LastError   string         `json:"last_error"`
	SentAt      *time.Time     `json:"sent_at"`
	EntityType  string         `json:"entity_type"`
	EntityID    *int64         `json:"entity_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (s *Service) Routes(r chi.Router) {
	r.Post("/email/templates", s.CreateTemplate)
	r.Get("/email/templates", s.ListTemplates)
	r.Put("/email/templates/{id}", s.UpdateTemplate)
	r.Delete("/email/templates/{id}", s.DeactivateTemplate)
	r.Post("/email/queue", s.Enqueue)
	r.Get("/email/queue", s.ListQueue)
	r.Post("/email/queue/{id}/send", s.Send)
	r.Post("/email/queue/{id}/cancel", s.Cancel)
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func (s *Service) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateTemplate(req.Code, req.Name, req.Subject, req.BodyHTML, req.TriggerEvent); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	var resp TemplateResponse
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO email_templates (
			tenant_id, code, name, subject, body_html, body_text,
			trigger_event, is_active
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, code, name, subject, body_html, body_text,
			trigger_event, is_active, created_at, updated_at
	`,
		tid, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
		strings.TrimSpace(req.Subject), req.BodyHTML, req.BodyText,
		strings.ToUpper(strings.TrimSpace(req.TriggerEvent)), active,
	).Scan(
		&resp.ID, &resp.Code, &resp.Name, &resp.Subject, &resp.BodyHTML,
		&resp.BodyText, &resp.TriggerEvent, &resp.IsActive, &resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) ListTemplates(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, code, name, subject, body_html, body_text,
			trigger_event, is_active, created_at, updated_at
		FROM email_templates
		WHERE tenant_id = $1
		ORDER BY code
	`, tid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()
	results := make([]TemplateResponse, 0)
	for rows.Next() {
		var t TemplateResponse
		if err := rows.Scan(
			&t.ID, &t.Code, &t.Name, &t.Subject, &t.BodyHTML, &t.BodyText,
			&t.TriggerEvent, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Service) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
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
	var req UpdateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
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
	if req.Subject != nil {
		sets = append(sets, "subject = $"+strconv.Itoa(idx))
		args = append(args, strings.TrimSpace(*req.Subject))
		idx++
	}
	if req.BodyHTML != nil {
		sets = append(sets, "body_html = $"+strconv.Itoa(idx))
		args = append(args, *req.BodyHTML)
		idx++
	}
	if req.BodyText != nil {
		sets = append(sets, "body_text = $"+strconv.Itoa(idx))
		args = append(args, req.BodyText)
		idx++
	}
	if req.TriggerEvent != nil {
		sets = append(sets, "trigger_event = $"+strconv.Itoa(idx))
		args = append(args, strings.ToUpper(strings.TrimSpace(*req.TriggerEvent)))
		idx++
	}
	if req.IsActive != nil {
		sets = append(sets, "is_active = $"+strconv.Itoa(idx))
		args = append(args, *req.IsActive)
		idx++
	}
	if len(sets) == 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "no fields to update")
		return
	}
	sets = append(sets, "updated_at = now()")

	var resp TemplateResponse
	err := s.pool.QueryRow(r.Context(), `
		UPDATE email_templates SET `+strings.Join(sets, ", ")+`
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, code, name, subject, body_html, body_text,
			trigger_event, is_active, created_at, updated_at
	`, args...).Scan(
		&resp.ID, &resp.Code, &resp.Name, &resp.Subject, &resp.BodyHTML,
		&resp.BodyText, &resp.TriggerEvent, &resp.IsActive, &resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "email template not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) DeactivateTemplate(w http.ResponseWriter, r *http.Request) {
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
	var resp TemplateResponse
	err := s.pool.QueryRow(r.Context(), `
		UPDATE email_templates
		SET is_active = false, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, code, name, subject, body_html, body_text,
			trigger_event, is_active, created_at, updated_at
	`, tid, id).Scan(
		&resp.ID, &resp.Code, &resp.Name, &resp.Subject, &resp.BodyHTML,
		&resp.BodyText, &resp.TriggerEvent, &resp.IsActive, &resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "email template not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Queue
// ---------------------------------------------------------------------------

func (s *Service) Enqueue(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	var req EnqueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateEnqueue(req); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}

	// Resolve subject/body from template when a template_id is supplied.
	var subject, bodyHTML, bodyText string
	if req.TemplateID != nil && *req.TemplateID > 0 {
		err := s.pool.QueryRow(r.Context(), `
			SELECT subject, body_html, COALESCE(body_text, '')
			FROM email_templates
			WHERE tenant_id = $1 AND id = $2 AND is_active = true
		`, tid, *req.TemplateID).Scan(&subject, &bodyHTML, &bodyText)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "TEMPLATE_NOT_FOUND", "active email template not found")
			return
		}
	} else {
		if req.Subject == nil || strings.TrimSpace(*req.Subject) == "" {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "either template_id or subject is required")
			return
		}
		subject = strings.TrimSpace(*req.Subject)
		if req.BodyHTML != nil {
			bodyHTML = *req.BodyHTML
		}
		if req.BodyText != nil {
			bodyText = *req.BodyText
		}
	}

	var resp QueueResponse
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO email_queue (
			tenant_id, template_id, to_email, cc_email, bcc_email,
			subject, body_html, body_text, status, entity_type, entity_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'PENDING',$9,$10)
		RETURNING id, template_id, to_email, cc_email, bcc_email, subject,
			body_html, body_text, status, retry_count, max_retries, last_error,
			sent_at, entity_type, entity_id, created_at, updated_at
	`,
		tid, req.TemplateID, strings.TrimSpace(req.ToEmail), req.CCEmail,
		req.BCCEmail, subject, bodyHTML, bodyText, req.EntityType, req.EntityID,
	).Scan(
		&resp.ID, &resp.TemplateID, &resp.ToEmail, &resp.CCEmail, &resp.BCCEmail,
		&resp.Subject, &resp.BodyHTML, &resp.BodyText, &resp.Status,
		&resp.RetryCount, &resp.MaxRetries, &resp.LastError, &resp.SentAt,
		&resp.EntityType, &resp.EntityID, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) ListQueue(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	query := `
		SELECT id, template_id, to_email, cc_email, bcc_email, subject,
			body_html, body_text, status, retry_count, max_retries, last_error,
			sent_at, entity_type, entity_id, created_at, updated_at
		FROM email_queue
		WHERE tenant_id = $1`
	args := []any{tid}
	if statusFilter != "" {
		args = append(args, strings.ToUpper(statusFilter))
		query += " AND status = $" + strconv.Itoa(len(args))
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()
	results := make([]QueueResponse, 0)
	for rows.Next() {
		var q QueueResponse
		if err := rows.Scan(
			&q.ID, &q.TemplateID, &q.ToEmail, &q.CCEmail, &q.BCCEmail,
			&q.Subject, &q.BodyHTML, &q.BodyText, &q.Status, &q.RetryCount,
			&q.MaxRetries, &q.LastError, &q.SentAt, &q.EntityType, &q.EntityID,
			&q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		results = append(results, q)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// Send attempts to send a queued email. Actual SMTP delivery is deferred to a
// background worker; this endpoint records a send attempt by marking the
// message SENT (when still PENDING) and returns 202 Accepted.
func (s *Service) Send(w http.ResponseWriter, r *http.Request) {
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
	var resp QueueResponse
	err := s.pool.QueryRow(r.Context(), `
		UPDATE email_queue
		SET status = 'SENT', sent_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status = 'PENDING'
		RETURNING id, template_id, to_email, cc_email, bcc_email, subject,
			body_html, body_text, status, retry_count, max_retries, last_error,
			sent_at, entity_type, entity_id, created_at, updated_at
	`, tid, id).Scan(
		&resp.ID, &resp.TemplateID, &resp.ToEmail, &resp.CCEmail, &resp.BCCEmail,
		&resp.Subject, &resp.BodyHTML, &resp.BodyText, &resp.Status,
		&resp.RetryCount, &resp.MaxRetries, &resp.LastError, &resp.SentAt,
		&resp.EntityType, &resp.EntityID, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusConflict, "INVALID_STATE", "email cannot be sent (must be in PENDING status)")
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Service) Cancel(w http.ResponseWriter, r *http.Request) {
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
	var resp QueueResponse
	err := s.pool.QueryRow(r.Context(), `
		UPDATE email_queue
		SET status = 'CANCELLED', updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status = 'PENDING'
		RETURNING id, template_id, to_email, cc_email, bcc_email, subject,
			body_html, body_text, status, retry_count, max_retries, last_error,
			sent_at, entity_type, entity_id, created_at, updated_at
	`, tid, id).Scan(
		&resp.ID, &resp.TemplateID, &resp.ToEmail, &resp.CCEmail, &resp.BCCEmail,
		&resp.Subject, &resp.BodyHTML, &resp.BodyText, &resp.Status,
		&resp.RetryCount, &resp.MaxRetries, &resp.LastError, &resp.SentAt,
		&resp.EntityType, &resp.EntityID, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusConflict, "INVALID_STATE", "email cannot be cancelled (must be in PENDING status)")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Validation & Helpers
// ---------------------------------------------------------------------------

func validateTemplate(code, name, subject, bodyHTML, triggerEvent string) (string, string) {
	if strings.TrimSpace(code) == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if strings.TrimSpace(name) == "" {
		return "INVALID_REQUEST", "name is required"
	}
	if strings.TrimSpace(subject) == "" {
		return "INVALID_REQUEST", "subject is required"
	}
	if strings.TrimSpace(bodyHTML) == "" {
		return "INVALID_REQUEST", "body_html is required"
	}
	if strings.TrimSpace(triggerEvent) == "" {
		return "INVALID_REQUEST", "trigger_event is required"
	}
	return "", ""
}

// validateEnqueue checks the pure (non-DB) validation rules for an enqueue
// request: to_email is always required, and when no template_id is supplied
// (or it is non-positive) a subject must be provided instead. Returns an
// error code and message, or ("","") when the request is valid.
func validateEnqueue(req EnqueueRequest) (string, string) {
	if strings.TrimSpace(req.ToEmail) == "" {
		return "INVALID_REQUEST", "to_email is required"
	}
	if req.TemplateID == nil || *req.TemplateID <= 0 {
		if req.Subject == nil || strings.TrimSpace(*req.Subject) == "" {
			return "INVALID_REQUEST", "either template_id or subject is required"
		}
	}
	return "", ""
}

// Email queue status constants.
const (
	EmailStatusPending   = "PENDING"
	EmailStatusSent      = "SENT"
	EmailStatusCancelled = "CANCELLED"
)

// canTransitionEmailStatus reports whether a queue item in the current status
// is allowed to move to the target status. The lifecycle is:
//
//	PENDING → SENT
//	PENDING → CANCELLED
//
// SENT and CANCELLED are terminal.
func canTransitionEmailStatus(current, target string) bool {
	c := strings.ToUpper(strings.TrimSpace(current))
	t := strings.ToUpper(strings.TrimSpace(target))
	if c != EmailStatusPending {
		return false
	}
	return t == EmailStatusSent || t == EmailStatusCancelled
}

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
