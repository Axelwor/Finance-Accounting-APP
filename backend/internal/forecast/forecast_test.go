package forecast

import (
	"testing"
	"time"
)

// TestBucketDate verifies the date label used for each forecast bucket is
// computed as today + i days, formatted YYYY-MM-DD. This is the core date
// math that drives the forecast horizon and must be stable across days.
func TestBucketDate(t *testing.T) {
	tests := []struct {
		name string
		base time.Time
		i    int
		want string
	}{
		{
			name: "first bucket is base day",
			base: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
			i:    0,
			want: "2026-08-10",
		},
		{
			name: "one day later",
			base: time.Date(2026, 8, 10, 23, 59, 0, 0, time.UTC),
			i:    1,
			want: "2026-08-11",
		},
		{
			name: "cross month boundary",
			base: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			i:    1,
			want: "2026-02-01",
		},
		{
			name: "cross year boundary",
			base: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			i:    1,
			want: "2027-01-01",
		},
		{
			name: "30 day horizon end",
			base: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), // non-leap 2026
			i:    27,
			want: "2026-02-28",
		},
		{
			name: "leap day Feb 29 in 2028",
			base: time.Date(2028, 2, 28, 0, 0, 0, 0, time.UTC), // 2028 is leap
			i:    1,
			want: "2026-02-29", // placeholder, corrected below
		},
	}
	// Fix the leap-day expectation explicitly (kept out of table for clarity).
	tests[5].want = "2028-02-29"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bucketDate(tt.base, tt.i).Format("2006-01-02")
			if got != tt.want {
				t.Fatalf("bucketDate(%s, %d) = %q, want %q",
					tt.base.Format("2006-01-02"), tt.i, got, tt.want)
			}
		})
	}
}

// TestBucketDateConsistency ensures successive buckets are exactly one
// calendar day apart and never skip or repeat a date across a horizon.
// We compare calendar days (not raw Durations) so DST transitions, where
// a day can be 23 or 25 hours, do not produce false failures.
func TestBucketDateConsistency(t *testing.T) {
	base := time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC)
	horizon := 60
	prev := bucketDate(base, 0)
	for i := 1; i < horizon; i++ {
		cur := bucketDate(base, i)
		// The previous day's date must equal cur minus one day.
		if !cur.AddDate(0, 0, -1).Equal(prev) {
			t.Fatalf("bucket %d: expected %s to be exactly one day after %s",
				i, cur.Format("2006-01-02"), prev.Format("2006-01-02"))
		}
		// And the formatted label must differ (no duplicate dates).
		if cur.Format("2006-01-02") == prev.Format("2006-01-02") {
			t.Fatalf("bucket %d: date %s repeated",
				i, cur.Format("2006-01-02"))
		}
		prev = cur
	}
}

// TestNetCents verifies the per-bucket net = inflow - outflow, the elementary
// identity the running balance rolls up.
func TestNetCents(t *testing.T) {
	tests := []struct {
		name    string
		inflow  int64
		outflow int64
		wantNet int64
	}{
		{name: "zero bucket", inflow: 0, outflow: 0, wantNet: 0},
		{name: "pure inflow", inflow: 500000, outflow: 0, wantNet: 500000},
		{name: "pure outflow", inflow: 0, outflow: 250000, wantNet: -250000},
		{name: "balanced", inflow: 1000000, outflow: 1000000, wantNet: 0},
		{name: "net outflow", inflow: 300000, outflow: 750000, wantNet: -450000},
		{name: "large values", inflow: 9999999999, outflow: 1, wantNet: 9999999998},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := forecastBucket{InflowCents: tt.inflow, OutflowCents: tt.outflow}
			if got := netCents(b); got != tt.wantNet {
				t.Fatalf("netCents(%+v) = %d, want %d", b, got, tt.wantNet)
			}
		})
	}
}

// TestEndingBalance verifies the forecast totals identity:
//
//	ending = starting + totalInflow - totalOutflow
//
// This is the headline number the forecast endpoint returns; it must hold for
// any combination of starting balance and bucket flows.
func TestEndingBalance(t *testing.T) {
	tests := []struct {
		name     string
		starting int64
		in       int64
		out      int64
		want     int64
	}{
		{name: "all zero", starting: 0, in: 0, out: 0, want: 0},
		{name: "no flows", starting: 5_000_000, in: 0, out: 0, want: 5_000_000},
		{name: "net positive", starting: 1_000_000, in: 3_000_000, out: 1_500_000, want: 2_500_000},
		{name: "net negative stays positive", starting: 10_000_000, in: 1_000_000, out: 4_000_000, want: 7_000_000},
		{name: "net negative below zero", starting: 1_000_000, in: 0, out: 3_000_000, want: -2_000_000},
		{name: "balanced flows", starting: 2_500_000, in: 7_500_000, out: 7_500_000, want: 2_500_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := endingBalance(tt.starting, tt.in, tt.out); got != tt.want {
				t.Fatalf("endingBalance(%d, %d, %d) = %d, want %d",
					tt.starting, tt.in, tt.out, got, tt.want)
			}
		})
	}
}

// TestRunningBalanceRollup verifies the cumulative running balance across a
// sequence of buckets matches the closed-form ending balance. This ties the
// per-bucket computation to the headline total.
func TestRunningBalanceRollup(t *testing.T) {
	starting := int64(1_000_000)
	buckets := []forecastBucket{
		{InflowCents: 0, OutflowCents: 200_000},       // net -200k
		{InflowCents: 500_000, OutflowCents: 0},       // net +500k
		{InflowCents: 100_000, OutflowCents: 350_000}, // net -250k
		{InflowCents: 0, OutflowCents: 0},             // net 0
		{InflowCents: 800_000, OutflowCents: 50_000},  // net +750k
	}
	// Roll up running balances exactly as the handler does.
	var totalIn, totalOut int64
	prev := starting
	for i := range buckets {
		buckets[i].NetCents = netCents(buckets[i])
		if i == 0 {
			buckets[i].RunningBalance = starting + buckets[i].NetCents
		} else {
			buckets[i].RunningBalance = prev + buckets[i].NetCents
		}
		prev = buckets[i].RunningBalance
		totalIn += buckets[i].InflowCents
		totalOut += buckets[i].OutflowCents
	}
	wantEnding := endingBalance(starting, totalIn, totalOut)
	if got := buckets[len(buckets)-1].RunningBalance; got != wantEnding {
		t.Fatalf("rolled-up running balance %d != endingBalance %d", got, wantEnding)
	}
}
