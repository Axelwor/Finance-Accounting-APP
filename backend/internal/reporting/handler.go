package reporting

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// TrialBalanceRow is one per-account line in the trial balance.
type TrialBalanceRow struct {
	AccountID   int64  `json:"account_id"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	DebitCents  int64  `json:"debit_cents"`
	CreditCents int64  `json:"credit_cents"`
}

// TrialBalanceResult is the JSON shape returned by GET /reports/trial-balance.
type TrialBalanceResult struct {
	Rows             []TrialBalanceRow `json:"rows"`
	TotalDebitCents  int64             `json:"total_debit_cents"`
	TotalCreditCents int64             `json:"total_credit_cents"`
	Balanced         bool              `json:"balanced"`
}

// ProfitLossSection is one grouped line in a framework-presented P&L. The
// same posted totals are re-grouped by account_type and relabelled per the
// selected framework (EMKM = simplest, SAK_UMUM = full PSAK breakdown).
type ProfitLossSection struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	AmountCents int64  `json:"amount_cents"`
}

// ProfitLossResult is the JSON shape returned by GET /reports/profit-loss.
// The flat revenue/expense/profit totals are always present. When a framework
// query param is supplied, Sections carries the framework-grouped breakdown
// (same data, different presentation).
type ProfitLossResult struct {
	RevenueCents int64               `json:"revenue_cents"`
	ExpenseCents int64               `json:"expense_cents"`
	ProfitCents  int64               `json:"profit_cents"`
	Framework    string              `json:"framework,omitempty"`
	Sections     []ProfitLossSection `json:"sections,omitempty"`
}

// CashFlowResult is the JSON shape returned by GET /reports/cash-flow.
type CashFlowResult struct {
	InflowCents      int64 `json:"inflow_cents"`
	OutflowCents     int64 `json:"outflow_cents"`
	NetCashFlowCents int64 `json:"net_cash_flow_cents"`
}

// BalanceSheetResult is the JSON shape returned by GET /reports/balance-sheet.
// Current-period profit is folded into equity so the sheet balances before
// the period is closed (engine §21.2: current earnings real-time).
type BalanceSheetResult struct {
	AssetCents     int64 `json:"asset_cents"`
	LiabilityCents int64 `json:"liability_cents"`
	EquityCents    int64 `json:"equity_cents"`
	ProfitCents    int64 `json:"profit_cents"`
	Balanced       bool  `json:"balanced"`
}

// reportType is the canonical slug used in URLs and dispatch tables.
type reportType string

const (
	reportTrialBalance reportType = "trial-balance"
	reportProfitLoss   reportType = "profit-loss"
	reportBalanceSheet reportType = "balance-sheet"
	reportCashFlow     reportType = "cash-flow"
)

var reportTitles = map[reportType]string{
	reportTrialBalance: "Trial Balance",
	reportProfitLoss:   "Profit & Loss",
	reportBalanceSheet: "Balance Sheet",
	reportCashFlow:     "Cash Flow",
}

// dateRange captures the optional from_date / to_date query params used to
// scope a report. When a bound is empty the report aggregates over all posted
// entries (current behaviour). For trial-balance and balance-sheet the range
// is interpreted as "as of to_date" (cumulative); for P&L and cash-flow the
// movement is measured within [from_date, to_date].
type dateRange struct {
	fromDate string // YYYY-MM-DD, may be ""
	toDate   string // YYYY-MM-DD, may be ""
}

// reportFilter bundles the optional query params that narrow a report. It
// extends dateRange with a dimension_id scope (only journal lines tagged with
// the dimension are aggregated) and a framework presentation selector.
type reportFilter struct {
	dateRange
	dimensionID int64  // 0 = no dimension filter
	framework   string // EMKM | ETAP | SAK_UMUM, or "" for the default
}

// parseDateRange reads the optional from_date / to_date query params and
// validates the YYYY-MM-DD format. Invalid values are silently ignored so the
// report degrades to "all posted entries" rather than erroring — the toolbar
// sends blanks when the user clears a picker.
func parseDateRange(r *http.Request) dateRange {
	parse := func(raw string) string {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return ""
		}
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return ""
		}
		return raw
	}
	return dateRange{
		fromDate: parse(r.URL.Query().Get("from_date")),
		toDate:   parse(r.URL.Query().Get("to_date")),
	}
}

// parseReportFilter reads from_date / to_date plus the optional framework and
// dimension_id query params used by the reporting endpoints. Invalid values
// are silently dropped so the report degrades to the default rather than
// erroring.
func parseReportFilter(r *http.Request) reportFilter {
	dr := parseDateRange(r)
	f := reportFilter{dateRange: dr}
	if dimRaw := strings.TrimSpace(r.URL.Query().Get("dimension_id")); dimRaw != "" {
		if id, err := strconv.ParseInt(dimRaw, 10, 64); err == nil && id > 0 {
			f.dimensionID = id
		}
	}
	switch strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("framework"))) {
	case "EMKM", "ETAP", "SAK_UMUM":
		f.framework = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("framework")))
	}
	return f
}

// dateRangeLabel renders a human-readable range for export headers, e.g.
// "Jan 2 – Jan 31 2025" or "All posted entries".
func dateRangeLabel(fromDate, toDate string) string {
	fmtDate := func(s string) string {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return s
		}
		return t.Format("Jan 2 2006")
	}
	switch {
	case fromDate != "" && toDate != "":
		return fmtDate(fromDate) + " – " + fmtDate(toDate)
	case toDate != "":
		return "As of " + fmtDate(toDate)
	case fromDate != "":
		return "From " + fmtDate(fromDate)
	default:
		return "All posted entries"
	}
}

// dateFilter builds a SQL fragment that restricts journal_entries by entry_date.
// The returned fragment starts with " AND " so it can be appended to an
// existing WHERE clause. baseArg is the placeholder offset to use (so the
// caller can keep tenant_id as $1); date params are appended to *args and the
// offset incremented for each. When neither bound is supplied the fragment is
// empty (report aggregates over all posted entries).
func dateFilter(d dateRange, baseArg int, args *[]any) string {
	var clauses []string
	if d.fromDate != "" {
		*args = append(*args, d.fromDate)
		clauses = append(clauses, "je.entry_date >= $"+strconv.Itoa(baseArg))
		baseArg++
	}
	if d.toDate != "" {
		*args = append(*args, d.toDate)
		clauses = append(clauses, "je.entry_date <= $"+strconv.Itoa(baseArg))
		baseArg++
	}
	if len(clauses) == 0 {
		return ""
	}
	return " AND " + strings.Join(clauses, " AND ")
}

// TrialBalance returns per-account debit/credit totals across all posted
// journals for a tenant. With to_date supplied the balance is cumulative from
// inception to to_date; from_date is ignored (a trial balance is a snapshot,
// not a movement).
func (service *Service) TrialBalance(writer http.ResponseWriter, request *http.Request) {
	f := parseReportFilter(request)
	result, err := service.fetchTrialBalance(request.Context(), tenantFrom(request), f)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ProfitLoss aggregates revenue and expense groups for the requested range.
// An optional `framework` query param switches the presentation (EMKM /
// ETAP / SAK Umum); the underlying totals are identical. An optional
// `dimension_id` query param narrows the aggregation to journal lines tagged
// with that dimension (cabang / proyek).
func (service *Service) ProfitLoss(writer http.ResponseWriter, request *http.Request) {
	f := parseReportFilter(request)
	result, err := service.fetchProfitLoss(request.Context(), tenantFrom(request), f)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// BalanceSheet aggregates asset, liability, and equity groups. Current-period
// profit (revenue − expense) is added to equity so the balance sheet balances
// before the period is closed (engine §21.2: current earnings real-time). With
// to_date supplied the snapshot is taken as of that date.
func (service *Service) BalanceSheet(writer http.ResponseWriter, request *http.Request) {
	f := parseReportFilter(request)
	result, err := service.fetchBalanceSheet(request.Context(), tenantFrom(request), f)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// CashFlow aggregates movements across CASH/BANK accounts within the range.
func (service *Service) CashFlow(writer http.ResponseWriter, request *http.Request) {
	f := parseReportFilter(request)
	result, err := service.fetchCashFlow(request.Context(), tenantFrom(request), f)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}
