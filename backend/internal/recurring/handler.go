package recurring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// F-07: Recurring / Scheduled Transactions
//   Templates for rent, insurance, salary, subscriptions, etc.
//   The scheduler checks next_date and auto-posts if due.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Post("/recurring", service.Create)
	router.Get("/recurring", service.List)
	router.Get("/recurring/{id}", service.Get)
	router.Put("/recurring/{id}", service.Update)
	router.Delete("/recurring/{id}", service.Deactivate)
	router.Post("/recurring/{id}/post", service.PostNow)
}

type CreateRecurringRequest struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	IntentType         string `json:"intent_type"` // CASH_IN, CASH_OUT, TRANSFER, MANUAL_JOURNAL
	Frequency          string `json:"frequency"`   // daily, weekly, monthly, quarterly, yearly
	NextDate           string `json:"next_date"`
	EndDate            string `json:"end_date"`
	AmountCents        int64  `json:"amount_cents"`
	FromAccountID      int64  `json:"from_account_id"`
	ToAccountID        int64  `json:"to_account_id"`
	PaymentDescription string `json:"payment_description"`
}

func (service *Service) Create(writer http.ResponseWriter, request *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenantID <= 0 {
		writeJSON(writer, http.StatusUnauthorized, errBody{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	userID, _ := auth.UserIDFromContext(request.Context())

	var req CreateRecurringRequest
	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", err.Error()})
		return
	}
	if code, msg := validateRecurring(req); code != "" {
		writeJSON(writer, http.StatusBadRequest, errBody{code, msg})
		return
	}

	var id int64
	err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(request.Context(), `
			INSERT INTO recurring_transactions
			    (tenant_id, code, name, description, intent_type, frequency, next_date, end_date,
			     amount_cents, from_account_id, to_account_id, payment_description, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id
		`, tenantID, req.Code, req.Name, req.Description, req.IntentType, req.Frequency,
			req.NextDate, nullIfEmpty(req.EndDate), req.AmountCents,
			nullIfZero(req.FromAccountID), nullIfZero(req.ToAccountID),
			req.PaymentDescription, userID).Scan(&id); err != nil {
			return err
		}
		return audit.Log(request.Context(), tx, tenantID, userID, "recurring_transaction", id, audit.ActionCreate, nil, map[string]any{
			"code":         req.Code,
			"name":         req.Name,
			"intent_type":  req.IntentType,
			"frequency":    req.Frequency,
			"amount_cents": req.AmountCents,
		})
	})
	if err != nil {
		writeJSON(writer, http.StatusConflict, errBody{"CREATE_FAILED", err.Error()})
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{"id": id, "code": req.Code, "name": req.Name})
}

func (service *Service) List(writer http.ResponseWriter, request *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenantID <= 0 {
		writeJSON(writer, http.StatusUnauthorized, errBody{"TENANT_REQUIRED", "tenant context is required"})
		return
	}

	activeOnly := request.URL.Query().Get("active") == "true"
	query := `
		SELECT id, code, name, description, intent_type, frequency, next_date, end_date,
		       last_posted_date, amount_cents, is_active
		FROM recurring_transactions
		WHERE tenant_id = $1`
	args := []any{tenantID}
	if activeOnly {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY next_date`

	rows, err := service.pool.Query(request.Context(), query, args...)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errBody{"QUERY_FAILED", err.Error()})
		return
	}
	defer rows.Close()

	type item struct {
		ID             int64  `json:"id"`
		Code           string `json:"code"`
		Name           string `json:"name"`
		Description    string `json:"description"`
		IntentType     string `json:"intent_type"`
		Frequency      string `json:"frequency"`
		NextDate       string `json:"next_date"`
		EndDate        string `json:"end_date,omitempty"`
		LastPostedDate string `json:"last_posted_date,omitempty"`
		AmountCents    int64  `json:"amount_cents"`
		IsActive       bool   `json:"is_active"`
	}
	var items []item
	for rows.Next() {
		var it item
		var nextDate, endDate, lastPosted time.Time
		var desc string
		if err := rows.Scan(&it.ID, &it.Code, &it.Name, &desc, &it.IntentType, &it.Frequency,
			&nextDate, &endDate, &lastPosted, &it.AmountCents, &it.IsActive); err != nil {
			writeJSON(writer, http.StatusInternalServerError, errBody{"SCAN_FAILED", err.Error()})
			return
		}
		it.Description = desc
		it.NextDate = nextDate.Format("2006-01-02")
		if !endDate.IsZero() {
			it.EndDate = endDate.Format("2006-01-02")
		}
		if !lastPosted.IsZero() {
			it.LastPostedDate = lastPosted.Format("2006-01-02")
		}
		items = append(items, it)
	}
	if items == nil {
		items = []item{}
	}
	writeJSON(writer, http.StatusOK, items)
}

func (service *Service) Get(writer http.ResponseWriter, request *http.Request) {
	// Simplified: reuse List logic with ID filter
	writeJSON(writer, http.StatusNotImplemented, errBody{"NOT_IMPLEMENTED", "use List"})
}

func (service *Service) Update(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusNotImplemented, errBody{"NOT_IMPLEMENTED", "TODO"})
}

func (service *Service) Deactivate(writer http.ResponseWriter, request *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(request.Context())
	userID, _ := auth.UserIDFromContext(request.Context())
	id := pathID(chi.URLParam(request, "id"))
	if id <= 0 {
		writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", "id required"})
		return
	}
	err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(),
			`UPDATE recurring_transactions SET is_active = false, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
			tenantID, id); err != nil {
			return err
		}
		return audit.Log(request.Context(), tx, tenantID, userID, "recurring_transaction", id, audit.ActionClose, nil, map[string]any{
			"is_active": false,
		})
	})
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errBody{"UPDATE_FAILED", err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "deactivated"})
}

// PostNow manually triggers a recurring transaction.
func (service *Service) PostNow(writer http.ResponseWriter, request *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(request.Context())
	userID, _ := auth.UserIDFromContext(request.Context())
	id := pathID(chi.URLParam(request, "id"))
	if id <= 0 {
		writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", "id required"})
		return
	}

	// Load the recurring template
	var code, name, intentType, frequency string
	var nextDate time.Time
	var amountCents int64
	var fromAcct, toAcct int64
	var paymentDesc string
	var isActive bool
	err := service.pool.QueryRow(request.Context(), `
		SELECT code, name, intent_type, frequency, next_date, amount_cents,
		       from_account_id, to_account_id, payment_description, is_active
		FROM recurring_transactions
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id).Scan(&code, &name, &intentType, &frequency, &nextDate, &amountCents,
		&fromAcct, &toAcct, &paymentDesc, &isActive)
	if err != nil {
		writeJSON(writer, http.StatusNotFound, errBody{"NOT_FOUND", "recurring transaction not found"})
		return
	}
	if !isActive {
		writeJSON(writer, http.StatusBadRequest, errBody{"INACTIVE", "recurring transaction is inactive"})
		return
	}

	// Update next_date based on frequency
	nextNext := computeNextDate(nextDate, frequency)
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `
			UPDATE recurring_transactions
			SET last_posted_date = now()::date, next_date = $3, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, id, nextNext); err != nil {
			return err
		}
		return audit.Log(request.Context(), tx, tenantID, userID, "recurring_transaction", id, audit.ActionPost, map[string]any{
			"next_date": nextDate.Format("2006-01-02"),
		}, map[string]any{
			"code":           code,
			"intent_type":    intentType,
			"amount_cents":   amountCents,
			"last_posted_at": time.Now().Format("2006-01-02"),
			"next_date":      nextNext.Format("2006-01-02"),
			"posted_by":      userID,
		})
	})
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errBody{"UPDATE_FAILED", err.Error()})
		return
	}

	// Note: Actual journal posting would call the appropriate handler
	// (cash-in, cash-out, transfer, etc.) based on intent_type.
	// For now, we return the posting intent so the caller can follow up.
	writeJSON(writer, http.StatusOK, map[string]any{
		"id":              id,
		"code":            code,
		"name":            name,
		"intent_type":     intentType,
		"amount_cents":    amountCents,
		"from_account_id": fromAcct,
		"to_account_id":   toAcct,
		"description":     paymentDesc,
		"posted_at":       time.Now().Format("2006-01-02"),
		"next_date":       nextNext.Format("2006-01-02"),
		"posted_by":       userID,
		"message":         "recurring transaction triggered — post the journal via the intent endpoint",
	})
}

func validateRecurring(req CreateRecurringRequest) (string, string) {
	if strings.TrimSpace(req.Code) == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if strings.TrimSpace(req.Name) == "" {
		return "INVALID_REQUEST", "name is required"
	}
	if req.AmountCents <= 0 {
		return "INVALID_REQUEST", "amount_cents must be > 0"
	}
	switch req.IntentType {
	case "CASH_IN", "CASH_OUT", "TRANSFER", "MANUAL_JOURNAL":
	default:
		return "INVALID_REQUEST", "intent_type must be one of: CASH_IN, CASH_OUT, TRANSFER, MANUAL_JOURNAL"
	}
	switch req.Frequency {
	case "daily", "weekly", "monthly", "quarterly", "yearly":
	default:
		return "INVALID_REQUEST", "frequency must be one of: daily, weekly, monthly, quarterly, yearly"
	}
	if req.NextDate == "" {
		return "INVALID_REQUEST", "next_date is required (YYYY-MM-DD)"
	}
	if _, err := time.Parse("2006-01-02", req.NextDate); err != nil {
		return "INVALID_REQUEST", "next_date must be YYYY-MM-DD"
	}
	if req.EndDate != "" {
		if _, err := time.Parse("2006-01-02", req.EndDate); err != nil {
			return "INVALID_REQUEST", "end_date must be YYYY-MM-DD"
		}
	}
	return "", ""
}

// computeNextDate advances the date by the frequency interval.
func computeNextDate(current time.Time, frequency string) time.Time {
	switch frequency {
	case "daily":
		return current.AddDate(0, 0, 1)
	case "weekly":
		return current.AddDate(0, 0, 7)
	case "monthly":
		return current.AddDate(0, 1, 0)
	case "quarterly":
		return current.AddDate(0, 3, 0)
	case "yearly":
		return current.AddDate(1, 0, 0)
	default:
		return current.AddDate(0, 1, 0)
	}
}

func pathID(raw string) int64 {
	var id int64
	fmt.Sscanf(raw, "%d", &id)
	return id
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
