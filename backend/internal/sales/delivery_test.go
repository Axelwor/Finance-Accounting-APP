package sales

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/costing"
)

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

// TestDeliveryErrorFor covers the QA-11 fix: an insufficient-stock failure
// (raised by costing.ResolveCOGS before any journal work) must map to a
// 4xx INSUFFICIENT_STOCK response instead of the generic 500.
func TestDeliveryErrorFor(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "insufficient stock maps to 422",
			err:        fmt.Errorf("%w: item 1 on_hand=50 need=100", costing.ErrInsufficientStock),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "INSUFFICIENT_STOCK",
		},
		{
			name:       "bare sentinel still matches",
			err:        costing.ErrInsufficientStock,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "INSUFFICIENT_STOCK",
		},
		{
			name:       "unknown order stays 404 ORDER_NOT_FOUND",
			err:        pgx.ErrNoRows,
			wantStatus: http.StatusNotFound,
			wantCode:   "ORDER_NOT_FOUND",
		},
		{
			name:       "other errors stay 500 DELIVERY_CREATE_FAILED",
			err:        errors.New("costing: ResolveCOGS layer update"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "DELIVERY_CREATE_FAILED",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, code, _ := deliveryErrorFor(tc.err)
			if status != tc.wantStatus || code != tc.wantCode {
				t.Errorf("deliveryErrorFor = (%d, %s), want (%d, %s)", status, code, tc.wantStatus, tc.wantCode)
			}
		})
	}
}
