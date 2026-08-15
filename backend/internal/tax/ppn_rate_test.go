package tax

import (
	"strings"
	"testing"
)

// TestCheckPPNRateMatches verifies the PPN rate enforcement comparison rules.
func TestCheckPPNRateMatches(t *testing.T) {
	tests := []struct {
		name       string
		rate       float64
		configured float64
		wantErr    bool
		wantSubstr string
	}{
		{name: "exact match 11%", rate: 11, configured: 11.0, wantErr: false},
		{name: "exact match 11.000000", rate: 11.000000, configured: 11.0, wantErr: false},
		{name: "within epsilon", rate: 11.00005, configured: 11.0, wantErr: false},
		{name: "zero rate accepted (untaxed)", rate: 0, configured: 11.0, wantErr: false},
		{name: "higher rate rejected", rate: 12, configured: 11.0, wantErr: true, wantSubstr: "does not match"},
		{name: "lower rate rejected", rate: 10, configured: 11.0, wantErr: true, wantSubstr: "does not match"},
		{name: "slightly off beyond epsilon rejected", rate: 11.001, configured: 11.0, wantErr: true, wantSubstr: "does not match"},
		{name: "configured zero with nonzero rate rejected", rate: 5, configured: 0, wantErr: true, wantSubstr: "does not match"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPPNRateMatches(tc.rate, tc.configured)
			if (err != nil) != tc.wantErr {
				t.Fatalf("checkPPNRateMatches(%v, %v) error = %v, wantErr %v", tc.rate, tc.configured, err, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q must contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// TestCheckPPNRateMatchesErrorMessage verifies the error mentions both rates
// so the user knows what the configured value is.
func TestCheckPPNRateMatchesErrorMessage(t *testing.T) {
	err := checkPPNRateMatches(12, 11)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "12.0000") || !strings.Contains(err.Error(), "11.0000") {
		t.Errorf("error should mention both supplied and configured rates, got: %q", err.Error())
	}
}
