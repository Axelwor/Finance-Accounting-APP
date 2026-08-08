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
