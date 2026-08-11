package recurring

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
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

// UpdateRecurringRequest is the JSON body for PUT /recurring/{id}. All fields
// are optional; only provided fields are updated.
type UpdateRecurringRequest struct {
	Description *string `json:"description"`
	AmountCents *int64  `json:"amount_cents"`
	NextDate    *string `json:"next_date"`
	IsActive    *bool   `json:"is_active"`
}

// Update modifies an existing recurring transaction's description, amount,
// next_date, and/or is_active flag. Runs inside a transaction so the RLS
// tenant context (app.tenant_id) is set before the UPDATE — without it the
// FORCE RLS policy on recurring_transactions filters out every row.
func (service *Service) Update(writer http.ResponseWriter, request *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenantID <= 0 {
		writeJSON(writer, http.StatusUnauthorized, errBody{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	id := pathID(chi.URLParam(request, "id"))
	if id <= 0 {
		writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", "id required"})
		return
	}

	var req UpdateRecurringRequest
	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", err.Error()})
		return
	}

	var sets []string
	var args []any
	args = append(args, tenantID, id)
	idx := 3
	if req.Description != nil {
		sets = append(sets, "description = $"+strconv.Itoa(idx))
		args = append(args, *req.Description)
		idx++
	}
	if req.AmountCents != nil {
		if *req.AmountCents <= 0 {
			writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", "amount_cents must be > 0"})
			return
		}
		sets = append(sets, "amount_cents = $"+strconv.Itoa(idx))
		args = append(args, *req.AmountCents)
		idx++
	}
	if req.NextDate != nil {
		if _, err := time.Parse("2006-01-02", *req.NextDate); err != nil {
			writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", "next_date must be YYYY-MM-DD"})
			return
		}
		sets = append(sets, "next_date = $"+strconv.Itoa(idx))
		args = append(args, *req.NextDate)
		idx++
	}
	if req.IsActive != nil {
		sets = append(sets, "is_active = $"+strconv.Itoa(idx))
		args = append(args, *req.IsActive)
		idx++
	}
	if len(sets) == 0 {
		writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", "no fields to update"})
		return
	}
	sets = append(sets, "updated_at = now()")
	updatedFields := sets[:len(sets)-1] // exclude the "updated_at = now()" entry

	var rowsAffected int64
	err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenantID); err != nil {
			return err
		}
		tag, err := tx.Exec(request.Context(), `
			UPDATE recurring_transactions SET `+strings.Join(sets, ", ")+`
			WHERE tenant_id = $1 AND id = $2
		`, args...)
		if err != nil {
			return err
		}
		rowsAffected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		log.Printf("recurring: update failed: tenant=%d id=%d: %v", tenantID, id, err)
		writeJSON(writer, http.StatusInternalServerError, errBody{"UPDATE_FAILED", "failed to update recurring transaction"})
		return
	}
	if rowsAffected == 0 {
		writeJSON(writer, http.StatusNotFound, errBody{"NOT_FOUND", "recurring transaction not found"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"id":      id,
		"status":  "updated",
		"updated": updatedFields,
	})
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

// PostNow manually triggers a recurring transaction: it reads the template,
// posts a journal (Dr from_account / Cr to_account for amount_cents), advances
// next_date, and records last_posted_date + last_journal_id.
func (service *Service) PostNow(writer http.ResponseWriter, request *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(request.Context())
	userID, _ := auth.UserIDFromContext(request.Context())
	id := pathID(chi.URLParam(request, "id"))
	if id <= 0 {
		writeJSON(writer, http.StatusBadRequest, errBody{"INVALID_REQUEST", "id required"})
		return
	}
	idem := idempotencyKeyOrGenerate(request)

	var result map[string]any
	err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenantID); err != nil {
			return err
		}
		// Idempotent replay.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenantID,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			result = map[string]any{
				"idempotent_replay": true,
				"journal_entry_id":  existing.ID,
				"journal_number":    existing.Number,
			}
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Lock the recurring row so concurrent PostNow calls serialize and
		// cannot both post the same period (FOR UPDATE).
		var code, name, intentType, frequency, description string
		var nextDate time.Time
		var amountCents int64
		var fromAcct, toAcct int64
		var isActive bool
		err = tx.QueryRow(request.Context(), `
			SELECT code, name, intent_type, frequency, next_date, amount_cents,
			       from_account_id, to_account_id, COALESCE(description, ''), is_active
			FROM recurring_transactions
			WHERE tenant_id = $1 AND id = $2
			FOR UPDATE
		`, tenantID, id).Scan(&code, &name, &intentType, &frequency, &nextDate, &amountCents,
			&fromAcct, &toAcct, &description, &isActive)
		if err != nil {
			return errNotFound
		}
		if !isActive {
			return errInactive
		}
		if fromAcct <= 0 || toAcct <= 0 {
			return errMissingAccounts
		}

		// Post the journal: Dr from_account / Cr to_account.
		entryDate := nextDate.Format("2006-01-02")
		sourceRef := fmt.Sprintf("REC-%d-%s", id, entryDate)
		journal := accounting.Journal{
			TenantID:    tenantID,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType(intentType),
			EntryDate:   entryDate,
			Description: description,
			Lines: []accounting.Line{
				{AccountID: fromAcct, DebitCents: amountCents, SourceLineRef: "debit"},
				{AccountID: toAcct, CreditCents: amountCents, SourceLineRef: "credit"},
			},
		}
		posted, err := postJournal(request.Context(), tx, tenantID, idem, journal, userID, 0)
		if err != nil {
			return err
		}

		// Advance next_date and record last_posted_date + last_journal_id.
		nextNext := computeNextDate(nextDate, frequency)
		if _, err := tx.Exec(request.Context(), `
			UPDATE recurring_transactions
			SET last_posted_date = now()::date, next_date = $3, last_journal_id = $4, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, id, nextNext, posted.ID); err != nil {
			return err
		}

		result = map[string]any{
			"id":               id,
			"code":             code,
			"name":             name,
			"intent_type":      intentType,
			"amount_cents":     amountCents,
			"from_account_id":  fromAcct,
			"to_account_id":    toAcct,
			"description":      description,
			"posted_at":        time.Now().Format("2006-01-02"),
			"next_date":        nextNext.Format("2006-01-02"),
			"posted_by":        userID,
			"journal_entry_id": posted.ID,
			"journal_number":   posted.Number,
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			writeJSON(writer, http.StatusNotFound, errBody{"NOT_FOUND", "recurring transaction not found"})
		case errors.Is(err, errInactive):
			writeJSON(writer, http.StatusBadRequest, errBody{"INACTIVE", "recurring transaction is inactive"})
		case errors.Is(err, errMissingAccounts):
			writeJSON(writer, http.StatusBadRequest, errBody{"MISSING_ACCOUNTS", "recurring transaction must have from_account_id and to_account_id set"})
		case errors.Is(err, errPeriodClosed):
			writeJSON(writer, http.StatusConflict, errBody{"PERIOD_CLOSED", "recurring date is outside an open accounting period"})
		default:
			log.Printf("recurring: post now failed: tenant=%d id=%d: %v", tenantID, id, err)
			writeJSON(writer, http.StatusInternalServerError, errBody{"POST_FAILED", "failed to post recurring transaction"})
		}
		return
	}
	writeJSON(writer, http.StatusOK, result)
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

// ---------------------------------------------------------------------------
// Journal posting helpers (mirror pettycash/handler.go & tax/helpers.go)
// ---------------------------------------------------------------------------

var (
	errNotFound        = errors.New("recurring transaction not found")
	errInactive        = errors.New("recurring transaction is inactive")
	errMissingAccounts = errors.New("recurring transaction must have from_account_id and to_account_id set")
	errPeriodClosed    = errors.New("entry date is outside an open period")
)

type postedEntry struct {
	ID     int64
	Number string
	Hash   string
}

func postJournal(ctx context.Context, tx pgx.Tx, tenant int64, idem string, journal accounting.Journal, uid int64, reversalOfID int64) (postedEntry, error) {
	existing, err := db.New(tx).GetJournalByIdempotencyKey(ctx, db.GetJournalByIdempotencyKeyParams{
		TenantID:       tenant,
		IdempotencyKey: uuidValue(idem),
	})
	if err == nil {
		return postedEntry{ID: existing.ID, Number: existing.Number, Hash: existing.Hash}, nil
	} else if !isNoRows(err) {
		return postedEntry{}, err
	}

	head, err := lockOrSeedHead(ctx, tx, tenant)
	if err != nil {
		return postedEntry{}, err
	}
	journal.TenantID = tenant
	journal.PreviousHash = head.LastHash
	journal.Hash = accounting.HashJournal(journal)

	periodID, err := resolvePeriod(ctx, tx, tenant, journal.EntryDate)
	if err != nil {
		return postedEntry{}, err
	}
	number, err := nextJournalNumber(ctx, tx, tenant)
	if err != nil {
		return postedEntry{}, err
	}

	entry, err := db.New(tx).InsertJournalEntry(ctx, db.InsertJournalEntryParams{
		TenantID:       tenant,
		Number:         number,
		EntryDate:      parseDatePG(journal.EntryDate),
		PeriodID:       periodID,
		Description:    textValuePG(journal.Description),
		SourceRef:      textValuePG(journal.SourceRef),
		IntentType:     textValuePG(string(journal.IntentType)),
		IdempotencyKey: uuidValue(idem),
		Hash:           journal.Hash,
		PrevHash:       journal.PreviousHash,
		CreatedBy:      int8Value(uid),
	})
	if err != nil {
		return postedEntry{}, err
	}
	for _, line := range journal.Lines {
		if err := db.New(tx).InsertJournalLine(ctx, db.InsertJournalLineParams{
			TenantID:      tenant,
			EntryID:       entry.ID,
			AccountID:     line.AccountID,
			DebitCents:    line.DebitCents,
			CreditCents:   line.CreditCents,
			Description:   textValuePG(line.SourceLineRef),
			SourceLineRef: textValuePG(line.SourceLineRef),
			DimensionIds:  []byte("[]"),
		}); err != nil {
			return postedEntry{}, err
		}
	}

	if reversalOfID > 0 {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.void_context', '1', true)`); err != nil {
			return postedEntry{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE journal_entries SET reversal_of_id = $1
			WHERE tenant_id = $2 AND id = $3
		`, reversalOfID, tenant, entry.ID); err != nil {
			return postedEntry{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE journal_entries
			SET status = 'VOID', void_reason = 'reversed', voided_by = $1, voided_at = now()
			WHERE tenant_id = $2 AND id = $3
		`, uid, tenant, reversalOfID); err != nil {
			return postedEntry{}, err
		}
	}

	if err := upsertHead(ctx, tx, tenant, entry.ID, journal.Hash); err != nil {
		return postedEntry{}, err
	}
	if err := insertOutbox(ctx, tx, tenant, "recurring.posted", mustJSON(map[string]any{
		"journal_id": entry.ID, "number": number, "intent": string(journal.IntentType),
		"source_ref": journal.SourceRef,
	})); err != nil {
		return postedEntry{}, err
	}
	return postedEntry{ID: entry.ID, Number: number, Hash: journal.Hash}, nil
}

func withTenant(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10))
	return err
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
		return 0, errPeriodClosed
	}
	return periodID, nil
}

func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'JRN', 'JRN', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
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

// idempotencyKeyOrGenerate returns the validated Idempotency-Key header when
// present, or a freshly generated UUID when absent (keeps older clients that
// never send the header working, while still giving each request a stable
// idempotency key within the journal).
func idempotencyKeyOrGenerate(r *http.Request) string {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return randomUUID()
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(key); err != nil {
		return randomUUID()
	}
	return key
}

func randomUUID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func uuidValue(raw string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(raw)
	return value
}

func int8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func textValuePG(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func parseDatePG(raw string) pgtype.Date {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
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
