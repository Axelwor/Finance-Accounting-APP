package pettycash

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
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// F-08: Petty Cash (Imprest System)
//   Fund has a fixed float (imprest amount). Vouchers reduce the fund.
//   Replenishment restores the fund to the imprest amount.
//   Journal: Fund/Replenish → Dr Petty Cash / Cr Cash-Bank
//            Voucher        → Dr Expense / Cr Petty Cash
// ---------------------------------------------------------------------------

// cashAccountCode is the default funding source account (Cash).
const cashAccountCode = "1101"

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Post("/petty-cash/funds", service.CreateFund)
	router.Get("/petty-cash/funds", service.ListFunds)
	router.Post("/petty-cash/vouchers", service.CreateVoucher)
	router.Get("/petty-cash/vouchers", service.ListVouchers)
	router.Post("/petty-cash/funds/{id}/replenish", service.Replenish)
}

type CreateFundRequest struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	CashAccountID      int64  `json:"cash_account_id"`
	ImprestAmountCents int64  `json:"imprest_amount_cents"`
	// PaymentAccountCode is the cash/bank account the initial funding comes
	// from (credit side). Defaults to "1101" (Cash) when empty.
	PaymentAccountCode string `json:"payment_account_code"`
}

func (service *Service) CreateFund(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())

	var req CreateFundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if !validateFundRequest(req) {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "code, name, cash_account_id, and imprest_amount_cents are required")
		return
	}
	idem := idempotencyKeyOrGenerate(r)

	// Resolve the payment (credit-side) account code, defaulting to Cash.
	paymentCode := strings.TrimSpace(req.PaymentAccountCode)
	if paymentCode == "" {
		paymentCode = cashAccountCode
	}

	var result map[string]any
	err := db.WithTransaction(r.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(r.Context(), tx, tenantID); err != nil {
			return err
		}
		// Idempotent replay: if the key exists, return the stored fund+journal.
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(r.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenantID,
			IdempotencyKey: uuidValue(idem),
		}); err == nil {
			result = fundReplayResponse(r.Context(), tx, tenantID, existing.ID, existing.Number)
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Insert the fund record.
		var fundID int64
		err := tx.QueryRow(r.Context(), `
			INSERT INTO petty_cash_funds (tenant_id, code, name, cash_account_id, imprest_amount_cents, custodian_user_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, tenantID, req.Code, req.Name, req.CashAccountID, req.ImprestAmountCents, userID).Scan(&fundID)
		if err != nil {
			return err
		}

		// Resolve the credit-side account.
		paymentAcctID, err := resolveAccountByCode(r.Context(), tx, tenantID, paymentCode)
		if err != nil {
			return err
		}

		// Post the initial funding journal: Dr Petty Cash / Cr Cash-Bank.
		journal := accounting.Journal{
			TenantID:    tenantID,
			SourceRef:   fmt.Sprintf("PCF-%d", fundID),
			IntentType:  accounting.IntentType("PETTY_CASH_FUND"),
			EntryDate:   time.Now().Format("2006-01-02"),
			Description: "Petty cash fund created: " + req.Name,
			Lines: []accounting.Line{
				{AccountID: req.CashAccountID, DebitCents: req.ImprestAmountCents, SourceLineRef: "petty-cash"},
				{AccountID: paymentAcctID, CreditCents: req.ImprestAmountCents, SourceLineRef: "cash-bank"},
			},
		}
		posted, err := postJournal(r.Context(), tx, tenantID, idem, journal, userID, 0)
		if err != nil {
			return err
		}

		result = map[string]any{
			"id":                   fundID,
			"code":                 req.Code,
			"name":                 req.Name,
			"cash_account_id":      req.CashAccountID,
			"imprest_amount_cents": req.ImprestAmountCents,
			"journal_entry_id":     posted.ID,
			"journal_number":       posted.Number,
		}
		return nil
	})
	if err != nil {
		log.Printf("pettycash: create fund failed: tenant=%d code=%s: %v", tenantID, req.Code, err)
		writeErr(w, http.StatusConflict, "CREATE_FAILED", "failed to create petty cash fund")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (service *Service) ListFunds(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}

	rows, err := service.pool.Query(r.Context(), `
		SELECT id, code, name, cash_account_id, imprest_amount_cents, is_active
		FROM petty_cash_funds
		WHERE tenant_id = $1
		ORDER BY code
	`, tenantID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", "failed to list funds")
		return
	}
	defer rows.Close()

	type fund struct {
		ID                 int64  `json:"id"`
		Code               string `json:"code"`
		Name               string `json:"name"`
		CashAccountID      int64  `json:"cash_account_id"`
		ImprestAmountCents int64  `json:"imprest_amount_cents"`
		IsActive           bool   `json:"is_active"`
	}
	var funds []fund
	for rows.Next() {
		var f fund
		if err := rows.Scan(&f.ID, &f.Code, &f.Name, &f.CashAccountID, &f.ImprestAmountCents, &f.IsActive); err != nil {
			writeErr(w, http.StatusInternalServerError, "SCAN_FAILED", "failed to read funds")
			return
		}
		funds = append(funds, f)
	}
	if funds == nil {
		funds = []fund{}
	}
	writeJSON(w, http.StatusOK, funds)
}

type CreateVoucherRequest struct {
	FundID           int64  `json:"fund_id"`
	VoucherDate      string `json:"voucher_date"`
	AmountCents      int64  `json:"amount_cents"`
	ExpenseAccountID int64  `json:"expense_account_id"`
	Description      string `json:"description"`
	Recipient        string `json:"recipient"`
}

func (service *Service) CreateVoucher(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())

	var req CreateVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if !validateVoucherRequest(req) {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "fund_id, amount_cents, expense_account_id, and description are required")
		return
	}
	if req.VoucherDate == "" {
		req.VoucherDate = time.Now().Format("2006-01-02")
	}
	idem := idempotencyKeyOrGenerate(r)

	var result map[string]any
	err := db.WithTransaction(r.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(r.Context(), tx, tenantID); err != nil {
			return err
		}
		// Idempotent replay.
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(r.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenantID,
			IdempotencyKey: uuidValue(idem),
		}); err == nil {
			result = voucherReplayResponse(r.Context(), tx, tenantID, existing.ID, existing.Number)
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load the fund to get its petty cash account (credit side).
		var pettyCashAcctID int64
		if err := tx.QueryRow(r.Context(), `
			SELECT cash_account_id FROM petty_cash_funds
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, req.FundID).Scan(&pettyCashAcctID); err != nil {
			return errFundNotFound
		}

		// Generate voucher number.
		year := time.Now().Year()
		var seq int64
		_ = tx.QueryRow(r.Context(), `
			INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
			VALUES ($1, 'PCV', 'PCV', $2, 1)
			ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
			SET last_seq = document_numbering.last_seq + 1
			RETURNING last_seq
		`, tenantID, year).Scan(&seq)
		number := fmt.Sprintf("PCV-%d-%06d", year, seq)

		// Insert the voucher.
		var voucherID int64
		err := tx.QueryRow(r.Context(), `
			INSERT INTO petty_cash_vouchers (tenant_id, fund_id, number, voucher_date, amount_cents, expense_account_id, description, recipient, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id
		`, tenantID, req.FundID, number, req.VoucherDate, req.AmountCents, req.ExpenseAccountID, req.Description, req.Recipient, userID).Scan(&voucherID)
		if err != nil {
			return err
		}

		// Post the voucher journal: Dr Expense / Cr Petty Cash.
		journal := accounting.Journal{
			TenantID:    tenantID,
			SourceRef:   fmt.Sprintf("PCV-%d", voucherID),
			IntentType:  accounting.IntentType("PETTY_CASH_VOUCHER"),
			EntryDate:   req.VoucherDate,
			Description: req.Description,
			Lines: []accounting.Line{
				{AccountID: req.ExpenseAccountID, DebitCents: req.AmountCents, SourceLineRef: "expense"},
				{AccountID: pettyCashAcctID, CreditCents: req.AmountCents, SourceLineRef: "petty-cash"},
			},
		}
		posted, err := postJournal(r.Context(), tx, tenantID, idem, journal, userID, 0)
		if err != nil {
			return err
		}

		// Link the voucher to its journal entry.
		if _, err := tx.Exec(r.Context(), `
			UPDATE petty_cash_vouchers SET journal_entry_id = $3
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, voucherID, posted.ID); err != nil {
			return err
		}

		result = map[string]any{
			"id":                 voucherID,
			"number":             number,
			"fund_id":            req.FundID,
			"voucher_date":       req.VoucherDate,
			"amount_cents":       req.AmountCents,
			"expense_account_id": req.ExpenseAccountID,
			"description":        req.Description,
			"status":             "POSTED",
			"journal_entry_id":   posted.ID,
			"journal_number":     posted.Number,
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errFundNotFound) {
			writeErr(w, http.StatusNotFound, "FUND_NOT_FOUND", "petty cash fund not found")
			return
		}
		log.Printf("pettycash: create voucher failed: tenant=%d fund=%d: %v", tenantID, req.FundID, err)
		writeErr(w, http.StatusConflict, "CREATE_FAILED", "failed to create voucher")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (service *Service) ListVouchers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}

	fundIDStr := r.URL.Query().Get("fund_id")
	args := []any{tenantID}
	query := `
		SELECT id, fund_id, number, voucher_date, amount_cents, description, recipient, status
		FROM petty_cash_vouchers
		WHERE tenant_id = $1`
	if fundIDStr != "" {
		if fundID, err := strconv.ParseInt(fundIDStr, 10, 64); err == nil && fundID > 0 {
			query += ` AND fund_id = $2`
			args = append(args, fundID)
		}
	}
	query += ` ORDER BY voucher_date DESC LIMIT 100`

	rows, err := service.pool.Query(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", "failed to list vouchers")
		return
	}
	defer rows.Close()

	type voucher struct {
		ID          int64  `json:"id"`
		FundID      int64  `json:"fund_id"`
		Number      string `json:"number"`
		VoucherDate string `json:"voucher_date"`
		AmountCents int64  `json:"amount_cents"`
		Description string `json:"description"`
		Recipient   string `json:"recipient,omitempty"`
		Status      string `json:"status"`
	}
	var vouchers []voucher
	for rows.Next() {
		var v voucher
		if err := rows.Scan(&v.ID, &v.FundID, &v.Number, &v.VoucherDate, &v.AmountCents, &v.Description, &v.Recipient, &v.Status); err != nil {
			writeErr(w, http.StatusInternalServerError, "SCAN_FAILED", "failed to read vouchers")
			return
		}
		vouchers = append(vouchers, v)
	}
	if vouchers == nil {
		vouchers = []voucher{}
	}
	writeJSON(w, http.StatusOK, vouchers)
}

// ReplenishRequest is the optional JSON body for POST /petty-cash/funds/{id}/replenish.
type ReplenishRequest struct {
	// PaymentAccountCode is the cash/bank account the replenishment comes
	// from (credit side). Defaults to "1101" (Cash) when empty.
	PaymentAccountCode string `json:"payment_account_code"`
}

// Replenish restores the petty cash fund to its imprest amount.
// POST /petty-cash/funds/{id}/replenish
// Body: { "payment_account_code": "1101" }
func (service *Service) Replenish(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())
	fundID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if fundID <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be positive")
		return
	}
	idem := idempotencyKeyOrGenerate(r)

	// Parse the optional body (may be empty).
	var body ReplenishRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	paymentCode := strings.TrimSpace(body.PaymentAccountCode)
	if paymentCode == "" {
		paymentCode = cashAccountCode
	}

	var result map[string]any
	err := db.WithTransaction(r.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(r.Context(), tx, tenantID); err != nil {
			return err
		}
		// Idempotent replay.
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(r.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenantID,
			IdempotencyKey: uuidValue(idem),
		}); err == nil {
			result = replenishReplayResponse(r.Context(), tx, tenantID, fundID, existing.ID, existing.Number)
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load fund.
		var imprestCents, cashAcctID int64
		var fundName string
		err := tx.QueryRow(r.Context(), `
			SELECT imprest_amount_cents, cash_account_id, name
			FROM petty_cash_funds
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, fundID).Scan(&imprestCents, &cashAcctID, &fundName)
		if err != nil {
			return errFundNotFound
		}

		// Sum all posted vouchers that have NOT yet been replenished.
		var spentCents int64
		_ = tx.QueryRow(r.Context(), `
			SELECT COALESCE(SUM(amount_cents), 0)
			FROM petty_cash_vouchers
			WHERE tenant_id = $1 AND fund_id = $2
			  AND status = 'POSTED' AND replenished_at IS NULL
		`, tenantID, fundID).Scan(&spentCents)

		replenishAmount, ok := computeReplenishAmount(imprestCents, spentCents)
		if !ok {
			return errNoVouchers
		}

		// Resolve the credit-side account.
		paymentAcctID, err := resolveAccountByCode(r.Context(), tx, tenantID, paymentCode)
		if err != nil {
			return err
		}

		// Post the replenishment journal: Dr Petty Cash / Cr Cash-Bank.
		journal := accounting.Journal{
			TenantID:    tenantID,
			SourceRef:   fmt.Sprintf("PCR-%d", fundID),
			IntentType:  accounting.IntentType("PETTY_CASH_REPLENISH"),
			EntryDate:   time.Now().Format("2006-01-02"),
			Description: "Petty cash replenishment: " + fundName,
			Lines: []accounting.Line{
				{AccountID: cashAcctID, DebitCents: replenishAmount, SourceLineRef: "petty-cash"},
				{AccountID: paymentAcctID, CreditCents: replenishAmount, SourceLineRef: "cash-bank"},
			},
		}
		posted, err := postJournal(r.Context(), tx, tenantID, idem, journal, userID, 0)
		if err != nil {
			return err
		}

		// Mark the consumed vouchers as replenished so a subsequent Replenish
		// does not re-sum them (prevents double-posting).
		if _, err := tx.Exec(r.Context(), `
			UPDATE petty_cash_vouchers SET replenished_at = now()
			WHERE tenant_id = $1 AND fund_id = $2
			  AND status = 'POSTED' AND replenished_at IS NULL
		`, tenantID, fundID); err != nil {
			return err
		}

		result = map[string]any{
			"fund_id":                fundID,
			"fund_name":              fundName,
			"imprest_amount_cents":   imprestCents,
			"vouchers_total_cents":   spentCents,
			"replenish_amount_cents": replenishAmount,
			"cash_account_id":        cashAcctID,
			"journal_entry_id":       posted.ID,
			"journal_number":         posted.Number,
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errFundNotFound):
			writeErr(w, http.StatusNotFound, "FUND_NOT_FOUND", "petty cash fund not found")
		case errors.Is(err, errNoVouchers):
			writeErr(w, http.StatusBadRequest, "NO_VOUCHERS", "no unreplenished vouchers to replenish")
		default:
			log.Printf("pettycash: replenish failed: tenant=%d fund=%d: %v", tenantID, fundID, err)
			writeErr(w, http.StatusConflict, "REPLENISH_FAILED", "failed to replenish fund")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Validation & pure helpers (unchanged — covered by existing unit tests)
// ---------------------------------------------------------------------------

// validateFundRequest checks the create-fund request fields without touching
// the database. Returns true when the request is valid.
func validateFundRequest(req CreateFundRequest) bool {
	return strings.TrimSpace(req.Code) != "" && req.Name != "" &&
		req.CashAccountID > 0 && req.ImprestAmountCents > 0
}

// validateVoucherRequest checks the create-voucher request fields without
// touching the database. Returns true when the request is valid.
func validateVoucherRequest(req CreateVoucherRequest) bool {
	return req.FundID > 0 && req.AmountCents > 0 &&
		req.ExpenseAccountID > 0 && req.Description != ""
}

// computeReplenishAmount returns the amount needed to restore a petty cash
// fund to its imprest level given the total of posted (un-replenished)
// vouchers. In the imprest system the replenishment equals the amount spent.
// Returns the replenishment amount and true when there is something to
// replenish; returns 0 and false when no vouchers are outstanding.
func computeReplenishAmount(imprestCents, spentCents int64) (int64, bool) {
	amount := spentCents
	if amount <= 0 {
		return 0, false
	}
	return amount, true
}

// ---------------------------------------------------------------------------
// Journal posting helpers (mirror sales/down_payments.go & tax/helpers.go)
// ---------------------------------------------------------------------------

var (
	errFundNotFound   = errors.New("petty cash fund not found")
	errNoVouchers     = errors.New("no unreplenished vouchers to replenish")
	errPeriodClosed   = errors.New("entry date is outside an open period")
	errAccountMissing = errors.New("required account is not configured for this tenant")
)

// postedEntry carries the inserted journal entry id, number, and hash.
type postedEntry struct {
	ID     int64
	Number string
	Hash   string
}

// postJournal inserts a journal entry + its lines inside an existing
// transaction. It performs the idempotency check, head lock, journal insert,
// chain-head advance, reversal void (when reversalOfID > 0), and outbox write.
func postJournal(ctx context.Context, tx pgx.Tx, tenant int64, idem string, journal accounting.Journal, uid int64, reversalOfID int64) (postedEntry, error) {
	// Idempotent replay: an identical retry returns the stored journal.
	existing, err := db.New(tx).GetJournalByIdempotencyKey(ctx, db.GetJournalByIdempotencyKeyParams{
		TenantID:       tenant,
		IdempotencyKey: uuidValue(idem),
	})
	if err == nil {
		return postedEntry{ID: existing.ID, Number: existing.Number, Hash: existing.Hash}, nil
	} else if !isNoRows(err) {
		return postedEntry{}, err
	}

	// Lock the chain head so concurrent postings serialize on one row.
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

	// Reversal: link the new entry to the original and mark the original VOID.
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
	if err := insertOutbox(ctx, tx, tenant, "petty-cash.posted", mustJSON(map[string]any{
		"journal_id": entry.ID, "number": number, "intent": string(journal.IntentType),
		"source_ref": journal.SourceRef,
	})); err != nil {
		return postedEntry{}, err
	}
	return postedEntry{ID: entry.ID, Number: number, Hash: journal.Hash}, nil
}

// ---------------------------------------------------------------------------
// Replay responses: on idempotent replay, return the SAME shape as the
// success path plus "idempotent_replay": true.
// ---------------------------------------------------------------------------

func fundReplayResponse(ctx context.Context, tx pgx.Tx, tenant, journalID int64, journalNumber string) map[string]any {
	var id, cashAcctID, imprest int64
	var code, name string
	if err := tx.QueryRow(ctx, `
		SELECT id, code, name, cash_account_id, imprest_amount_cents
		FROM petty_cash_funds WHERE tenant_id = $1 AND id = $2
	`, tenant, sourceRefID(ctx, tx, tenant, journalID, "PCF-")).Scan(&id, &code, &name, &cashAcctID, &imprest); err != nil {
		return map[string]any{"idempotent_replay": true, "journal_entry_id": journalID, "journal_number": journalNumber}
	}
	return map[string]any{
		"id": id, "code": code, "name": name, "cash_account_id": cashAcctID,
		"imprest_amount_cents": imprest, "journal_entry_id": journalID,
		"journal_number": journalNumber, "idempotent_replay": true,
	}
}

func voucherReplayResponse(ctx context.Context, tx pgx.Tx, tenant, journalID int64, journalNumber string) map[string]any {
	var id, fundID, amount int64
	var number, date, description, status string
	if err := tx.QueryRow(ctx, `
		SELECT id, fund_id, number, voucher_date, amount_cents, description, status
		FROM petty_cash_vouchers WHERE tenant_id = $1 AND id = $2
	`, tenant, sourceRefID(ctx, tx, tenant, journalID, "PCV-")).Scan(&id, &fundID, &number, &date, &amount, &description, &status); err != nil {
		return map[string]any{"idempotent_replay": true, "journal_entry_id": journalID, "journal_number": journalNumber}
	}
	return map[string]any{
		"id": id, "number": number, "fund_id": fundID, "voucher_date": date,
		"amount_cents": amount, "description": description, "status": status,
		"journal_entry_id": journalID, "journal_number": journalNumber,
		"idempotent_replay": true,
	}
}

func replenishReplayResponse(ctx context.Context, tx pgx.Tx, tenant, fundID, journalID int64, journalNumber string) map[string]any {
	var name string
	var imprest int64
	if err := tx.QueryRow(ctx, `
		SELECT name, imprest_amount_cents FROM petty_cash_funds
		WHERE tenant_id = $1 AND id = $2
	`, tenant, fundID).Scan(&name, &imprest); err != nil {
		return map[string]any{"idempotent_replay": true, "journal_entry_id": journalID, "journal_number": journalNumber}
	}
	return map[string]any{
		"fund_id": fundID, "fund_name": name, "imprest_amount_cents": imprest,
		"journal_entry_id": journalID, "journal_number": journalNumber,
		"idempotent_replay": true,
	}
}

// sourceRefID extracts the entity id from a journal entry's source_ref
// (format "<prefix><id>"). Returns 0 when not parseable.
func sourceRefID(ctx context.Context, tx pgx.Tx, tenant, journalID int64, prefix string) int64 {
	var ref pgtype.Text
	if err := tx.QueryRow(ctx, `
		SELECT source_ref FROM journal_entries WHERE tenant_id = $1 AND id = $2
	`, tenant, journalID).Scan(&ref); err != nil || !ref.Valid {
		return 0
	}
	id, _ := strconv.ParseInt(strings.TrimPrefix(ref.String, prefix), 10, 64)
	return id
}

// ---------------------------------------------------------------------------
// Shared DB helpers
// ---------------------------------------------------------------------------

// withTenant scopes ROW LEVEL SECURITY to the tenant for the transaction.
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
		return 0, errAccountMissing
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
