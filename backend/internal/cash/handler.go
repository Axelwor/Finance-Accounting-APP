package cash

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// Service exposes the cash journaling endpoints. Tenant id comes from the
// X-Tenant-ID header (temporary until JWT auth carries it).
type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Routes registers the cash endpoints on the chi router.
func (service *Service) Routes(router chi.Router) {
	router.Get("/cash-entries", service.ListCashEntries)
	router.Post("/cash-in", service.CashIn)
	router.Post("/cash-out", service.CashOut)
	router.Post("/transfers", service.Transfer)
	router.Post("/opening-balances", service.OpeningBalance)
	router.Post("/journal-entries/{id}/reverse", service.Reverse)
}

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

type CashRequest struct {
	SourceRef        string `json:"source_ref"`
	EntryDate        string `json:"entry_date"`
	CashAccountID    int64  `json:"cash_account_id"`
	CounterAccountID int64  `json:"counter_account_id"`
	AmountCents      int64  `json:"amount_cents"`
	Description      string `json:"description"`
	// CounterLines lets one journal split the counter side across multiple
	// accounts. When non-empty, CounterAccountID is ignored and the sum of
	// AmountCents across lines must equal AmountCents.
	CounterLines []CounterLineRequest `json:"counter_lines"`
}

// CounterLineRequest is the request shape for one line on the counter
// side of a CASH_IN / CASH_OUT journal.
type CounterLineRequest struct {
	AccountID   int64  `json:"account_id"`
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`
}

type TransferRequest struct {
	SourceRef   string `json:"source_ref"`
	EntryDate   string `json:"entry_date"`
	FromAccount int64  `json:"from_account_id"`
	ToAccount   int64  `json:"to_account_id"`
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`
}

type OpeningBalanceLineRequest struct {
	AccountID   int64 `json:"account_id"`
	DebitCents  int64 `json:"debit_cents"`
	CreditCents int64 `json:"credit_cents"`
}

type OpeningBalanceRequest struct {
	SourceRef       string                      `json:"source_ref"`
	EntryDate       string                      `json:"entry_date"`
	EquityAccountID int64                       `json:"equity_account_id"`
	Balances        []OpeningBalanceLineRequest `json:"balances"`
	Description     string                      `json:"description"`
}

type ReverseRequest struct {
	SourceRef string `json:"source_ref"`
	EntryDate string `json:"entry_date"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// CashIn posts a CASH_IN journal (cash account debited, counter account
// credited) via the pure engine inside one transaction.
func (service *Service) CashIn(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CashRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCashRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	service.post(writer, request, tenant, idem, func(ctx context.Context, tx pgx.Tx) (accounting.Journal, error) {
		cashAccount, err := loadAccount(ctx, tx, tenant, req.CashAccountID)
		if err != nil {
			return accounting.Journal{}, err
		}
		counterLines, counterAccount, err := loadCounter(ctx, tx, tenant, req)
		if err != nil {
			return accounting.Journal{}, err
		}
		return accounting.CashIn(accounting.CashIntent{
			TenantID:       tenant,
			SourceRef:      req.SourceRef,
			EntryDate:      req.EntryDate,
			CashAccount:    accountForEngine(cashAccount),
			CounterAccount: accountForEngine(counterAccount),
			CounterLines:   counterLines,
			AmountCents:    req.AmountCents,
			Description:    req.Description,
		})
	}, 0)
}

// CashOut posts a CASH_OUT journal (counter account debited, cash credited).
func (service *Service) CashOut(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CashRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCashRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	service.post(writer, request, tenant, idem, func(ctx context.Context, tx pgx.Tx) (accounting.Journal, error) {
		cashAccount, err := loadAccount(ctx, tx, tenant, req.CashAccountID)
		if err != nil {
			return accounting.Journal{}, err
		}
		counterLines, counterAccount, err := loadCounter(ctx, tx, tenant, req)
		if err != nil {
			return accounting.Journal{}, err
		}
		return accounting.CashOut(accounting.CashIntent{
			TenantID:       tenant,
			SourceRef:      req.SourceRef,
			EntryDate:      req.EntryDate,
			CashAccount:    accountForEngine(cashAccount),
			CounterAccount: accountForEngine(counterAccount),
			CounterLines:   counterLines,
			AmountCents:    req.AmountCents,
			Description:    req.Description,
		})
	}, 0)
}

// Transfer posts a TRANSFER journal between two cash/bank accounts.
func (service *Service) Transfer(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req TransferRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateTransferRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	service.post(writer, request, tenant, idem, func(ctx context.Context, tx pgx.Tx) (accounting.Journal, error) {
		fromAccount, err := loadAccount(ctx, tx, tenant, req.FromAccount)
		if err != nil {
			return accounting.Journal{}, err
		}
		toAccount, err := loadAccount(ctx, tx, tenant, req.ToAccount)
		if err != nil {
			return accounting.Journal{}, err
		}
		return accounting.Transfer(accounting.TransferIntent{
			TenantID:    tenant,
			SourceRef:   req.SourceRef,
			EntryDate:   req.EntryDate,
			FromAccount: accountForEngine(fromAccount),
			ToAccount:   accountForEngine(toAccount),
			AmountCents: req.AmountCents,
			Description: req.Description,
		})
	}, 0)
}

// OpeningBalance posts the opening entry with the equity plug.
func (service *Service) OpeningBalance(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req OpeningBalanceRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateOpeningBalanceRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	service.post(writer, request, tenant, idem, func(ctx context.Context, tx pgx.Tx) (accounting.Journal, error) {
		// The equity account defaults to the seeded "Capital" account (code 3101)
		// when the client does not send one: the seed runs on registration, so
		// the tenant's own equity id is not known to onboarding clients.
		equityAccountID := req.EquityAccountID
		if equityAccountID <= 0 {
			equityAccountID, err = resolveEquityAccount(ctx, tx, tenant)
			if err != nil {
				return accounting.Journal{}, err
			}
		}
		equityAccount, err := loadAccount(ctx, tx, tenant, equityAccountID)
		if err != nil {
			return accounting.Journal{}, err
		}
		balances := make([]accounting.OpeningBalanceLine, 0, len(req.Balances))
		for _, balance := range req.Balances {
			balances = append(balances, accounting.OpeningBalanceLine{
				AccountID:   balance.AccountID,
				DebitCents:  balance.DebitCents,
				CreditCents: balance.CreditCents,
			})
		}
		return accounting.OpeningBalance(accounting.OpeningIntent{
			TenantID:      tenant,
			SourceRef:     req.SourceRef,
			EntryDate:     req.EntryDate,
			Balances:      balances,
			EquityAccount: accountForEngine(equityAccount),
			Description:   req.Description,
		})
	}, 0)
}

// Reverse posts a REVERSAL journal for an existing posted entry.
func (service *Service) Reverse(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	entryID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req ReverseRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateReverseRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	service.post(writer, request, tenant, idem, func(ctx context.Context, tx pgx.Tx) (accounting.Journal, error) {
		original, err := db.New(tx).GetJournalEntry(ctx, db.GetJournalEntryParams{TenantID: tenant, ID: entryID})
		if err != nil {
			return accounting.Journal{}, err
		}
		if original.Status != "POSTED" {
			return accounting.Journal{}, fmt.Errorf("journal %d is not posted", entryID)
		}
		lines, err := journalLines(ctx, tx, tenant, entryID)
		if err != nil {
			return accounting.Journal{}, err
		}
		reversal, err := accounting.Reverse(accounting.Journal{
			TenantID:    tenant,
			SourceRef:   stringOrEmpty(original.SourceRef),
			IntentType:  accounting.IntentType(stringOrEmpty(original.IntentType)),
			EntryDate:   original.EntryDate.Time.Format("2006-01-02"),
			Description: stringOrEmpty(original.Description),
			Lines:       lines,
		}, req.SourceRef, req.EntryDate)
		if err != nil {
			return accounting.Journal{}, err
		}
		reversal.TenantID = tenant
		reversal.SourceRef = req.SourceRef
		return reversal, nil
	}, entryID)
}

// ---------------------------------------------------------------------------
// Validation (pure — no database access)
// ---------------------------------------------------------------------------

func validateCashRequest(req CashRequest) (string, string) {
	if req.SourceRef == "" {
		return "INVALID_REQUEST", "source_ref is required"
	}
	date, err := entryDate(req.EntryDate)
	if err != nil {
		return "INVALID_REQUEST", err.Error()
	}
	req.EntryDate = date
	if req.CashAccountID <= 0 {
		return "INVALID_REQUEST", "cash_account_id must be a positive integer"
	}
	if err := validateAmount(req.AmountCents); err != nil {
		return "INVALID_REQUEST", err.Error()
	}
	if len(req.CounterLines) > 0 {
		total := int64(0)
		for index, line := range req.CounterLines {
			if line.AccountID <= 0 || line.AmountCents <= 0 {
				return "INVALID_REQUEST", fmt.Sprintf("counter_lines[%d] must have positive account_id and amount_cents", index)
			}
			total += line.AmountCents
		}
		if total != req.AmountCents {
			return "INVALID_REQUEST", "sum of counter_lines[].amount_cents must equal amount_cents"
		}
		return "", ""
	}
	if req.CounterAccountID <= 0 {
		return "INVALID_REQUEST", "counter_account_id is required when counter_lines is empty"
	}
	return "", ""
}

func validateTransferRequest(req TransferRequest) (string, string) {
	if req.SourceRef == "" {
		return "INVALID_REQUEST", "source_ref is required"
	}
	if _, err := entryDate(req.EntryDate); err != nil {
		return "INVALID_REQUEST", err.Error()
	}
	if req.FromAccount <= 0 || req.ToAccount <= 0 {
		return "INVALID_REQUEST", "from_account_id and to_account_id must be positive integers"
	}
	if err := validateAmount(req.AmountCents); err != nil {
		return "INVALID_REQUEST", err.Error()
	}
	return "", ""
}

func validateOpeningBalanceRequest(req OpeningBalanceRequest) (string, string) {
	if req.SourceRef == "" {
		return "INVALID_REQUEST", "source_ref is required"
	}
	if _, err := entryDate(req.EntryDate); err != nil {
		return "INVALID_REQUEST", err.Error()
	}
	if req.EquityAccountID < 0 {
		return "INVALID_REQUEST", "equity_account_id must be a positive integer or omitted to use the seeded equity account"
	}
	if len(req.Balances) == 0 {
		return "INVALID_REQUEST", "balances must contain at least one line"
	}
	for index, balance := range req.Balances {
		if balance.AccountID <= 0 || balance.DebitCents < 0 || balance.CreditCents < 0 ||
			(balance.DebitCents > 0 && balance.CreditCents > 0) || (balance.DebitCents == 0 && balance.CreditCents == 0) {
			return "INVALID_REQUEST", fmt.Sprintf("balances[%d] must have a positive account_id and exactly one of debit_cents or credit_cents", index)
		}
	}
	return "", ""
}

func validateReverseRequest(req ReverseRequest) (string, string) {
	if req.SourceRef == "" {
		return "INVALID_REQUEST", "source_ref is required"
	}
	if _, err := entryDate(req.EntryDate); err != nil {
		return "INVALID_REQUEST", err.Error()
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// journalLines loads the lines of a journal entry inside the transaction.
func journalLines(ctx context.Context, tx pgx.Tx, tenantID, entryID int64) ([]accounting.Line, error) {
	rows, err := tx.Query(ctx, `
		SELECT account_id, debit_cents, credit_cents, source_line_ref
		FROM journal_lines
		WHERE tenant_id = $1 AND entry_id = $2
	`, tenantID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := make([]accounting.Line, 0)
	for rows.Next() {
		var line accounting.Line
		var ref pgtype.Text
		if err := rows.Scan(&line.AccountID, &line.DebitCents, &line.CreditCents, &ref); err != nil {
			return nil, err
		}
		line.SourceLineRef = stringOrEmpty(ref)
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func stringOrEmpty(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
