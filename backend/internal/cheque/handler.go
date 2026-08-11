package cheque

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

<<<<<<< HEAD
	"finance-accounting-app/backend/internal/accounting"
=======
	"finance-accounting-app/backend/internal/audit"
>>>>>>> fix-backend-audit-idem
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// F-14: Giro & Cheque Management
//   Register cheques and giros, track them through deposit → clear / bounce,
//   and keep an auditable status lifecycle per cheque.
//
// Journal posting (A-27):
//   Deposit (RECEIVED):  Dr 1305 Cheques in Transit / Cr 1201 AR
//   Deposit (ISSUED):    Dr 2101 AP / Cr 2106 Cheques Issued Outstanding
//   Clear   (RECEIVED, from DEPOSITED):  Dr 1102 Bank / Cr 1305 Cheques in Transit
//   Clear   (ISSUED,   from DEPOSITED):  Dr 2106 Cheques Issued Outstanding / Cr 1102 Bank
//   Clear   (RECEIVED, from REGISTERED): Dr 1102 Bank / Cr 1201 AR   (skip transit)
//   Clear   (ISSUED,   from REGISTERED): Dr 2101 AP / Cr 1102 Bank    (skip transit)
//   Bounce:  reverse the deposit journal (if one was posted) and mark original VOID
//
// Transit/issued account codes use 1305/2106 because 1304/2105 collide with
// the default COA (Finished Goods / Uninvoiced Payables); 1305/2106 are
// seeded by migration 000045.
// ---------------------------------------------------------------------------

// Seeded account codes (see auth/seed.go and migration 000045).
const (
	bankCode          = "1102" // Bank
	arCode            = "1201" // Accounts Receivable
	apCode            = "2101" // Accounts Payable
	chequeTransitCode = "1305" // Cheques in Transit (migration 000045)
	chequeIssuedCode  = "2106" // Cheques Issued Outstanding (migration 000045)
)

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

// ClearRequest is the optional JSON body for POST /cheques/{id}/clear.
// BankAccountCode overrides the bank-side account (defaults to "1102").
type ClearRequest struct {
	BankAccountCode string `json:"bank_account_code"`
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

// scanCheque scans a full cheque row into a ChequeResponse.
func scanCheque(scanner interface{ Scan(...any) error }) (ChequeResponse, error) {
	var c ChequeResponse
	err := scanner.Scan(
		&c.ID, &c.ChequeNumber, &c.ChequeType, &c.Direction, &c.BankName,
		&c.BankAccountNumber, &c.Payee, &c.Drawer, &c.AmountCents,
		&c.IssueDate, &c.DueDate, &c.ClearingDate, &c.Status,
		&c.BouncedReason, &c.JournalEntryID, &c.PaymentID, &c.Description,
		&c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// formatDates normalises the date fields for a clean API response.
func (c *ChequeResponse) formatDates() *ChequeResponse {
	c.IssueDate = formatScannedDate(c.IssueDate)
	c.DueDate = formatScannedDate(c.DueDate)
	c.ClearingDate = formatScannedDate(c.ClearingDate)
	return c
}

const chequeSelectColumns = `id, cheque_number, cheque_type, direction, bank_name,
	bank_account_number, payee, drawer, amount_cents, issue_date,
	due_date, clearing_date, status, bounced_reason, journal_entry_id,
	payment_id, description, created_at, updated_at`

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
<<<<<<< HEAD
	err = s.pool.QueryRow(r.Context(), `
		INSERT INTO cheques (
			tenant_id, cheque_number, cheque_type, direction, bank_name,
			bank_account_number, payee, drawer, amount_cents, issue_date,
			due_date, status, journal_entry_id, payment_id, description
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'REGISTERED',$12,$13,$14)
		RETURNING `+chequeSelectColumns,
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
	)
=======
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
>>>>>>> fix-backend-audit-idem
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "failed to create cheque")
		return
	}
	writeJSON(w, http.StatusCreated, resp.formatDates())
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	directionFilter := strings.TrimSpace(r.URL.Query().Get("direction"))

	query := `SELECT ` + chequeSelectColumns + ` FROM cheques WHERE tenant_id = $1`
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
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "failed to list cheques")
		return
	}
	defer rows.Close()
	results := make([]ChequeResponse, 0)
	for rows.Next() {
		c, err := scanCheque(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "failed to read cheques")
			return
		}
		results = append(results, *c.formatDates())
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "failed to read cheques")
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
		SELECT `+chequeSelectColumns+` FROM cheques WHERE tenant_id = $1 AND id = $2
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
	writeJSON(w, http.StatusOK, resp.formatDates())
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
		RETURNING `+chequeSelectColumns,
		args...).Scan(
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
	writeJSON(w, http.StatusOK, resp.formatDates())
}

// Deposit transitions a cheque from REGISTERED to DEPOSITED and posts the
// corresponding journal entry:
//
//	RECEIVED: Dr 1305 Cheques in Transit / Cr 1201 AR
//	ISSUED:   Dr 2101 AP / Cr 2106 Cheques Issued Outstanding
func (s *Service) Deposit(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
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
	idem := idempotencyKeyOrGenerate(r)

	var resp ChequeResponse
<<<<<<< HEAD
	err := db.WithTransaction(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := withTenant(r.Context(), tx, tid); err != nil {
			return err
		}
		// Idempotent replay: return the current cheque + stored journal.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(r.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tid,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			resp, err = loadCheque(r.Context(), tx, tid, id)
			if err != nil {
				return err
			}
			resp.JournalEntryID = &existing.ID
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Lock and load the cheque (must be REGISTERED).
		var direction string
		var amountCents int64
		var chequeNumber string
		err = tx.QueryRow(r.Context(), `
			SELECT direction, amount_cents, cheque_number
			FROM cheques
			WHERE tenant_id = $1 AND id = $2 AND status = 'REGISTERED'
			FOR UPDATE
		`, tid, id).Scan(&direction, &amountCents, &chequeNumber)
		if err != nil {
			return errInvalidState
		}

		// Build the deposit journal.
		journal, err := buildDepositJournal(tid, id, direction, amountCents, chequeNumber, r.Context(), tx)
		if err != nil {
			return err
		}
		posted, err := postJournal(r.Context(), tx, tid, idem, journal, uid, 0)
		if err != nil {
			return err
		}

		// Update cheque status + link the journal.
		resp, err = updateChequeStatus(r.Context(), tx, tid, id, "DEPOSITED", "", posted.ID, false)
		return err
	})
=======
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
>>>>>>> fix-backend-audit-idem
	if err != nil {
		writePostingErr(w, err, "deposit", tid, id)
		return
	}
	writeJSON(w, http.StatusOK, resp.formatDates())
}

// Clear transitions a cheque to CLEARED and posts the corresponding journal:
//
//	from DEPOSITED (RECEIVED): Dr 1102 Bank / Cr 1305 Cheques in Transit
//	from DEPOSITED (ISSUED):   Dr 2106 Cheques Issued Outstanding / Cr 1102 Bank
//	from REGISTERED (RECEIVED): Dr 1102 Bank / Cr 1201 AR   (direct clearance)
//	from REGISTERED (ISSUED):   Dr 2101 AP / Cr 1102 Bank    (direct payment)
func (s *Service) Clear(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
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
	idem := idempotencyKeyOrGenerate(r)

	var body ClearRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	bankCodeResolved := resolveBankCode(body.BankAccountCode)

	var resp ChequeResponse
<<<<<<< HEAD
	err := db.WithTransaction(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := withTenant(r.Context(), tx, tid); err != nil {
			return err
		}
		// Idempotent replay.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(r.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tid,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			resp, err = loadCheque(r.Context(), tx, tid, id)
			if err != nil {
				return err
			}
			resp.JournalEntryID = &existing.ID
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Lock and load the cheque (must be REGISTERED or DEPOSITED).
		var direction, prevStatus string
		var amountCents int64
		var chequeNumber string
		err = tx.QueryRow(r.Context(), `
			SELECT direction, amount_cents, cheque_number, status
			FROM cheques
			WHERE tenant_id = $1 AND id = $2 AND status IN ('REGISTERED', 'DEPOSITED')
			FOR UPDATE
		`, tid, id).Scan(&direction, &amountCents, &chequeNumber, &prevStatus)
		if err != nil {
			return errInvalidState
		}

		// Build the clear journal based on direction + previous status.
		journal, err := buildClearJournal(tid, id, direction, prevStatus, amountCents, chequeNumber, bankCodeResolved, r.Context(), tx)
		if err != nil {
			return err
		}
		posted, err := postJournal(r.Context(), tx, tid, idem, journal, uid, 0)
		if err != nil {
			return err
		}

		// Update cheque status, set clearing_date, and link the journal in
		// one statement.
		resp, err = updateChequeStatus(r.Context(), tx, tid, id, "CLEARED", "", posted.ID, true)
		return err
	})
=======
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
>>>>>>> fix-backend-audit-idem
	if err != nil {
		writePostingErr(w, err, "clear", tid, id)
		return
	}
	writeJSON(w, http.StatusOK, resp.formatDates())
}

// Bounce transitions a cheque to BOUNCED. If a deposit journal was posted
// (cheque was in DEPOSITED status with a journal_entry_id), the deposit
// journal is reversed: a new reversal journal is posted (swapped lines) and
// the original is marked VOID via reversal_of_id. A cheque bounced straight
// from REGISTERED never touched the ledger, so no journal is posted — the
// bounce is recorded in the outbox (with the idempotency key for replays).
func (s *Service) Bounce(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantIDFromContext(r)
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
	var req BounceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateBounceReason(req.Reason); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	idem := idempotencyKeyOrGenerate(r)
	reason := strings.TrimSpace(req.Reason)

	var resp ChequeResponse
<<<<<<< HEAD
	err := db.WithTransaction(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := withTenant(r.Context(), tx, tid); err != nil {
			return err
		}
		// Idempotent replay: journal-backed bounce (DEPOSITED).
		existing, err := db.New(tx).GetJournalByIdempotencyKey(r.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tid,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			resp, err = loadCheque(r.Context(), tx, tid, id)
			if err != nil {
				return err
			}
			resp.JournalEntryID = &existing.ID
			return nil
		} else if !isNoRows(err) {
			return err
		}
		// Idempotent replay: journal-less bounce (REGISTERED) recorded only
		// in the outbox with the same idempotency key.
		var replayPayload []byte
		err = tx.QueryRow(r.Context(), `
			SELECT payload FROM outbox_events
			WHERE tenant_id = $1 AND topic = 'cheque.bounced'
			  AND payload->>'idempotency_key' = $2
			LIMIT 1
		`, tid, idem).Scan(&replayPayload)
		if err == nil {
			resp, err = loadCheque(r.Context(), tx, tid, id)
			return err
		} else if !isNoRows(err) {
			return err
		}

		// Lock and load the cheque (must be REGISTERED or DEPOSITED).
		var direction string
		var amountCents int64
		var chequeNumber string
		var depositJournalID pgtype.Int8
		err = tx.QueryRow(r.Context(), `
			SELECT direction, amount_cents, cheque_number, status, journal_entry_id
			FROM cheques
			WHERE tenant_id = $1 AND id = $2 AND status IN ('REGISTERED', 'DEPOSITED')
			FOR UPDATE
		`, tid, id).Scan(&direction, &amountCents, &chequeNumber, &depositJournalID)
		if err != nil {
			return errInvalidState
		}

		// If a deposit journal was posted, reverse it.
		var reversalID int64
		if depositJournalID.Valid {
			originalID := depositJournalID.Int64
			origLines, err := loadJournalLines(r.Context(), tx, tid, originalID)
			if err != nil {
				return fmt.Errorf("load original journal lines: %w", err)
			}
			// Build the reversal journal (swap debit/credit).
			reversedLines := make([]accounting.Line, len(origLines))
			for i, line := range origLines {
				reversedLines[i] = accounting.Line{
					AccountID:     line.AccountID,
					DebitCents:    line.CreditCents,
					CreditCents:   line.DebitCents,
					SourceLineRef: line.SourceLineRef,
				}
			}
			reversalJournal := accounting.Journal{
				TenantID:    tid,
				SourceRef:   fmt.Sprintf("CHQ-%d-BOUNCE", id),
				IntentType:  accounting.IntentType("CHEQUE_BOUNCE"),
				EntryDate:   time.Now().Format("2006-01-02"),
				Description: fmt.Sprintf("Cheque bounce reversal: %s", chequeNumber),
				Lines:       reversedLines,
			}
			posted, err := postJournal(r.Context(), tx, tid, idem, reversalJournal, uid, originalID)
			if err != nil {
				return err
			}
			reversalID = posted.ID
		} else {
			// No deposit journal to reverse — record the bounce in the
			// outbox, carrying the idempotency key so replays are detected.
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO outbox_events (tenant_id, topic, payload)
				VALUES ($1, 'cheque.bounced', $2::jsonb)
			`, tid, mustJSON(map[string]any{
				"cheque_id": id, "reason": reason, "direction": direction,
				"amount_cents": amountCents, "idempotency_key": idem,
			})); err != nil {
				return err
			}
		}

		// Update cheque status.
		resp, err = updateChequeStatus(r.Context(), tx, tid, id, "BOUNCED", reason, reversalID, false)
		return err
	})
=======
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
>>>>>>> fix-backend-audit-idem
	if err != nil {
		writePostingErr(w, err, "bounce", tid, id)
		return
	}
	writeJSON(w, http.StatusOK, resp.formatDates())
}

// ---------------------------------------------------------------------------
// Journal builders
// ---------------------------------------------------------------------------

// resolveBankCode normalises an optional bank_account_code override.
func resolveBankCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return bankCode
	}
	return code
}

// buildDepositJournal constructs the deposit journal for a cheque.
func buildDepositJournal(tenant, chequeID int64, direction string, amountCents int64, chequeNumber string, ctx context.Context, tx pgx.Tx) (accounting.Journal, error) {
	sourceRef := fmt.Sprintf("CHQ-%d-DEPOSIT", chequeID)
	desc := "Cheque deposit: " + chequeNumber
	entryDate := time.Now().Format("2006-01-02")

	switch strings.ToUpper(direction) {
	case "RECEIVED":
		// Dr 1305 Cheques in Transit / Cr 1201 AR
		transitID, err := resolveAccountByCode(ctx, tx, tenant, chequeTransitCode)
		if err != nil {
			return accounting.Journal{}, err
		}
		arID, err := resolveAccountByCode(ctx, tx, tenant, arCode)
		if err != nil {
			return accounting.Journal{}, err
		}
		return accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("CHEQUE_DEPOSIT"),
			EntryDate:   entryDate,
			Description: desc,
			Lines: []accounting.Line{
				{AccountID: transitID, DebitCents: amountCents, SourceLineRef: "transit"},
				{AccountID: arID, CreditCents: amountCents, SourceLineRef: "ar"},
			},
		}, nil
	case "ISSUED":
		// Dr 2101 AP / Cr 2106 Cheques Issued Outstanding
		apID, err := resolveAccountByCode(ctx, tx, tenant, apCode)
		if err != nil {
			return accounting.Journal{}, err
		}
		issuedID, err := resolveAccountByCode(ctx, tx, tenant, chequeIssuedCode)
		if err != nil {
			return accounting.Journal{}, err
		}
		return accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("CHEQUE_DEPOSIT"),
			EntryDate:   entryDate,
			Description: desc,
			Lines: []accounting.Line{
				{AccountID: apID, DebitCents: amountCents, SourceLineRef: "ap"},
				{AccountID: issuedID, CreditCents: amountCents, SourceLineRef: "issued"},
			},
		}, nil
	default:
		return accounting.Journal{}, errInvalidState
	}
}

// buildClearJournal constructs the clear journal for a cheque. The
// bankCodeOverride selects the bank-side account (defaults to "1102").
func buildClearJournal(tenant, chequeID int64, direction, prevStatus string, amountCents int64, chequeNumber, bankCodeOverride string, ctx context.Context, tx pgx.Tx) (accounting.Journal, error) {
	sourceRef := fmt.Sprintf("CHQ-%d-CLEAR", chequeID)
	desc := "Cheque cleared: " + chequeNumber
	entryDate := time.Now().Format("2006-01-02")
	fromDeposited := strings.ToUpper(prevStatus) == "DEPOSITED"

	bankID, err := resolveAccountByCode(ctx, tx, tenant, bankCodeOverride)
	if err != nil {
		return accounting.Journal{}, err
	}

	switch strings.ToUpper(direction) {
	case "RECEIVED":
		creditCode := arCode
		if fromDeposited {
			creditCode = chequeTransitCode
		}
		creditID, err := resolveAccountByCode(ctx, tx, tenant, creditCode)
		if err != nil {
			return accounting.Journal{}, err
		}
		return accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("CHEQUE_CLEAR"),
			EntryDate:   entryDate,
			Description: desc,
			Lines: []accounting.Line{
				{AccountID: bankID, DebitCents: amountCents, SourceLineRef: "bank"},
				{AccountID: creditID, CreditCents: amountCents, SourceLineRef: creditCode},
			},
		}, nil
	case "ISSUED":
		debitCode := apCode
		if fromDeposited {
			debitCode = chequeIssuedCode
		}
		debitID, err := resolveAccountByCode(ctx, tx, tenant, debitCode)
		if err != nil {
			return accounting.Journal{}, err
		}
		return accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("CHEQUE_CLEAR"),
			EntryDate:   entryDate,
			Description: desc,
			Lines: []accounting.Line{
				{AccountID: debitID, DebitCents: amountCents, SourceLineRef: debitCode},
				{AccountID: bankID, CreditCents: amountCents, SourceLineRef: "bank"},
			},
		}, nil
	default:
		return accounting.Journal{}, errInvalidState
	}
}

// ---------------------------------------------------------------------------
// Cheque DB helpers
// ---------------------------------------------------------------------------

// loadCheque loads a single cheque by ID inside a transaction.
func loadCheque(ctx context.Context, tx pgx.Tx, tenant, id int64) (ChequeResponse, error) {
	var c ChequeResponse
	err := tx.QueryRow(ctx, `
		SELECT `+chequeSelectColumns+` FROM cheques WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(
		&c.ID, &c.ChequeNumber, &c.ChequeType, &c.Direction,
		&c.BankName, &c.BankAccountNumber, &c.Payee, &c.Drawer,
		&c.AmountCents, &c.IssueDate, &c.DueDate, &c.ClearingDate,
		&c.Status, &c.BouncedReason, &c.JournalEntryID, &c.PaymentID,
		&c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// updateChequeStatus updates the cheque's status, bounced_reason (if
// non-empty), journal_entry_id (if non-zero), and optionally the
// clearing_date, returning the updated row.
func updateChequeStatus(ctx context.Context, tx pgx.Tx, tenant, id int64, status, bouncedReason string, journalEntryID int64, setClearingDate bool) (ChequeResponse, error) {
	clearingDate := ""
	if setClearingDate {
		clearingDate = ", clearing_date = CURRENT_DATE"
	}
	var c ChequeResponse
	err := tx.QueryRow(ctx, `
		UPDATE cheques
		SET status = $3, updated_at = now(),
		    bounced_reason = CASE WHEN $4 = '' THEN bounced_reason ELSE $4 END,
		    journal_entry_id = CASE WHEN $5 = 0 THEN journal_entry_id ELSE $5 END`+clearingDate+`
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+chequeSelectColumns,
		tenant, id, status, bouncedReason, journalEntryID).Scan(
		&c.ID, &c.ChequeNumber, &c.ChequeType, &c.Direction,
		&c.BankName, &c.BankAccountNumber, &c.Payee, &c.Drawer,
		&c.AmountCents, &c.IssueDate, &c.DueDate, &c.ClearingDate,
		&c.Status, &c.BouncedReason, &c.JournalEntryID, &c.PaymentID,
		&c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// loadJournalLines loads the lines of a stored journal entry (for reversal).
func loadJournalLines(ctx context.Context, tx pgx.Tx, tenant, entryID int64) ([]accounting.Line, error) {
	rows, err := tx.Query(ctx, `
		SELECT account_id, debit_cents, credit_cents, source_line_ref
		FROM journal_lines
		WHERE tenant_id = $1 AND entry_id = $2
	`, tenant, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []accounting.Line
	for rows.Next() {
		var line accounting.Line
		if err := rows.Scan(&line.AccountID, &line.DebitCents, &line.CreditCents, &line.SourceLineRef); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

// ---------------------------------------------------------------------------
// Journal posting helpers (mirror pettycash/handler.go & tax/helpers.go)
// ---------------------------------------------------------------------------

var (
	errInvalidState   = errors.New("cheque is not in a valid state for this operation")
	errPeriodClosed   = errors.New("entry date is outside an open period")
	errAccountMissing = errors.New("required account is not configured for this tenant")
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
	if err := insertOutbox(ctx, tx, tenant, "cheque.posted", mustJSON(map[string]any{
		"journal_id": entry.ID, "number": number, "intent": string(journal.IntentType),
		"source_ref": journal.SourceRef,
	})); err != nil {
		return postedEntry{}, err
	}
	return postedEntry{ID: entry.ID, Number: number, Hash: journal.Hash}, nil
}

// writePostingErr maps internal posting errors to stable client-facing codes
// and logs the full error server-side (never leaks DB internals to clients).
func writePostingErr(w http.ResponseWriter, err error, op string, tenant, chequeID int64) {
	switch {
	case errors.Is(err, errInvalidState):
		writeErr(w, http.StatusConflict, "INVALID_STATE", "cheque cannot be "+op+"ed in its current status")
	case errors.Is(err, errPeriodClosed):
		writeErr(w, http.StatusConflict, "PERIOD_CLOSED", "no open accounting period for the posting date")
	case errors.Is(err, errAccountMissing):
		writeErr(w, http.StatusConflict, "ACCOUNT_MISSING", "a required GL account is not configured for this tenant")
	default:
		log.Printf("cheque: %s failed: tenant=%d cheque=%d: %v", op, tenant, chequeID, err)
		writeErr(w, http.StatusInternalServerError, "POST_FAILED", "failed to "+op+" cheque")
	}
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

func resolveAccountByCode(ctx context.Context, tx pgx.Tx, tenantID int64, code string) (int64, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2`,
		tenantID, code).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", errAccountMissing, code)
	}
	return accountID, nil
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

// ---------------------------------------------------------------------------
// Validation & Helpers (unchanged — covered by existing unit tests)
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
