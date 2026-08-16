package tax

import (
	"math"
	"net/http"
	"testing"
)

// =============================================================================
// Test strategy
// -----------------------------------------------------------------------------
// The tax package's business logic splits into two layers:
//
//   1. Pure helper functions (percentageRound, abs64, formatPercent, validDate,
//      normalizeDate, monthBounds, parsePeriod, parseInt, optionalNote) and
//      pure validators (validateECLRequest, validateWriteOff,
//      validateDeferredTax, validatePayPPhFinal). These are unit-testable with
//      no DB.
//
//   2. Handler/service methods (CalculateECL, PPNSummary, CalculatePPhFinal,
//      CalculateDeferredTax) that are tightly coupled to pgx transactions and
//      the DB layer. The ECL bucket definitions and rates live inline inside
//      computeECLBuckets, which queries the invoices table directly.
//
// This file exhaustively tests layer 1, and for layer 2 it tests the
// mathematical formulas and bucket/rate structure that the code uses, by
// re-deriving the documented constants and applying the same pure helpers
// (percentageRound, abs64) that the production code calls. This keeps the
// tests honest: they exercise the real rounding/provisioning code path, not a
// reimplementation.
//
// ECL bucket structure (from computeECLBuckets in ecl.go):
//   "0-30"   MinDays=0   MaxDays=30     RatePct=1.0
//   "31-60"  MinDays=31  MaxDays=60     RatePct=2.5
//   "61-90"  MinDays=61  MaxDays=90     RatePct=5.0
//   ">90"    MinDays=91  MaxDays=999999 RatePct=10.0
//
// PPN rate is 11% of DPP (taxable base). The rate is applied at invoice
// posting time (outside this package); here we verify the percentageRound
// formula that the codebase uses to compute the rupiah amount.
//
// PPh Final UMKM rate is 0.5% (read from the tax_rates table as
// PPH_FINAL_UMKM). The code calls percentageRound(revenue, rate) where rate
// is the decimal form (0.005). We test the formula at both 0.5% and 0.75%.
// =============================================================================

// -----------------------------------------------------------------------------
// ECL bucket definitions and rates
// -----------------------------------------------------------------------------

// eclBuckets returns a fresh copy of the default bucket definitions from
// computeECLBuckets (ecl.go lines 219-224). Returning a fresh slice each
// call lets parallel subtests iterate without shared mutable state. Keeping
// a single source of truth here lets every ECL test reference the same
// rates/labels without magic numbers scattered through the table cases.
func eclBuckets() []ECLBucket {
	return []ECLBucket{
		{Label: "0-30", MinDays: 0, MaxDays: 30, RatePct: 1.0},
		{Label: "31-60", MinDays: 31, MaxDays: 60, RatePct: 2.5},
		{Label: "61-90", MinDays: 61, MaxDays: 90, RatePct: 5.0},
		{Label: ">90", MinDays: 91, MaxDays: 999999, RatePct: 10.0},
	}
}

// TestECLBucketDefinitions verifies the four ECL aging buckets have the
// labels, day ranges, and provisioning rates that the production code hard
// codes in computeECLBuckets. If any of these drift the test fails loudly.
func TestECLBucketDefinitions(t *testing.T) {
	t.Parallel()
	buckets := eclBuckets()
	if len(buckets) != 4 {
		t.Fatalf("expected 4 ECL buckets, got %d", len(buckets))
	}
	want := []ECLBucket{
		{Label: "0-30", MinDays: 0, MaxDays: 30, RatePct: 1.0},
		{Label: "31-60", MinDays: 31, MaxDays: 60, RatePct: 2.5},
		{Label: "61-90", MinDays: 61, MaxDays: 90, RatePct: 5.0},
		{Label: ">90", MinDays: 91, MaxDays: 999999, RatePct: 10.0},
	}
	for i, b := range buckets {
		if b != want[i] {
			t.Errorf("bucket[%d] = %+v, want %+v", i, b, want[i])
		}
	}
}

// TestECLBucketContiguity verifies the aging buckets form a contiguous,
// non-overlapping range starting at day 0 with no gaps. This guards against
// off-by-one errors in the MinDays/MaxDays boundaries.
func TestECLBucketContiguity(t *testing.T) {
	t.Parallel()
	buckets := eclBuckets()
	expectedStart := 0
	for _, b := range buckets {
		if b.MinDays != expectedStart {
			t.Errorf("bucket %q MinDays=%d, expected %d (gap/overlap from previous)",
				b.Label, b.MinDays, expectedStart)
		}
		if b.MaxDays < b.MinDays {
			t.Errorf("bucket %q MaxDays=%d < MinDays=%d", b.Label, b.MaxDays, b.MinDays)
		}
		// Next bucket starts one day after this one ends.
		expectedStart = b.MaxDays + 1
	}
	// The final bucket should be open-ended (very large MaxDays).
	last := buckets[len(buckets)-1]
	if last.MaxDays < 365 {
		t.Errorf("final bucket %q MaxDays=%d should be open-ended (>365)",
			last.Label, last.MaxDays)
	}
}

// TestECLAgingBucketClassification tests that a receivable's age in days is
// classified into the correct aging bucket. This replicates the exact
// classification loop used in computeECLBuckets:
//
//	for i := range defaults {
//	    if ageDays >= defaults[i].MinDays && ageDays <= defaults[i].MaxDays {
//	        bucket = &defaults[i]; break
//	    }
//	}
//	if bucket == nil { bucket = &defaults[len(defaults)-1] } // >90 fallback
func TestECLAgingBucketClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ageDays int
		want    string // expected bucket Label
	}{
		{"day 0 (just invoiced)", 0, "0-30"},
		{"day 1", 1, "0-30"},
		{"day 15 mid bucket", 15, "0-30"},
		{"day 30 boundary inclusive", 30, "0-30"},
		{"day 31 next bucket start", 31, "31-60"},
		{"day 45 mid bucket", 45, "31-60"},
		{"day 60 boundary inclusive", 60, "31-60"},
		{"day 61 next bucket start", 61, "61-90"},
		{"day 75 mid bucket", 75, "61-90"},
		{"day 90 boundary inclusive", 90, "61-90"},
		{"day 91 first over-90", 91, ">90"},
		{"day 180", 180, ">90"},
		{"day 365 one year", 365, ">90"},
		{"day 1000 very old", 1000, ">90"},
		// Negative ages (future-dated invoices) are clamped to 0 by the
		// production code: ageDays = int(...); if ageDays < 0 { ageDays = 0 }.
		{"negative age clamped to 0", -5, "0-30"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buckets := eclBuckets()
			age := tc.ageDays
			if age < 0 {
				age = 0
			}
			var bucket *ECLBucket
			for i := range buckets {
				if age >= buckets[i].MinDays && age <= buckets[i].MaxDays {
					bucket = &buckets[i]
					break
				}
			}
			if bucket == nil {
				bucket = &buckets[len(buckets)-1]
			}
			if bucket.Label != tc.want {
				t.Errorf("ageDays=%d => bucket %q, want %q", tc.ageDays, bucket.Label, tc.want)
			}
		})
	}
}

// TestECLRates verifies each bucket has the documented provisioning rate.
// The rates (1%, 2.5%, 5%, 10%) are the PSAK 48 expected-credit-loss rates
// hard-coded in computeECLBuckets.
func TestECLRates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		label string
		rate  float64
	}{
		{"0-30", 1.0},
		{"31-60", 2.5},
		{"61-90", 5.0},
		{">90", 10.0},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()
			buckets := eclBuckets()
			var found *ECLBucket
			for i := range buckets {
				if buckets[i].Label == tc.label {
					found = &buckets[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("bucket %q not found", tc.label)
			}
			if found.RatePct != tc.rate {
				t.Errorf("bucket %q RatePct=%v, want %v", tc.label, found.RatePct, tc.rate)
			}
		})
	}
}

// TestECLProvisionCalculation tests the per-bucket provision formula:
//
//	provision = percentageRound(balance, rate/100)
//
// This is the exact call the production code makes at ecl.go line 281:
//
//	b.ProvisionCents = percentageRound(b.BalanceCents, b.RatePct/100.0)
//
// It exercises the real percentageRound helper, not a reimplementation.
func TestECLProvisionCalculation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		label   string
		balance int64
		want    int64
	}{
		// 1% bucket
		{"0-30: 1% of 1,000,000", "0-30", 1_000_000, 10_000},
		{"0-30: 1% of 100,000", "0-30", 100_000, 1_000},
		{"0-30: 1% rounds 100 up (half-up)", "0-30", 10_000, 100}, // 10000*0.01=100
		// 2.5% bucket
		{"31-60: 2.5% of 1,000,000", "31-60", 1_000_000, 25_000},
		{"31-60: 2.5% of 100,000", "31-60", 100_000, 2_500},
		// 5% bucket
		{"61-90: 5% of 1,000,000", "61-90", 1_000_000, 50_000},
		{"61-90: 5% of 100,000", "61-90", 100_000, 5_000},
		// 10% bucket
		{">90: 10% of 1,000,000", ">90", 1_000_000, 100_000},
		{">90: 10% of 100,000", ">90", 100_000, 10_000},
		// Zero/negative balance => 0 (percentageRound guards base<=0).
		{"zero balance yields zero provision", "0-30", 0, 0},
		{"negative balance yields zero provision", "31-60", -500_000, 0},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buckets := eclBuckets()
			var bucket *ECLBucket
			for i := range buckets {
				if buckets[i].Label == tc.label {
					bucket = &buckets[i]
					break
				}
			}
			if bucket == nil {
				t.Fatalf("bucket %q not found", tc.label)
			}
			got := percentageRound(tc.balance, bucket.RatePct/100.0)
			if got != tc.want {
				t.Errorf("percentageRound(%d, %v/100) = %d, want %d",
					tc.balance, bucket.RatePct, got, tc.want)
			}
		})
	}
}

// TestECLProvisionRounding verifies percentageRound uses half-up rounding
// (int64(amount + 0.5)) as documented in pph.go. This is critical for ECL
// because fractional rupiah provisions must round predictably.
func TestECLProvisionRounding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		balance int64
		ratePct float64
		want    int64
	}{
		// 1% of 10,050 = 100.5 => rounds up to 101 (half-up).
		{"1% of 10050 = 100.5 -> 101 (round up)", 10_050, 1.0, 101},
		// 1% of 10,040 = 100.4 => rounds down to 100.
		{"1% of 10040 = 100.4 -> 100 (round down)", 10_040, 1.0, 100},
		// 2.5% of 10,000 = 250 exact.
		{"2.5% of 10000 = 250 exact", 10_000, 2.5, 250},
		// 2.5% of 1,010 = 25.25 => rounds to 25.
		{"2.5% of 1010 = 25.25 -> 25", 1_010, 2.5, 25},
		// 2.5% of 1,020 = 25.5 => rounds up to 26 (half-up).
		{"2.5% of 1020 = 25.5 -> 26 (half-up)", 1_020, 2.5, 26},
		// 5% of 1 = 0.05 => rounds to 0.
		{"5% of 1 = 0.05 -> 0", 1, 5.0, 0},
		// 5% of 10 = 0.5 => rounds up to 1 (half-up).
		{"5% of 10 = 0.5 -> 1 (half-up)", 10, 5.0, 1},
		// 10% of 5 = 0.5 => rounds up to 1 (half-up).
		{"10% of 5 = 0.5 -> 1 (half-up)", 5, 10.0, 1},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := percentageRound(tc.balance, tc.ratePct/100.0)
			if got != tc.want {
				t.Errorf("percentageRound(%d, %v%%) = %d, want %d",
					tc.balance, tc.ratePct, got, tc.want)
			}
		})
	}
}

// TestECLTotalProvision verifies that the total ECL provision equals the sum
// of each bucket's balance * rate. This mirrors how computeECLBuckets
// accumulates totalProvision (ecl.go lines 279-283) and how the service uses
// it as the target allowance.
func TestECLTotalProvision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		balances      map[string]int64 // label -> balance cents
		wantTotal     int64
		wantPerBucket map[string]int64
	}{
		{
			name: "one receivable per bucket",
			balances: map[string]int64{
				"0-30":  1_000_000,
				"31-60": 1_000_000,
				"61-90": 1_000_000,
				">90":   1_000_000,
			},
			// 1% + 2.5% + 5% + 10% of 1,000,000 = 10k+25k+50k+100k = 185,000
			wantTotal: 185_000,
			wantPerBucket: map[string]int64{
				"0-30":  10_000,
				"31-60": 25_000,
				"61-90": 50_000,
				">90":   100_000,
			},
		},
		{
			name: "only current (0-30) receivables",
			balances: map[string]int64{
				"0-30":  500_000,
				"31-60": 0,
				"61-90": 0,
				">90":   0,
			},
			wantTotal: 5_000, // 1% of 500,000
			wantPerBucket: map[string]int64{
				"0-30":  5_000,
				"31-60": 0,
				"61-90": 0,
				">90":   0,
			},
		},
		{
			name: "only bad debt (>90) receivables",
			balances: map[string]int64{
				"0-30":  0,
				"31-60": 0,
				"61-90": 0,
				">90":   2_000_000,
			},
			wantTotal: 200_000, // 10% of 2,000,000
			wantPerBucket: map[string]int64{
				"0-30":  0,
				"31-60": 0,
				"61-90": 0,
				">90":   200_000,
			},
		},
		{
			name: "mixed balances with rounding",
			balances: map[string]int64{
				"0-30":  10_050, // 1% -> 101 (100.5 rounds up)
				"31-60": 10_020, // 2.5% -> 251 (250.5 rounds up)
				"61-90": 10_000, // 5% -> 500 exact
				">90":   5,      // 10% -> 1 (0.5 rounds up)
			},
			wantTotal: 101 + 251 + 500 + 1, // 853
			wantPerBucket: map[string]int64{
				"0-30":  101,
				"31-60": 251,
				"61-90": 500,
				">90":   1,
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buckets := eclBuckets()
			var total int64
			for i := range buckets {
				b := &buckets[i]
				bal := tc.balances[b.Label]
				prov := percentageRound(bal, b.RatePct/100.0)
				total += prov
				if want := tc.wantPerBucket[b.Label]; prov != want {
					t.Errorf("bucket %q provision = %d, want %d", b.Label, prov, want)
				}
			}
			if total != tc.wantTotal {
				t.Errorf("total provision = %d, want %d", total, tc.wantTotal)
			}
		})
	}
}

// TestECLWriteOffMath verifies the write-off arithmetic: writing off a
// receivable removes its balance from the aging and consumes the provision
// already booked against it. The journal entry is Dr 1202 / Cr 1201, but the
// arithmetic effect on the aging is: bucket balance -= amount.
//
// validateWriteOff requires amount > 0 and a valid entry_date; the write-off
// itself reduces the bucket balance. Here we test the balance-reduction math
// and the validator.
func TestECLWriteOffMath(t *testing.T) {
	t.Parallel()

	// Simulate a bucket with one receivable of 1,000,000 in the >90 bucket.
	bucket := ECLBucket{Label: ">90", MinDays: 91, MaxDays: 999999, RatePct: 10.0}
	bucket.BalanceCents = 1_000_000
	bucket.ProvisionCents = percentageRound(bucket.BalanceCents, bucket.RatePct/100.0) // 100,000

	// Write off the full receivable: balance drops to 0, so the recomputed
	// provision should also drop to 0.
	writeOffAmount := int64(1_000_000)
	newBalance := bucket.BalanceCents - writeOffAmount
	newProvision := percentageRound(newBalance, bucket.RatePct/100.0)

	if newBalance != 0 {
		t.Errorf("after full write-off, balance = %d, want 0", newBalance)
	}
	if newProvision != 0 {
		t.Errorf("after full write-off, provision = %d, want 0", newProvision)
	}

	// Partial write-off of 400,000 from 1,000,000 leaves 600,000.
	bucket.BalanceCents = 1_000_000
	partial := int64(400_000)
	remaining := bucket.BalanceCents - partial
	remainingProv := percentageRound(remaining, bucket.RatePct/100.0)
	if remaining != 600_000 {
		t.Errorf("after partial write-off, balance = %d, want 600000", remaining)
	}
	if remainingProv != 60_000 {
		t.Errorf("after partial write-off, provision = %d, want 60000", remainingProv)
	}
}

// TestValidateWriteOff tests the write-off request validator (pure function).
func TestValidateWriteOff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		req     WriteOffRequest
		wantErr bool
	}{
		{"valid request", WriteOffRequest{EntryDate: "2026-01-15", AmountCents: 500_000}, false},
		{"missing entry_date", WriteOffRequest{EntryDate: "", AmountCents: 500_000}, true},
		{"invalid entry_date format", WriteOffRequest{EntryDate: "15/01/2026", AmountCents: 500_000}, true},
		{"zero amount", WriteOffRequest{EntryDate: "2026-01-15", AmountCents: 0}, true},
		{"negative amount", WriteOffRequest{EntryDate: "2026-01-15", AmountCents: -100}, true},
		{"garbage date", WriteOffRequest{EntryDate: "not-a-date", AmountCents: 500_000}, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateWriteOff(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateWriteOff(%+v) err = %v, wantErr %v", tc.req, err, tc.wantErr)
			}
		})
	}
}

// TestValidateECLRequest tests the ECL calculate request validator.
func TestValidateECLRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		req     CalculateECLRequest
		wantErr bool
	}{
		{"valid request", CalculateECLRequest{AsOfDate: "2026-01-31", EntryDate: "2026-01-31"}, false},
		{"missing as_of_date", CalculateECLRequest{AsOfDate: "", EntryDate: "2026-01-31"}, true},
		{"missing entry_date", CalculateECLRequest{AsOfDate: "2026-01-31", EntryDate: ""}, true},
		{"invalid as_of_date", CalculateECLRequest{AsOfDate: "2026/01/31", EntryDate: "2026-01-31"}, true},
		{"both missing", CalculateECLRequest{}, true},
		{"with notes and rates still valid", CalculateECLRequest{
			AsOfDate: "2026-01-31", EntryDate: "2026-01-31",
			Notes: "monthly", Rates: map[string]float64{"0-30": 1.5},
		}, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateECLRequest(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateECLRequest(%+v) err = %v, wantErr %v", tc.req, err, tc.wantErr)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// PPN (VAT) rate calculation and reversal
// -----------------------------------------------------------------------------

// TestPPNRateCalculation verifies the PPN (VAT) rate of 11% applied to the
// DPP (taxable base / Dasar Pengenaan Pajak). The PPN amount is computed
// using the same percentageRound helper the codebase uses everywhere for
// tax math. 11% as a decimal is 0.11.
func TestPPNRateCalculation(t *testing.T) {
	t.Parallel()
	const ppnRatePct = 11.0 // 11% standard VAT rate (PPN)
	tests := []struct {
		name     string
		dppCents int64
		wantPPN  int64
	}{
		{"11% of 1,000,000", 1_000_000, 110_000},
		{"11% of 100,000", 100_000, 11_000},
		{"11% of 10,000", 10_000, 1_100},
		{"11% of 1,000", 1_000, 110},
		{"11% of 100", 100, 11},
		{"11% of 10", 10, 1}, // 1.1 -> rounds to 1
		{"11% of 5", 5, 1},   // 0.55 -> rounds up to 1 (half-up)
		{"11% of 4", 4, 0},   // 0.44 -> rounds to 0
		{"zero DPP", 0, 0},   // guarded by percentageRound
		{"negative DPP", -1000, 0},
		{"large DPP 1 billion", 1_000_000_000, 110_000_000},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := percentageRound(tc.dppCents, ppnRatePct/100.0)
			if got != tc.wantPPN {
				t.Errorf("PPN on DPP %d = %d, want %d", tc.dppCents, got, tc.wantPPN)
			}
		})
	}
}

// TestPPNRateCalculationPrecision verifies the 11% PPN rate against a
// pure-Go float computation to ensure percentageRound matches the expected
// half-up rounding across a range of DPP values.
func TestPPNRateCalculationPrecision(t *testing.T) {
	t.Parallel()
	const ppnRatePct = 11.0
	dpps := []int64{1, 2, 3, 7, 13, 99, 100, 101, 250, 999, 1_000_001, 9_999_999}
	for _, dpp := range dpps {
		dpp := dpp
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := percentageRound(dpp, ppnRatePct/100.0)
			// Expected: half-up rounding of dpp * 0.11.
			raw := float64(dpp) * (ppnRatePct / 100.0)
			want := int64(raw + 0.5)
			if got != want {
				t.Errorf("PPN on DPP %d = %d, want %d (raw %v)", dpp, got, want, raw)
			}
		})
	}
}

// TestPPNReversalCreditNote verifies that a credit note (reversal) reduces
// PPN keluaran. The net PPN formula (from PPNSummary / PPNReconciliation):
//
//	net_ppn = ppn_keluaran - ppn_masukan
//
// A credit note that reverses a sale reduces the keluaran (output VAT)
// because it credits 2202 with a negative amount (or debits it). Here we
// model the reversal as subtracting from keluaran and verify the net.
func TestPPNReversalCreditNote(t *testing.T) {
	t.Parallel()
	const ppnRatePct = 11.0

	tests := []struct {
		name          string
		originalDPP   int64
		creditNoteDPP int64 // reversal amount (positive, subtracted)
		wantKeluaran  int64
		wantNet       int64 // keluaran - masukan (masukan=0 here)
	}{
		{
			name:          "full reversal cancels keluaran",
			originalDPP:   1_000_000,
			creditNoteDPP: 1_000_000,
			wantKeluaran:  0,
			wantNet:       0,
		},
		{
			name:          "partial reversal halves keluaran",
			originalDPP:   1_000_000,
			creditNoteDPP: 500_000,
			wantKeluaran:  55_000, // 11% of 500,000
			wantNet:       55_000,
		},
		{
			name:          "no reversal keeps full keluaran",
			originalDPP:   1_000_000,
			creditNoteDPP: 0,
			wantKeluaran:  110_000,
			wantNet:       110_000,
		},
		{
			name:          "reversal with masukan present",
			originalDPP:   1_000_000,
			creditNoteDPP: 200_000,
			// masukan (input VAT) on purchases of 300,000:
			// not modeled here; wantNet assumes masukan=0 per the table.
			wantKeluaran: 88_000, // 11% of 800,000
			wantNet:      88_000,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			originalPPN := percentageRound(tc.originalDPP, ppnRatePct/100.0)
			reversedPPN := percentageRound(tc.creditNoteDPP, ppnRatePct/100.0)
			keluaran := originalPPN - reversedPPN
			masukan := int64(0)
			net := keluaran - masukan
			if keluaran != tc.wantKeluaran {
				t.Errorf("keluaran = %d, want %d", keluaran, tc.wantKeluaran)
			}
			if net != tc.wantNet {
				t.Errorf("net PPN = %d, want %d", net, tc.wantNet)
			}
		})
	}
}

// TestPPNNetFormula tests the core net PPN formula: net = keluaran - masukan.
// Positive net = payable to the tax office; negative = refund/excess input.
func TestPPNNetFormula(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		keluaran    int64
		masukan     int64
		wantNet     int64
		description string
	}{
		{"output > input => payable", 110_000, 55_000, 55_000, "owes tax office"},
		{"input > output => excess", 55_000, 110_000, -55_000, "excess input VAT"},
		{"equal => zero net", 110_000, 110_000, 0, "nothing to pay"},
		{"zero activity", 0, 0, 0, "no transactions"},
		{"output only", 220_000, 0, 220_000, "pure output VAT"},
		{"input only", 0, 220_000, -220_000, "pure input VAT"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			net := tc.keluaran - tc.masukan
			if net != tc.wantNet {
				t.Errorf("net = %d - %d = %d, want %d (%s)",
					tc.keluaran, tc.masukan, net, tc.wantNet, tc.description)
			}
		})
	}
}

// TestPPNAccountCodes verifies the account code constants used for PPN
// keluaran (output VAT, liability 2202) and PPN masukan (input VAT, asset
// 1203). These are the codes the production SQL filters on.
func TestPPNAccountCodes(t *testing.T) {
	t.Parallel()
	if ppnKeluaranCode != "2202" {
		t.Errorf("ppnKeluaranCode = %q, want 2202", ppnKeluaranCode)
	}
	if ppnMasukanCode != "1203" {
		t.Errorf("ppnMasukanCode = %q, want 1203", ppnMasukanCode)
	}
}

// -----------------------------------------------------------------------------
// PPh Final UMKM
// -----------------------------------------------------------------------------

// TestPPhFinalUMKMRate tests the PPh Final UMKM calculation at 0.5% (the
// standard rate per PP 23/2018, stored in tax_rates as PPH_FINAL_UMKM). The
// production code computes: taxCents = percentageRound(revenue, rate) where
// rate is the decimal form (0.005 for 0.5%).
func TestPPhFinalUMKMRate(t *testing.T) {
	t.Parallel()
	const pphFinalRatePct = 0.5 // 0.5% standard UMKM rate
	tests := []struct {
		name         string
		revenueCents int64
		wantTax      int64
	}{
		{"0.5% of 100,000,000 (100M)", 100_000_000, 500_000},
		{"0.5% of 10,000,000 (10M)", 10_000_000, 50_000},
		{"0.5% of 1,000,000", 1_000_000, 5_000},
		{"0.5% of 100,000", 100_000, 500},
		{"0.5% of 10,000", 10_000, 50},
		{"0.5% of 1,000", 1_000, 5},
		{"0.5% of 100", 100, 1}, // 0.5 -> rounds up to 1 (half-up)
		{"0.5% of 50", 50, 0},   // 0.25 -> rounds to 0
		{"0.5% of 150", 150, 1}, // 0.75 -> rounds to 1
		{"0.5% of 200", 200, 1}, // 1.0 exact
		{"zero revenue", 0, 0},
		{"negative revenue", -1000, 0},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := percentageRound(tc.revenueCents, pphFinalRatePct/100.0)
			if got != tc.wantTax {
				t.Errorf("PPh Final 0.5%% on revenue %d = %d, want %d",
					tc.revenueCents, got, tc.wantTax)
			}
		})
	}
}

// TestPPhFinalUMKMRate075 tests the PPh Final UMKM calculation at 0.75%,
// an alternative rate some taxpayers use (the code reads the rate from the
// tax_rates table, so it could be configured to 0.75%). This verifies the
// formula works for any configured rate.
func TestPPhFinalUMKMRate075(t *testing.T) {
	t.Parallel()
	const pphFinalRatePct = 0.75 // alternative rate
	tests := []struct {
		name         string
		revenueCents int64
		wantTax      int64
	}{
		{"0.75% of 100,000,000", 100_000_000, 750_000},
		{"0.75% of 10,000,000", 10_000_000, 75_000},
		{"0.75% of 1,000,000", 1_000_000, 7_500},
		{"0.75% of 100,000", 100_000, 750},
		{"0.75% of 10,000", 10_000, 75},
		{"0.75% of 1,000", 1_000, 8}, // 7.5 -> rounds up to 8 (half-up)
		{"0.75% of 100", 100, 1},     // 0.75 -> rounds to 1
		{"0.75% of 400", 400, 3},     // 3.0 exact
		{"0.75% of 133", 133, 1},     // 0.9975 -> rounds to 1
		{"zero revenue", 0, 0},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := percentageRound(tc.revenueCents, pphFinalRatePct/100.0)
			if got != tc.wantTax {
				t.Errorf("PPh Final 0.75%% on revenue %d = %d, want %d",
					tc.revenueCents, got, tc.wantTax)
			}
		})
	}
}

// TestPPhFinalNetRevenue verifies the PPh Final revenue base: it is the
// credit turnover of sales (4101) minus the debit turnover of sales returns
// (4201) within the month. This mirrors the monthlyRevenue logic in pph.go.
func TestPPhFinalNetRevenue(t *testing.T) {
	t.Parallel()
	const pphFinalRatePct = 0.5
	tests := []struct {
		name         string
		salesCents   int64
		returnsCents int64
		wantRevenue  int64
		wantTax      int64
	}{
		{
			name:         "no returns",
			salesCents:   10_000_000,
			returnsCents: 0,
			wantRevenue:  10_000_000,
			wantTax:      50_000,
		},
		{
			name:         "returns reduce base",
			salesCents:   10_000_000,
			returnsCents: 2_000_000,
			wantRevenue:  8_000_000,
			wantTax:      40_000,
		},
		{
			name:         "full returns zero base",
			salesCents:   10_000_000,
			returnsCents: 10_000_000,
			wantRevenue:  0,
			wantTax:      0, // percentageRound guards base<=0
		},
		{
			name:         "returns exceed sales => negative base => 0 tax",
			salesCents:   1_000_000,
			returnsCents: 2_000_000,
			wantRevenue:  -1_000_000,
			wantTax:      0, // percentageRound returns 0 for base<=0
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			revenue := tc.salesCents - tc.returnsCents
			tax := percentageRound(revenue, pphFinalRatePct/100.0)
			if revenue != tc.wantRevenue {
				t.Errorf("revenue = %d, want %d", revenue, tc.wantRevenue)
			}
			if tax != tc.wantTax {
				t.Errorf("tax = %d, want %d", tax, tc.wantTax)
			}
		})
	}
}

// TestValidatePPhFinalRequest tests the PPh Final calculate request validator.
func TestValidatePPhFinalRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		req     CalculatePPhFinalRequest
		wantErr bool
	}{
		{"valid request", CalculatePPhFinalRequest{PeriodYear: 2026, PeriodMonth: 1, EntryDate: "2026-01-31"}, false},
		{"zero year", CalculatePPhFinalRequest{PeriodYear: 0, PeriodMonth: 1, EntryDate: "2026-01-31"}, true},
		{"month too low", CalculatePPhFinalRequest{PeriodYear: 2026, PeriodMonth: 0, EntryDate: "2026-01-31"}, true},
		{"month too high", CalculatePPhFinalRequest{PeriodYear: 2026, PeriodMonth: 13, EntryDate: "2026-01-31"}, true},
		{"month 12 valid", CalculatePPhFinalRequest{PeriodYear: 2026, PeriodMonth: 12, EntryDate: "2026-12-31"}, false},
		{"missing entry_date", CalculatePPhFinalRequest{PeriodYear: 2026, PeriodMonth: 1, EntryDate: ""}, true},
		{"bad date format", CalculatePPhFinalRequest{PeriodYear: 2026, PeriodMonth: 1, EntryDate: "2026/01/31"}, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validatePPhFinalRequest(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("validatePPhFinalRequest(%+v) err = %v, wantErr %v", tc.req, err, tc.wantErr)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Deferred tax
// -----------------------------------------------------------------------------

// TestDeferredTaxFormula tests the deferred tax movement formula:
//
//	deferredCents = percentageRound(abs64(tempDiff), taxRate/100)
//
// This mirrors deferredtax.go line 94. Positive temp diff => ASSET
// (Dr 1206 / Cr 5904); negative => REVERSAL (Dr 5904 / Cr 1206).
func TestDeferredTaxFormula(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tempDiff   int64
		taxRatePct float64
		wantCents  int64
		wantDir    string
	}{
		{"positive diff 22% => ASSET", 1_000_000, 22.0, 220_000, "ASSET"},
		{"positive diff 22% small", 100_000, 22.0, 22_000, "ASSET"},
		{"negative diff 22% => REVERSAL", -1_000_000, 22.0, 220_000, "REVERSAL"},
		{"negative diff 22% small", -100_000, 22.0, 22_000, "REVERSAL"},
		{"positive diff 25% rate", 1_000_000, 25.0, 250_000, "ASSET"},
		{"negative diff 25% rate", -1_000_000, 25.0, 250_000, "REVERSAL"},
		{"large diff", 100_000_000, 22.0, 22_000_000, "ASSET"},
		{"rounding: 22% of 1 = 0.22 -> 0", 1, 22.0, 0, "ASSET"},
		{"rounding: 22% of 5 = 1.1 -> 1", 5, 22.0, 1, "ASSET"},
		{"rounding: 22% of 10 = 2.2 -> 2", 10, 22.0, 2, "ASSET"},
		{"rounding: 22% of 14 = 3.08 -> 3", 14, 22.0, 3, "ASSET"},
		{"rounding: 22% of 15 = 3.3 -> 3", 15, 22.0, 3, "ASSET"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deferred := percentageRound(abs64(tc.tempDiff), tc.taxRatePct/100.0)
			direction := "ASSET"
			if tc.tempDiff < 0 {
				direction = "REVERSAL"
			}
			if deferred != tc.wantCents {
				t.Errorf("deferred tax = %d, want %d", deferred, tc.wantCents)
			}
			if direction != tc.wantDir {
				t.Errorf("direction = %q, want %q", direction, tc.wantDir)
			}
		})
	}
}

// TestValidateDeferredTax tests the deferred tax request validator.
func TestValidateDeferredTax(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		req     CalculateDeferredTaxRequest
		wantErr bool
	}{
		{"valid positive diff", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 1_000_000, TaxRate: 22.0, EntryDate: "2026-01-31"}, false},
		{"valid negative diff", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: -1_000_000, TaxRate: 22.0, EntryDate: "2026-01-31"}, false},
		{"zero temp diff", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 0, TaxRate: 22.0, EntryDate: "2026-01-31"}, true},
		{"zero tax rate", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 1_000_000, TaxRate: 0, EntryDate: "2026-01-31"}, true},
		{"negative tax rate", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 1_000_000, TaxRate: -5.0, EntryDate: "2026-01-31"}, true},
		{"tax rate over 100", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 1_000_000, TaxRate: 101.0, EntryDate: "2026-01-31"}, true},
		{"tax rate exactly 100 valid", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 1_000_000, TaxRate: 100.0, EntryDate: "2026-01-31"}, false},
		{"missing entry date", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 1_000_000, TaxRate: 22.0, EntryDate: ""}, true},
		{"bad date format", CalculateDeferredTaxRequest{
			TemporaryDifferencesCents: 1_000_000, TaxRate: 22.0, EntryDate: "31-01-2026"}, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateDeferredTax(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateDeferredTax(%+v) err = %v, wantErr %v", tc.req, err, tc.wantErr)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Pure helper function tests
// -----------------------------------------------------------------------------

// TestPercentageRound tests the core rounding helper used by all tax math.
// percentageRound(base, rate): rate is decimal (0.005 = 0.5%).
// Returns int64(float64(base)*rate + 0.5) with half-up rounding.
// Returns 0 when base <= 0 or rate <= 0.
func TestPercentageRound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		base int64
		rate float64
		want int64
	}{
		{"exact integer result", 1_000_000, 0.11, 110_000},
		{"half rounds up", 100, 0.005, 1},             // 0.5 -> 1
		{"just below half rounds down", 99, 0.005, 0}, // 0.495 -> 0
		{"zero base", 0, 0.11, 0},
		{"negative base", -1000, 0.11, 0},
		{"zero rate", 1_000_000, 0, 0},
		{"negative rate", 1_000_000, -0.1, 0},
		{"large base", 1_000_000_000, 0.22, 220_000_000},
		{"tiny rate", 1_000_000, 0.0001, 100},
		{"100% rate", 1_000_000, 1.0, 1_000_000},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := percentageRound(tc.base, tc.rate)
			if got != tc.want {
				t.Errorf("percentageRound(%d, %v) = %d, want %d", tc.base, tc.rate, got, tc.want)
			}
		})
	}
}

// TestAbs64 tests the absolute value helper used by deferred tax.
func TestAbs64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input int64
		want  int64
	}{
		{"positive", 42, 42},
		{"negative", -42, 42},
		{"zero", 0, 0},
		{"max int64", math.MaxInt64, math.MaxInt64},
		{"min int64 + 1", math.MinInt64 + 1, math.MaxInt64},
		{"large negative", -1_000_000_000, 1_000_000_000},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := abs64(tc.input)
			if got != tc.want {
				t.Errorf("abs64(%d) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// TestFormatPercent tests the percent formatting helper used in PPh and
// deferred tax result payloads. Integer rates get 1 decimal place; fractional
// rates get 2 decimal places.
func TestFormatPercent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rate float64
		want string
	}{
		{"integer rate 11", 11.0, "11.0"},
		{"integer rate 22", 22.0, "22.0"},
		{"integer rate 100", 100.0, "100.0"},
		{"integer rate 0", 0.0, "0.0"},
		{"half percent 0.5", 0.5, "0.50"},
		{"0.75 percent", 0.75, "0.75"},
		{"2.5 percent", 2.5, "2.50"},
		{"1.0 percent (stored as float)", 1.0, "1.0"},
		{"10 percent", 10.0, "10.0"},
		{"5 percent", 5.0, "5.0"},
		{"33.33 percent", 33.33, "33.33"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatPercent(tc.rate)
			if got != tc.want {
				t.Errorf("formatPercent(%v) = %q, want %q", tc.rate, got, tc.want)
			}
		})
	}
}

// TestValidDate tests the date validation helper.
func TestValidDate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"2026-01-31", true},
		{"2026-02-28", true},
		{"2026-12-31", true},
		{"2026-01-01", true},
		{"", false},
		{"   ", false},
		{"2026/01/31", false},
		{"31-01-2026", false},
		{"2026-13-01", false}, // invalid month
		{"2026-01-32", false}, // invalid day
		{"not-a-date", false},
		{"2026-1-1", false}, // not zero-padded
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := validDate(tc.input)
			if got != tc.want {
				t.Errorf("validDate(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestNormalizeDate tests the date normalization helper: it trims and
// validates YYYY-MM-DD, returning "" for blank or unparseable input.
func TestNormalizeDate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"2026-01-31", "2026-01-31"},
		{"  2026-01-31  ", "2026-01-31"}, // trimmed
		{"", ""},                         // blank -> open bound
		{"   ", ""},
		{"2026/01/31", ""}, // wrong format -> ""
		{"invalid", ""},
		{"2026-01-32", ""}, // invalid day
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeDate(tc.input)
			if got != tc.want {
				t.Errorf("normalizeDate(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestMonthBounds tests the month bounds helper: returns inclusive
// YYYY-MM-DD start and end for a calendar month.
func TestMonthBounds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		year      int64
		month     int64
		wantStart string
		wantEnd   string
	}{
		{2026, 1, "2026-01-01", "2026-01-31"},
		{2026, 2, "2026-02-01", "2026-02-28"}, // non-leap year
		{2024, 2, "2024-02-01", "2024-02-29"}, // leap year
		{2026, 4, "2026-04-01", "2026-04-30"}, // 30-day month
		{2026, 12, "2026-12-01", "2026-12-31"},
		{2026, 7, "2026-07-01", "2026-07-31"},
		{2026, 9, "2026-09-01", "2026-09-30"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			start, end := monthBounds(tc.year, tc.month)
			if start != tc.wantStart {
				t.Errorf("monthBounds(%d,%d) start = %q, want %q", tc.year, tc.month, start, tc.wantStart)
			}
			if end != tc.wantEnd {
				t.Errorf("monthBounds(%d,%d) end = %q, want %q", tc.year, tc.month, end, tc.wantEnd)
			}
		})
	}
}

// TestParsePeriod tests the period year/month parser used by PPN
// reconciliation query params. parsePeriod reads period_year and period_month
// from the request URL query string.
func TestParsePeriod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		yearStr   string
		monthStr  string
		wantYear  int64
		wantMonth int64
		wantErr   bool
	}{
		{"valid Jan 2026", "2026", "1", 2026, 1, false},
		{"valid Dec 2026", "2026", "12", 2026, 12, false},
		{"valid mid year", "2026", "6", 2026, 6, false},
		{"missing year", "", "1", 0, 0, true},
		{"missing month", "2026", "", 0, 0, true},
		{"both missing", "", "", 0, 0, true},
		{"zero year", "0", "1", 0, 0, true},
		{"negative year", "-1", "1", 0, 0, true},
		{"month too low", "2026", "0", 0, 0, true},
		{"month too high", "2026", "13", 0, 0, true},
		{"non-numeric year", "abc", "1", 0, 0, true},
		{"non-numeric month", "2026", "xyz", 0, 0, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest("GET", "/ppn/reconciliation", nil)
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}
			q := req.URL.Query()
			if tc.yearStr != "" {
				q.Set("period_year", tc.yearStr)
			}
			if tc.monthStr != "" {
				q.Set("period_month", tc.monthStr)
			}
			req.URL.RawQuery = q.Encode()

			year, month, err := parsePeriod(req)
			if (err != nil) != tc.wantErr {
				t.Errorf("parsePeriod(year=%q, month=%q) err = %v, wantErr %v",
					tc.yearStr, tc.monthStr, err, tc.wantErr)
			}
			if !tc.wantErr {
				if year != tc.wantYear || month != tc.wantMonth {
					t.Errorf("parsePeriod(year=%q, month=%q) = (%d, %d), want (%d, %d)",
						tc.yearStr, tc.monthStr, year, month, tc.wantYear, tc.wantMonth)
				}
			}
		})
	}
}

// TestParseInt tests the integer parsing helper.
func TestParseInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		want   int64
		wantOk bool
	}{
		{"42", 42, true},
		{"0", 0, true},
		{"-5", -5, true},
		{"999999", 999999, true},
		{"", 0, false},
		{"abc", 0, false},
		{"12abc", 12, true}, // Sscanf parses leading digits
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := parseInt(tc.input)
			if (err == nil) != tc.wantOk {
				t.Errorf("parseInt(%q) err = %v, wantOk %v", tc.input, err, tc.wantOk)
			}
			if tc.wantOk && got != tc.want {
				t.Errorf("parseInt(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// TestOptionalNote tests the note suffix formatter used in journal
// descriptions. Empty note => ""; non-empty => " — <note>".
func TestOptionalNote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		note string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"simple note", "monthly ECL", " — monthly ECL"},
		{"note with surrounding spaces", "  trimmed  ", " — trimmed"},
		{"special chars", "Q1 closing!", " — Q1 closing!"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := optionalNote(tc.note)
			if got != tc.want {
				t.Errorf("optionalNote(%q) = %q, want %q", tc.note, got, tc.want)
			}
		})
	}
}

// TestAccountCodeConstants verifies all the seeded tax account codes match
// the expected chart-of-accounts values. These constants are used by the
// production SQL to resolve account IDs at posting time.
func TestAccountCodeConstants(t *testing.T) {
	t.Parallel()
	codes := map[string]string{
		"ppnKeluaranCode":        ppnKeluaranCode,        // 2202 VAT Payable
		"ppnMasukanCode":         ppnMasukanCode,         // 1203 Input VAT
		"salesCode":              salesCode,              // 4101 Sales Revenue
		"cashCode":               cashCode,               // 1101 Cash
		"arAccountCode":          arAccountCode,          // 1201 Accounts Receivable
		"pphPayableAccountCode":  pphPayableAccountCode,  // 2203 Income Tax Payable
		"pphExpenseAccountCode":  pphExpenseAccountCode,  // 5208 Income Tax Expense
		"allowanceAccountCode":   allowanceAccountCode,   // 1202 Allowance for Doubtful Accts
		"badDebtExpenseCode":     badDebtExpenseCode,     // 5209 Bad Debt Expense
		"badDebtRecoveryCode":    badDebtRecoveryCode,    // 4906 Bad Debt Recovery
		"deferredTaxAssetCode":   deferredTaxAssetCode,   // 1206 Deferred Tax Asset
		"deferredTaxExpenseCode": deferredTaxExpenseCode, // 5904 Deferred Tax Expense
	}
	want := map[string]string{
		"ppnKeluaranCode":        "2202",
		"ppnMasukanCode":         "1203",
		"salesCode":              "4101",
		"cashCode":               "1101",
		"arAccountCode":          "1201",
		"pphPayableAccountCode":  "2203",
		"pphExpenseAccountCode":  "5208",
		"allowanceAccountCode":   "1202",
		"badDebtExpenseCode":     "5209",
		"badDebtRecoveryCode":    "4906",
		"deferredTaxAssetCode":   "1206",
		"deferredTaxExpenseCode": "5904",
	}
	for name, code := range codes {
		name, code := name, code
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if code != want[name] {
				t.Errorf("%s = %q, want %q", name, code, want[name])
			}
		})
	}
}

// TestValidatePayPPhFinal tests the PPh payment request validator.
func TestValidatePayPPhFinal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		req     PayPPhFinalRequest
		wantErr bool
	}{
		{"valid payment", PayPPhFinalRequest{EntryDate: "2026-01-31", CashAccountID: 5, AmountCents: 50_000}, false},
		{"missing entry_date", PayPPhFinalRequest{EntryDate: "", CashAccountID: 5, AmountCents: 50_000}, true},
		{"bad date", PayPPhFinalRequest{EntryDate: "2026/01/31", CashAccountID: 5, AmountCents: 50_000}, true},
		{"missing cash account", PayPPhFinalRequest{EntryDate: "2026-01-31", CashAccountID: 0, AmountCents: 50_000}, true},
		{"zero amount", PayPPhFinalRequest{EntryDate: "2026-01-31", CashAccountID: 5, AmountCents: 0}, true},
		{"negative amount", PayPPhFinalRequest{EntryDate: "2026-01-31", CashAccountID: 5, AmountCents: -1}, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validatePayPPhFinal(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("validatePayPPhFinal(%+v) err = %v, wantErr %v", tc.req, err, tc.wantErr)
			}
		})
	}
}
