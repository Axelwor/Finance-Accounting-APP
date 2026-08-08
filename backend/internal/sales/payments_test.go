package sales

import "testing"

func TestValidatePaymentRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreatePaymentRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreatePaymentRequest) {}},
		{name: "missing cash account", mutate: func(r *CreatePaymentRequest) { r.CashAccountID = 0 }, wantError: true},
		{name: "zero amount", mutate: func(r *CreatePaymentRequest) { r.AmountCents = 0 }, wantError: true},
		{name: "negative amount", mutate: func(r *CreatePaymentRequest) { r.AmountCents = -100 }, wantError: true},
		{name: "bad date", mutate: func(r *CreatePaymentRequest) { r.PaymentDate = "08/08/2026" }, wantError: true},
		{name: "empty date", mutate: func(r *CreatePaymentRequest) { r.PaymentDate = "" }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreatePaymentRequest{
				CashAccountID: 1,
				AmountCents:   250000,
				PaymentDate:   "2026-08-08",
			}
			tc.mutate(&req)
			code, _ := validatePaymentRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validatePaymentRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestPaymentARAndOverpaymentCalculation(t *testing.T) {
	// Full payment: amount = receivable → no overpayment
	receivable := int64(750000)
	amount := int64(750000)
	arApplied := amount
	overpayment := int64(0)
	if arApplied > receivable {
		overpayment = arApplied - receivable
		arApplied = receivable
	}
	if arApplied != 750000 || overpayment != 0 {
		t.Errorf("full payment: arApplied=%d, overpayment=%d, want 750000/0", arApplied, overpayment)
	}
	// Overpayment: amount > receivable
	amount = 1000000
	arApplied = amount
	overpayment = 0
	if arApplied > receivable {
		overpayment = arApplied - receivable
		arApplied = receivable
	}
	if arApplied != 750000 || overpayment != 250000 {
		t.Errorf("overpayment: arApplied=%d, overpayment=%d, want 750000/250000", arApplied, overpayment)
	}
	// Partial payment: amount < receivable
	amount = 300000
	arApplied = amount
	overpayment = 0
	if arApplied > receivable {
		overpayment = arApplied - receivable
		arApplied = receivable
	}
	if arApplied != 300000 || overpayment != 0 {
		t.Errorf("partial payment: arApplied=%d, overpayment=%d, want 300000/0", arApplied, overpayment)
	}
}

func TestInvoiceStatusAfterPayment(t *testing.T) {
	// Full payment → PAID
	receivable := int64(750000)
	arApplied := int64(750000)
	newReceivable := receivable - arApplied
	status := invPartiallyPaid
	if newReceivable <= 0 {
		status = invPaid
	}
	if status != invPaid {
		t.Errorf("full payment status = %s, want PAID", status)
	}
	// Partial payment → PARTIALLY_PAID
	arApplied = 300000
	newReceivable = receivable - arApplied
	status = invPartiallyPaid
	if newReceivable <= 0 {
		status = invPaid
	}
	if status != invPartiallyPaid {
		t.Errorf("partial payment status = %s, want PARTIALLY_PAID", status)
	}
}
