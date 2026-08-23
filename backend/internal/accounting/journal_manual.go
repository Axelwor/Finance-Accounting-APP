package accounting

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/approval"
	"finance-accounting-app/backend/internal/auth"

	"finance-accounting-app/backend/internal/db"
)

// ManualLineRequest is one line of a manual journal entry request.
type ManualLineRequest struct {
	AccountID   int64  `json:"account_id"`
	DebitCents  int64  `json:"debit_cents"`
	CreditCents int64  `json:"credit_cents"`
	Description string `json:"description"`
}

// ManualJournalRequest is the body of POST /journal-entries.
type ManualJournalRequest struct {
	EntryDate   string              `json:"entry_date"`
	Description string              `json:"description"`
	Lines       []ManualLineRequest `json:"lines"`
}

// JournalEntryListItem is one row in GET /journal-entries.
type JournalEntryListItem struct {
	ID               int64  `json:"id"`
	Number           string `json:"number"`
	EntryDate        string `json:"entry_date"`
	Description      string `json:"description"`
	IntentType       string `json:"intent_type"`
	Status           string `json:"status"`
	TotalDebitCents  int64  `json:"total_debit_cents"`
	TotalCreditCents int64  `json:"total_credit_cents"`
}

// JournalEntryLine is one line on a journal entry detail response.
type JournalEntryLine struct {
	AccountID   int64  `json:"account_id"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	DebitCents  int64  `json:"debit_cents"`
	CreditCents int64  `json:"credit_cents"`
	Description string `json:"description"`
}

// JournalEntryDetail is the full entry returned by GET /journal-entries/{id}.
type JournalEntryDetail struct {
	JournalEntryListItem
	SourceRef string             `json:"source_ref"`
	Lines     []JournalEntryLine `json:"lines"`
}

// CreateManualJournal posts a free-form MANUAL_JOURNAL entry. The lines must
// balance (total debit = total credit) and reference postable accounts. The
// posting path mirrors cash.post: RLS scoping, idempotent replay, hash-chain
// head lock, period resolution, JRN numbering, and outbox event.
func (service *Service) CreateManualJournal(writer http.ResponseWriter, request *http.Request) {
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
	var req ManualJournalRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateManualJournal(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}

	result, err := service.post(request, tenant, idem, func(ctx context.Context, tx pgx.Tx) (Journal, error) {
		// Load every account once and validate postability (not a group,
		// active, belongs to this tenant). Account-type rules are not
		// enforced for manual journals — the accountant owns composition.
		lines := make([]Line, 0, len(req.Lines))
		var totalDebit int64
		for i, rl := range req.Lines {
			account, loadErr := loadAccount(ctx, tx, tenant, rl.AccountID)
			if loadErr != nil {
				return Journal{}, loadErr
			}
			if err := validatePostable(accountForEngine(account)); err != nil {
				return Journal{}, err
			}
			ref := rl.Description
			if ref == "" {
				ref = "manual-" + itoa(i+1)
			}
			totalDebit += rl.DebitCents
			lines = append(lines, Line{
				AccountID:     account.ID,
				DebitCents:    rl.DebitCents,
				CreditCents:   rl.CreditCents,
				SourceLineRef: ref,
			})
		}
		// A-30: approval gate on manual journals. If the tenant has an active
		// "journal_entry" workflow whose min_amount_cents <= total debit, the
		// posting requires an APPROVED, unconsumed approval.
		if err := service.gate.CheckAmount(ctx, tx, tenant, "journal_entry", totalDebit); err != nil {
			return Journal{}, err
		}
		return ManualJournal(ManualIntent{
			TenantID:    tenant,
			SourceRef:   "MANUAL-" + strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", ""),
			EntryDate:   req.EntryDate,
			Description: req.Description,
			Lines:       lines,
		})
	})
	if err != nil {
		status, code, message := classifyPostingError(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// classifyPostingError maps a posting error from CreateManualJournal to an
// HTTP status, stable error code, and message. Domain sentinels are checked
// first; the pgx.ErrNoRows fallback must stay after ErrEntryDateOutsideOpen-
// Period because resolvePeriod failures also surface as no-row errors.
// N5/NEW-1: an account_id belonging to another tenant returns zero rows under
// RLS — that is a missing resource for this tenant (404), not an internal
// failure.
func classifyPostingError(err error) (int, string, string) {
	switch {
	case errors.Is(err, approval.ErrApprovalRequired):
		return http.StatusConflict, "APPROVAL_REQUIRED", err.Error()
	case errors.Is(err, ErrEntryDateOutsideOpenPeriod):
		return http.StatusUnprocessableEntity, "ENTRY_DATE_OUTSIDE_OPEN_PERIOD", "entry_date does not fall inside an OPEN accounting period; reopen the period or choose a date inside it"
	case isNoRows(err):
		return http.StatusNotFound, "ACCOUNT_NOT_FOUND", "account does not exist for this tenant"
	case isValidationError(err):
		return http.StatusBadRequest, errorCode(err), err.Error()
	}
	return http.StatusInternalServerError, errorCode(err), err.Error()
}

// ListJournalEntries returns all posted journal entries for the tenant with
// optional date range and account filters. Each row carries the entry totals.
func (service *Service) ListJournalEntries(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	query := request.URL.Query()
	fromDate := strings.TrimSpace(query.Get("from_date"))
	toDate := strings.TrimSpace(query.Get("to_date"))
	accountIDRaw := strings.TrimSpace(query.Get("account_id"))

	if err := validateDateRange(fromDate, toDate); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	args := []any{tenant}
	where := "WHERE je.tenant_id = $1 AND je.status = 'POSTED'"
	idx := 2
	if fromDate != "" {
		where += " AND je.entry_date >= $" + itoa(idx)
		args = append(args, parseDate(fromDate))
		idx++
	}
	if toDate != "" {
		where += " AND je.entry_date <= $" + itoa(idx)
		args = append(args, parseDate(toDate))
		idx++
	}
	// When an account filter is present, restrict to entries that touch it.
	if accountIDRaw != "" {
		accountID, convErr := parseInt64(accountIDRaw)
		if convErr != nil || accountID <= 0 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "account_id must be a positive integer")
			return
		}
		where += " AND EXISTS (SELECT 1 FROM journal_lines jl2 WHERE jl2.tenant_id = je.tenant_id AND jl2.entry_id = je.id AND jl2.account_id = $" + itoa(idx) + ")"
		args = append(args, accountID)
		idx++
	}
	args = append(args, 200) // limit
	items := make([]JournalEntryListItem, 0)
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT je.id, je.number, je.entry_date::text, COALESCE(je.description, ''),
			       COALESCE(je.intent_type, ''), je.status,
			       COALESCE(SUM(jl.debit_cents), 0), COALESCE(SUM(jl.credit_cents), 0)
			FROM journal_entries je
			LEFT JOIN journal_lines jl ON jl.tenant_id = je.tenant_id AND jl.entry_id = je.id
			`+where+`
			GROUP BY je.id, je.number, je.entry_date, je.description, je.intent_type, je.status
			ORDER BY je.entry_date DESC, je.number DESC
			LIMIT $`+itoa(idx)+`
		`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item JournalEntryListItem
			if err := rows.Scan(&item.ID, &item.Number, &item.EntryDate, &item.Description,
				&item.IntentType, &item.Status, &item.TotalDebitCents, &item.TotalCreditCents); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": items})
}

// GetJournalEntry returns one journal entry with its lines (account code/name
// resolved) for the detail / drill-down view.
func (service *Service) GetJournalEntry(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	entryID, err := pathID(request, "id")
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var detail JournalEntryDetail
	var sourceRef pgtype.Text
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			SELECT je.id, je.number, je.entry_date::text, COALESCE(je.description, ''),
			       COALESCE(je.intent_type, ''), je.status,
			       COALESCE(SUM(jl.debit_cents), 0), COALESCE(SUM(jl.credit_cents), 0),
			       je.source_ref
			FROM journal_entries je
			LEFT JOIN journal_lines jl ON jl.tenant_id = je.tenant_id AND jl.entry_id = je.id
			WHERE je.tenant_id = $1 AND je.id = $2
			GROUP BY je.id, je.number, je.entry_date, je.description, je.intent_type, je.status, je.source_ref
		`, tenant, entryID).Scan(
			&detail.ID, &detail.Number, &detail.EntryDate, &detail.Description,
			&detail.IntentType, &detail.Status, &detail.TotalDebitCents, &detail.TotalCreditCents,
			&sourceRef,
		)
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "NOT_FOUND", "journal entry not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	detail.SourceRef = textValue(sourceRef)

	detail.Lines = make([]JournalEntryLine, 0)
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		lineRows, err := tx.Query(request.Context(), `
			SELECT jl.account_id, COALESCE(a.code, ''), COALESCE(a.name, ''),
			       jl.debit_cents, jl.credit_cents, COALESCE(jl.description, '')
			FROM journal_lines jl
			LEFT JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
			WHERE jl.tenant_id = $1 AND jl.entry_id = $2
			ORDER BY jl.debit_cents DESC, jl.credit_cents DESC
		`, tenant, entryID)
		if err != nil {
			return err
		}
		defer lineRows.Close()
		for lineRows.Next() {
			var line JournalEntryLine
			if err := lineRows.Scan(&line.AccountID, &line.AccountCode, &line.AccountName,
				&line.DebitCents, &line.CreditCents, &line.Description); err != nil {
				return err
			}
			detail.Lines = append(detail.Lines, line)
		}
		return lineRows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, detail)
}

// validateManualJournal checks the request shape before posting: a valid
// entry date and at least two balanced lines.
func validateManualJournal(req ManualJournalRequest) (string, string) {
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(req.EntryDate)); err != nil {
		return "INVALID_REQUEST", "entry_date must be YYYY-MM-DD"
	}
	if len(req.Lines) < 2 {
		return "INVALID_REQUEST", "at least two lines are required"
	}
	var debit, credit int64
	for _, line := range req.Lines {
		if line.AccountID <= 0 {
			return "INVALID_REQUEST", "every line needs an account_id"
		}
		if line.DebitCents < 0 || line.CreditCents < 0 {
			return "INVALID_REQUEST", "debit_cents and credit_cents must be non-negative"
		}
		if line.DebitCents > 0 && line.CreditCents > 0 {
			return "INVALID_REQUEST", "a line cannot have both debit and credit"
		}
		if line.DebitCents == 0 && line.CreditCents == 0 {
			return "INVALID_REQUEST", "a line cannot be zero"
		}
		debit += line.DebitCents
		credit += line.CreditCents
	}
	if debit != credit {
		return "NOT_BALANCED", "total debit must equal total credit"
	}
	return "", ""
}

// itoa wraps strconv.Itoa for query-parameter index building.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// textValue reads a pgtype.Text into a plain string.
func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

// isValidationError reports whether err is one of the engine's validation
// errors (so the handler can map it to 400 instead of 500).
func isValidationError(err error) bool {
	switch err {
	case ErrNotBalanced, ErrNoLines, ErrInvalidOpening, ErrInvalidAmount, ErrAccountNotPostable, ErrAccountTypeMismatch, ErrSameTransferAccount:
		return true
	}
	return false
}

// errorCode returns the stable error code for an engine error.
func errorCode(err error) string {
	switch err {
	case ErrNotBalanced:
		return "NOT_BALANCED"
	case ErrNoLines:
		return "NO_JOURNAL_LINES"
	case ErrInvalidOpening:
		return "INVALID_REQUEST"
	case ErrInvalidAmount:
		return "INVALID_AMOUNT"
	case ErrAccountNotPostable:
		return "ACCOUNT_NOT_POSTABLE"
	case ErrAccountTypeMismatch:
		return "ACCOUNT_TYPE_MISMATCH"
	case ErrSameTransferAccount:
		return "SAME_TRANSFER_ACCOUNT"
	}
	return "POST_FAILED"
}

// validateDateRange ensures both dates (when present) parse as YYYY-MM-DD.
func validateDateRange(fromDate, toDate string) error {
	if fromDate != "" {
		if _, err := time.Parse("2006-01-02", fromDate); err != nil {
			return err
		}
	}
	if toDate != "" {
		if _, err := time.Parse("2006-01-02", toDate); err != nil {
			return err
		}
	}
	return nil
}

// ensure these helpers are retained for reference (used by journal_manual.go
// and ledger/register handlers). The _ guard keeps unused-import checks off
// during incremental edits.
var _ = auth.TenantIDFromContext

// Note: this unused import guard was removed in audit session (m-004).
// The db package is still used via db.New() elsewhere, but not needed here.
