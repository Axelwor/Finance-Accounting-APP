package purchase

import "testing"

func TestValidateSupplierInvoiceRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateSupplierInvoiceRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateSupplierInvoiceRequest) {}},
		{name: "missing supplier", mutate: func(r *CreateSupplierInvoiceRequest) { r.SupplierID = 0 }, wantError: true},
		{name: "bad invoice date", mutate: func(r *CreateSupplierInvoiceRequest) { r.InvoiceDate = "not-a-date" }, wantError: true},
		{name: "bad due date", mutate: func(r *CreateSupplierInvoiceRequest) { r.DueDate = "08-08-2026" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateSupplierInvoiceRequest) { r.Lines = nil }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateSupplierInvoiceRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative price", mutate: func(r *CreateSupplierInvoiceRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
		{name: "negative discount", mutate: func(r *CreateSupplierInvoiceRequest) { r.Lines[0].DiscountCents = -1 }, wantError: true},
		{name: "tax above 100", mutate: func(r *CreateSupplierInvoiceRequest) { r.Lines[0].TaxRate = 101 }, wantError: true},
		{name: "negative tax", mutate: func(r *CreateSupplierInvoiceRequest) { r.Lines[0].TaxRate = -1 }, wantError: true},
		{name: "missing item", mutate: func(r *CreateSupplierInvoiceRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateSupplierInvoiceRequest{
				SupplierID:  1,
				InvoiceDate: "2026-08-09",
				DueDate:     "2026-09-09",
				Lines: []SupplierInvoiceLineRequest{
					{ItemID: 1, Qty: 2, UnitPriceCents: 500000, DiscountCents: 0, TaxRate: 11},
				},
			}
			tc.mutate(&req)
			code, _ := validateSupplierInvoiceRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateSupplierInvoiceRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestPrepareSupplierInvoiceLinesTotal(t *testing.T) {
	prepared, dpp, vat, err := prepareSupplierInvoiceLines([]SupplierInvoiceLineRequest{
		{ItemID: 1, Qty: 2, UnitPriceCents: 500000, DiscountCents: 0, TaxRate: 11},
		{ItemID: 2, Qty: 1, UnitPriceCents: 1000000, DiscountCents: 50000, TaxRate: 0},
	})
	if err != nil {
		t.Fatalf("prepareSupplierInvoiceLines error: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(prepared))
	}
	// Line 1: 2 × 500000 = 1000000 (DPP), VAT = 1000000 × 11% = 110000
	// Line 2: 1 × 1000000 − 50000 = 950000 (DPP), VAT = 0
	wantDPP := int64(1000000 + 950000)
	if dpp != wantDPP {
		t.Errorf("dpp = %d, want %d", dpp, wantDPP)
	}
	wantVAT := int64(110000)
	if vat != wantVAT {
		t.Errorf("vat = %d, want %d", vat, wantVAT)
	}
	if prepared[0].LineTotalCents != 1000000 {
		t.Errorf("line 1 total = %d, want 1000000", prepared[0].LineTotalCents)
	}
	if prepared[1].LineTotalCents != 950000 {
		t.Errorf("line 2 total = %d, want 950000", prepared[1].LineTotalCents)
	}
}

func TestSupplierInvoiceDPPVATTotalComputation(t *testing.T) {
	// DPP = 1950000, VAT = 110000 → total = 2060000, payable = total (no DP)
	dpp := int64(1950000)
	vat := int64(110000)
	total := dpp + vat
	dpApplied := int64(0)
	payable := total - dpApplied
	if total != 2060000 {
		t.Errorf("total = %d, want 2060000", total)
	}
	if payable != 2060000 {
		t.Errorf("payable = %d, want 2060000", payable)
	}
	// DP clamped to total when DP > total
	dpApplied = 3000000
	if dpApplied > total {
		dpApplied = total
	}
	payable = total - dpApplied
	if payable != 0 {
		t.Errorf("payable with clamped DP = %d, want 0", payable)
	}
}

func TestSupplierVATCents(t *testing.T) {
	tests := []struct {
		lineTotal int64
		taxRate   float64
		want      int64
	}{
		{1000000, 11, 110000}, // 11% of 1M
		{950000, 11, 104500},  // 11% of 950k = 104500
		{1000000, 0, 0},       // 0% → 0
		{100, 10, 10},         // 10% of 100 = 10
		{5, 11, 1},            // 0.55 rounds to 1 (round half away from zero)
		{4, 11, 0},            // 0.44 rounds to 0
	}
	for _, tc := range tests {
		got := supplierVATCents(tc.lineTotal, tc.taxRate)
		if got != tc.want {
			t.Errorf("supplierVATCents(%d, %v) = %d, want %d", tc.lineTotal, tc.taxRate, got, tc.want)
		}
	}
}

func TestSupplierLineTotalCents(t *testing.T) {
	// 2 × 500000 = 1000000
	if got := supplierLineTotalCents(2, 500000, 0); got != 1000000 {
		t.Errorf("supplierLineTotalCents(2, 500000, 0) = %d, want 1000000", got)
	}
	// 1 × 1000000 − 50000 = 950000
	if got := supplierLineTotalCents(1, 1000000, 50000); got != 950000 {
		t.Errorf("supplierLineTotalCents(1, 1000000, 50000) = %d, want 950000", got)
	}
	// 0.5 × 1000000 = 500000
	if got := supplierLineTotalCents(0.5, 1000000, 0); got != 500000 {
		t.Errorf("supplierLineTotalCents(0.5, 1000000, 0) = %d, want 500000", got)
	}
}
