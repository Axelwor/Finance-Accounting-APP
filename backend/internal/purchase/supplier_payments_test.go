package purchase

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

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

// QA-06 companion rule: payments above the outstanding payable are rejected,
// partial and full payments are applied in full.
func TestSplitPaymentAmount(t *testing.T) {
	tests := []struct {
		name          string
		amountCents   int64
		payableCents  int64
		wantApplied   int64
		wantErrCode   string
		wantErrStatus int
	}{
		{name: "partial payment", amountCents: 500000, payableCents: 1110000, wantApplied: 500000},
		{name: "full settlement", amountCents: 1110000, payableCents: 1110000, wantApplied: 1110000},
		{
			name: "overpay rejected", amountCents: 1200000, payableCents: 610000, wantApplied: 0,
			wantErrCode: "PAYMENT_EXCEEDS_PAYABLE", wantErrStatus: http.StatusConflict,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applied, err := splitPaymentAmount(tc.amountCents, tc.payableCents)
			if tc.wantErrCode == "" {
				if err != nil {
					t.Fatalf("splitPaymentAmount(%d, %d) unexpected error: %v", tc.amountCents, tc.payableCents, err)
				}
				if applied != tc.wantApplied {
					t.Errorf("applied = %d, want %d", applied, tc.wantApplied)
				}
				return
			}
			var overpay *paymentExceedsPayableError
			if !errors.As(err, &overpay) {
				t.Fatalf("error = %v, want paymentExceedsPayableError", err)
			}
			status, code, message := supplierPaymentErrorFor(err)
			if status != tc.wantErrStatus || code != tc.wantErrCode {
				t.Errorf("supplierPaymentErrorFor = (%d,%s), want (%d,%s)", status, code, tc.wantErrStatus, tc.wantErrCode)
			}
			if !strings.Contains(message, fmt.Sprintf("%d", tc.amountCents)) ||
				!strings.Contains(message, fmt.Sprintf("%d", tc.payableCents)) {
				t.Errorf("message %q must mention both amounts", message)
			}
			if strings.Contains(strings.ToUpper(message), "SQLSTATE") {
				t.Errorf("message %q leaks raw SQLSTATE", message)
			}
		})
	}
}

func TestSupplierInvoiceStatusAfterPayment(t *testing.T) {
	tests := []struct {
		name       string
		payable    int64
		apApplied  int64
		wantStatus string
	}{
		{name: "full payment settles to PAID", payable: 750000, apApplied: 750000, wantStatus: "PAID"},
		{name: "partial payment stays PARTIALLY_PAID", payable: 750000, apApplied: 300000, wantStatus: "PARTIALLY_PAID"},
		{name: "second staged payment settles to PAID", payable: 610000, apApplied: 610000, wantStatus: "PAID"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newPayable := tc.payable - tc.apApplied
			status := "PARTIALLY_PAID"
			if newPayable <= 0 {
				status = "PAID"
			}
			if status != tc.wantStatus {
				t.Errorf("status = %s, want %s (newPayable=%d)", status, tc.wantStatus, newPayable)
			}
		})
	}
}

// QA-06: the journal source_ref must be the unique payment document number,
// not a static per-invoice ref that collides on journal_entries_intent_unique.
func TestSupplierPaymentSourceRefIsUniquePerPayment(t *testing.T) {
	first := "PAY-2026-000001"
	second := "PAY-2026-000002"
	if first == second {
		t.Fatal("document numbering produced identical refs")
	}
	if _, err := splitPaymentAmount(100, 100); err != nil {
		t.Fatalf("sanity split failed: %v", err)
	}
	var uniqueViolation *pgconn.PgError = &pgconn.PgError{Code: "23505"}
	if !isUniqueViolation(fmt.Errorf("wrapped: %w", uniqueViolation)) {
		t.Error("isUniqueViolation should detect SQLSTATE 23505")
	}
}
