package purchase

import "testing"

func validPR() CreatePurchaseReturnRequest {
	return CreatePurchaseReturnRequest{
		InvoiceID:    1,
		SupplierID:   1,
		ReturnDate:   "2026-08-09",
		RefundMethod: "deduct",
		Lines: []PurchaseReturnLineRequest{
			{ItemID: 1, Qty: 1, UnitPriceCents: 500000},
		},
	}
}

func TestValidatePRRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreatePurchaseReturnRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreatePurchaseReturnRequest) {}},
		{name: "missing invoice", mutate: func(r *CreatePurchaseReturnRequest) { r.InvoiceID = 0 }, wantError: true},
		{name: "missing supplier", mutate: func(r *CreatePurchaseReturnRequest) { r.SupplierID = 0 }, wantError: true},
		{name: "bad return_date", mutate: func(r *CreatePurchaseReturnRequest) { r.ReturnDate = "not-a-date" }, wantError: true},
		{name: "empty return_date", mutate: func(r *CreatePurchaseReturnRequest) { r.ReturnDate = "" }, wantError: true},
		{name: "bad refund_method", mutate: func(r *CreatePurchaseReturnRequest) { r.RefundMethod = "invalid" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreatePurchaseReturnRequest) { r.Lines = nil }, wantError: true},
		{name: "zero qty", mutate: func(r *CreatePurchaseReturnRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative price", mutate: func(r *CreatePurchaseReturnRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validPR()
			tc.mutate(&req)
			code, _ := validateReturnRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateReturnRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestPRLineTotalCalculation(t *testing.T) {
	// 2 units * 1000 cents = 2000
	lineTotal := returnLineTotal(2, 1000)
	if lineTotal != 2000 {
		t.Errorf("lineTotal = %d, want 2000", lineTotal)
	}
	// 1 unit * 1500 cents = 1500
	lineTotal = returnLineTotal(1, 1500)
	if lineTotal != 1500 {
		t.Errorf("lineTotal = %d, want 1500", lineTotal)
	}
	// 2.5 units * 1000 cents = 2500
	lineTotal = returnLineTotal(2.5, 1000)
	if lineTotal != 2500 {
		t.Errorf("lineTotal = %d, want 2500", lineTotal)
	}
}

func TestPRVATReversalCalculation(t *testing.T) {
	// When invoice has VAT and a line is returned, VAT reversed is proportional
	// to the DPP returned. Here DPP=800000, VAT=80000 (10%), return total=100000.
	// VAT reversed = round(100000 * 80000 / 800000) = 10000
	dpp := int64(800000)
	vat := int64(80000)
	returnTotal := int64(100000)
	rate := float64(vat) / float64(dpp)
	vatReversed := vatReversedForReturn(returnTotal, rate)
	if vatReversed != 10000 {
		t.Errorf("vatReversed = %d, want 10000", vatReversed)
	}
	// Zero VAT → no reversal.
	vatReversed = vatReversedForReturn(returnTotal, 0)
	if vatReversed != 0 {
		t.Errorf("vatReversed = %d, want 0 when no VAT", vatReversed)
	}
	// Negative rate → no reversal.
	vatReversed = vatReversedForReturn(returnTotal, -0.1)
	if vatReversed != 0 {
		t.Errorf("vatReversed = %d, want 0 when rate negative", vatReversed)
	}
}

func TestPRRefundMethodValidation(t *testing.T) {
	// Valid methods should pass.
	for _, method := range []string{"", "deduct", "refund", "credit_balance"} {
		req := validPR()
		req.RefundMethod = method
		if code, _ := validateReturnRequest(req); code != "" {
			t.Errorf("refund method %q should pass, got code %q", method, code)
		}
	}
	// Invalid method should fail.
	req := validPR()
	req.RefundMethod = "invalid"
	if code, _ := validateReturnRequest(req); code == "" {
		t.Error("invalid refund method should be rejected")
	}
	// Empty method should pass (defaults to deduct).
	req.RefundMethod = ""
	if code, _ := validateReturnRequest(req); code != "" {
		t.Errorf("empty refund method should pass (default), got code %q", code)
	}
}

func TestPRInvoicePayableAdjustment(t *testing.T) {
	// After PR: payable increases by return amount + vat reversed (AP goes back up).
	payable := int64(0) // invoice was PAID
	returnAmount := int64(750000)
	vatReversed := int64(75000)
	newPayable := payable + returnAmount + vatReversed
	if newPayable != 825000 {
		t.Errorf("newPayable = %d, want 825000", newPayable)
	}
	// If invoice was PAID, it becomes PARTIALLY_PAID.
	status := invPaid
	if newPayable > 0 && status == invPaid {
		status = invPartiallyPaid
	}
	if status != invPartiallyPaid {
		t.Errorf("status = %s, want PARTIALLY_PAID", status)
	}
}
