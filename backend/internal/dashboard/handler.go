package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

// ---------------------------------------------------------------------------
// F-Dashboard: Per-user dashboard widget system
//   Each user can have a custom dashboard layout with configurable widgets.
//   Widget data is fetched dynamically from the backend based on widget type.
// ---------------------------------------------------------------------------

// Widget types supported by the system.
const (
	WidgetCashBalance       = "cash_balance"
	WidgetARAging           = "ar_aging_summary"
	WidgetAPAging           = "ap_aging_summary"
	WidgetPLSnapshot        = "pl_snapshot"
	WidgetBudgetVsActual    = "budget_vs_actual"
	WidgetRevenueByCustomer = "revenue_by_customer"
	WidgetLowStock          = "low_stock_alert"
	WidgetRecentTxns        = "recent_transactions"
	WidgetOutstandingInvoices = "outstanding_invoices"
	WidgetTaxSummary        = "tax_summary"
	WidgetPeriodStatus      = "period_status"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Get("/dashboard/layout", service.GetLayout)
	router.Put("/dashboard/layout", service.SaveLayout)
	router.Get("/dashboard/widgets", service.ListWidgets)
	router.Post("/dashboard/widgets", service.AddWidget)
	router.Put("/dashboard/widgets/{id}", service.UpdateWidget)
	router.Delete("/dashboard/widgets/{id}", service.DeleteWidget)
	router.Get("/dashboard/widgets/{id}/data", service.GetWidgetData)
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

type DashboardLayout struct {
	UserID  int64    `json:"user_id"`
	Widgets []Widget `json:"widgets"`
}

type Widget struct {
	ID        int64   `json:"id"`
	WidgetType string  `json:"widget_type"`
	Title     string  `json:"title"`
	Config    json.RawMessage `json:"config,omitempty"`
	Position  int     `json:"position"`
	ColSpan   int     `json:"col_span"`
	RowSpan   int     `json:"row_span"`
}

// knownWidgetTypes lists every supported widget type constant. Used for
// membership validation. Kept in sync with the Widget* constants above.
var knownWidgetTypes = map[string]bool{
	WidgetCashBalance:         true,
	WidgetARAging:             true,
	WidgetAPAging:             true,
	WidgetPLSnapshot:          true,
	WidgetBudgetVsActual:      true,
	WidgetRevenueByCustomer:   true,
	WidgetLowStock:            true,
	WidgetRecentTxns:          true,
	WidgetOutstandingInvoices: true,
	WidgetTaxSummary:          true,
	WidgetPeriodStatus:        true,
}

// validateWidgetType returns true when the widget type is non-empty. The
// handler treats any non-empty type as acceptable; use isKnownWidgetType to
// additionally check membership against the supported set. Pure: no DB/HTTP.
func validateWidgetType(widgetType string) bool {
	return widgetType != ""
}

// isKnownWidgetType returns true when widgetType matches one of the supported
// Widget* constants. Pure: no DB/HTTP coupling.
func isKnownWidgetType(widgetType string) bool {
	return knownWidgetTypes[widgetType]
}

// normalizeWidgetGrid applies default grid spans when the caller omits them.
// ColSpan defaults to 2, RowSpan defaults to 1 — matching the seeded defaults
// and the AddWidget handler's inline normalization. Returns the normalized
// (colSpan, rowSpan) pair. Pure: no DB/HTTP coupling.
func normalizeWidgetGrid(colSpan, rowSpan int) (int, int) {
	if colSpan == 0 {
		colSpan = 2
	}
	if rowSpan == 0 {
		rowSpan = 1
	}
	return colSpan, rowSpan
}

func (service *Service) GetLayout(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		writeErr(w, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}

	rows, err := service.pool.Query(r.Context(), `
		SELECT id, widget_type, title, config, position, col_span, row_span
		FROM dashboard_widgets
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY position
	`, tenantID, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
		return
	}
	defer rows.Close()

	var widgets []Widget
	for rows.Next() {
		var widget Widget
		var config []byte
		if err := rows.Scan(&widget.ID, &widget.WidgetType, &widget.Title, &config, &widget.Position, &widget.ColSpan, &widget.RowSpan); err != nil {
			writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
			return
		}
		if len(config) > 0 {
			widget.Config = json.RawMessage(config)
		}
		widgets = append(widgets, widget)
	}
	if widgets == nil {
		widgets = []Widget{}
	}

	// If user has no widgets, seed defaults.
	if len(widgets) == 0 {
		widgets = service.seedDefaultWidgets(r.Context(), tenantID, userID)
	}

	writeJSON(w, http.StatusOK, DashboardLayout{UserID: userID, Widgets: widgets})
}

func (service *Service) seedDefaultWidgets(ctx context.Context, tenantID, userID int64) []Widget {
	defaults := []Widget{
		{WidgetType: WidgetCashBalance, Title: "Cash Balance", Position: 0, ColSpan: 3, RowSpan: 1},
		{WidgetType: WidgetPLSnapshot, Title: "P&L Snapshot", Position: 1, ColSpan: 3, RowSpan: 1},
		{WidgetType: WidgetARAging, Title: "AR Aging", Position: 2, ColSpan: 3, RowSpan: 1},
		{WidgetType: WidgetAPAging, Title: "AP Aging", Position: 3, ColSpan: 3, RowSpan: 1},
		{WidgetType: WidgetLowStock, Title: "Low Stock Alert", Position: 4, ColSpan: 2, RowSpan: 1},
		{WidgetType: WidgetPeriodStatus, Title: "Period Status", Position: 5, ColSpan: 2, RowSpan: 1},
	}
	for i := range defaults {
		_, _ = service.pool.Exec(ctx, `
			INSERT INTO dashboard_widgets (tenant_id, user_id, widget_type, title, position, col_span, row_span)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, tenantID, userID, defaults[i].WidgetType, defaults[i].Title,
			defaults[i].Position, defaults[i].ColSpan, defaults[i].RowSpan)
	}
	return defaults
}

type SaveLayoutRequest struct {
	Widgets []Widget `json:"widgets"`
}

func (service *Service) SaveLayout(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		writeErr(w, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}

	var req SaveLayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Delete existing and re-insert (full replace strategy).
	_, err := service.pool.Exec(r.Context(), `
		DELETE FROM dashboard_widgets WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
		return
	}

	for i, widget := range req.Widgets {
		pos := widget.Position
		if pos == 0 {
			pos = i
		}
		colSpan := widget.ColSpan
		if colSpan == 0 {
			colSpan = 2
		}
		rowSpan := widget.RowSpan
		if rowSpan == 0 {
			rowSpan = 1
		}
		_, err := service.pool.Exec(r.Context(), `
			INSERT INTO dashboard_widgets (tenant_id, user_id, widget_type, title, config, position, col_span, row_span)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, tenantID, userID, widget.WidgetType, widget.Title,
			[]byte(widget.Config), pos, colSpan, rowSpan)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "layout saved"})
}

// ---------------------------------------------------------------------------
// Widget CRUD
// ---------------------------------------------------------------------------

func (service *Service) ListWidgets(w http.ResponseWriter, r *http.Request) {
	// Return available widget types with metadata.
	widgetTypes := []map[string]any{
		{"type": WidgetCashBalance, "title": "Cash Balance", "default_col_span": 3, "default_row_span": 1, "category": "financial"},
		{"type": WidgetARAging, "title": "AR Aging Summary", "default_col_span": 3, "default_row_span": 1, "category": "receivables"},
		{"type": WidgetAPAging, "title": "AP Aging Summary", "default_col_span": 3, "default_row_span": 1, "category": "payables"},
		{"type": WidgetPLSnapshot, "title": "P&L Snapshot", "default_col_span": 3, "default_row_span": 1, "category": "financial"},
		{"type": WidgetBudgetVsActual, "title": "Budget vs Actual", "default_col_span": 3, "default_row_span": 1, "category": "planning"},
		{"type": WidgetRevenueByCustomer, "title": "Revenue by Customer", "default_col_span": 3, "default_row_span": 1, "category": "sales"},
		{"type": WidgetLowStock, "title": "Low Stock Alert", "default_col_span": 2, "default_row_span": 1, "category": "inventory"},
		{"type": WidgetRecentTxns, "title": "Recent Transactions", "default_col_span": 4, "default_row_span": 2, "category": "activity"},
		{"type": WidgetOutstandingInvoices, "title": "Outstanding Invoices", "default_col_span": 3, "default_row_span": 1, "category": "receivables"},
		{"type": WidgetTaxSummary, "title": "Tax Summary", "default_col_span": 3, "default_row_span": 1, "category": "tax"},
		{"type": WidgetPeriodStatus, "title": "Period Status", "default_col_span": 2, "default_row_span": 1, "category": "system"},
	}
	writeJSON(w, http.StatusOK, widgetTypes)
}

type AddWidgetRequest struct {
	WidgetType string  `json:"widget_type"`
	Title     string  `json:"title"`
	Config    json.RawMessage `json:"config,omitempty"`
	Position  int     `json:"position"`
	ColSpan   int     `json:"col_span"`
	RowSpan   int     `json:"row_span"`
}

func (service *Service) AddWidget(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		writeErr(w, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}

	var req AddWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if !validateWidgetType(req.WidgetType) {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "widget_type is required")
		return
	}
	req.ColSpan, req.RowSpan = normalizeWidgetGrid(req.ColSpan, req.RowSpan)

	var id int64
	err := service.pool.QueryRow(r.Context(), `
		INSERT INTO dashboard_widgets (tenant_id, user_id, widget_type, title, config, position, col_span, row_span)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, tenantID, userID, req.WidgetType, req.Title,
		[]byte(req.Config), req.Position, req.ColSpan, req.RowSpan).Scan(&id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "WIDGET_CREATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          id,
		"widget_type": req.WidgetType,
		"title":       req.Title,
		"position":    req.Position,
		"col_span":    req.ColSpan,
		"row_span":    req.RowSpan,
	})
}

func (service *Service) UpdateWidget(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		writeErr(w, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	widgetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || widgetID <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "widget id must be a positive integer")
		return
	}

	var req AddWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	_, err = service.pool.Exec(r.Context(), `
		UPDATE dashboard_widgets
		SET title = $3, config = $4, position = $5, col_span = $6, row_span = $7, updated_at = now()
		WHERE tenant_id = $1 AND user_id = $2 AND id = $8
	`, tenantID, userID, req.Title, []byte(req.Config), req.Position, req.ColSpan, req.RowSpan, widgetID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "WIDGET_UPDATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "widget updated"})
}

func (service *Service) DeleteWidget(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		writeErr(w, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	widgetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || widgetID <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "widget id must be a positive integer")
		return
	}

	_, err = service.pool.Exec(r.Context(), `
		DELETE FROM dashboard_widgets
		WHERE tenant_id = $1 AND user_id = $2 AND id = $3
	`, tenantID, userID, widgetID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "WIDGET_DELETE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "widget deleted"})
}

// ---------------------------------------------------------------------------
// Widget Data — fetches live data for a specific widget
// ---------------------------------------------------------------------------

func (service *Service) GetWidgetData(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tenantID <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	widgetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || widgetID <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "widget id must be a positive integer")
		return
	}

	// Look up the widget type.
	var widgetType string
	err = service.pool.QueryRow(r.Context(), `
		SELECT widget_type FROM dashboard_widgets
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, widgetID).Scan(&widgetType)
	if err != nil {
		writeErr(w, http.StatusNotFound, "WIDGET_NOT_FOUND", "widget not found")
		return
	}

	// Dispatch to the appropriate data fetcher.
	switch widgetType {
	case WidgetCashBalance:
		service.fetchCashBalanceData(w, r, tenantID)
	case WidgetPLSnapshot:
		service.fetchPLSnapshotData(w, r, tenantID)
	case WidgetARAging:
		service.fetchARAgingData(w, r, tenantID)
	case WidgetAPAging:
		service.fetchAPAgingData(w, r, tenantID)
	case WidgetLowStock:
		service.fetchLowStockData(w, r, tenantID)
	case WidgetRecentTxns:
		service.fetchRecentTxnsData(w, r, tenantID)
	case WidgetPeriodStatus:
		service.fetchPeriodStatusData(w, r, tenantID)
	case WidgetOutstandingInvoices:
		service.fetchOutstandingInvoicesData(w, r, tenantID)
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"widget_type": widgetType,
			"message":     "data source not yet implemented for this widget type",
		})
	}
}

func (service *Service) fetchCashBalanceData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	var cashInflow, cashOutflow, cashBalance int64
	_ = service.pool.QueryRow(r.Context(), `
		SELECT
		  COALESCE(SUM(CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN jl.credit_cents > 0 THEN jl.credit_cents ELSE 0 END), 0),
		  COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.account_type IN ('CASH', 'BANK')
	`, tenantID).Scan(&cashInflow, &cashOutflow, &cashBalance)

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type":   WidgetCashBalance,
		"inflow_cents":  cashInflow,
		"outflow_cents": cashOutflow,
		"balance_cents": cashBalance,
	})
}

func (service *Service) fetchPLSnapshotData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	var revenue, expense int64
	_ = service.pool.QueryRow(r.Context(), `
		SELECT
		  COALESCE(SUM(CASE WHEN a.report_group = 'revenue' THEN jl.credit_cents - jl.debit_cents ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN a.report_group = 'expense' THEN jl.debit_cents - jl.credit_cents ELSE 0 END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')
	`, tenantID).Scan(&revenue, &expense)

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type":   WidgetPLSnapshot,
		"revenue_cents": revenue,
		"expense_cents": expense,
		"profit_cents":  revenue - expense,
	})
}

func (service *Service) fetchARAgingData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	rows, err := service.pool.Query(r.Context(), `
		SELECT bucket, COALESCE(SUM(amount_cents), 0) as total
		FROM (
			SELECT
			  CASE
			    WHEN COALESCE(invoice_age_days(i.invoice_date, CURRENT_DATE), 0) <= 0 THEN 'current'
			    WHEN COALESCE(invoice_age_days(i.invoice_date, CURRENT_DATE), 0) <= 30 THEN '1-30'
			    WHEN COALESCE(invoice_age_days(i.invoice_date, CURRENT_DATE), 0) <= 60 THEN '31-60'
			    WHEN COALESCE(invoice_age_days(i.invoice_date, CURRENT_DATE), 0) <= 90 THEN '61-90'
			    ELSE '90+'
			  END AS bucket,
			  i.receivable_cents AS amount_cents
			FROM invoices i
			WHERE i.tenant_id = $1 AND i.status IN ('POSTED', 'PARTIALLY_PAID')
			  AND i.receivable_cents > 0
		) sub
		GROUP BY bucket
	`, tenantID)
	if err != nil {
		// Fallback: simple count if function doesn't exist yet.
		var totalReceivable int64
		_ = service.pool.QueryRow(r.Context(), `
			SELECT COALESCE(SUM(receivable_cents), 0)
			FROM invoices
			WHERE tenant_id = $1 AND status IN ('POSTED', 'PARTIALLY_PAID')
			  AND receivable_cents > 0
		`, tenantID).Scan(&totalReceivable)
		writeJSON(w, http.StatusOK, map[string]any{
			"widget_type":      WidgetARAging,
			"total_cents":      totalReceivable,
			"buckets":          []any{},
		})
		return
	}
	defer rows.Close()

	buckets := []map[string]any{}
	for rows.Next() {
		var bucket string
		var total int64
		if err := rows.Scan(&bucket, &total); err != nil {
			writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
			return
		}
		buckets = append(buckets, map[string]any{"bucket": bucket, "total_cents": total})
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type": WidgetARAging,
		"buckets":     buckets,
	})
}

func (service *Service) fetchAPAgingData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	var totalPayable int64
	_ = service.pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(receivable_cents), 0)
		FROM supplier_invoices
		WHERE tenant_id = $1 AND status IN ('POSTED', 'PARTIALLY_PAID')
		  AND receivable_cents > 0
	`, tenantID).Scan(&totalPayable)

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type":  WidgetAPAging,
		"total_cents":  totalPayable,
	})
}

func (service *Service) fetchLowStockData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	rows, err := service.pool.Query(r.Context(), `
		SELECT i.code, i.name, sb.qty_on_hand, i.min_stock_qty
		FROM items i
		LEFT JOIN stock_balances sb ON sb.tenant_id = i.tenant_id AND sb.item_id = i.id
		WHERE i.tenant_id = $1 AND i.is_tracked_stock = true
		  AND COALESCE(sb.qty_on_hand, 0) <= COALESCE(i.min_stock_qty, 0)
		  AND i.min_stock_qty > 0
		ORDER BY i.code
		LIMIT 10
	`, tenantID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"widget_type": WidgetLowStock,
			"items":       []any{},
		})
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var code, name string
		var onHand, minStock float64
		if err := rows.Scan(&code, &name, &onHand, &minStock); err != nil {
			writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
			return
		}
		items = append(items, map[string]any{
			"code":          code,
			"name":          name,
			"qty_on_hand":   onHand,
			"min_stock_qty": minStock,
		})
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type": WidgetLowStock,
		"items":       items,
	})
}

func (service *Service) fetchRecentTxnsData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	rows, err := service.pool.Query(r.Context(), `
		SELECT je.number, je.entry_date::text, je.description, je.intent_type, je.status
		FROM journal_entries je
		WHERE je.tenant_id = $1
		ORDER BY je.created_at DESC
		LIMIT 10
	`, tenantID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"widget_type":  WidgetRecentTxns,
			"transactions": []any{},
		})
		return
	}
	defer rows.Close()

	txns := []map[string]any{}
	for rows.Next() {
		var number, entryDate, description, intentType, status string
		if err := rows.Scan(&number, &entryDate, &description, &intentType, &status); err != nil {
			writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
			return
		}
		txns = append(txns, map[string]any{
			"number":      number,
			"entry_date":  entryDate,
			"description": description,
			"intent_type": intentType,
			"status":      status,
		})
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type":  WidgetRecentTxns,
		"transactions": txns,
	})
}

func (service *Service) fetchPeriodStatusData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	var status, periodStart, periodEnd string
	var periodID int64
	_ = service.pool.QueryRow(r.Context(), `
		SELECT id, status, period_start::text, period_end::text
		FROM accounting_periods
		WHERE tenant_id = $1 AND status IN ('OPEN', 'REOPENED')
		ORDER BY period_start DESC LIMIT 1
	`, tenantID).Scan(&periodID, &status, &periodStart, &periodEnd)

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type":  WidgetPeriodStatus,
		"period_id":    periodID,
		"status":       status,
		"period_start": periodStart,
		"period_end":   periodEnd,
	})
}

func (service *Service) fetchOutstandingInvoicesData(w http.ResponseWriter, r *http.Request, tenantID int64) {
	rows, err := service.pool.Query(r.Context(), `
		SELECT i.number, c.name, i.invoice_date::text, i.due_date::text,
		       i.receivable_cents, i.status
		FROM invoices i
		JOIN customers c ON c.tenant_id = i.tenant_id AND c.id = i.customer_id
		WHERE i.tenant_id = $1 AND i.status IN ('POSTED', 'PARTIALLY_PAID')
		  AND i.receivable_cents > 0
		ORDER BY i.due_date ASC
		LIMIT 10
	`, tenantID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"widget_type": WidgetOutstandingInvoices,
			"invoices":    []any{},
		})
		return
	}
	defer rows.Close()

	invoices := []map[string]any{}
	for rows.Next() {
		var number, customerName, invoiceDate, dueDate, status string
		var receivableCents int64
		if err := rows.Scan(&number, &customerName, &invoiceDate, &dueDate, &receivableCents, &status); err != nil {
			writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
			return
		}
		invoices = append(invoices, map[string]any{
			"number":            number,
			"customer_name":     customerName,
			"invoice_date":      invoiceDate,
			"due_date":          dueDate,
			"receivable_cents":  receivableCents,
			"status":            status,
		})
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "DASHBOARD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type": WidgetOutstandingInvoices,
		"invoices":    invoices,
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type errResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errResponse{Code: code, Message: message})
}

// Keep time imported (used in future scheduled widget refresh).
var _ = time.Now
