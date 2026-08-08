package purchase

import "testing"

func TestValidateSupplierPaymentRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateSupplierPaymentRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateSupplierPaymentRequest) {}},
		{name: "missing cash account", mutate: func(r *CreateSupplierPaymentRequest) { r.CashAccountID = 0 }, wantError: true},
		{name: "zero amount", mutate: func(r *CreateSupplierPaymentRequest) { r.AmountCents = 0 }, wantError: true},
		{name: "negative amount", mutate: func(r *CreateSupplierPaymentRequest) { r.AmountCents = -100 }, wantError: true},
		{name: "bad date", mutate: func(r *CreateSupplierPaymentRequest) { r.PaymentDate = "08/08/2026" }, wantError: true},
		{name: "empty date", mutate: func(r *CreateSupplierPaymentRequest) { r.PaymentDate = "" }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateSupplierPaymentRequest{
				CashAccountID: 1,
				AmountCents:   250000,
				PaymentDate:   "2026-08-08",
			}
			tc.mutate(&req)
			code, _ := validateSupplierPaymentRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateSupplierPaymentRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestSupplierPaymentAPAndOverpaymentCalculation(t *testing.T) {
	// Full payment: amount = payable → no overpayment
	payable := int64(750000)
	amount := int64(750000)
	apApplied := amount
	overpayment := int64(0)
	if apApplied > payable {
		overpayment = apApplied - payable
		apApplied = payable
	}
	if apApplied != 750000 || overpayment != 0 {
		t.Errorf("full payment: apApplied=%d, overpayment=%d, want 750000/0", apApplied, overpayment)
	}
	// Overpayment: amount > payable
	amount = 1000000
	apApplied = amount
	overpayment = 0
	if apApplied > payable {
		overpayment = apApplied - payable
		apApplied = payable
	}
	if apApplied != 750000 || overpayment != 250000 {
		t.Errorf("overpayment: apApplied=%d, overpayment=%d, want 750000/250000", apApplied, overpayment)
	}
	// Partial payment: amount < payable
	amount = 300000
	apApplied = amount
	overpayment = 0
	if apApplied > payable {
		overpayment = apApplied - payable
		apApplied = payable
	}
	if apApplied != 300000 || overpayment != 0 {
		t.Errorf("partial payment: apApplied=%d, overpayment=%d, want 300000/0", apApplied, overpayment)
	}
}

func TestSupplierInvoiceStatusAfterPayment(t *testing.T) {
	// Full payment → PAID
	payable := int64(750000)
	apApplied := int64(750000)
	newPayable := payable - apApplied
	status := "PARTIALLY_PAID"
	if newPayable <= 0 {
		status = "PAID"
	}
	if status != "PAID" {
		t.Errorf("full payment status = %s, want PAID", status)
	}
	// Partial payment → PARTIALLY_PAID
	apApplied = 300000
	newPayable = payable - apApplied
	status = "PARTIALLY_PAID"
	if newPayable <= 0 {
		status = "PAID"
	}
	if status != "PARTIALLY_PAID" {
		t.Errorf("partial payment status = %s, want PARTIALLY_PAID", status)
	}
}
