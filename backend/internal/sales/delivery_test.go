package sales

import "testing"

func TestValidateDeliveryRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateDeliveryRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateDeliveryRequest) {}},
		{name: "missing so id", mutate: func(r *CreateDeliveryRequest) { r.SalesOrderID = 0 }, wantError: true},
		{name: "bad date", mutate: func(r *CreateDeliveryRequest) { r.DeliveryDate = "not-a-date" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateDeliveryRequest) { r.Lines = nil }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateDeliveryRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative cost", mutate: func(r *CreateDeliveryRequest) { r.Lines[0].UnitCostCents = -1 }, wantError: true},
		{name: "missing item", mutate: func(r *CreateDeliveryRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateDeliveryRequest{
				SalesOrderID: 1,
				DeliveryDate: "2026-08-08",
				Lines: []DeliveryLineRequest{
					{ItemID: 1, Qty: 5, UnitCostCents: 300000},
				},
			}
			tc.mutate(&req)
			code, _ := validateDeliveryRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateDeliveryRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestRoundQty(t *testing.T) {
	tests := []struct {
		qty  float64
		want int64
	}{
		{qty: 5, want: 5},
		{qty: 2.5, want: 3},
		{qty: 2.4, want: 2},
		{qty: 10, want: 10},
	}
	for _, tc := range tests {
		got := roundQty(tc.qty)
		if got != tc.want {
			t.Errorf("roundQty(%v) = %d, want %d", tc.qty, got, tc.want)
		}
	}
}

func TestCOGSCalculation(t *testing.T) {
	// 5 units * 300000 cents = 1500000
	cogs := roundQty(5) * 300000
	if cogs != 1500000 {
		t.Errorf("cogs = %d, want 1500000", cogs)
	}
}
