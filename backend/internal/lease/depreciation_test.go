package lease

import "testing"

func TestLeaseTermMonths(t *testing.T) {
	tests := []struct {
		name          string
		totalPayments int
		frequency     string
		want          int
	}{
		{"monthly 12x", 12, freqMonthly, 12},
		{"quarterly 4x", 4, freqQuarterly, 12},
		{"annually 1x", 1, freqAnnually, 12},
		{"annually 3x", 3, freqAnnually, 36},
		{"unknown frequency treated as monthly", 24, "WEEKLY", 24},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := leaseTermMonths(tc.totalPayments, tc.frequency); got != tc.want {
				t.Errorf("leaseTermMonths(%d, %q) = %d, want %d",
					tc.totalPayments, tc.frequency, got, tc.want)
			}
		})
	}
}
