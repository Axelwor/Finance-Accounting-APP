package assets

import (
	"testing"
)

// TestComputeNBV verifies net book value computation for various scenarios.
func TestComputeNBV(t *testing.T) {
	tests := []struct {
		name     string
		cost     int64
		accumDep int64
		wantNBV  int64
	}{
		{
			name:     "normal depreciation",
			cost:     10_000_000,
			accumDep: 3_000_000,
			wantNBV:  7_000_000,
		},
		{
			name:     "zero accumulated depreciation",
			cost:     10_000_000,
			accumDep: 0,
			wantNBV:  10_000_000,
		},
		{
			name:     "full depreciation to zero",
			cost:     10_000,
			accumDep: 10_000,
			wantNBV:  0,
		},
		{
			name:     "over-depreciated clamped to zero",
			cost:     5_000,
			accumDep: 6_000,
			wantNBV:  0,
		},
		{
			name:     "small asset with partial depreciation",
			cost:     120_000,
			accumDep: 30_000,
			wantNBV:  90_000,
		},
		{
			name:     "large enterprise asset",
			cost:     1_000_000_000,
			accumDep: 450_000_000,
			wantNBV:  550_000_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNBV := computeNBV(tt.cost, tt.accumDep)
			if gotNBV != tt.wantNBV {
				t.Fatalf("computeNBV(%d, %d) = %d, want %d",
					tt.cost, tt.accumDep, gotNBV, tt.wantNBV)
			}
		})
	}
}
