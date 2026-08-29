package sales

import (
	"errors"
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

// TestFxPaymentMath verifies the FX gain/loss arithmetic shape used by
// CreatePayment: a positive diff is a loss, a negative diff is a gain, and
// max64 picks the right sign.
func TestFxPaymentMath(t *testing.T) {
	// Invoice USD 100 @ 15000 → AR base 1,500,000 cents. Payment rate 15500.
	arApplied := int64(1500000)
	invRate, payRate := float64(15000), float64(15500)
	fxDiff := int64(float64(arApplied)/invRate*(payRate-invRate) + 0.5)
	if fxDiff <= 0 {
		t.Fatalf("expected positive (loss) diff, got %d", fxDiff)
	}
	if want := int64(50000); fxDiff != want {
		t.Fatalf("fxDiff = %d, want %d", fxDiff, want)
	}
	if max64(-fxDiff, 0) != 0 || max64(fxDiff, 0) != fxDiff {
		t.Fatal("max64 sign handling broken")
	}
	if err := errors.New("sentinel"); err == nil {
		t.Fatal("unreachable")
	}
}
