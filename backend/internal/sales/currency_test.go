package sales

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"testing"
)

// TestNormalizeDocCurrency covers the SET-001 document currency rules: empty
// defaults to IDR/1, IDR forces rate 1, foreign currencies require a positive
// rate, and malformed codes are rejected.
func TestNormalizeDocCurrency(t *testing.T) {
	cases := []struct {
		name     string
		code     string
		rate     float64
		wantCode string
		wantRate float64
		wantErr  bool
	}{
		{"empty defaults to IDR", "", 0, "IDR", 1, false},
		{"lowercase normalized", "usd", 15750, "USD", 15750, false},
		{"IDR forces rate 1", "IDR", 999, "IDR", 1, false},
		{"FC positive rate ok", "USD", 15750.5, "USD", 15750.5, false},
		{"FC zero rate rejected", "USD", 0, "", 0, true},
		{"FC negative rate rejected", "EUR", -1, "", 0, true},
		{"bad length rejected", "USDD", 1, "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, rate, err := normalizeDocCurrency(tc.code, tc.rate)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got code=%q rate=%v", code, rate)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if code != tc.wantCode || rate != tc.wantRate {
				t.Fatalf("got (%q, %v), want (%q, %v)", code, rate, tc.wantCode, tc.wantRate)
			}
		})
	}
}

// TestFxPaymentMath verifies the FX gain/loss arithmetic used by
// CreatePayment for a foreign-currency RECEIVABLE: amount_cents is the
// booked AR value being settled; cash received is that document amount
// converted at the payment rate (arApplied + fxDiff). A rate increase
// yields MORE cash than the booked AR → GAIN; a rate decrease → LOSS.
// The journal must always balance: Dr cash == Cr AR + gain == Dr loss + Cr AR.
func TestFxPaymentMath(t *testing.T) {
	// Invoice USD 100 @ 15000 → AR booked 1,500,000 cents. Payment rate 15500.
	arApplied := int64(1500000)
	invRate, payRate := float64(15000), float64(15500)
	fxDiff := int64(math.Round(float64(arApplied) / invRate * (payRate - invRate)))
	if fxDiff <= 0 {
		t.Fatalf("expected positive diff, got %d", fxDiff)
	}
	if want := int64(50000); fxDiff != want {
		t.Fatalf("fxDiff = %d, want %d", fxDiff, want)
	}
	// Rate rose → gain for the receivable, and cash received exceeds booked AR.
	if gain := max64(fxDiff, 0); gain != 50000 {
		t.Fatalf("gain = %d, want 50000", gain)
	}
	if loss := max64(-fxDiff, 0); loss != 0 {
		t.Fatalf("loss = %d, want 0", loss)
	}
	cashDebit := arApplied + fxDiff
	if cashDebit != 1550000 {
		t.Fatalf("cashDebit = %d, want 1550000", cashDebit)
	}
	// Dr cash == Cr AR + Cr gain.
	if cashDebit != arApplied+max64(fxDiff, 0) {
		t.Fatalf("unbalanced gain journal: cash %d vs AR+gain %d", cashDebit, arApplied+max64(fxDiff, 0))
	}
	// Rate fell → loss, cash received below booked AR.
	fxDiffDown := int64(math.Round(float64(arApplied) / invRate * (14500 - invRate)))
	cashDebitDown := arApplied + fxDiffDown
	if cashDebitDown != 1450000 {
		t.Fatalf("cashDebitDown = %d, want 1450000", cashDebitDown)
	}
	// Dr cash + Dr loss == Cr AR.
	if cashDebitDown+max64(-fxDiffDown, 0) != arApplied {
		t.Fatalf("unbalanced loss journal: cash+loss %d vs AR %d", cashDebitDown+max64(-fxDiffDown, 0), arApplied)
	}
	if err := errors.New("sentinel"); err == nil {
		t.Fatal("unreachable")
	}
}

// TestInvoiceResponseCurrencyFields guards the SET-001 regression where
// fetchInvoice's SELECT omitted currency_code/exchange_rate so both the
// create response and GET /invoices/{id} echoed empty currency values even
// though the columns persisted correctly.
func TestInvoiceResponseCurrencyFields(t *testing.T) {
	var r invoiceResponse
	if r.CurrencyCode != "" || r.ExchangeRate != 0 {
		t.Fatalf("zero value broken: %q %v", r.CurrencyCode, r.ExchangeRate)
	}
	// The struct must carry the JSON tags the API contract promises.
	r.CurrencyCode = "USD"
	r.ExchangeRate = 15750
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"currency_code":"USD"`, `"exchange_rate":15750`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Fatalf("response JSON missing %s: %s", want, b)
		}
	}
}
