package costcenter

import (
	"testing"
)

// TestValidateCostCenter covers the pure validation helper for create-cost-
// center requests: code, name, and center_type rules.
func TestValidateCostCenter(t *testing.T) {
	t.Parallel()
	type tc struct {
		name       string
		code       string
		nameVal    string
		centerType string
		wantCode   string
		wantMsg    string
	}
	cases := []tc{
		{
			name:       "valid COST",
			code:       "CC-001",
			nameVal:    "Marketing",
			centerType: "COST",
			wantCode:   "",
			wantMsg:    "",
		},
		{
			name:       "valid lowercase profit normalized",
			code:       "PC-01",
			nameVal:    "Sales",
			centerType: "profit",
			wantCode:   "",
			wantMsg:    "",
		},
		{
			name:       "valid investment with surrounding whitespace",
			code:       "IC-01",
			nameVal:    "R&D Fund",
			centerType: "  INVESTMENT  ",
			wantCode:   "",
			wantMsg:    "",
		},
		{
			name:       "missing code",
			code:       "   ",
			nameVal:    "Sales",
			centerType: "PROFIT",
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "code is required",
		},
		{
			name:       "missing name",
			code:       "CC-001",
			nameVal:    "",
			centerType: "COST",
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "name is required",
		},
		{
			name:       "invalid center type",
			code:       "CC-001",
			nameVal:    "Marketing",
			centerType: "OVERHEAD",
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "center_type must be COST, PROFIT, or INVESTMENT",
		},
		{
			name:       "empty center type",
			code:       "CC-001",
			nameVal:    "Marketing",
			centerType: "",
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "center_type must be COST, PROFIT, or INVESTMENT",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gotCode, gotMsg := validateCostCenter(c.code, c.nameVal, c.centerType)
			if gotCode != c.wantCode {
				t.Fatalf("code = %q, want %q", gotCode, c.wantCode)
			}
			if gotMsg != c.wantMsg {
				t.Fatalf("msg = %q, want %q", gotMsg, c.wantMsg)
			}
		})
	}
}

// TestValidateAllocation covers the pure validation helper for allocation
// requests: source/target IDs and percentage range.
func TestValidateAllocation(t *testing.T) {
	t.Parallel()
	type tc struct {
		name       string
		sourceID   int64
		targetID   int64
		percentage float64
		wantCode   string
		wantMsg    string
	}
	cases := []tc{
		{
			name:       "valid allocation",
			sourceID:   1,
			targetID:   2,
			percentage: 50,
			wantCode:   "",
			wantMsg:    "",
		},
		{
			name:       "valid 100 percent",
			sourceID:   1,
			targetID:   2,
			percentage: 100,
			wantCode:   "",
			wantMsg:    "",
		},
		{
			name:       "valid fractional percentage",
			sourceID:   1,
			targetID:   2,
			percentage: 33.33,
			wantCode:   "",
			wantMsg:    "",
		},
		{
			name:       "missing source ID",
			sourceID:   0,
			targetID:   2,
			percentage: 50,
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "source_cost_center_id and target_cost_center_id are required",
		},
		{
			name:       "negative source ID",
			sourceID:   -1,
			targetID:   2,
			percentage: 50,
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "source_cost_center_id and target_cost_center_id are required",
		},
		{
			name:       "missing target ID",
			sourceID:   1,
			targetID:   0,
			percentage: 50,
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "source_cost_center_id and target_cost_center_id are required",
		},
		{
			name:       "zero percentage",
			sourceID:   1,
			targetID:   2,
			percentage: 0,
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "allocation_percentage must be between 0 and 100",
		},
		{
			name:       "negative percentage",
			sourceID:   1,
			targetID:   2,
			percentage: -10,
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "allocation_percentage must be between 0 and 100",
		},
		{
			name:       "percentage over 100",
			sourceID:   1,
			targetID:   2,
			percentage: 100.01,
			wantCode:   "INVALID_REQUEST",
			wantMsg:    "allocation_percentage must be between 0 and 100",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gotCode, gotMsg := validateAllocation(c.sourceID, c.targetID, c.percentage)
			if gotCode != c.wantCode {
				t.Fatalf("code = %q, want %q", gotCode, c.wantCode)
			}
			if gotMsg != c.wantMsg {
				t.Fatalf("msg = %q, want %q", gotMsg, c.wantMsg)
			}
		})
	}
}

// TestNormalizeAllocationBasis verifies the defaulting and uppercasing of the
// allocation basis field.
func TestNormalizeAllocationBasis(t *testing.T) {
	t.Parallel()
	type tc struct {
		name string
		in   string
		want string
	}
	cases := []tc{
		{name: "empty defaults to REVENUE", in: "", want: "REVENUE"},
		{name: "whitespace defaults to REVENUE", in: "   ", want: "REVENUE"},
		{name: "lowercase uppercased", in: "headcount", want: "HEADCOUNT"},
		{name: "mixed case uppercased", in: "  Revenue  ", want: "REVENUE"},
		{name: "already upper preserved", in: "REVENUE", want: "REVENUE"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeAllocationBasis(c.in); got != c.want {
				t.Fatalf("normalizeAllocationBasis(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestPathID covers the URL path-ID parser used by every handler.
func TestPathID(t *testing.T) {
	t.Parallel()
	type tc struct {
		name string
		raw  string
		want int64
	}
	cases := []tc{
		{name: "positive integer", raw: "42", want: 42},
		{name: "one", raw: "1", want: 1},
		{name: "zero", raw: "0", want: 0},
		{name: "negative", raw: "-5", want: -5},
		{name: "non-numeric", raw: "abc", want: 0},
		{name: "empty", raw: "", want: 0},
		{name: "float string truncated to zero", raw: "1.5", want: 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := pathID(c.raw); got != c.want {
				t.Fatalf("pathID(%q) = %d, want %d", c.raw, got, c.want)
			}
		})
	}
}
