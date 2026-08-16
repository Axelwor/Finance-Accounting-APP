package cash

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"finance-accounting-app/backend/internal/accounting"
)

// ---------------------------------------------------------------------------
// List response
// ---------------------------------------------------------------------------

// CashEntryListItem is the unified history row returned by /cash-entries.
// Each item carries the journal entry id, its kind (cash-in | cash-out |
// transfer), display fields, and the resolved account names for the two
// sides of the journal (cash + counter, or from + to). Amounts are cents.
type CashEntryListItem struct {
	ID                int64  `json:"id"`
	Number            string `json:"number"`
	Kind              string `json:"kind"`
	EntryDate         string `json:"entry_date"`
	Status            string `json:"status"`
	Description       string `json:"description"`
	AmountCents       int64  `json:"amount_cents"`
	CashAccountID     int64  `json:"cash_account_id"`
	CashAccountCode   string `json:"cash_account_code"`
	CashAccountName   string `json:"cash_account_name"`
	CounterAccountID  int64  `json:"counter_account_id"`
	CounterAccountCod string `json:"counter_account_code"`
	CounterAccountNam string `json:"counter_account_name"`
	FromAccountID     int64  `json:"from_account_id"`
	FromAccountCode   string `json:"from_account_code"`
	FromAccountName   string `json:"from_account_name"`
	ToAccountID       int64  `json:"to_account_id"`
	ToAccountCode     string `json:"to_account_code"`
	ToAccountName     string `json:"to_account_name"`
	Reference         string `json:"reference"`
	ReversalOfID      int64  `json:"reversal_of_id"`
}

// ListCashEntries returns the cash & bank history for the current tenant.
// Supports filtering by intent (cash-in | cash-out | transfer), date range,
// account id, and a free-text match on number or description. Pagination is
// limit/offset; defaults are 50 rows starting at 0.
func (service *Service) ListCashEntries(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	query := request.URL.Query()
	kindFilter := strings.ToUpper(strings.TrimSpace(query.Get("kind")))
	fromDate := strings.TrimSpace(query.Get("from"))
	toDate := strings.TrimSpace(query.Get("to"))
	accountIDRaw := strings.TrimSpace(query.Get("account_id"))
	search := strings.TrimSpace(query.Get("q"))
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))

	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var accountID int64
	if accountIDRaw != "" {
		parsed, err := strconv.ParseInt(accountIDRaw, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "account_id must be a positive integer")
			return
		}
		accountID = parsed
	}

	items, err := service.queryCashEntries(request.Context(), tenant, kindFilter, fromDate, toDate, accountID, search, limit, offset)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  limit,
		"offset": offset,
		"count":  len(items),
	})
}

func (service *Service) queryCashEntries(
	ctx context.Context,
	tenantID int64,
	kindFilter string,
	fromDate string,
	toDate string,
	accountID int64,
	search string,
	limit int,
	offset int,
) ([]CashEntryListItem, error) {
	// Build the WHERE clause incrementally so unused filters stay out of
	// the parameter list. Only the cash intents are listed (CASH_IN,
	// CASH_OUT, TRANSFER); opening balances and reversals live under their
	// own tabs.
	conditions := []string{
		"e.tenant_id = $1",
		"e.intent_type IN ('CASH_IN', 'CASH_OUT', 'TRANSFER')",
	}
	args := []any{tenantID}

	if kindFilter != "" {
		switch kindFilter {
		case "CASH_IN", "MONEY_IN", "MONEY-IN":
			conditions = append(conditions, "e.intent_type = 'CASH_IN'")
		case "CASH_OUT", "MONEY_OUT", "MONEY-OUT":
			conditions = append(conditions, "e.intent_type = 'CASH_OUT'")
		case "TRANSFER":
			conditions = append(conditions, "e.intent_type = 'TRANSFER'")
		default:
			conditions = append(conditions, "1 = 0")
		}
	}
	if fromDate != "" {
		args = append(args, fromDate)
		conditions = append(conditions, "e.entry_date >= $"+strconv.Itoa(len(args)))
	}
	if toDate != "" {
		args = append(args, toDate)
		conditions = append(conditions, "e.entry_date <= $"+strconv.Itoa(len(args)))
	}
	if accountID > 0 {
		args = append(args, accountID)
		conditions = append(conditions, "EXISTS (SELECT 1 FROM journal_lines jl WHERE jl.tenant_id = e.tenant_id AND jl.entry_id = e.id AND jl.account_id = $"+strconv.Itoa(len(args))+")")
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		conditions = append(conditions, "(e.number ILIKE $"+strconv.Itoa(len(args))+" OR COALESCE(e.description, '') ILIKE $"+strconv.Itoa(len(args))+")")
	}

	args = append(args, limit)
	limitPos := len(args)
	args = append(args, offset)
	offsetPos := len(args)

	query := `
		SELECT
			e.id, e.number, e.intent_type, e.entry_date, e.status,
			COALESCE(e.description, '') AS description,
			COALESCE(e.source_ref, '') AS source_ref,
			COALESCE(e.reversal_of_id, 0) AS reversal_of_id,
			COALESCE(MAX(CASE WHEN l.debit_cents > 0 THEN l.account_id END), 0) AS debit_account_id,
			COALESCE(MAX(CASE WHEN l.credit_cents > 0 THEN l.account_id END), 0) AS credit_account_id,
			-- The cash amount of the entry: the cash side of the journal. For
			-- CASH_IN/TRANSFER the cash account is debited; for CASH_OUT it is
			-- credited. journal_lines has no amount_cents column — only
			-- debit_cents/credit_cents — so sum the side that carries the cash.
			COALESCE(SUM(CASE
				WHEN e.intent_type IN ('CASH_IN', 'TRANSFER') THEN l.debit_cents
				ELSE l.credit_cents
			END), 0) AS amount_cents
		FROM journal_entries e
		JOIN journal_lines l ON l.tenant_id = e.tenant_id AND l.entry_id = e.id
		WHERE ` + strings.Join(conditions, " AND ") + `
		GROUP BY e.id
		ORDER BY e.entry_date DESC, e.id DESC
		LIMIT $` + strconv.Itoa(limitPos) + ` OFFSET $` + strconv.Itoa(offsetPos) + `
	`

	rows, err := service.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		id, debit, credit, amount                      int64
		number, intent, status, description, sourceRef string
		date                                           time.Time
		reversalOfID                                   int64
	}
	out := []CashEntryListItem{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.number, &r.intent, &r.date, &r.status, &r.description, &r.sourceRef, &r.reversalOfID, &r.debit, &r.credit, &r.amount); err != nil {
			return nil, err
		}
		item := CashEntryListItem{
			ID:          r.id,
			Number:      r.number,
			Kind:        mapKind(r.intent),
			EntryDate:   r.date.Format("2006-01-02"),
			Status:      r.status,
			Description: r.description,
			AmountCents: r.amount,
			Reference:   r.sourceRef,

			ReversalOfID: r.reversalOfID,
		}
		if r.intent == string(accounting.IntentTransfer) {
			item.FromAccountID = r.debit
			item.ToAccountID = r.credit
		} else {
			// For CASH_IN, the cash account is debited; for CASH_OUT, the cash
			// account is credited. The counter account is the opposite side.
			if r.intent == string(accounting.IntentCashIn) {
				item.CashAccountID = r.debit
				item.CounterAccountID = r.credit
			} else {
				item.CashAccountID = r.credit
				item.CounterAccountID = r.debit
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Resolve account codes + names in a single batched query.
	if err := service.hydrateAccountNames(ctx, tenantID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func mapKind(intent string) string {
	switch intent {
	case "CASH_IN":
		return "money-in"
	case "CASH_OUT":
		return "money-out"
	case "TRANSFER":
		return "transfer"
	default:
		return strings.ToLower(intent)
	}
}

// hydrateAccountNames loads code + display name for every account id
// referenced in the items, then mutates each item in place. Missing
// accounts leave the name blank.
func (service *Service) hydrateAccountNames(ctx context.Context, tenantID int64, items []CashEntryListItem) error {
	idSet := map[int64]struct{}{}
	for _, item := range items {
		for _, id := range []int64{item.CashAccountID, item.CounterAccountID, item.FromAccountID, item.ToAccountID} {
			if id > 0 {
				idSet[id] = struct{}{}
			}
		}
	}
	if len(idSet) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	rows, err := service.pool.Query(ctx, `
		SELECT id, code, name
		FROM accounts
		WHERE tenant_id = $1 AND id = ANY($2)
	`, tenantID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	names := map[int64]struct{ code, name string }{}
	for rows.Next() {
		var id int64
		var code, name string
		if err := rows.Scan(&id, &code, &name); err != nil {
			return err
		}
		names[id] = struct{ code, name string }{code, name}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		item := &items[i]
		if entry, ok := names[item.CashAccountID]; ok {
			item.CashAccountCode = entry.code
			item.CashAccountName = entry.name
		}
		if entry, ok := names[item.CounterAccountID]; ok {
			item.CounterAccountCod = entry.code
			item.CounterAccountNam = entry.name
		}
		if entry, ok := names[item.FromAccountID]; ok {
			item.FromAccountCode = entry.code
			item.FromAccountName = entry.name
		}
		if entry, ok := names[item.ToAccountID]; ok {
			item.ToAccountCode = entry.code
			item.ToAccountName = entry.name
		}
	}
	return nil
}
