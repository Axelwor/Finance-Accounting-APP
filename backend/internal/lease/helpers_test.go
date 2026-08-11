package lease

import (
	"testing"
	"time"
)

func TestPresentValueCents(t *testing.T) {
	tests := []struct {
		name       string
		payment    int64
		rate       float64
		n          int
		wantApprox int64 // expected within rounding tolerance
	}{
		{
			name:       "monthly 1M at 1% for 12 months",
			payment:    1_000_000,
			rate:       0.01,
			n:          12,
			wantApprox: 11_255_077,
		},
		{
			name:       "quarterly 3M at 3% for 4 quarters",
			payment:    3_000_000,
			rate:       0.03,
			n:          4,
			wantApprox: 11_151_295,
		},
		{
			name:       "zero rate returns undiscounted sum (m-013 fix)",
			payment:    1_000_000,
			rate:       0,
			n:          12,
			wantApprox: 12_000_000, // payment * n
		},
		{
			name:       "zero payments returns zero",
			payment:    1_000_000,
			rate:       0.01,
			n:          0,
			wantApprox: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := presentValueCents(tc.payment, tc.rate, tc.n)
			if tc.wantApprox == 0 {
				if got != 0 {
					t.Fatalf("expected 0, got %d", got)
				}
				return
			}
			// Allow ±1 cent rounding tolerance.
			if got < tc.wantApprox-1 || got > tc.wantApprox+1 {
				t.Fatalf("PV = %d, expected approx %d", got, tc.wantApprox)
			}
		})
	}
}

func TestParseDiscountRate(t *testing.T) {
	tests := []struct {
		raw  string
		want float64
	}{
		{"0.01", 0.01},
		{"0.03", 0.03},
		{"0.005", 0.005},
		{"invalid", 0},
		{"", 0},
	}
	for _, tc := range tests {
		got := parseDiscountRate(tc.raw)
		if got != tc.want {
			t.Errorf("parseDiscountRate(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestBuildPaymentSchedule(t *testing.T) {
	// 1M monthly at 1% for 12 months, PV ~11,255,077
	pv := presentValueCents(1_000_000, 0.01, 12)
	if pv <= 0 {
		t.Fatalf("PV must be positive, got %d", pv)
	}

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule := buildPaymentSchedule(startDate, freqMonthly, 12, 1_000_000, 0.01, pv)

	if len(schedule) != 12 {
		t.Fatalf("expected 12 payments, got %d", len(schedule))
	}

	// First payment: interest = pv * 0.01, principal = 1M - interest.
	expectedInterest1 := int64(float64(pv) * 0.01)
	if schedule[0].InterestCents != expectedInterest1 {
		t.Errorf("payment 1 interest = %d, expected %d", schedule[0].InterestCents, expectedInterest1)
	}
	expectedPrincipal1 := 1_000_000 - expectedInterest1
	if schedule[0].PrincipalCents != expectedPrincipal1 {
		t.Errorf("payment 1 principal = %d, expected %d", schedule[0].PrincipalCents, expectedPrincipal1)
	}

	// Last payment should bring remaining to 0.
	if schedule[11].RemainingLiabilityCents != 0 {
		t.Errorf("last payment remaining = %d, expected 0", schedule[11].RemainingLiabilityCents)
	}

	// Total principal should equal PV (within rounding tolerance — each
	// payment rounds interest/principal independently).
	var totalPrincipal int64
	for _, p := range schedule {
		totalPrincipal += p.PrincipalCents
	}
	if totalPrincipal < pv-50 || totalPrincipal > pv+50 {
		t.Errorf("total principal = %d, expected approx PV %d", totalPrincipal, pv)
	}

	// Payment dates should advance by 1 month each.
	if schedule[0].PaymentDate.Format("2006-01") != "2025-01" {
		t.Errorf("payment 1 date = %s, expected 2025-01", schedule[0].PaymentDate.Format("2006-01"))
	}
	if schedule[1].PaymentDate.Format("2006-01") != "2025-02" {
		t.Errorf("payment 2 date = %s, expected 2025-02", schedule[1].PaymentDate.Format("2006-01"))
	}
	if schedule[11].PaymentDate.Format("2006-01") != "2025-12" {
		t.Errorf("payment 12 date = %s, expected 2025-12", schedule[11].PaymentDate.Format("2006-01"))
	}
}

func TestBuildPaymentScheduleQuarterly(t *testing.T) {
	pv := presentValueCents(3_000_000, 0.03, 4)
	if pv <= 0 {
		t.Fatalf("PV must be positive, got %d", pv)
	}

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule := buildPaymentSchedule(startDate, freqQuarterly, 4, 3_000_000, 0.03, pv)

	if len(schedule) != 4 {
		t.Fatalf("expected 4 payments, got %d", len(schedule))
	}

	// Payment dates should advance by 3 months each.
	if schedule[0].PaymentDate.Format("2006-01") != "2025-01" {
		t.Errorf("payment 1 date = %s, expected 2025-01", schedule[0].PaymentDate.Format("2006-01"))
	}
	if schedule[1].PaymentDate.Format("2006-01") != "2025-04" {
		t.Errorf("payment 2 date = %s, expected 2025-04", schedule[1].PaymentDate.Format("2006-01"))
	}
	// Payment 4 = start + 9 months = 2025-10 (Jan, Apr, Jul, Oct).
	if schedule[3].PaymentDate.Format("2006-01") != "2025-10" {
		t.Errorf("payment 4 date = %s, expected 2025-10", schedule[3].PaymentDate.Format("2006-01"))
	}

	// Total principal should equal PV (allow ±5 cents rounding per step).
	var totalPrincipal int64
	for _, p := range schedule {
		totalPrincipal += p.PrincipalCents
	}
	if totalPrincipal < pv-5 || totalPrincipal > pv+5 {
		t.Errorf("total principal = %d, expected approx PV %d", totalPrincipal, pv)
	}
}

func TestFrequencyMonths(t *testing.T) {
	if frequencyMonths(freqMonthly) != 1 {
		t.Error("MONTHLY should be 1 month")
	}
	if frequencyMonths(freqQuarterly) != 3 {
		t.Error("QUARTERLY should be 3 months")
	}
	if frequencyMonths(freqAnnually) != 12 {
		t.Error("ANNUALLY should be 12 months")
	}
	if frequencyMonths("UNKNOWN") != 1 {
		t.Error("unknown frequency should default to 1 month")
	}
}

func TestFormatIntSlice(t *testing.T) {
	if got := formatIntSlice([]int64{1, 2, 3}); got != "[1, 2, 3]" {
		t.Errorf("formatIntSlice = %q", got)
	}
	if got := formatIntSlice(nil); got != "[]" {
		t.Errorf("formatIntSlice(nil) = %q", got)
	}
}
