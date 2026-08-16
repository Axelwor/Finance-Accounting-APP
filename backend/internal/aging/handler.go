package aging

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

// ---------------------------------------------------------------------------
// F-04/F-05: AR/AP Aging & Collection Management
//   Provides aging buckets (current, 1-30, 31-60, 61-90, 90+ days) for
//   receivables and payables, with customer/supplier breakdown.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Get("/aging/ar", service.ARAging)
	router.Get("/aging/ap", service.APAging)
}

type agingRow struct {
	PartyID          int64  `json:"party_id"`
	PartyName        string `json:"party_name"`
	InvoiceNumber    string `json:"invoice_number"`
	InvoiceDate      string `json:"invoice_date"`
	DueDate          string `json:"due_date"`
	OutstandingCents int64  `json:"outstanding_cents"`
	Bucket           string `json:"bucket"`
	DaysOverdue      int    `json:"days_overdue"`
}

type agingSummary struct {
	AsOfDate          string     `json:"as_of_date"`
	TotalCents        int64      `json:"total_cents"`
	CurrentCents      int64      `json:"current_cents"`
	Bucket130Cents    int64      `json:"bucket_1_30_cents"`
	Bucket3160Cents   int64      `json:"bucket_31_60_cents"`
	Bucket6190Cents   int64      `json:"bucket_61_90_cents"`
	Bucket90PlusCents int64      `json:"bucket_90_plus_cents"`
	Rows              []agingRow `json:"rows"`
}

func (service *Service) ARAging(writer http.ResponseWriter, request *http.Request) {
	asOf := parseAsOfDate(request)
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenantID <= 0 {
		writeJSON(writer, http.StatusUnauthorized, errorBody{"TENANT_REQUIRED", "tenant context is required"})
		return
	}

	rows, err := fetchARAging(request.Context(), service.pool, tenantID, asOf)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorBody{"QUERY_FAILED", err.Error()})
		return
	}

	summary := buildAgingSummary(rows, asOf)
	writeJSON(writer, http.StatusOK, summary)
}

func (service *Service) APAging(writer http.ResponseWriter, request *http.Request) {
	asOf := parseAsOfDate(request)
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenantID <= 0 {
		writeJSON(writer, http.StatusUnauthorized, errorBody{"TENANT_REQUIRED", "tenant context is required"})
		return
	}

	rows, err := fetchAPAging(request.Context(), service.pool, tenantID, asOf)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorBody{"QUERY_FAILED", err.Error()})
		return
	}

	summary := buildAgingSummary(rows, asOf)
	writeJSON(writer, http.StatusOK, summary)
}

func fetchARAging(ctx context.Context, pool *pgxpool.Pool, tenantID int64, asOf time.Time) ([]agingRow, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10)); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT i.id, COALESCE(c.name, ''), i.number, i.invoice_date, i.due_date,
		       i.receivable_cents
		FROM invoices i
		LEFT JOIN customers c ON c.tenant_id = i.tenant_id AND c.id = i.customer_id
		WHERE i.tenant_id = $1 AND i.status IN ('ISSUED', 'PARTIALLY_PAID') AND i.receivable_cents > 0
		  AND i.invoice_date <= $2
		ORDER BY i.due_date
	`, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []agingRow
	for rows.Next() {
		var r agingRow
		var invoiceDate, dueDate time.Time
		if err := rows.Scan(&r.PartyID, &r.PartyName, &r.InvoiceNumber, &invoiceDate, &dueDate, &r.OutstandingCents); err != nil {
			return nil, err
		}
		r.InvoiceDate = invoiceDate.Format("2006-01-02")
		r.DueDate = dueDate.Format("2006-01-02")
		r.DaysOverdue = int(asOf.Sub(dueDate).Hours() / 24)
		r.Bucket = classifyBucket(r.DaysOverdue)
		result = append(result, r)
	}
	return result, rows.Err()
}

func fetchAPAging(ctx context.Context, pool *pgxpool.Pool, tenantID int64, asOf time.Time) ([]agingRow, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10)); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT si.id, COALESCE(s.name, ''), si.supplier_invoice_number, si.invoice_date, si.due_date,
		       si.payable_cents
		FROM supplier_invoices si
		LEFT JOIN suppliers s ON s.tenant_id = si.tenant_id AND s.id = si.supplier_id
		WHERE si.tenant_id = $1 AND si.status IN ('ISSUED', 'PARTIALLY_PAID') AND si.payable_cents > 0
		  AND si.invoice_date <= $2
		ORDER BY si.due_date
	`, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []agingRow
	for rows.Next() {
		var r agingRow
		var invoiceDate, dueDate time.Time
		if err := rows.Scan(&r.PartyID, &r.PartyName, &r.InvoiceNumber, &invoiceDate, &dueDate, &r.OutstandingCents); err != nil {
			return nil, err
		}
		r.InvoiceDate = invoiceDate.Format("2006-01-02")
		r.DueDate = dueDate.Format("2006-01-02")
		r.DaysOverdue = int(asOf.Sub(dueDate).Hours() / 24)
		r.Bucket = classifyBucket(r.DaysOverdue)
		result = append(result, r)
	}
	return result, rows.Err()
}

// classifyBucket maps days overdue to a bucket label.
func classifyBucket(daysOverdue int) string {
	switch {
	case daysOverdue <= 0:
		return "current"
	case daysOverdue <= 30:
		return "1-30"
	case daysOverdue <= 60:
		return "31-60"
	case daysOverdue <= 90:
		return "61-90"
	default:
		return "90+"
	}
}

func buildAgingSummary(rows []agingRow, asOf time.Time) agingSummary {
	s := agingSummary{
		AsOfDate: asOf.Format("2006-01-02"),
		Rows:     rows,
	}
	if s.Rows == nil {
		s.Rows = []agingRow{}
	}
	for _, r := range rows {
		s.TotalCents += r.OutstandingCents
		switch r.Bucket {
		case "current":
			s.CurrentCents += r.OutstandingCents
		case "1-30":
			s.Bucket130Cents += r.OutstandingCents
		case "31-60":
			s.Bucket3160Cents += r.OutstandingCents
		case "61-90":
			s.Bucket6190Cents += r.OutstandingCents
		case "90+":
			s.Bucket90PlusCents += r.OutstandingCents
		}
	}
	return s
}

func parseAsOfDate(request *http.Request) time.Time {
	raw := request.URL.Query().Get("as_of")
	if raw == "" {
		return time.Now()
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Now()
	}
	return parsed
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

// Ensure pgx is imported (used by pool.Query in fetch functions)
var _ = pgx.ErrNoRows
