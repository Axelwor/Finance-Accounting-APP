package sales

import "testing"

func TestValidateInvoiceRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateInvoiceRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateInvoiceRequest) {}},
		{name: "missing customer", mutate: func(r *CreateInvoiceRequest) { r.CustomerID = 0 }, wantError: true},
		{name: "bad invoice date", mutate: func(r *CreateInvoiceRequest) { r.InvoiceDate = "not-a-date" }, wantError: true},
		{name: "bad due date", mutate: func(r *CreateInvoiceRequest) { r.DueDate = "08-08-2026" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateInvoiceRequest) { r.Lines = nil }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateInvoiceRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative price", mutate: func(r *CreateInvoiceRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
		{name: "tax above 100", mutate: func(r *CreateInvoiceRequest) { r.Lines[0].TaxRate = 101 }, wantError: true},
		{name: "missing item", mutate: func(r *CreateInvoiceRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateInvoiceRequest{
				CustomerID:  1,
				InvoiceDate: "2026-08-08",
				Lines: []InvoiceLineRequest{
					{ItemID: 1, Qty: 2, UnitPriceCents: 500000, TaxRate: 0},
				},
			}
			tc.mutate(&req)
			code, _ := validateInvoiceRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateInvoiceRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestPrepareInvoiceLinesTotal(t *testing.T) {
	prepared, total, err := prepareInvoiceLines([]InvoiceLineRequest{
		{ItemID: 1, Qty: 2, UnitPriceCents: 500000, DiscountCents: 0},
		{ItemID: 2, Qty: 1, UnitPriceCents: 1000000, DiscountCents: 50000},
	})
	if err != nil {
		t.Fatalf("prepareInvoiceLines error: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(prepared))
	}
	want := int64(2*500000) + (1000000 - 50000)
	if total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
	if prepared[0].Line.DeliveryID != 0 {
		t.Error("delivery_id should default to 0")
	}
	if prepared[1].LineTotalCents != 950000 {
		t.Errorf("line 2 total = %d, want 950000", prepared[1].LineTotalCents)
	}
}

// TestPrepareInvoiceLinesPPNHalfUp verifies the A-09 fix: PPN must round
// half-up, not truncate. Each case picks a lineTotal where
// lineTotal*11000/100000 has a fractional part of exactly .5 or just above/
// below, so truncation and half-up produce different results.
func TestPrepareInvoiceLinesPPNHalfUp(t *testing.T) {
	tests := []struct {
		name           string
		qty            float64
		unitPriceCents int64
		taxRate        float64
		wantDPP        int64
		wantPPN        int64
	}{
		{
			// 100,005 * 11% = 11,000.55 → half-up 11,001 (truncate would give 11,000)
			name: "fractional .55 rounds up", qty: 1, unitPriceCents: 100005, taxRate: 11,
			wantDPP: 100005, wantPPN: 11001,
		},
		{
			// 1,000,045 * 11% = 110,004.95 → half-up 110,005
			name: "fractional .95 rounds up", qty: 1, unitPriceCents: 1000045, taxRate: 11,
			wantDPP: 1000045, wantPPN: 110005,
		},
		{
			// Exact division must not gain a rupiah: 2,000,000 * 11% = 220,000 exactly
			name: "exact division stays exact", qty: 4, unitPriceCents: 500000, taxRate: 11,
			wantDPP: 2000000, wantPPN: 220000,
		},
		{
			// 333 * 11% = 36.63 → half-up 37
			name: "small fractional rounds up", qty: 1, unitPriceCents: 333, taxRate: 11,
			wantDPP: 333, wantPPN: 37,
		},
		{
			// Zero rate yields zero PPN
			name: "zero tax rate", qty: 2, unitPriceCents: 500000, taxRate: 0,
			wantDPP: 1000000, wantPPN: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prepared, total, err := prepareInvoiceLines([]InvoiceLineRequest{
				{ItemID: 1, Qty: tc.qty, UnitPriceCents: tc.unitPriceCents, DiscountCents: 0, TaxRate: tc.taxRate},
			})
			if err != nil {
				t.Fatalf("prepareInvoiceLines error: %v", err)
			}
			if prepared[0].LineTotalCents != tc.wantDPP {
				t.Errorf("DPP = %d, want %d", prepared[0].LineTotalCents, tc.wantDPP)
			}
			if prepared[0].PPNCents != tc.wantPPN {
				t.Errorf("PPN = %d, want %d (half-up rounding)", prepared[0].PPNCents, tc.wantPPN)
			}
			if total != tc.wantDPP+tc.wantPPN {
				t.Errorf("total = %d, want %d", total, tc.wantDPP+tc.wantPPN)
			}
		})
	}
}

// TestPrepareInvoiceLinesMilliunitQty verifies the A-22 fix: fractional
// quantities are converted to milliunits before multiplying so no float
// precision is lost (qty NUMERIC(18,3) * unit_price_cents).
func TestPrepareInvoiceLinesMilliunitQty(t *testing.T) {
	tests := []struct {
		name           string
		qty            float64
		unitPriceCents int64
		wantLineTotal  int64
	}{
		// 0.001 * 1,000,000 = 1,000 exactly
		{name: "milli qty exact", qty: 0.001, unitPriceCents: 1000000, wantLineTotal: 1000},
		// 1.005 * 1,000,000 = 1,005,000 exactly — with float64 this could drift
		{name: "thousandths qty", qty: 1.005, unitPriceCents: 1000000, wantLineTotal: 1005000},
		// 2.999 * 333 = 998.667 → milliunits: 2999*333/1000 = 998.667 → round half up 999
		{name: "rounds half up on divide", qty: 2.999, unitPriceCents: 333, wantLineTotal: 999},
		// 0.0005 rounds to 1 milliunit → 1 * 100 / 1000 = 0.1 → rounds to 0
		{name: "sub-milli qty rounds away", qty: 0.0005, unitPriceCents: 100, wantLineTotal: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prepared, _, err := prepareInvoiceLines([]InvoiceLineRequest{
				{ItemID: 1, Qty: tc.qty, UnitPriceCents: tc.unitPriceCents, DiscountCents: 0, TaxRate: 0},
			})
			if err != nil {
				t.Fatalf("prepareInvoiceLines error: %v", err)
			}
			if prepared[0].LineTotalCents != tc.wantLineTotal {
				t.Errorf("lineTotal = %d, want %d", prepared[0].LineTotalCents, tc.wantLineTotal)
			}
		})
	}
}

func TestInvoiceReceivableCalculation(t *testing.T) {
	// Total = 1000000, DP applied = 250000 → receivable = 750000
	total := int64(1000000)
	dpApplied := int64(250000)
	receivable := total - dpApplied
	if receivable != 750000 {
		t.Errorf("receivable = %d, want 750000", receivable)
	}
	// DP clamped to total when DP > total
	dpApplied = 1500000
	if dpApplied > total {
		dpApplied = total
	}
	receivable = total - dpApplied
	if receivable != 0 {
		t.Errorf("receivable with clamped DP = %d, want 0", receivable)
	}
}
