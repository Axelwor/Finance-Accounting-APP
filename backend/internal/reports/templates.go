package reports

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// =====================================================================
// REPORT TEMPLATES (N-01..N-10: NextReport integration layer)
// =====================================================================

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// =====================================================================
// TEMPLATE CRUD
// =====================================================================

func (s *Service) ListTemplates(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, code, name, document_type, is_default, is_active
		FROM report_templates WHERE tenant_id = $1
		ORDER BY document_type, name
	`, tid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"QUERY_FAILED", err.Error()})
		return
	}
	defer rows.Close()
	type tmpl struct {
		ID           int64  `json:"id"`
		Code         string `json:"code"`
		Name         string `json:"name"`
		DocumentType string `json:"document_type"`
		IsDefault    bool   `json:"is_default"`
		IsActive     bool   `json:"is_active"`
	}
	result := []tmpl{}
	for rows.Next() {
		var t tmpl
		if err := rows.Scan(&t.ID, &t.Code, &t.Name, &t.DocumentType, &t.IsDefault, &t.IsActive); err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp{"SCAN_FAILED", err.Error()})
			return
		}
		result = append(result, t)
	}
	writeJSON(w, http.StatusOK, result)
}

type CreateTemplateRequest struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	DocumentType string `json:"document_type"`
	TemplateYAML string `json:"template_yaml"`
	IsDefault    bool   `json:"is_default"`
}

// validateTemplateRequest checks that all required template fields are present.
// Pure: no DB or HTTP coupling. Returns true when valid.
func validateTemplateRequest(req CreateTemplateRequest) bool {
	return req.Code != "" && req.Name != "" && req.DocumentType != "" && req.TemplateYAML != ""
}

func (s *Service) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{"INVALID_REQUEST", err.Error()})
		return
	}
	if !validateTemplateRequest(req) {
		writeJSON(w, http.StatusBadRequest, errResp{"INVALID_REQUEST", "code, name, document_type, and template_yaml are required"})
		return
	}
	var id int64
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO report_templates (tenant_id, code, name, document_type, template_yaml, is_default, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, tid, req.Code, req.Name, req.DocumentType, req.TemplateYAML, req.IsDefault, uid).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusConflict, errResp{"CREATE_FAILED", err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "code": req.Code})
}

func (s *Service) GetTemplate(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	var code, name, docType, yaml string
	var isDefault, isActive bool
	err := s.pool.QueryRow(r.Context(), `
		SELECT code, name, document_type, template_yaml, is_default, is_active
		FROM report_templates WHERE tenant_id = $1 AND id = $2
	`, tid, id).Scan(&code, &name, &docType, &yaml, &isDefault, &isActive)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp{"NOT_FOUND", "template not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "code": code, "name": name, "document_type": docType,
		"template_yaml": yaml, "is_default": isDefault, "is_active": isActive,
	})
}

func (s *Service) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{"INVALID_REQUEST", err.Error()})
		return
	}
	_, err := s.pool.Exec(r.Context(), `
		UPDATE report_templates SET code=$3, name=$4, document_type=$5, template_yaml=$6, is_default=$7, updated_at=now()
		WHERE tenant_id=$1 AND id=$2
	`, tid, id, req.Code, req.Name, req.DocumentType, req.TemplateYAML, req.IsDefault)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"UPDATE_FAILED", err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "updated"})
}

func (s *Service) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	_, err := s.pool.Exec(r.Context(), `DELETE FROM report_templates WHERE tenant_id=$1 AND id=$2`, tid, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"DELETE_FAILED", err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "deleted"})
}

// =====================================================================
// RENDER — Proxy to NextReport Engine sidecar
// =====================================================================

func (s *Service) RenderReport(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "pdf"
	}
	var yaml string
	err := s.pool.QueryRow(r.Context(), `SELECT template_yaml FROM report_templates WHERE tenant_id=$1 AND id=$2`, tid, id).Scan(&yaml)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp{"NOT_FOUND", "template not found"})
		return
	}
	endpoint := os.Getenv("NEXTREPORT_URL")
	if endpoint == "" {
		endpoint = "http://localhost:3100"
	}
	body, _ := json.Marshal(map[string]string{"template_yaml": yaml, "format": format})
	resp, err := http.Post(endpoint+"/render", "application/json", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errResp{"RENDER_FAILED", "NextReport engine unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// =====================================================================
// DASHBOARD LAYOUT & WIDGETS
// =====================================================================

func (s *Service) GetLayout(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	layoutID, err := s.ensureLayout(r.Context(), tid, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"LAYOUT_FAILED", err.Error()})
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, widget_type, title, config_json, grid_x, grid_y, grid_w, grid_h
		FROM dashboard_widgets WHERE tenant_id=$1 AND layout_id=$2
		ORDER BY grid_y, grid_x
	`, tid, layoutID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"QUERY_FAILED", err.Error()})
		return
	}
	defer rows.Close()
	type widget struct {
		ID         int64           `json:"id"`
		WidgetType string          `json:"widget_type"`
		Title      string          `json:"title"`
		Config     json.RawMessage `json:"config"`
		GridX      int             `json:"grid_x"`
		GridY      int             `json:"grid_y"`
		GridW      int             `json:"grid_w"`
		GridH      int             `json:"grid_h"`
	}
	widgets := []widget{}
	for rows.Next() {
		var wt widget
		var cfg []byte
		if err := rows.Scan(&wt.ID, &wt.WidgetType, &wt.Title, &cfg, &wt.GridX, &wt.GridY, &wt.GridW, &wt.GridH); err != nil {
			continue
		}
		wt.Config = cfg
		widgets = append(widgets, wt)
	}
	writeJSON(w, http.StatusOK, map[string]any{"layout_id": layoutID, "widgets": widgets})
}

func (s *Service) SaveLayout(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "Default"
	}
	layoutID, err := s.ensureLayout(r.Context(), tid, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"LAYOUT_FAILED", err.Error()})
		return
	}
	_, _ = s.pool.Exec(r.Context(), `UPDATE dashboard_layouts SET name=$3 WHERE id=$1 AND tenant_id=$2`, layoutID, tid, req.Name)
	writeJSON(w, http.StatusOK, map[string]any{"layout_id": layoutID, "message": "saved"})
}

type AddWidgetRequest struct {
	WidgetType string          `json:"widget_type"`
	Title      string          `json:"title"`
	Config     json.RawMessage `json:"config"`
	GridX      int             `json:"grid_x"`
	GridY      int             `json:"grid_y"`
	GridW      int             `json:"grid_w"`
	GridH      int             `json:"grid_h"`
}

func (s *Service) AddWidget(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	var req AddWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{"INVALID_REQUEST", err.Error()})
		return
	}
	if req.WidgetType == "" {
		writeJSON(w, http.StatusBadRequest, errResp{"INVALID_REQUEST", "widget_type is required"})
		return
	}
	layoutID, err := s.ensureLayout(r.Context(), tid, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"LAYOUT_FAILED", err.Error()})
		return
	}
	var id int64
	err = s.pool.QueryRow(r.Context(), `
		INSERT INTO dashboard_widgets (tenant_id, layout_id, widget_type, title, config_json, grid_x, grid_y, grid_w, grid_h)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id
	`, tid, layoutID, req.WidgetType, req.Title, []byte(req.Config), req.GridX, req.GridY, req.GridW, req.GridH).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"ADD_FAILED", err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Service) UpdateWidget(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	var req AddWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{"INVALID_REQUEST", err.Error()})
		return
	}
	_, err := s.pool.Exec(r.Context(), `
		UPDATE dashboard_widgets SET widget_type=$3, title=$4, config_json=$5, grid_x=$6, grid_y=$7, grid_w=$8, grid_h=$9
		WHERE tenant_id=$1 AND id=$2
	`, tid, id, req.WidgetType, req.Title, []byte(req.Config), req.GridX, req.GridY, req.GridW, req.GridH)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"UPDATE_FAILED", err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "updated"})
}

func (s *Service) DeleteWidget(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	_, err := s.pool.Exec(r.Context(), `DELETE FROM dashboard_widgets WHERE tenant_id=$1 AND id=$2`, tid, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp{"DELETE_FAILED", err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "deleted"})
}

// =====================================================================
// WIDGET DATA — fetch live data per widget type
// =====================================================================

func (s *Service) ListWidgets(w http.ResponseWriter, r *http.Request) {
	wtypes := []map[string]any{
		{"type": "kpi_cash", "title": "Cash Balance", "default_w": 3, "default_h": 2},
		{"type": "kpi_ar", "title": "AR Outstanding", "default_w": 3, "default_h": 2},
		{"type": "kpi_ap", "title": "AP Outstanding", "default_w": 3, "default_h": 2},
		{"type": "kpi_pl", "title": "Profit/Loss", "default_w": 3, "default_h": 2},
		{"type": "kpi_low_stock", "title": "Low Stock Alert", "default_w": 4, "default_h": 3},
		{"type": "recent_transactions", "title": "Recent Transactions", "default_w": 6, "default_h": 4},
		{"type": "bank_balance", "title": "Bank Balance", "default_w": 6, "default_h": 4},
		{"type": "ar_aging", "title": "AR Aging Summary", "default_w": 6, "default_h": 4},
		{"type": "ap_aging", "title": "AP Aging Summary", "default_w": 6, "default_h": 4},
		{"type": "cash_flow_forecast", "title": "Cash Flow Forecast", "default_w": 12, "default_h": 4},
		{"type": "budget_vs_actual", "title": "Budget vs Actual", "default_w": 6, "default_h": 4},
		{"type": "revenue_by_customer", "title": "Revenue by Customer", "default_w": 6, "default_h": 4},
	}
	writeJSON(w, http.StatusOK, wtypes)
}

func (s *Service) GetWidgetData(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errResp{"TENANT_REQUIRED", "tenant context is required"})
		return
	}
	wtype := chi.URLParam(r, "id")
	switch wtype {
	case "kpi_cash":
		s.widgetKPICash(w, r, tid)
	case "kpi_ar":
		s.widgetKPIAR(w, r, tid)
	case "kpi_ap":
		s.widgetKPIAP(w, r, tid)
	case "kpi_pl":
		s.widgetKPIPL(w, r, tid)
	case "kpi_low_stock":
		s.widgetKPILowStock(w, r, tid)
	case "recent_transactions":
		s.widgetRecentTransactions(w, r, tid)
	case "bank_balance":
		s.widgetBankBalance(w, r, tid)
	default:
		writeJSON(w, http.StatusBadRequest, errResp{"UNKNOWN_WIDGET", "unknown widget type: " + wtype})
	}
}

func (s *Service) widgetKPICash(w http.ResponseWriter, r *http.Request, tid int64) {
	var cash int64
	_ = s.pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.entry_id AND je.tenant_id = jl.tenant_id
		JOIN accounts a ON a.id = jl.account_id AND a.tenant_id = jl.tenant_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED' AND a.account_type IN ('CASH', 'BANK')
	`, tid).Scan(&cash)
	writeJSON(w, http.StatusOK, map[string]any{"widget_type": "kpi_cash", "cash_cents": cash})
}

func (s *Service) widgetKPIAR(w http.ResponseWriter, r *http.Request, tid int64) {
	var ar int64
	_ = s.pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.entry_id AND je.tenant_id = jl.tenant_id
		JOIN accounts a ON a.id = jl.account_id AND a.tenant_id = jl.tenant_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED' AND a.account_type = 'ACCOUNTS_RECEIVABLE'
	`, tid).Scan(&ar)
	writeJSON(w, http.StatusOK, map[string]any{"widget_type": "kpi_ar", "ar_cents": ar})
}

func (s *Service) widgetKPIAP(w http.ResponseWriter, r *http.Request, tid int64) {
	var ap int64
	_ = s.pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.entry_id AND je.tenant_id = jl.tenant_id
		JOIN accounts a ON a.id = jl.account_id AND a.tenant_id = jl.tenant_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED' AND a.account_type = 'ACCOUNTS_PAYABLE'
	`, tid).Scan(&ap)
	writeJSON(w, http.StatusOK, map[string]any{"widget_type": "kpi_ap", "ap_cents": ap})
}

func (s *Service) widgetKPIPL(w http.ResponseWriter, r *http.Request, tid int64) {
	var rev, exp int64
	_ = s.pool.QueryRow(r.Context(), `
		SELECT
		  COALESCE(SUM(CASE WHEN a.report_group='revenue' THEN jl.credit_cents - jl.debit_cents ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN a.report_group='expense' THEN jl.debit_cents - jl.credit_cents ELSE 0 END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.entry_id AND je.tenant_id = jl.tenant_id
		JOIN accounts a ON a.id = jl.account_id AND a.tenant_id = jl.tenant_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED' AND a.report_group IN ('revenue', 'expense')
	`, tid).Scan(&rev, &exp)
	writeJSON(w, http.StatusOK, map[string]any{
		"widget_type":   "kpi_pl",
		"revenue_cents": rev,
		"expense_cents": exp,
		"profit_cents":  rev - exp,
	})
}

func (s *Service) widgetKPILowStock(w http.ResponseWriter, r *http.Request, tid int64) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT i.code, i.name, COALESCE(sb.qty_on_hand, 0), i.min_stock_qty
		FROM items i
		LEFT JOIN stock_balances sb ON sb.tenant_id = i.tenant_id AND sb.item_id = i.id
		WHERE i.tenant_id = $1 AND i.is_tracked_stock = true
		  AND COALESCE(sb.qty_on_hand, 0) <= i.min_stock_qty
		LIMIT 20
	`, tid)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"widget_type": "kpi_low_stock", "items": []any{}})
		return
	}
	defer rows.Close()
	type item struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		OnHand   float64 `json:"on_hand"`
		MinStock float64 `json:"min_stock"`
	}
	items := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.Code, &it.Name, &it.OnHand, &it.MinStock); err != nil {
			continue
		}
		items = append(items, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"widget_type": "kpi_low_stock", "items": items})
}

func (s *Service) widgetRecentTransactions(w http.ResponseWriter, r *http.Request, tid int64) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT je.number, je.entry_date, je.description, je.source_ref, je.intent_type,
		       (SELECT SUM(jl.debit_cents) FROM journal_lines jl WHERE jl.entry_id = je.id) AS total_debit
		FROM journal_entries je
		WHERE je.tenant_id = $1 AND je.status = 'POSTED'
		ORDER BY je.created_at DESC LIMIT 10
	`, tid)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"widget_type": "recent_transactions", "transactions": []any{}})
		return
	}
	defer rows.Close()
	type txn struct {
		Number      string `json:"number"`
		EntryDate   string `json:"entry_date"`
		Description string `json:"description"`
		SourceRef   string `json:"source_ref"`
		IntentType  string `json:"intent_type"`
		TotalCents  int64  `json:"total_cents"`
	}
	txns := []txn{}
	for rows.Next() {
		var t txn
		if err := rows.Scan(&t.Number, &t.EntryDate, &t.Description, &t.SourceRef, &t.IntentType, &t.TotalCents); err != nil {
			continue
		}
		txns = append(txns, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"widget_type": "recent_transactions", "transactions": txns})
}

func (s *Service) widgetBankBalance(w http.ResponseWriter, r *http.Request, tid int64) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT a.code, a.name, COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM accounts a
		LEFT JOIN journal_lines jl ON jl.account_id = a.id AND jl.tenant_id = a.tenant_id
		LEFT JOIN journal_entries je ON je.id = jl.entry_id AND je.status = 'POSTED'
		WHERE a.tenant_id = $1 AND a.account_type IN ('CASH', 'BANK') AND a.is_group = false
		GROUP BY a.code, a.name ORDER BY a.code
	`, tid)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"widget_type": "bank_balance", "accounts": []any{}})
		return
	}
	defer rows.Close()
	type acct struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		BalCents int64  `json:"balance_cents"`
	}
	banks := []acct{}
	for rows.Next() {
		var b acct
		if err := rows.Scan(&b.Code, &b.Name, &b.BalCents); err != nil {
			continue
		}
		banks = append(banks, b)
	}
	writeJSON(w, http.StatusOK, map[string]any{"widget_type": "bank_balance", "accounts": banks})
}

// =====================================================================
// HELPERS
// =====================================================================

func (s *Service) ensureLayout(ctx context.Context, tid, uid int64) (int64, error) {
	var layoutID int64
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM dashboard_layouts WHERE tenant_id = $1 AND user_id = $2 AND is_active = true LIMIT 1
	`, tid, uid).Scan(&layoutID)
	if err == nil {
		return layoutID, nil
	}
	if !isNoRows(err) {
		return 0, err
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO dashboard_layouts (tenant_id, user_id, name, is_active)
		VALUES ($1, $2, 'Default', true) RETURNING id
	`, tid, uid).Scan(&layoutID)
	return layoutID, err
}

func isNoRows(err error) bool {
	return err == pgx.ErrNoRows
}

func pathID(raw string) int64 {
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

type errResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// Ensure db import is used
var _ = db.WithTransaction
