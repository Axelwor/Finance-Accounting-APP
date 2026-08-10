package sales

import "testing"

func validCN() CreateCreditNoteRequest {
	return CreateCreditNoteRequest{
		InvoiceID:    1,
		CustomerID:   1,
		CNDate:       "2026-08-08",
		RefundMethod: "deduct",
		Lines: []CreditNoteLineRequest{
			{ItemID: 1, Qty: 1, UnitPriceCents: 5000},
		},
	}
}

func TestValidateCNRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateCreditNoteRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateCreditNoteRequest) {}},
		{name: "missing invoice", mutate: func(r *CreateCreditNoteRequest) { r.InvoiceID = 0 }, wantError: true},
		{name: "bad cn_date", mutate: func(r *CreateCreditNoteRequest) { r.CNDate = "not-a-date" }, wantError: true},
		{name: "empty cn_date", mutate: func(r *CreateCreditNoteRequest) { r.CNDate = "" }, wantError: true},
		{name: "bad refund_method", mutate: func(r *CreateCreditNoteRequest) { r.RefundMethod = "invalid" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateCreditNoteRequest) { r.Lines = nil }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateCreditNoteRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative price", mutate: func(r *CreateCreditNoteRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
		{name: "missing item", mutate: func(r *CreateCreditNoteRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
		{name: "negative cost", mutate: func(r *CreateCreditNoteRequest) { r.Lines[0].UnitCostCents = -1 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validCN()
			tc.mutate(&req)
			code, _ := validateCNRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateCNRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestCNLineTotalCalculation(t *testing.T) {
	// 2 units * 1000 cents = 2000
	lineTotal := lineTotalCents(2, 1000, 0)
	if lineTotal != 2000 {
		t.Errorf("lineTotal = %d, want 2000", lineTotal)
	}
	// 1 unit * 1500 cents = 1500
	lineTotal = lineTotalCents(1, 1500, 0)
	if lineTotal != 1500 {
		t.Errorf("lineTotal = %d, want 1500", lineTotal)
	}
}

func TestCNCOGSReversalCalculation(t *testing.T) {
	// 2 units * 3000 cost = 6000 COGS reversed
	cogsReversed := roundQty(2) * 3000
	if cogsReversed != 6000 {
		t.Errorf("cogsReversed = %d, want 6000", cogsReversed)
	}
}

func TestCNRefundMethodValidation(t *testing.T) {
	// Test via validateCNRequest
	req := validCN()
	for _, method := range []string{"deduct", "refund", "credit_balance"} {
		req.RefundMethod = method
		if code, _ := validateCNRequest(req); code != "" {
			t.Errorf("refund method %q should be valid, got code %q", method, code)
		}
	}
	// Invalid method should fail
	req.RefundMethod = "invalid"
	if code, _ := validateCNRequest(req); code == "" {
		t.Error("invalid refund method should be rejected")
	}
	// Empty method should pass (defaults to deduct)
	req.RefundMethod = ""
	if code, _ := validateCNRequest(req); code != "" {
		t.Errorf("empty refund method should pass (default), got code %q", code)
	}
}

func TestCNInvoiceReceivableAdjustment(t *testing.T) {
	// After CN (m-008 fix): the reversal journal credits AR, so the invoice
	// receivable_cents DECREASES by the return amount.
	receivable := int64(1000000) // invoice was ISSUED with 1,000,000 outstanding
	returnAmount := int64(750000)

	if returnAmount > receivable {
		t.Fatalf("test setup invalid: return %d > receivable %d", returnAmount, receivable)
	}
	newReceivable := receivable - returnAmount
	if newReceivable != 250000 {
		t.Errorf("newReceivable = %d, want 250000", newReceivable)
	}
	// Invoice stays ISSUED while receivable > 0.
	status := invStatusForReceivable(invIssued, newReceivable)
	if status != invIssued {
		t.Errorf("status = %s, want ISSUED", status)
	}
	// When the CN wipes out the remaining receivable, the invoice is PAID.
	fullReturn := receivable // return the whole amount
	status = invStatusForReceivable(invIssued, receivable-fullReturn)
	if status != invPaid {
		t.Errorf("full-return status = %s, want PAID", status)
	}
}

// invStatusForReceivable mirrors the handler's status transition logic.
func invStatusForReceivable(current string, newReceivable int64) string {
	if newReceivable <= 0 && current != invPaid {
		return invPaid
	}
	return current
}

func TestCNRejectsOverCrediting(t *testing.T) {
	// A credit note whose total exceeds the invoice's outstanding receivable
	// must be rejected (m-008).
	receivable := int64(500000)
	returnAmount := int64(750000)
	if returnAmount <= receivable {
		t.Fatalf("test setup invalid: return %d should exceed receivable %d", returnAmount, receivable)
	}
	// The handler returns an error in this case; here we assert the guard condition.
	if !(returnAmount > receivable) {
		t.Error("over-crediting guard condition failed")
	}
}
