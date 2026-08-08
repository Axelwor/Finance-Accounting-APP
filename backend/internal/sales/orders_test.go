package sales

import (
	"errors"
	"testing"
)

func validOrderRequest() CreateSalesOrderRequest {
	return CreateSalesOrderRequest{
		CustomerID: 1,
		OrderDate:  "2026-08-08",
		Lines: []SalesOrderLineRequest{
			{ItemID: 1, Qty: 2, UnitPriceCents: 500000, TaxRate: 0},
		},
	}
}

func TestValidateOrderRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateSalesOrderRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateSalesOrderRequest) {}},
		{name: "missing customer", mutate: func(r *CreateSalesOrderRequest) { r.CustomerID = 0 }, wantError: true},
		{name: "bad date", mutate: func(r *CreateSalesOrderRequest) { r.OrderDate = "not-a-date" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateSalesOrderRequest) { r.Lines = nil }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateSalesOrderRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative price", mutate: func(r *CreateSalesOrderRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
		{name: "tax above 100", mutate: func(r *CreateSalesOrderRequest) { r.Lines[0].TaxRate = 101 }, wantError: true},
		{name: "missing item", mutate: func(r *CreateSalesOrderRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validOrderRequest()
			tc.mutate(&req)
			code, _ := validateOrderRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateOrderRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestPrepareOrderLinesTotal(t *testing.T) {
	prepared, total, err := prepareOrderLines([]SalesOrderLineRequest{
		{ItemID: 1, Qty: 2, UnitPriceCents: 500000, DiscountCents: 0},
		{ItemID: 2, Qty: 1, UnitPriceCents: 1000000, DiscountCents: 50000},
	})
	if err != nil {
		t.Fatalf("prepareOrderLines error: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(prepared))
	}
	want := int64(2*500000) + (1000000 - 50000)
	if total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
}

func TestValidateDPRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateDownPaymentRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateDownPaymentRequest) {}},
		{name: "missing cash account", mutate: func(r *CreateDownPaymentRequest) { r.CashAccountID = 0 }, wantError: true},
		{name: "zero amount", mutate: func(r *CreateDownPaymentRequest) { r.AmountCents = 0 }, wantError: true},
		{name: "negative amount", mutate: func(r *CreateDownPaymentRequest) { r.AmountCents = -100 }, wantError: true},
		{name: "bad date", mutate: func(r *CreateDownPaymentRequest) { r.DPDate = "08/08/2026" }, wantError: true},
		{name: "empty date", mutate: func(r *CreateDownPaymentRequest) { r.DPDate = "" }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateDownPaymentRequest{
				CashAccountID: 1,
				AmountCents:   250000,
				DPDate:        "2026-08-08",
			}
			tc.mutate(&req)
			code, _ := validateDPRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateDPRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestDPOverflowError(t *testing.T) {
	err := dpOverflowError{max: 250000}
	if err.Error() == "" {
		t.Error("dpOverflowError should have a non-empty message")
	}
	var target dpOverflowError
	if !errors.As(err, &target) {
		t.Error("dpOverflowError should match via errors.As")
	}
	if target.max != 250000 {
		t.Errorf("target.max = %d, want 250000", target.max)
	}
}
