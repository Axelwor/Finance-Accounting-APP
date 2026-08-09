package budget

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// US-093: Budgets — planned amounts per account / month for a fiscal year.
// A budget may be scoped to a dimension (e.g. a branch budget). The vs-actual
// report compares budget_lines against actual posted journal movements for
// the same account / month.

type budgetLineInput struct {
	AccountID   int64 `json:"account_id"`
	DimensionID int64 `json:"dimension_id"`
	Month       int   `json:"month"`
	AmountCents int64 `json:"amount_cents"`
}

type budgetRequest struct {
	Name        string            `json:"name"`
	FiscalYear  int               `json:"fiscal_year"`
	DimensionID int64             `json:"dimension_id"`
	Lines       []budgetLineInput `json:"lines"`
}

type budgetLineResponse struct {
	ID          int64 `json:"id"`
	BudgetID    int64 `json:"budget_id"`
	AccountID   int64 `json:"account_id"`
	DimensionID int64 `json:"dimension_id"`
	Month       int   `json:"month"`
	AmountCents int64 `json:"amount_cents"`
}

type budgetResponse struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	FiscalYear  int                  `json:"fiscal_year"`
	DimensionID int64                `json:"dimension_id"`
	Status      string               `json:"status"`
	CreatedAt   string               `json:"created_at"`
	Lines       []budgetLineResponse `json:"lines,omitempty"`
}

type budgetListItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FiscalYear  int    `json:"fiscal_year"`
	DimensionID int64  `json:"dimension_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	LineCount   int    `json:"line_count"`
	TotalCents  int64  `json:"total_cents"`
}

// CreateBudget — POST /budgets
func (service *Service) CreateBudget(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req budgetRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}
	if req.FiscalYear <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "fiscal_year is required")
		return
	}
	if len(req.Lines) == 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "at least one budget line is required")
		return
	}
	for i, line := range req.Lines {
		if line.AccountID <= 0 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST",
				fmt.Sprintf("line %d: account_id is required", i+1))
			return
		}
		if line.Month < 1 || line.Month > 12 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST",
				fmt.Sprintf("line %d: month must be 1..12", i+1))
			return
		}
		if line.AmountCents < 0 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST",
				fmt.Sprintf("line %d: amount_cents cannot be negative", i+1))
			return
		}
	}

	ctx := request.Context()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "BUDGET_CREATE_FAILED", err.Error())
		return
	}
	defer tx.Rollback(ctx)
	if err := withTenant(ctx, tx, tenant); err != nil {
		writeError(writer, http.StatusInternalServerError, "BUDGET_CREATE_FAILED", err.Error())
		return
	}

	dimArg := nullableInt8(req.DimensionID)
	var b budgetResponse
	b.Status = "DRAFT"
	err = tx.QueryRow(ctx, `
		INSERT INTO budgets (tenant_id, name, fiscal_year, dimension_id, status)
		VALUES ($1, $2, $3, $4, 'DRAFT')
		RETURNING id, name, fiscal_year, COALESCE(dimension_id, 0), status, created_at
	`, tenant, name, req.FiscalYear, dimArg).Scan(
		&b.ID, &b.Name, &b.FiscalYear, &b.DimensionID, &b.Status, &b.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "BUDGET_EXISTS",
				"a budget with this name and fiscal_year already exists")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(writer, http.StatusBadRequest, "DIMENSION_NOT_FOUND",
				"dimension_id does not exist for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "BUDGET_CREATE_FAILED", err.Error())
		return
	}

	b.Lines = make([]budgetLineResponse, 0, len(req.Lines))
	for _, line := range req.Lines {
		var bl budgetLineResponse
		lineDimArg := nullableInt8(line.DimensionID)
		err = tx.QueryRow(ctx, `
			INSERT INTO budget_lines (tenant_id, budget_id, account_id, dimension_id, month, amount_cents)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, budget_id, account_id, COALESCE(dimension_id, 0), month, amount_cents
		`, tenant, b.ID, line.AccountID, lineDimArg, line.Month, line.AmountCents).Scan(
			&bl.ID, &bl.BudgetID, &bl.AccountID, &bl.DimensionID, &bl.Month, &bl.AmountCents)
		if err != nil {
			if isForeignKeyViolation(err) {
				writeError(writer, http.StatusBadRequest, "ACCOUNT_OR_DIMENSION_NOT_FOUND",
					"account_id or dimension_id does not exist for this tenant")
				return
			}
			writeError(writer, http.StatusInternalServerError, "BUDGET_CREATE_FAILED", err.Error())
			return
		}
		b.Lines = append(b.Lines, bl)
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(writer, http.StatusInternalServerError, "BUDGET_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, b)
}

// ListBudgets — GET /budgets?fiscal_year=&status=
func (service *Service) ListBudgets(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	q := request.URL.Query()
	args := []any{tenant, optionalInt(q.Get("fiscal_year")), nullableStr(strings.TrimSpace(q.Get("status")))}
	rows, err := service.pool.Query(request.Context(), `
		SELECT b.id, b.name, b.fiscal_year, COALESCE(b.dimension_id, 0), b.status, b.created_at,
		       COUNT(bl.id), COALESCE(SUM(bl.amount_cents), 0)
		FROM budgets b
		LEFT JOIN budget_lines bl ON bl.tenant_id = b.tenant_id AND bl.budget_id = b.id
		WHERE b.tenant_id = $1
		  AND ($2::int IS NULL OR b.fiscal_year = $2)
		  AND ($3::text IS NULL OR b.status = $3)
		GROUP BY b.id
		ORDER BY b.fiscal_year DESC, b.name
	`, args...)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "BUDGET_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	items := []budgetListItem{}
	for rows.Next() {
		var b budgetListItem
		if err := rows.Scan(&b.ID, &b.Name, &b.FiscalYear, &b.DimensionID, &b.Status,
			&b.CreatedAt, &b.LineCount, &b.TotalCents); err != nil {
			writeError(writer, http.StatusInternalServerError, "BUDGET_LIST_FAILED", err.Error())
			return
		}
		items = append(items, b)
	}
	writeJSON(writer, http.StatusOK, items)
}

// GetBudget — GET /budgets/{id}
func (service *Service) GetBudget(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var b budgetResponse
	err = service.pool.QueryRow(request.Context(), `
		SELECT id, name, fiscal_year, COALESCE(dimension_id, 0), status, created_at
		FROM budgets WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(&b.ID, &b.Name, &b.FiscalYear, &b.DimensionID, &b.Status, &b.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "BUDGET_NOT_FOUND", "budget not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "BUDGET_GET_FAILED", err.Error())
		return
	}

	rows, err := service.pool.Query(request.Context(), `
		SELECT id, budget_id, account_id, COALESCE(dimension_id, 0), month, amount_cents
		FROM budget_lines
		WHERE tenant_id = $1 AND budget_id = $2
		ORDER BY month, account_id
	`, tenant, id)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "BUDGET_GET_FAILED", err.Error())
		return
	}
	defer rows.Close()
	b.Lines = []budgetLineResponse{}
	for rows.Next() {
		var bl budgetLineResponse
		if err := rows.Scan(&bl.ID, &bl.BudgetID, &bl.AccountID, &bl.DimensionID, &bl.Month, &bl.AmountCents); err != nil {
			writeError(writer, http.StatusInternalServerError, "BUDGET_GET_FAILED", err.Error())
			return
		}
		b.Lines = append(b.Lines, bl)
	}
	writeJSON(writer, http.StatusOK, b)
}

// BudgetVsActualResult is the response shape for GET /budgets/{id}/vs-actual.
type BudgetVsActualResult struct {
	BudgetID      int64               `json:"budget_id"`
	Name          string              `json:"name"`
	FiscalYear    int                 `json:"fiscal_year"`
	DimensionID   int64               `json:"dimension_id"`
	Rows          []BudgetVsActualRow `json:"rows"`
	TotalBudget   int64               `json:"total_budget_cents"`
	TotalActual   int64               `json:"total_actual_cents"`
	TotalVariance int64               `json:"total_variance_cents"`
}

// BudgetVsActualRow compares one account/month budget line against the actual
// posted movement for the same account/month (within the budget dimension).
type BudgetVsActualRow struct {
	AccountID     int64  `json:"account_id"`
	AccountCode   string `json:"account_code"`
	AccountName   string `json:"account_name"`
	Month         int    `json:"month"`
	BudgetCents   int64  `json:"budget_cents"`
	ActualCents   int64  `json:"actual_cents"`
	VarianceCents int64  `json:"variance_cents"`
}

// BudgetVsActual — GET /budgets/{id}/vs-actual
// Compares each budget_lines row against the sum of posted journal movements
// for the same account/month in the budget's fiscal year. When the budget is
// dimension-scoped, actuals are filtered to journal lines tagged with that
// dimension.
func (service *Service) BudgetVsActual(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Load the budget header.
	var (
		name        string
		fiscalYear  int
		dimensionID int64
	)
	err = service.pool.QueryRow(request.Context(), `
		SELECT name, fiscal_year, COALESCE(dimension_id, 0)
		FROM budgets WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(&name, &fiscalYear, &dimensionID)
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "BUDGET_NOT_FOUND", "budget not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "BUDGET_VS_ACTUAL_FAILED", err.Error())
		return
	}

	// Load budget lines grouped by (account_id, month).
	budgetRows, err := service.pool.Query(request.Context(), `
		SELECT account_id, month, COALESCE(SUM(amount_cents), 0)
		FROM budget_lines
		WHERE tenant_id = $1 AND budget_id = $2
		GROUP BY account_id, month
	`, tenant, id)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "BUDGET_VS_ACTUAL_FAILED", err.Error())
		return
	}
	defer budgetRows.Close()

	type key struct {
		accountID int64
		month     int
	}
	budgetMap := make(map[key]int64)
	accountIDs := make(map[int64]bool)
	for budgetRows.Next() {
		var accountID int64
		var month int
		var amount int64
		if err := budgetRows.Scan(&accountID, &month, &amount); err != nil {
			writeError(writer, http.StatusInternalServerError, "BUDGET_VS_ACTUAL_FAILED", err.Error())
			return
		}
		budgetMap[key{accountID, month}] = amount
		accountIDs[accountID] = true
	}

	// Build the set of months for the fiscal year so we can compute actuals per
	// account/month. We use calendar months 1..12 of fiscalYear.
	// Actual = signed movement on the account within that month:
	//   revenue/other_income: credit - debit (income increases on credit)
	//   expense/other expense/cogs: debit - credit (expense increases on debit)
	// This matches the reporting layer's sign convention.
	args := []any{tenant, fiscalYear}
	if len(accountIDs) > 0 {
		// Narrow the actuals query to the budgeted accounts.
		ids := make([]any, 0, len(accountIDs))
		for id := range accountIDs {
			ids = append(ids, id)
		}
		args = append(args, ids)
	} else {
		args = append(args, []any{})
	}

	// When the budget is dimension-scoped, join journal_line_dimensions on the
	// dimension_id. Otherwise aggregate all posted movements.
	dimensionJoin := ""
	if dimensionID > 0 {
		dimensionJoin = `
			JOIN journal_line_dimensions jld
				ON jld.tenant_id = jl.tenant_id AND jld.journal_line_id = jl.id
			   AND jld.dimension_id = $` + strconv.Itoa(len(args)) + `
		`
		args = append(args, dimensionID)
	}

	// We need a per-account net sign. Resolve the report_group so revenue is
	// credit-led and expense is debit-led.
	actualRows, err := service.pool.Query(request.Context(), `
		SELECT jl.account_id,
		       EXTRACT(MONTH FROM je.entry_date)::int AS month,
		       a.report_group,
		       COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0) AS signed_credits,
		       COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0) AS signed_debits
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1
		  AND je.status = 'POSTED'
		  AND EXTRACT(YEAR FROM je.entry_date)::int = $2
		  AND jl.account_id = ANY ($3)
		`+dimensionJoin+`
		GROUP BY jl.account_id, EXTRACT(MONTH FROM je.entry_date), a.report_group
	`, args...)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "BUDGET_VS_ACTUAL_FAILED", err.Error())
		return
	}
	defer actualRows.Close()

	actualMap := make(map[key]int64)
	for actualRows.Next() {
		var accountID int64
		var month int
		var reportGroup string
		var signedCredits, signedDebits int64
		if err := actualRows.Scan(&accountID, &month, &reportGroup, &signedCredits, &signedDebits); err != nil {
			writeError(writer, http.StatusInternalServerError, "BUDGET_VS_ACTUAL_FAILED", err.Error())
			return
		}
		var net int64
		switch reportGroup {
		case "revenue":
			net = signedCredits // credit - debit
		default:
			net = signedDebits // debit - credit (expense/asset/liability/eq)
		}
		actualMap[key{accountID, month}] += net
	}

	// Resolve account code/name for the budgeted accounts.
	accountMeta := make(map[int64]struct{ code, name string })
	if len(accountIDs) > 0 {
		ids := make([]any, 0, len(accountIDs))
		for id := range accountIDs {
			ids = append(ids, id)
		}
		metaRows, err := service.pool.Query(request.Context(), `
			SELECT id, code, name FROM accounts WHERE tenant_id = $1 AND id = ANY ($2)
		`, tenant, ids)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "BUDGET_VS_ACTUAL_FAILED", err.Error())
			return
		}
		for metaRows.Next() {
			var id2 int64
			var code, aname string
			if err := metaRows.Scan(&id2, &code, &aname); err != nil {
				metaRows.Close()
				writeError(writer, http.StatusInternalServerError, "BUDGET_VS_ACTUAL_FAILED", err.Error())
				return
			}
			accountMeta[id2] = struct{ code, name string }{code, aname}
		}
		metaRows.Close()
	}

	result := BudgetVsActualResult{
		BudgetID:    id,
		Name:        name,
		FiscalYear:  fiscalYear,
		DimensionID: dimensionID,
		Rows:        []BudgetVsActualRow{},
	}
	for k, budgetCents := range budgetMap {
		actualCents := actualMap[k]
		row := BudgetVsActualRow{
			AccountID:     k.accountID,
			Month:         k.month,
			BudgetCents:   budgetCents,
			ActualCents:   actualCents,
			VarianceCents: actualCents - budgetCents,
		}
		if meta, ok := accountMeta[k.accountID]; ok {
			row.AccountCode = meta.code
			row.AccountName = meta.name
		}
		result.Rows = append(result.Rows, row)
		result.TotalBudget += budgetCents
		result.TotalActual += actualCents
	}
	result.TotalVariance = result.TotalActual - result.TotalBudget
	if len(result.Rows) == 0 {
		result.Rows = []BudgetVsActualRow{}
	}
	writeJSON(writer, http.StatusOK, result)
}

// nullableInt8 converts an int64 to a nullable value: 0 -> NULL.
func nullableInt8(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

// unused guards so the linter keeps time/fmt/pgx imports used.
var _ = time.Now
var _ = fmt.Sprintf
var _ = pgx.ErrNoRows
var _ = errors.New
