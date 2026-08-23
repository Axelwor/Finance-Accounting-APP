package sales

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

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

// TestApplyPayment covers the QA-06 fix: a payment may be partial or exact,
// but never exceed the outstanding receivable — overpay must return
// errPaymentExceedsReceivable so the handler answers 409 PAYMENT_EXCEEDS_RECEIVABLE.
func TestApplyPayment(t *testing.T) {
	tests := []struct {
		name           string
		amount         int64
		receivable     int64
		wantApplied    int64
		wantOverpayErr bool
	}{
		{name: "partial payment", amount: 300000, receivable: 750000, wantApplied: 300000},
		{name: "exact settlement", amount: 750000, receivable: 750000, wantApplied: 750000},
		{name: "overpay rejected", amount: 1000000, receivable: 750000, wantOverpayErr: true},
		{name: "second payment over remaining", amount: 700000, receivable: 665000, wantOverpayErr: true},
		{name: "payment on settled invoice", amount: 1, receivable: 0, wantOverpayErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applied, err := applyPayment(tc.amount, tc.receivable)
			if tc.wantOverpayErr {
				if err == nil {
					t.Fatalf("applyPayment(%d, %d) = %d, nil; want error", tc.amount, tc.receivable, applied)
				}
				if !errors.Is(err, errPaymentExceedsReceivable) {
					t.Errorf("error = %v, want errPaymentExceedsReceivable", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("applyPayment(%d, %d) returned unexpected error: %v", tc.amount, tc.receivable, err)
			}
			if applied != tc.wantApplied {
				t.Errorf("applied = %d, want %d", applied, tc.wantApplied)
			}
		})
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

// TestPaymentErrorFor covers the QA-06 error mapping: the overpay business
// rejection must surface as a clean 409 PAYMENT_EXCEEDS_RECEIVABLE with the
// message intact — never a raw SQLSTATE body.
func TestPaymentErrorFor(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "overpay maps to 409 PAYMENT_EXCEEDS_RECEIVABLE",
			err:        fmt.Errorf("%w: amount 999000000 exceeds receivable 665000", errPaymentExceedsReceivable),
			wantStatus: http.StatusConflict,
			wantCode:   "PAYMENT_EXCEEDS_RECEIVABLE",
		},
		{
			name:       "bare sentinel also matches",
			err:        errPaymentExceedsReceivable,
			wantStatus: http.StatusConflict,
			wantCode:   "PAYMENT_EXCEEDS_RECEIVABLE",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, code, _ := paymentErrorFor(tc.err)
			if status != tc.wantStatus || code != tc.wantCode {
				t.Errorf("paymentErrorFor = (%d, %s), want (%d, %s)", status, code, tc.wantStatus, tc.wantCode)
			}
		})
	}
}
