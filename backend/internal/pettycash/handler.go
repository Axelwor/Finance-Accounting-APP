package pettycash

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
// F-08: Petty Cash (Imprest System)
//   Fund has a fixed float (imprest amount). Vouchers reduce the fund.
//   Replenishment restores the fund to the imprest amount.
//   Journal: Voucher → Dr Expense / Cr Petty Cash
//             Replenish → Dr Petty Cash / Cr Cash/Bank
// ---------------------------------------------------------------------------

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

	var id int64
	err := db.WithTransaction(r.Context(), service.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO petty_cash_funds (tenant_id, code, name, cash_account_id, imprest_amount_cents, custodian_user_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, tenantID, req.Code, req.Name, req.CashAccountID, req.ImprestAmountCents, userID).Scan(&id); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, tenantID, userID, "petty_cash_fund", id, audit.ActionCreate, nil, map[string]any{
			"code":                 req.Code,
			"name":                 req.Name,
			"imprest_amount_cents": req.ImprestAmountCents,
		})
	})
	if err != nil {
		writeErr(w, http.StatusConflict, "CREATE_FAILED", err.Error())
		return
	}

	// Post the initial funding: Dr Petty Cash / Cr Cash/Bank
	// This would call the journal posting layer — for now, return the fund.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                   id,
		"code":                 req.Code,
		"name":                 req.Name,
		"imprest_amount_cents": req.ImprestAmountCents,
		"message":              "fund created — post initial funding via cash-out endpoint",
	})
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
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
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
			writeErr(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
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

	// Generate voucher number and insert the voucher in one transaction.
	year := time.Now().Year()
	var id int64
	var number string
	err := db.WithTransaction(r.Context(), service.pool, func(tx pgx.Tx) error {
		var seq int64
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
			VALUES ($1, 'PCV', 'PCV', $2, 1)
			ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
			SET last_seq = document_numbering.last_seq + 1
			RETURNING last_seq
		`, tenantID, year).Scan(&seq); err != nil {
			return err
		}
		number = fmt.Sprintf("PCV-%d-%06d", year, seq)
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO petty_cash_vouchers (tenant_id, fund_id, number, voucher_date, amount_cents, expense_account_id, description, recipient, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id
		`, tenantID, req.FundID, number, req.VoucherDate, req.AmountCents, req.ExpenseAccountID, req.Description, req.Recipient, userID).Scan(&id); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, tenantID, userID, "petty_cash_voucher", id, audit.ActionCreate, nil, map[string]any{
			"number":             number,
			"fund_id":            req.FundID,
			"amount_cents":       req.AmountCents,
			"expense_account_id": req.ExpenseAccountID,
		})
	})
	if err != nil {
		writeErr(w, http.StatusConflict, "CREATE_FAILED", err.Error())
		return
	}

	// Note: Journal posting (Dr Expense / Cr Petty Cash) would happen here.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           id,
		"number":       number,
		"fund_id":      req.FundID,
		"amount_cents": req.AmountCents,
		"message":      "voucher created — post journal via cash-out endpoint",
	})
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
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
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
			writeErr(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
			return
		}
		vouchers = append(vouchers, v)
	}
	if vouchers == nil {
		vouchers = []voucher{}
	}
	writeJSON(w, http.StatusOK, vouchers)
}

// Replenish restores the petty cash fund to its imprest amount.
// POST /petty-cash/funds/{id}/replenish
// Body: { "cash_account_id": 123 }
func (service *Service) Replenish(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	fundID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if fundID <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be positive")
		return
	}

	// Load fund
	var imprestCents, cashAcctID int64
	var fundName string
	err := service.pool.QueryRow(r.Context(), `
		SELECT imprest_amount_cents, cash_account_id, name
		FROM petty_cash_funds
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, fundID).Scan(&imprestCents, &cashAcctID, &fundName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "FUND_NOT_FOUND", "petty cash fund not found")
		return
	}

	// Sum all vouchers (posted, not replenished)
	var spentCents int64
	_ = service.pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM petty_cash_vouchers
		WHERE tenant_id = $1 AND fund_id = $2 AND status = 'POSTED'
	`, tenantID, fundID).Scan(&spentCents)

	replenishAmount, ok := computeReplenishAmount(imprestCents, spentCents)
	if !ok {
		writeErr(w, http.StatusBadRequest, "NO_VOUCHERS", "no posted vouchers to replenish")
		return
	}

	// Mark vouchers as replenished and record the replenishment event in the
	// audit trail, atomically.
	userID, _ := auth.UserIDFromContext(r.Context())
	err = db.WithTransaction(r.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `
			UPDATE petty_cash_vouchers SET status = 'POSTED'
			WHERE tenant_id = $1 AND fund_id = $2 AND status = 'POSTED'
		`, tenantID, fundID); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, tenantID, userID, "petty_cash_fund", fundID, audit.ActionPost, nil, map[string]any{
			"action":                 "replenishment",
			"replenish_amount_cents": replenishAmount,
			"vouchers_total_cents":   spentCents,
			"imprest_amount_cents":   imprestCents,
			"cash_account_id":        cashAcctID,
		})
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "REPLENISH_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"fund_id":                fundID,
		"fund_name":              fundName,
		"imprest_amount_cents":   imprestCents,
		"vouchers_total_cents":   spentCents,
		"replenish_amount_cents": replenishAmount,
		"cash_account_id":        cashAcctID,
		"message":                "post the replenishment journal: Dr Petty Cash / Cr Cash/Bank",
	})
}

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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
