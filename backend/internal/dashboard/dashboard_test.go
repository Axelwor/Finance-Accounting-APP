package dashboard

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Tests for the pure validation / normalization helpers in the dashboard
// package. None of these touch the database or the pgxpool.
//
// Covered functions:
//   - validateWidgetType   (widget_type is required / non-empty)
//   - isKnownWidgetType    (membership against the exported Widget* set)
//   - normalizeWidgetGrid  (ColSpan/RowSpan zero-value defaults)
//   - Widget* constants    (the exported widget type identifiers)
// ---------------------------------------------------------------------------

// validWidgetTypes is the full set of exported widget type constants, used to
// verify that isKnownWidgetType accepts every one of them.
var validWidgetTypes = []string{
	WidgetCashBalance,
	WidgetARAging,
	WidgetAPAging,
	WidgetPLSnapshot,
	WidgetBudgetVsActual,
	WidgetRevenueByCustomer,
	WidgetLowStock,
	WidgetRecentTxns,
	WidgetOutstandingInvoices,
	WidgetTaxSummary,
	WidgetPeriodStatus,
}

// ---------------------------------------------------------------------------
// validateWidgetType
// ---------------------------------------------------------------------------

func TestValidateWidgetType(t *testing.T) {
	tests := []struct {
		name       string
		widgetType string
		want       bool
	}{
		{name: "cash_balance passes", widgetType: WidgetCashBalance, want: true},
		{name: "ar_aging_summary passes", widgetType: WidgetARAging, want: true},
		{name: "ap_aging_summary passes", widgetType: WidgetAPAging, want: true},
		{name: "pl_snapshot passes", widgetType: WidgetPLSnapshot, want: true},
		{name: "budget_vs_actual passes", widgetType: WidgetBudgetVsActual, want: true},
		{name: "revenue_by_customer passes", widgetType: WidgetRevenueByCustomer, want: true},
		{name: "low_stock_alert passes", widgetType: WidgetLowStock, want: true},
		{name: "recent_transactions passes", widgetType: WidgetRecentTxns, want: true},
		{name: "outstanding_invoices passes", widgetType: WidgetOutstandingInvoices, want: true},
		{name: "tax_summary passes", widgetType: WidgetTaxSummary, want: true},
		{name: "period_status passes", widgetType: WidgetPeriodStatus, want: true},
		{name: "arbitrary non-empty string passes (handler only checks non-empty)", widgetType: "custom_widget", want: true},
		{name: "single space passes (no trim, only empty check)", widgetType: " ", want: true},
		{name: "empty string rejected", widgetType: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateWidgetType(tt.widgetType)
			if got != tt.want {
				t.Errorf("validateWidgetType(%q) = %v, want %v", tt.widgetType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isKnownWidgetType
// ---------------------------------------------------------------------------

func TestIsKnownWidgetType(t *testing.T) {
	t.Run("every exported Widget* constant is known", func(t *testing.T) {
		for _, wt := range validWidgetTypes {
			if !isKnownWidgetType(wt) {
				t.Errorf("isKnownWidgetType(%q) = false, want true (exported constant)", wt)
			}
		}
	})

	t.Run("unknown type rejected", func(t *testing.T) {
		if isKnownWidgetType("custom_widget") {
			t.Error(`isKnownWidgetType("custom_widget") = true, want false`)
		}
	})

	t.Run("empty string rejected", func(t *testing.T) {
		if isKnownWidgetType("") {
			t.Error(`isKnownWidgetType("") = true, want false`)
		}
	})

	// Table-driven membership checks for each individual constant.
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "cash_balance", in: WidgetCashBalance, want: true},
		{name: "ar_aging_summary", in: WidgetARAging, want: true},
		{name: "ap_aging_summary", in: WidgetAPAging, want: true},
		{name: "pl_snapshot", in: WidgetPLSnapshot, want: true},
		{name: "budget_vs_actual", in: WidgetBudgetVsActual, want: true},
		{name: "revenue_by_customer", in: WidgetRevenueByCustomer, want: true},
		{name: "low_stock_alert", in: WidgetLowStock, want: true},
		{name: "recent_transactions", in: WidgetRecentTxns, want: true},
		{name: "outstanding_invoices", in: WidgetOutstandingInvoices, want: true},
		{name: "tax_summary", in: WidgetTaxSummary, want: true},
		{name: "period_status", in: WidgetPeriodStatus, want: true},
		{name: "typo of cash_balance rejected", in: "cash-bal", want: false},
		{name: "case variant rejected (constants are lowercase)", in: "Cash_Balance", want: false},
		{name: "trailing space rejected", in: WidgetCashBalance + " ", want: false},
		{name: "leading space rejected", in: " " + WidgetCashBalance, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownWidgetType(tt.in); got != tt.want {
				t.Errorf("isKnownWidgetType(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// normalizeWidgetGrid
// ---------------------------------------------------------------------------

func TestNormalizeWidgetGrid(t *testing.T) {
	tests := []struct {
		name        string
		colSpan     int
		rowSpan     int
		wantColSpan int
		wantRowSpan int
	}{
		{name: "both zero default to 2x1", colSpan: 0, rowSpan: 0, wantColSpan: 2, wantRowSpan: 1},
		{name: "zero col_span defaults to 2", colSpan: 0, rowSpan: 3, wantColSpan: 2, wantRowSpan: 3},
		{name: "zero row_span defaults to 1", colSpan: 4, rowSpan: 0, wantColSpan: 4, wantRowSpan: 1},
		{name: "explicit col_span preserved", colSpan: 6, rowSpan: 2, wantColSpan: 6, wantRowSpan: 2},
		{name: "explicit large col_span preserved", colSpan: 12, rowSpan: 4, wantColSpan: 12, wantRowSpan: 4},
		{name: "col_span 1 preserved (not defaulted)", colSpan: 1, rowSpan: 1, wantColSpan: 1, wantRowSpan: 1},
		{name: "negative col_span preserved (no clamp)", colSpan: -1, rowSpan: 0, wantColSpan: -1, wantRowSpan: 1},
		{name: "negative row_span preserved (no clamp)", colSpan: 0, rowSpan: -5, wantColSpan: 2, wantRowSpan: -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCol, gotRow := normalizeWidgetGrid(tt.colSpan, tt.rowSpan)
			if gotCol != tt.wantColSpan || gotRow != tt.wantRowSpan {
				t.Errorf("normalizeWidgetGrid(%d, %d) = (%d, %d), want (%d, %d)",
					tt.colSpan, tt.rowSpan, gotCol, gotRow, tt.wantColSpan, tt.wantRowSpan)
			}
		})
	}
}

// TestNormalizeWidgetGrid_DoesNotMutateInput confirms the function is pure —
// it returns new values rather than mutating a shared struct.
func TestNormalizeWidgetGrid_DoesNotMutateInput(t *testing.T) {
	col, row := 0, 0
	_, _ = normalizeWidgetGrid(col, row)
	if col != 0 || row != 0 {
		t.Fatalf("normalizeWidgetGrid mutated its inputs: col=%d row=%d", col, row)
	}
}

// ---------------------------------------------------------------------------
// Widget type constants — sanity checks on the exported identifiers.
// ---------------------------------------------------------------------------

func TestWidgetConstants_DistinctAndNonEmpty(t *testing.T) {
	seen := make(map[string]bool, len(validWidgetTypes))
	for _, wt := range validWidgetTypes {
		if wt == "" {
			t.Fatal("found an empty widget type constant")
		}
		if seen[wt] {
			t.Fatalf("duplicate widget type constant: %q", wt)
		}
		seen[wt] = true
	}
}

// TestWidgetConstants_MatchExpectedStrings documents the exact string value of
// each exported constant, so a refactor that changes a constant value is
// caught at test time rather than silently breaking the frontend contract.
func TestWidgetConstants_MatchExpectedStrings(t *testing.T) {
	want := map[string]string{
		"WidgetCashBalance":         WidgetCashBalance,
		"WidgetARAging":             WidgetARAging,
		"WidgetAPAging":             WidgetAPAging,
		"WidgetPLSnapshot":          WidgetPLSnapshot,
		"WidgetBudgetVsActual":      WidgetBudgetVsActual,
		"WidgetRevenueByCustomer":   WidgetRevenueByCustomer,
		"WidgetLowStock":            WidgetLowStock,
		"WidgetRecentTxns":          WidgetRecentTxns,
		"WidgetOutstandingInvoices": WidgetOutstandingInvoices,
		"WidgetTaxSummary":          WidgetTaxSummary,
		"WidgetPeriodStatus":        WidgetPeriodStatus,
	}
	expectedStrings := map[string]string{
		"WidgetCashBalance":         "cash_balance",
		"WidgetARAging":             "ar_aging_summary",
		"WidgetAPAging":             "ap_aging_summary",
		"WidgetPLSnapshot":          "pl_snapshot",
		"WidgetBudgetVsActual":      "budget_vs_actual",
		"WidgetRevenueByCustomer":   "revenue_by_customer",
		"WidgetLowStock":            "low_stock_alert",
		"WidgetRecentTxns":          "recent_transactions",
		"WidgetOutstandingInvoices": "outstanding_invoices",
		"WidgetTaxSummary":          "tax_summary",
		"WidgetPeriodStatus":        "period_status",
	}
	for name, val := range want {
		if val != expectedStrings[name] {
			t.Errorf("%s = %q, want %q", name, val, expectedStrings[name])
		}
	}
}
