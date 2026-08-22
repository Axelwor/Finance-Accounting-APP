package accounting

import (
	"github.com/jackc/pgx/v5"

	"net/http"
	"strings"

	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// Journal Register
// ---------------------------------------------------------------------------

// JournalRegisterItem is one row in GET /journal-register. It carries the
// entry header plus its totals; lines are loaded per-entry on drill-down
// via GET /journal-entries/{id}.
type JournalRegisterItem struct {
	ID               int64  `json:"id"`
	Number           string `json:"number"`
	EntryDate        string `json:"entry_date"`
	Description      string `json:"description"`
	IntentType       string `json:"intent_type"`
	TotalDebitCents  int64  `json:"total_debit_cents"`
	TotalCreditCents int64  `json:"total_credit_cents"`
}

// JournalRegisterResult wraps the register rows with the applied filters.
type JournalRegisterResult struct {
	Items      []JournalRegisterItem `json:"items"`
	FromDate   string                `json:"from_date"`
	ToDate     string                `json:"to_date"`
	IntentType string                `json:"intent_type"`
}

// GetJournalRegister lists all posted journal entries (any intent type) with
// their debit/credit totals. Optional filters: from_date, to_date, intent_type.
// Each row is a single journal entry; lines are fetched on demand by the
// detail view (GET /journal-entries/{id}).
func (service *Service) GetJournalRegister(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	query := request.URL.Query()
	fromDate := normalizeDate(query.Get("from_date"))
	toDate := normalizeDate(query.Get("to_date"))
	intentType := strings.ToUpper(strings.TrimSpace(query.Get("intent_type")))

	args := []any{tenant}
	where := "WHERE je.tenant_id = $1 AND je.status = 'POSTED'"
	idx := 2
	if fromDate != "" {
		where += " AND je.entry_date >= $" + itoa(idx)
		args = append(args, fromDate)
		idx++
	}
	if toDate != "" {
		where += " AND je.entry_date <= $" + itoa(idx)
		args = append(args, toDate)
		idx++
	}
	if intentType != "" {
		where += " AND UPPER(COALESCE(je.intent_type, '')) = $" + itoa(idx)
		args = append(args, intentType)
		idx++
	}
	args = append(args, 500) // limit
	items := make([]JournalRegisterItem, 0)
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT je.id, je.number, je.entry_date::text, COALESCE(je.description, ''),
			       COALESCE(je.intent_type, ''),
			       COALESCE(SUM(jl.debit_cents), 0), COALESCE(SUM(jl.credit_cents), 0)
			FROM journal_entries je
			LEFT JOIN journal_lines jl ON jl.tenant_id = je.tenant_id AND jl.entry_id = je.id
			`+where+`
			GROUP BY je.id, je.number, je.entry_date, je.description, je.intent_type
			ORDER BY je.entry_date DESC, je.number DESC
			LIMIT $`+itoa(idx)+`
		`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item JournalRegisterItem
			if err := rows.Scan(&item.ID, &item.Number, &item.EntryDate, &item.Description,
				&item.IntentType, &item.TotalDebitCents, &item.TotalCreditCents); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, JournalRegisterResult{
		Items:      items,
		FromDate:   fromDate,
		ToDate:     toDate,
		IntentType: intentType,
	})
}
