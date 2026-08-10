package customer

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestBuildStatementLines(t *testing.T) {
	tests := []struct {
		name        string
		invoices    []statementRow
		payments    []statementRow
		opening     int64
		wantLines   []StatementLine
		wantInvoice int64
		wantPaid    int64
	}{
		{
			name:        "empty inputs",
			invoices:    nil,
			payments:    nil,
			opening:     1500,
			wantLines:   []StatementLine{},
			wantInvoice: 0,
			wantPaid:    0,
		},
		{
			name: "single invoice",
			invoices: []statementRow{
				{ID: 1, Date: "2026-01-05", Type: "invoice", Reference: "INV-001", Description: "Sale", DebitCents: 10000},
			},
			opening: 0,
			wantLines: []StatementLine{
				{Date: "2026-01-05", Type: "invoice", Reference: "INV-001", Description: "Sale", DebitCents: 10000, CreditCents: 0, RunningBalanceCents: 10000},
			},
			wantInvoice: 10000,
			wantPaid:    0,
		},
		{
			name: "running balance and totals",
			invoices: []statementRow{
				{ID: 1, Date: "2026-01-05", Type: "invoice", Reference: "INV-001", DebitCents: 10000},
				{ID: 2, Date: "2026-01-10", Type: "invoice", Reference: "INV-002", DebitCents: 5000},
			},
			payments: []statementRow{
				{ID: 3, Date: "2026-01-08", Type: "payment", Reference: "PAY-001", CreditCents: 4000},
			},
			opening: 2000,
			wantLines: []StatementLine{
				{Date: "2026-01-05", Type: "invoice", Reference: "INV-001", DebitCents: 10000, CreditCents: 0, RunningBalanceCents: 12000},
				{Date: "2026-01-08", Type: "payment", Reference: "PAY-001", DebitCents: 0, CreditCents: 4000, RunningBalanceCents: 8000},
				{Date: "2026-01-10", Type: "invoice", Reference: "INV-002", DebitCents: 5000, CreditCents: 0, RunningBalanceCents: 13000},
			},
			wantInvoice: 15000,
			wantPaid:    4000,
		},
		{
			name: "same date sorted by id",
			invoices: []statementRow{
				{ID: 9, Date: "2026-02-01", Type: "invoice", Reference: "INV-009", DebitCents: 300},
			},
			payments: []statementRow{
				{ID: 5, Date: "2026-02-01", Type: "payment", Reference: "PAY-005", CreditCents: 100},
				{ID: 7, Date: "2026-02-01", Type: "payment", Reference: "PAY-007", CreditCents: 50},
			},
			opening: 0,
			wantLines: []StatementLine{
				{Date: "2026-02-01", Type: "payment", Reference: "PAY-005", DebitCents: 0, CreditCents: 100, RunningBalanceCents: -100},
				{Date: "2026-02-01", Type: "payment", Reference: "PAY-007", DebitCents: 0, CreditCents: 50, RunningBalanceCents: -150},
				{Date: "2026-02-01", Type: "invoice", Reference: "INV-009", DebitCents: 300, CreditCents: 0, RunningBalanceCents: 150},
			},
			wantInvoice: 300,
			wantPaid:    150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, invoiced, paid := buildStatementLines(tt.invoices, tt.payments, tt.opening)
			if invoiced != tt.wantInvoice {
				t.Errorf("invoiced = %d, want %d", invoiced, tt.wantInvoice)
			}
			if paid != tt.wantPaid {
				t.Errorf("paid = %d, want %d", paid, tt.wantPaid)
			}
			if len(lines) != len(tt.wantLines) {
				t.Fatalf("len(lines) = %d, want %d", len(lines), len(tt.wantLines))
			}
			for i := range lines {
				if lines[i] != tt.wantLines[i] {
					t.Errorf("line %d = %+v, want %+v", i, lines[i], tt.wantLines[i])
				}
			}
		})
	}
}

func TestValidateStatementDates(t *testing.T) {
	tests := []struct {
		name     string
		fromDate string
		toDate   string
		wantOK   bool
	}{
		{"valid range", "2026-01-01", "2026-01-31", true},
		{"same day", "2026-01-01", "2026-01-01", true},
		{"missing from", "", "2026-01-31", false},
		{"missing to", "2026-01-01", "", false},
		{"bad from format", "2026/01/01", "2026-01-31", false},
		{"bad to value", "2026-01-01", "2026-13-40", false},
		{"from after to", "2026-02-01", "2026-01-01", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _ := validateStatementDates(tt.fromDate, tt.toDate)
			if got := code == ""; got != tt.wantOK {
				t.Errorf("validateStatementDates(%q, %q) ok = %v, want %v (code=%q)", tt.fromDate, tt.toDate, got, tt.wantOK, code)
			}
		})
	}
}

func TestStatementRouteRegistered(t *testing.T) {
	service := newTestService()
	router := chi.NewRouter()
	router.Route("/api/v1", service.Routes)

	found := false
	_ = chi.Walk(router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if method == "GET" && route == "/api/v1/customers/{id}/statement" {
			found = true
		}
		return nil
	})
	if !found {
		t.Error("expected route GET /api/v1/customers/{id}/statement to be registered")
	}
}
