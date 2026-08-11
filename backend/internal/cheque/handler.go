package cheque

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// F-14: Giro & Cheque Management
//   Register cheques and giros, track them through deposit → clear / bounce,
//   and keep an auditable status lifecycle per cheque.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CreateChequeRequest is the JSON body for POST /cheques. Nullable fields use
// pointers so callers can distinguish "omitted" from zero.
type CreateChequeRequest struct {
	ChequeNumber      string  `json:"cheque_number"`
	ChequeType        string  `json:"cheque_type"`
	Direction         string  `json:"direction"`
	BankName          *string `json:"bank_name"`
	BankAccountNumber *string `json:"bank_account_number"`
	Payee             *string `json:"payee"`
	Drawer            *string `json:"drawer"`
	AmountCents       int64   `json:"amount_cents"`
	IssueDate         string  `json:"issue_date"`
	DueDate           *string `json:"due_date"`
	JournalEntryID    *int64  `json:"journal_entry_id"`
	PaymentID         *int64  `json:"payment_id"`
	Description       *string `json:"description"`
}

// UpdateChequeRequest is the JSON body for PUT /cheques/{id}. All fields are
// optional; only provided fields are updated.
type UpdateChequeRequest struct {
	BankName          *string `json:"bank_name"`
	BankAccountNumber *string `json:"bank_account_number"`
	Payee             *string `json:"payee"`
	Drawer            *string `json:"drawer"`
	AmountCents       *int64  `json:"amount_cents"`
	IssueDate         *string `json:"issue_date"`
	DueDate           *string `json:"due_date"`
	Description       *string `json:"description"`
}

// BounceRequest is the JSON body for POST /cheques/{id}/bounce.
type BounceRequest struct {
	Reason string `json:"reason"`
}

type ChequeResponse struct {
	ID                int64     `json:"id"`
	ChequeNumber      string    `json:"cheque_number"`
	ChequeType        string    `json:"cheque_type"`
	Direction         string    `json:"direction"`
	BankName          string    `json:"bank_name"`
	BankAccountNumber string    `json:"bank_account_number"`
	Payee             string    `json:"payee"`
	Drawer            string    `json:"drawer"`
	AmountCents       int64     `json:"amount_cents"`
	IssueDate         string    `json:"issue_date"`
	DueDate           string    `json:"due_date"`
	ClearingDate      string    `json:"clearing_date"`
	Status            string    `json:"status"`
	BouncedReason     string    `json:"bounced_reason"`
	JournalEntryID    *int64    `json:"journal_entry_id"`
	PaymentID         *int64    `json:"payment_id"`
	Description       string    `json:"description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (s *Service) Routes(r chi.Router) {
	r.Post("/cheques", s.Create)
	r.Get("/cheques", s.List)
	r.Get("/cheques/{id}", s.Get)
	r.Put("/cheques/{id}", s.Update)
	r.Post("/cheques/{id}/deposit", s.Deposit)
	r.Post("/cheques/{id}/clear", s.Clear)
	r.Post("/cheques/{id}/bounce", s.Bounce)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	var req CreateChequeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateCheque(req); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	issueDate, err := time.Parse("2006-01-02", req.IssueDate)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "issue_date must be YYYY-MM-DD")
		return
	}
	var dueDate pgtype.Date
	if req.DueDate != nil && strings.TrimSpace(*req.DueDate) != "" {
		d, err := time.Parse("2006-01-02", *req.DueDate)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "due_date must be YYYY-MM-DD")
			return
		}
		dueDate = pgtype.Date{Time: d, Valid: true}
	}
	var resp ChequeResponse
	uid, _ := auth.UserIDFromContext(r.Context())
	err = db.WithTransaction(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO cheques (
				tenant_id, cheque_number, cheque_type, direction, bank_name,
				bank_account_number, payee, drawer, amount_cents, issue_date,
				due_date, status, journal_entry_id, payment_id, description
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'REGISTERED',$12,$13,$14)
			RETURNING id, cheque_number, cheque_type, direction, bank_name,
				bank_account_number, payee, drawer, amount_cents, issue_date,
				due_date, clearing_date, status, bounced_reason, journal_entry_id,
				payment_id, description, created_at, updated_at
		`,
			tid, strings.TrimSpace(req.ChequeNumber),
			strings.ToUpper(strings.TrimSpace(req.ChequeType)),
			strings.ToUpper(strings.TrimSpace(req.Direction)),
			req.BankName, req.BankAccountNumber, req.Payee, req.Drawer,
			req.AmountCents, issueDate, dueDate, req.JournalEntryID, req.PaymentID,
			req.Description,
		).Scan(
			&resp.ID, &resp.ChequeNumber, &resp.ChequeType, &resp.Direction,
			&resp.BankName, &resp.BankAccountNumber, &resp.Payee, &resp.Drawer,
			&resp.AmountCents, &resp.IssueDate, &resp.DueDate, &resp.ClearingDate,
			&resp.Status, &resp.BouncedReason, &resp.JournalEntryID, &resp.PaymentID,
			&resp.Description, &resp.CreatedAt, &resp.UpdatedAt,
		); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, tid, uid, "cheque", resp.ID, audit.ActionCreate, nil, map[string]any{
			"cheque_number": resp.ChequeNumber,
			"cheque_type":   resp.ChequeType,
			"direction":     resp.Direction,
			"amount_cents":  resp.AmountCents,
			"status":        "REGISTERED",
		})
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	resp.IssueDate = formatScannedDate(resp.IssueDate)
	resp.DueDate = formatScannedDate(resp.DueDate)
	resp.ClearingDate = formatScannedDate(resp.ClearingDate)
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	directionFilter := strings.TrimSpace(r.URL.Query().Get("direction"))

	query := `
		SELECT id, cheque_number, cheque_type, direction, bank_name,
			bank_account_number, payee, drawer, amount_cents, issue_date,
			due_date, clearing_date, status, bounced_reason, journal_entry_id,
			payment_id, description, created_at, updated_at
		FROM cheques
		WHERE tenant_id = $1`
	args := []any{tid}
	if statusFilter != "" {
		args = append(args, strings.ToUpper(statusFilter))
		query += " AND status = $" + strconv.Itoa(len(args))
	}
	if directionFilter != "" {
		args = append(args, strings.ToUpper(directionFilter))
		query += " AND direction = $" + strconv.Itoa(len(args))
	}
	query += " ORDER BY issue_date DESC, id DESC"

	rows, err := s.pool.Query(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()
	results := make([]ChequeResponse, 0)
	for rows.Next() {
		var c ChequeResponse
		if err := rows.Scan(
			&c.ID, &c.ChequeNumber, &c.ChequeType, &c.Direction, &c.BankName,
			&c.BankAccountNumber, &c.Payee, &c.Drawer, &c.AmountCents,
			&c.IssueDate, &c.DueDate, &c.ClearingDate, &c.Status,
			&c.BouncedReason, &c.JournalEntryID, &c.PaymentID, &c.Description,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		c.IssueDate = formatScannedDate(c.IssueDate)
		c.DueDate = formatScannedDate(c.DueDate)
		c.ClearingDate = formatScannedDate(c.ClearingDate)
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
	var resp ChequeResponse
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, cheque_number, cheque_type, direction, bank_name,
			bank_account_number, payee, drawer, amount_cents, issue_date,
			due_date, clearing_date, status, bounced_reason, journal_entry_id,
			payment_id, description, created_at, updated_at
		FROM cheques
		WHERE tenant_id = $1 AND id = $2
	`, tid, id).Scan(
		&resp.ID, &resp.ChequeNumber, &resp.ChequeType, &resp.Direction,
		&resp.BankName, &resp.BankAccountNumber, &resp.Payee, &resp.Drawer,
		&resp.AmountCents, &resp.IssueDate, &resp.DueDate, &resp.ClearingDate,
		&resp.Status, &resp.BouncedReason, &resp.JournalEntryID, &resp.PaymentID,
		&resp.Description, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "cheque not found")
		return
	}
	resp.IssueDate = formatScannedDate(resp.IssueDate)
	resp.DueDate = formatScannedDate(resp.DueDate)
	resp.ClearingDate = formatScannedDate(resp.ClearingDate)
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
	var req UpdateChequeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var sets []string
	var args []any
	args = append(args, tid, id)
	idx := 3
	if req.BankName != nil {
		sets = append(sets, "bank_name = $"+strconv.Itoa(idx))
		args = append(args, req.BankName)
		idx++
	}
	if req.BankAccountNumber != nil {
		sets = append(sets, "bank_account_number = $"+strconv.Itoa(idx))
		args = append(args, req.BankAccountNumber)
		idx++
	}
	if req.Payee != nil {
		sets = append(sets, "payee = $"+strconv.Itoa(idx))
		args = append(args, req.Payee)
		idx++
	}
	if req.Drawer != nil {
		sets = append(sets, "drawer = $"+strconv.Itoa(idx))
		args = append(args, req.Drawer)
		idx++
	}
	if req.AmountCents != nil {
		if *req.AmountCents <= 0 {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "amount_cents must be positive")
			return
		}
		sets = append(sets, "amount_cents = $"+strconv.Itoa(idx))
		args = append(args, *req.AmountCents)
		idx++
	}
	if req.IssueDate != nil {
		d, err := time.Parse("2006-01-02", *req.IssueDate)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "issue_date must be YYYY-MM-DD")
			return
		}
		sets = append(sets, "issue_date = $"+strconv.Itoa(idx))
		args = append(args, d)
		idx++
	}
	if req.DueDate != nil {
		var dueDate pgtype.Date
		if strings.TrimSpace(*req.DueDate) != "" {
			d, err := time.Parse("2006-01-02", *req.DueDate)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "due_date must be YYYY-MM-DD")
				return
			}
			dueDate = pgtype.Date{Time: d, Valid: true}
		}
		sets = append(sets, "due_date = $"+strconv.Itoa(idx))
		args = append(args, dueDate)
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

	var resp ChequeResponse
	err := s.pool.QueryRow(r.Context(), `
		UPDATE cheques SET `+strings.Join(sets, ", ")+`
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, cheque_number, cheque_type, direction, bank_name,
			bank_account_number, payee, drawer, amount_cents, issue_date,
			due_date, clearing_date, status, bounced_reason, journal_entry_id,
			payment_id, description, created_at, updated_at
	`, args...).Scan(
		&resp.ID, &resp.ChequeNumber, &resp.ChequeType, &resp.Direction,
		&resp.BankName, &resp.BankAccountNumber, &resp.Payee, &resp.Drawer,
		&resp.AmountCents, &resp.IssueDate, &resp.DueDate, &resp.ClearingDate,
		&resp.Status, &resp.BouncedReason, &resp.JournalEntryID, &resp.PaymentID,
		&resp.Description, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "cheque not found")
		return
	}
	resp.IssueDate = formatScannedDate(resp.IssueDate)
	resp.DueDate = formatScannedDate(resp.DueDate)
	resp.ClearingDate = formatScannedDate(resp.ClearingDate)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Deposit(w http.ResponseWriter, r *http.Request) {
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
	var resp ChequeResponse
	uid, _ := auth.UserIDFromContext(r.Context())
	err := s.chequeStatusUpdate(r, tid, uid, id, audit.ActionUpdate, "DEPOSITED", `
		UPDATE cheques
		SET status = 'DEPOSITED', clearing_date = NULL, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status = 'REGISTERED'
		RETURNING id, cheque_number, cheque_type, direction, bank_name,
			bank_account_number, payee, drawer, amount_cents, issue_date,
			due_date, clearing_date, status, bounced_reason, journal_entry_id,
			payment_id, description, created_at, updated_at
	`, &resp, nil, tid, id)
	if err != nil {
		writeErr(w, http.StatusConflict, "INVALID_STATE", "cheque cannot be deposited (must be in REGISTERED status)")
		return
	}
	resp.IssueDate = formatScannedDate(resp.IssueDate)
	resp.DueDate = formatScannedDate(resp.DueDate)
	resp.ClearingDate = formatScannedDate(resp.ClearingDate)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Clear(w http.ResponseWriter, r *http.Request) {
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
	var resp ChequeResponse
	uid, _ := auth.UserIDFromContext(r.Context())
	err := s.chequeStatusUpdate(r, tid, uid, id, audit.ActionUpdate, "CLEARED", `
		UPDATE cheques
		SET status = 'CLEARED', clearing_date = CURRENT_DATE, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status IN ('REGISTERED', 'DEPOSITED')
		RETURNING id, cheque_number, cheque_type, direction, bank_name,
			bank_account_number, payee, drawer, amount_cents, issue_date,
			due_date, clearing_date, status, bounced_reason, journal_entry_id,
			payment_id, description, created_at, updated_at
	`, &resp, nil, tid, id)
	if err != nil {
		writeErr(w, http.StatusConflict, "INVALID_STATE", "cheque cannot be cleared (must be REGISTERED or DEPOSITED)")
		return
	}
	resp.IssueDate = formatScannedDate(resp.IssueDate)
	resp.DueDate = formatScannedDate(resp.DueDate)
	resp.ClearingDate = formatScannedDate(resp.ClearingDate)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Bounce(w http.ResponseWriter, r *http.Request) {
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
	var req BounceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateBounceReason(req.Reason); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	var resp ChequeResponse
	uid, _ := auth.UserIDFromContext(r.Context())
	err := s.chequeStatusUpdate(r, tid, uid, id, audit.ActionUpdate, "BOUNCED", `
		UPDATE cheques
		SET status = 'BOUNCED', bounced_reason = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status IN ('REGISTERED', 'DEPOSITED')
		RETURNING id, cheque_number, cheque_type, direction, bank_name,
			bank_account_number, payee, drawer, amount_cents, issue_date,
			due_date, clearing_date, status, bounced_reason, journal_entry_id,
			payment_id, description, created_at, updated_at
	`, &resp, map[string]any{"bounced_reason": strings.TrimSpace(req.Reason)}, tid, id, strings.TrimSpace(req.Reason))
	if err != nil {
		writeErr(w, http.StatusConflict, "INVALID_STATE", "cheque cannot be bounced (must be REGISTERED or DEPOSITED)")
		return
	}
	resp.IssueDate = formatScannedDate(resp.IssueDate)
	resp.DueDate = formatScannedDate(resp.DueDate)
	resp.ClearingDate = formatScannedDate(resp.ClearingDate)
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Validation & Helpers
// ---------------------------------------------------------------------------

// chequeStatusUpdate runs a cheque status-transition UPDATE inside a
// transaction and writes an audit_logs entry (before/after status snapshot)
// atomically with the mutation.
func (s *Service) chequeStatusUpdate(r *http.Request, tid, uid, chequeID int64, action audit.Action, newStatus, query string, resp *ChequeResponse, extraAfter map[string]any, args ...any) error {
	return db.WithTransaction(r.Context(), s.pool, func(tx pgx.Tx) error {
		var chequeNumber string
		var oldStatus string
		if err := tx.QueryRow(r.Context(), `
			SELECT cheque_number, status FROM cheques WHERE tenant_id = $1 AND id = $2 FOR UPDATE
		`, tid, chequeID).Scan(&chequeNumber, &oldStatus); err != nil {
			return err
		}
		if err := tx.QueryRow(r.Context(), query, args...).Scan(
			&resp.ID, &resp.ChequeNumber, &resp.ChequeType, &resp.Direction,
			&resp.BankName, &resp.BankAccountNumber, &resp.Payee, &resp.Drawer,
			&resp.AmountCents, &resp.IssueDate, &resp.DueDate, &resp.ClearingDate,
			&resp.Status, &resp.BouncedReason, &resp.JournalEntryID, &resp.PaymentID,
			&resp.Description, &resp.CreatedAt, &resp.UpdatedAt,
		); err != nil {
			return err
		}
		after := map[string]any{
			"cheque_number": chequeNumber,
			"status":        newStatus,
		}
		for k, v := range extraAfter {
			after[k] = v
		}
		return audit.Log(r.Context(), tx, tid, uid, "cheque", resp.ID, action, map[string]any{
			"cheque_number": chequeNumber,
			"status":        oldStatus,
		}, after)
	})
}

func validateCheque(req CreateChequeRequest) (string, string) {
	if strings.TrimSpace(req.ChequeNumber) == "" {
		return "INVALID_REQUEST", "cheque_number is required"
	}
	ct := strings.ToUpper(strings.TrimSpace(req.ChequeType))
	if ct != "CHEQUE" && ct != "GIRO" {
		return "INVALID_REQUEST", "cheque_type must be CHEQUE or GIRO"
	}
	dir := strings.ToUpper(strings.TrimSpace(req.Direction))
	if dir != "RECEIVED" && dir != "ISSUED" {
		return "INVALID_REQUEST", "direction must be RECEIVED or ISSUED"
	}
	if req.AmountCents <= 0 {
		return "INVALID_REQUEST", "amount_cents must be positive"
	}
	if strings.TrimSpace(req.IssueDate) == "" {
		return "INVALID_REQUEST", "issue_date is required"
	}
	return "", ""
}

// validateBounceReason validates the reason field for a bounce request.
func validateBounceReason(reason string) (string, string) {
	if strings.TrimSpace(reason) == "" {
		return "INVALID_REQUEST", "reason is required"
	}
	return "", ""
}

// chequeStatuses enumerates the lifecycle states a cheque may occupy.
const (
	StatusRegistered = "REGISTERED"
	StatusDeposited  = "DEPOSITED"
	StatusCleared    = "CLEARED"
	StatusBounced    = "BOUNCED"
)

// canTransitionTo reports whether a cheque in the given current status is
// allowed to move to the target status. The lifecycle is:
//
//	REGISTERED → DEPOSITED → CLEARED
//	REGISTERED → CLEARED
//	REGISTERED → BOUNCED
//	DEPOSITED  → BOUNCED
//	DEPOSITED  → CLEARED
//
// CLEARED and BOUNCED are terminal.
func canTransitionTo(current, target string) bool {
	c := strings.ToUpper(strings.TrimSpace(current))
	t := strings.ToUpper(strings.TrimSpace(target))
	switch t {
	case StatusDeposited:
		return c == StatusRegistered
	case StatusCleared:
		return c == StatusRegistered || c == StatusDeposited
	case StatusBounced:
		return c == StatusRegistered || c == StatusDeposited
	default:
		return false
	}
}

// formatScannedDate normalizes the date strings returned by pgx DATE scans.
// pgx scans DATE columns into a string like "2026-01-01T00:00:00Z"; we trim to
// the YYYY-MM-DD prefix for a clean API response.
func formatScannedDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
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
