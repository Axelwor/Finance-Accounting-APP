package sales

import "testing"

func TestLineTotalCents(t *testing.T) {
	tests := []struct {
		name           string
		qty            float64
		unitPriceCents int64
		discountCents  int64
		want           int64
	}{
		{name: "integer qty", qty: 2, unitPriceCents: 1500, discountCents: 0, want: 3000},
		{name: "fractional qty rounds", qty: 1.5, unitPriceCents: 1000, discountCents: 0, want: 1500},
		{name: "discount applied", qty: 10, unitPriceCents: 100, discountCents: 250, want: 750},
		{name: "rounds half up", qty: 0.125, unitPriceCents: 100, discountCents: 0, want: 13},
		{name: "qty three decimals", qty: 1.333, unitPriceCents: 3000, discountCents: 0, want: 3999},
		{name: "discount exceeds gross clamps to negative int", qty: 1, unitPriceCents: 100, discountCents: 500, want: -400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lineTotalCents(tc.qty, tc.unitPriceCents, tc.discountCents)
			if got != tc.want {
				t.Errorf("lineTotalCents(%v, %d, %d) = %d, want %d", tc.qty, tc.unitPriceCents, tc.discountCents, got, tc.want)
			}
		})
	}
}

func validCreate() CreateQuotationRequest {
	return CreateQuotationRequest{
		CustomerID:    1,
		QuotationDate: "2026-08-08",
		Lines: []QuotationLineRequest{
			{ItemID: 1, Qty: 2, UnitPriceCents: 1000, TaxRate: 11},
		},
	}
}

func TestValidateCreateRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateQuotationRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateQuotationRequest) {}},
		{name: "missing customer", mutate: func(r *CreateQuotationRequest) { r.CustomerID = 0 }, wantError: true},
		{name: "bad quotation date", mutate: func(r *CreateQuotationRequest) { r.QuotationDate = "not-a-date" }, wantError: true},
		{name: "bad valid_until", mutate: func(r *CreateQuotationRequest) { r.ValidUntil = "08-08-2026" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateQuotationRequest) { r.Lines = nil }, wantError: true},
		{name: "negative qty", mutate: func(r *CreateQuotationRequest) { r.Lines[0].Qty = -1 }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateQuotationRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative unit price", mutate: func(r *CreateQuotationRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
		{name: "negative discount", mutate: func(r *CreateQuotationRequest) { r.Lines[0].DiscountCents = -5 }, wantError: true},
		{name: "tax rate above 100", mutate: func(r *CreateQuotationRequest) { r.Lines[0].TaxRate = 101 }, wantError: true},
		{name: "tax rate negative", mutate: func(r *CreateQuotationRequest) { r.Lines[0].TaxRate = -1 }, wantError: true},
		{name: "missing item id", mutate: func(r *CreateQuotationRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validCreate()
			tc.mutate(&req)
			code, _ := validateCreateRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateCreateRequest errorCode=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestPrepareLinesTotal(t *testing.T) {
	prepared, total, err := prepareLines([]QuotationLineRequest{
		{ItemID: 1, Qty: 2, UnitPriceCents: 1000, DiscountCents: 0},   // 2000
		{ItemID: 2, Qty: 1.5, UnitPriceCents: 1000, DiscountCents: 0}, // 1500
	})
	if err != nil {
		t.Fatalf("prepareLines returned error: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("expected 2 prepared lines, got %d", len(prepared))
	}
	if total != 3500 {
		t.Errorf("total = %d, want 3500", total)
	}
	if prepared[0].LineTotalCents != 2000 || prepared[1].LineTotalCents != 1500 {
		t.Errorf("line totals = %d,%d want 2000,1500", prepared[0].LineTotalCents, prepared[1].LineTotalCents)
	}
}

func TestTransitions(t *testing.T) {
	if !canSend(statusDraft) {
		t.Error("DRAFT should be able to send")
	}
	if canSend(statusSent) || canSend(statusCancelled) {
		t.Error("only DRAFT can be sent")
	}
	for _, status := range []string{statusDraft, statusSent} {
		if !canCancel(status) {
			t.Errorf("%s should be cancellable", status)
		}
	}
	if canCancel(statusConverted) || canCancel(statusCancelled) {
		t.Error("CONVERTED/CANCELLED should not be cancellable")
	}
	for _, status := range []string{statusDraft, statusSent} {
		if !canExpire(status) {
			t.Errorf("%s should be expirable", status)
		}
	}
	if canExpire(statusCancelled) || canExpire(statusConverted) {
		t.Error("CANCELLED/CONVERTED should not be expirable")
	}
}
