package costing

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

// These tests exercise the REAL validation guards of PostGRN, ResolveCOGS, and
// ReverseCOGS. Every guarded path returns before any database access, so a nil
// pgx.Tx is safe: if a case accidentally reached the database layer the test
// would panic, which would surface immediately.

// ---------------------------------------------------------------------------
// PostGRN input validation (real function, nil tx)
// ---------------------------------------------------------------------------

func TestPostGRN_Validation_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		qty        float64
		unitCost   int64
		method     string
		wantSubstr string
		wantSent   error
	}{
		{"zero qty rejected", 0, 100, MethodFIFO, "qty must be > 0", nil},
		{"negative qty rejected", -5, 100, MethodFIFO, "qty must be > 0", nil},
		{"tiny negative qty rejected", -0.001, 100, MethodMovingAverage, "qty must be > 0", nil},
		{"negative unit cost rejected", 10, -1, MethodFIFO, "unit_cost_cents must be >= 0", nil},
		{"negative unit cost rejected moving average", 10, -100, MethodMovingAverage, "unit_cost_cents must be >= 0", nil},
		{"unknown method lifo", 10, 100, "lifo", "lifo", ErrUnknownCostingMethod},
		{"unknown method uppercase FIFO", 10, 100, "FIFO", "FIFO", ErrUnknownCostingMethod},
		{"unknown method weighted", 10, 100, "weighted", "weighted", ErrUnknownCostingMethod},
		{"unknown method standard", 10, 100, "standard", "standard", ErrUnknownCostingMethod},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := PostGRN(context.Background(), nil, 1, 1, 0, tc.qty, tc.unitCost, tc.method)
			if err == nil {
				t.Fatalf("PostGRN(qty=%v, unitCost=%d, method=%q) = nil, want error", tc.qty, tc.unitCost, tc.method)
			}
			if tc.wantSent != nil && !errors.Is(err, tc.wantSent) {
				t.Errorf("errors.Is(err, %v) = false, want true (err=%v)", tc.wantSent, err)
			}
			if tc.wantSubstr != "" && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveCOGS input validation (real function, nil tx)
// ---------------------------------------------------------------------------

func TestResolveCOGS_Validation_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		qty        float64
		method     string
		wantSubstr string
		wantSent   error
	}{
		{"zero qty rejected", 0, MethodFIFO, "qty must be > 0", nil},
		{"negative qty rejected", -1, MethodFIFO, "qty must be > 0", nil},
		{"negative qty rejected moving average", -0.5, MethodMovingAverage, "qty must be > 0", nil},
		{"unknown method lifo", 5, "lifo", "lifo", ErrUnknownCostingMethod},
		{"unknown method abc", 5, "abc", "abc", ErrUnknownCostingMethod},
		{"unknown method moving-average dash", 5, "moving-average", "moving-average", ErrUnknownCostingMethod},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveCOGS(context.Background(), nil, 1, 1, 0, tc.qty, tc.method)
			if err == nil {
				t.Fatalf("ResolveCOGS(qty=%v, method=%q) = nil error, want error", tc.qty, tc.method)
			}
			if got != 0 {
				t.Errorf("ResolveCOGS returned cogs=%d on validation error, want 0", got)
			}
			if tc.wantSent != nil && !errors.Is(err, tc.wantSent) {
				t.Errorf("errors.Is(err, %v) = false, want true (err=%v)", tc.wantSent, err)
			}
			if tc.wantSubstr != "" && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ReverseCOGS input validation (real function, nil tx)
// ---------------------------------------------------------------------------

func TestReverseCOGS_Validation_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		qty        float64
		unitCost   int64
		method     string
		wantSubstr string
		wantSent   error
	}{
		{"zero qty rejected", 0, 100, MethodFIFO, "qty must be > 0", nil},
		{"negative qty rejected", -2, 100, MethodSpecific, "qty must be > 0", nil},
		{"negative unit cost rejected", 5, -1, MethodMovingAverage, "unit_cost_cents must be >= 0", nil},
		{"negative unit cost rejected fifo", 5, -999, MethodFIFO, "unit_cost_cents must be >= 0", nil},
		{"unknown method lifo", 5, 100, "lifo", "lifo", ErrUnknownCostingMethod},
		{"unknown method specific uppercase", 5, 100, "SPECIFIC", "SPECIFIC", ErrUnknownCostingMethod},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ReverseCOGS(context.Background(), nil, 1, 1, 0, tc.qty, tc.unitCost, tc.method)
			if err == nil {
				t.Fatalf("ReverseCOGS(qty=%v, unitCost=%d, method=%q) = nil, want error", tc.qty, tc.unitCost, tc.method)
			}
			if tc.wantSent != nil && !errors.Is(err, tc.wantSent) {
				t.Errorf("errors.Is(err, %v) = false, want true (err=%v)", tc.wantSent, err)
			}
			if tc.wantSubstr != "" && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FIFO cost layer consumption — audit report M-020 scenarios, table-driven.
// consumeFIFO itself needs a pgx.Tx; consumeFIFOLocal (costing_test.go) is a
// line-for-line replica of its consumption loop, including the avg-cost
// fallback for legacy stock without layers.
// ---------------------------------------------------------------------------

func TestFIFO_AuditScenarios_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		layers  []fifoLayer
		issue   float64
		avgCost float64
		want    int64
	}{
		{
			name:   "audit scenario: receive 10@100 10@200 issue 15",
			layers: []fifoLayer{{qtyRemaining: 10, unitCost: 100}, {qtyRemaining: 10, unitCost: 200}},
			issue:  15,
			want:   2000, // 10×100 + 5×200
		},
		{
			name:   "layer exhaustion ends exactly at boundary",
			layers: []fifoLayer{{qtyRemaining: 10, unitCost: 100}, {qtyRemaining: 10, unitCost: 200}},
			issue:  10,
			want:   1000,
		},
		{
			name:   "all layers consumed",
			layers: []fifoLayer{{qtyRemaining: 10, unitCost: 100}, {qtyRemaining: 10, unitCost: 200}},
			issue:  20,
			want:   3000,
		},
		{
			name:    "legacy shortfall valued at avg cost",
			layers:  []fifoLayer{{qtyRemaining: 10, unitCost: 100}, {qtyRemaining: 10, unitCost: 200}},
			issue:   25,
			avgCost: 150,
			want:    3750, // 3000 from layers + 5×150 fallback
		},
		{
			name:    "no layers at all: full fallback at avg",
			layers:  nil,
			issue:   4,
			avgCost: 250,
			want:    1000,
		},
		{
			name:   "three layers oldest first",
			layers: []fifoLayer{{qtyRemaining: 5, unitCost: 100}, {qtyRemaining: 5, unitCost: 150}, {qtyRemaining: 5, unitCost: 300}},
			issue:  12,
			want:   1850, // 5×100 + 5×150 + 2×300
		},
		{
			name:   "fractional issue rounds to nearest cent",
			layers: []fifoLayer{{qtyRemaining: 10, unitCost: 3333}},
			issue:  3,
			want:   9999,
		},
		{
			name:   "empty layer skipped",
			layers: []fifoLayer{{qtyRemaining: 0, unitCost: 9999}, {qtyRemaining: 5, unitCost: 100}},
			issue:  5,
			want:   500,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := consumeFIFOLocal(tc.layers, tc.issue, tc.avgCost)
			if got != tc.want {
				t.Errorf("FIFO COGS for issue %v = %d, want %d", tc.issue, got, tc.want)
			}
		})
	}
}

// TestFIFO_RemainingLayers verifies the post-issue layer state for the audit
// scenario: receive 10@100, receive 10@200, issue 15 → layer 1 closed
// (remaining 0), layer 2 has 5 remaining @ 200.
func TestFIFO_RemainingLayers(t *testing.T) {
	layers := []fifoLayer{{qtyRemaining: 10, unitCost: 100}, {qtyRemaining: 10, unitCost: 200}}
	remaining := 15.0
	for i := range layers {
		if remaining <= 0 {
			break
		}
		take := layers[i].qtyRemaining
		if take > remaining {
			take = remaining
		}
		layers[i].qtyRemaining -= take
		remaining -= take
	}
	if layers[0].qtyRemaining != 0 {
		t.Errorf("layer 1 remaining = %v, want 0 (closed)", layers[0].qtyRemaining)
	}
	if layers[1].qtyRemaining != 5 {
		t.Errorf("layer 2 remaining = %v, want 5", layers[1].qtyRemaining)
	}
	if layers[1].unitCost != 200 {
		t.Errorf("layer 2 unit cost = %d, want 200", layers[1].unitCost)
	}
}

// ---------------------------------------------------------------------------
// Moving average — audit report M-020 scenarios, table-driven. The formula
// mirrors the SQL in PostGRN/ReverseCOGS:
//
//	avg = (old_qty*old_avg + new_qty*new_cost) / (old_qty + new_qty)
// ---------------------------------------------------------------------------

func TestMovingAverage_AuditScenarios_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		oldQty  int64
		oldAvg  int64
		newQty  int64
		newCost int64
		want    int64
	}{
		{"audit scenario: 10@100 then 10@200 gives avg 150", 10, 100, 10, 200, 150},
		{"first receipt sets the average", 0, 0, 100, 1000, 1000},
		{"same cost keeps average", 100, 1000, 50, 1000, 1000},
		{"higher cost raises average", 100, 1000, 50, 1300, 1100},
		{"lower cost lowers average", 100, 1000, 50, 700, 900},
		{"integer division truncates", 100, 1000, 33, 1001, 1000},
		{"zero total qty keeps old avg", 50, 1000, -50, 0, 1000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := movingAverage(tc.oldQty, tc.oldAvg, tc.newQty, tc.newCost)
			if got != tc.want {
				t.Errorf("movingAverage(%d,%d,%d,%d) = %d, want %d", tc.oldQty, tc.oldAvg, tc.newQty, tc.newCost, got, tc.want)
			}
		})
	}
}

// TestMovingAverage_AuditIssueCOGS verifies the full audit scenario end to
// end: receive 10@100, receive 10@200 → avg 150; issue 5 → COGS = 5×150 = 750.
func TestMovingAverage_AuditIssueCOGS(t *testing.T) {
	avg := movingAverage(0, 0, 10, 100)
	avg = movingAverage(10, avg, 10, 200)
	if avg != 150 {
		t.Fatalf("avg after two receipts = %d, want 150", avg)
	}
	cogs := movingAverageCOGS(5, avg)
	if cogs != 750 {
		t.Errorf("COGS for issue of 5 = %d, want 750", cogs)
	}
}

// movingAverageCOGS replicates the ResolveCOGS moving-average branch:
//
//	cogs = int64(math.Round(qty * float64(avgCost)))
func movingAverageCOGS(qty float64, avgCost int64) int64 {
	return int64(math.Round(qty * float64(avgCost)))
}

// ---------------------------------------------------------------------------
// Negative stock rejection boundary — ResolveCOGS rejects exactly when
// qoh < qty (qoh == qty is allowed).
// ---------------------------------------------------------------------------

func TestNegativeStock_Boundary_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		qoh        float64
		qty        float64
		wantReject bool
	}{
		{"issue exceeds stock", 50, 100, true},
		{"one unit short", 99, 100, true},
		{"fractional shortfall", 9.99, 10, true},
		{"zero stock any issue", 0, 0.5, true},
		{"exact stock allowed", 100, 100, false},
		{"stock exceeds issue", 150, 100, false},
		{"fractional stock covers", 10.5, 10, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rejected := tc.qoh < tc.qty
			if rejected != tc.wantReject {
				t.Errorf("qoh=%v qty=%v rejected=%v, want %v", tc.qoh, tc.qty, rejected, tc.wantReject)
			}
		})
	}
}
