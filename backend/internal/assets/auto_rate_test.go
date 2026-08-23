package assets

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestAutoStraightLineRate(t *testing.T) {
	tests := []struct {
		name             string
		usefulLifeMonths int
		want             string
	}{
		{"12 months", 12, "0.083333"},
		{"36 months", 36, "0.027778"},
		{"48 months", 48, "0.020833"},
		{"60 months", 60, "0.016667"},
		{"1 month", 1, "1.000000"},
		{"240 months", 240, "0.004167"},
		{"zero rejected", 0, "0"},
		{"negative rejected", -6, "0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := autoStraightLineRate(tc.usefulLifeMonths)
			if got != tc.want {
				t.Errorf("autoStraightLineRate(%d) = %q, want %q", tc.usefulLifeMonths, got, tc.want)
			}
		})
	}
}

// TestAutoStraightLineRate_ScannableAsNumeric guarantees the derived rate
// always parses into the pgtype.Numeric used for the fixed_assets.rate insert
// (QA-16: the NULL rate violated the NOT NULL constraint).
func TestAutoStraightLineRate_ScannableAsNumeric(t *testing.T) {
	for _, months := range []int{1, 6, 12, 36, 60, 120, 480} {
		var n pgtype.Numeric
		if err := n.Scan(autoStraightLineRate(months)); err != nil {
			t.Errorf("rate for %d months does not scan as NUMERIC: %v", months, err)
		}
	}
}

func TestValidateRegisterRequest_StraightLineStillAllowsEmptyRate(t *testing.T) {
	req := validRegisterRequest()
	req.DepreciationMethod = methodStraightLine
	req.Rate = ""
	if code, _ := validateRegisterRequest(req); code != "" {
		t.Fatalf("straight_line without rate must stay valid (rate is derived), got %s", code)
	}
}
