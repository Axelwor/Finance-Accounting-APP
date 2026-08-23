package costing

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// planFIFOConsumption — pure FIFO semantics used by consumeFIFO (QA-03).
// The layer rows are materialized before any UPDATE is issued, so the
// consumption planning itself is unit-testable without a database.
// ---------------------------------------------------------------------------

func layersByID(updates []fifoLayerUpdate) map[int64]float64 {
	out := make(map[int64]float64, len(updates))
	for _, u := range updates {
		out[u.LayerID] = u.NewRemaining
	}
	return out
}

func TestPlanFIFOConsumption_SingleLayerFullConsumption(t *testing.T) {
	layers := []fifoLayer{{ID: 1, QtyRemaining: 50, UnitCostCents: 80_000}}
	cogs, updates, uncovered := planFIFOConsumption(layers, 10, 0)

	if cogs != 800_000 {
		t.Errorf("cogs = %d, want 800000", cogs)
	}
	if uncovered != 0 {
		t.Errorf("uncovered = %v, want 0", uncovered)
	}
	if len(updates) != 1 {
		t.Fatalf("len(updates) = %d, want 1", len(updates))
	}
	if u := updates[0]; u.LayerID != 1 || u.NewRemaining != 40 {
		t.Errorf("update = %+v, want layer 1 with 40 remaining", u)
	}
}

func TestPlanFIFOConsumption_MultipleLayersOldestFirst(t *testing.T) {
	// Layer 1: 30 @ 80.000, layer 2: 20 @ 90.000. Consume 40:
	// 30*80000 + 10*90000 = 2.400.000 + 900.000 = 3.300.000.
	layers := []fifoLayer{
		{ID: 1, QtyRemaining: 30, UnitCostCents: 80_000},
		{ID: 2, QtyRemaining: 20, UnitCostCents: 90_000},
	}
	cogs, updates, uncovered := planFIFOConsumption(layers, 40, 0)

	if cogs != 3_300_000 {
		t.Errorf("cogs = %d, want 3300000", cogs)
	}
	if uncovered != 0 {
		t.Errorf("uncovered = %v, want 0", uncovered)
	}
	if len(updates) != 2 {
		t.Fatalf("len(updates) = %d, want 2", len(updates))
	}
	if u := updates[0]; u.LayerID != 1 || u.NewRemaining > 0 {
		t.Errorf("first update = %+v, want layer 1 fully consumed", u)
	}
	if u := updates[1]; u.LayerID != 2 || u.NewRemaining != 10 {
		t.Errorf("second update = %+v, want layer 2 with 10 remaining", u)
	}
}

func TestPlanFIFOConsumption_PartialSingleLayer(t *testing.T) {
	layers := []fifoLayer{{ID: 7, QtyRemaining: 100, UnitCostCents: 10_000}}
	cogs, updates, uncovered := planFIFOConsumption(layers, 25, 0)

	if cogs != 250_000 {
		t.Errorf("cogs = %d, want 250000", cogs)
	}
	if uncovered != 0 {
		t.Errorf("uncovered = %v, want 0", uncovered)
	}
	if len(updates) != 1 || updates[0].NewRemaining != 75 {
		t.Errorf("updates = %+v, want layer 7 with 75 remaining", updates)
	}
}

func TestPlanFIFOConsumption_UncoveredQtyReportedForFallback(t *testing.T) {
	// Layers cover 30 of 50; the uncovered 20 must be reported so the caller
	// values it at the balance's average cost (legacy-stock fallback).
	layers := []fifoLayer{{ID: 3, QtyRemaining: 30, UnitCostCents: 10_000}}
	cogs, updates, uncovered := planFIFOConsumption(layers, 50, 15_000)

	if cogs != 300_000 {
		t.Errorf("cogs = %d, want 300000", cogs)
	}
	if uncovered != 20 {
		t.Errorf("uncovered = %v, want 20", uncovered)
	}
	if len(updates) != 1 || updates[0].NewRemaining > 0 {
		t.Errorf("updates = %+v, want layer 3 closed", updates)
	}
	// The caller-side fallback valuation:
	cogs += int64(math.Round(uncovered * 15_000))
	if cogs != 600_000 {
		t.Errorf("cogs with fallback = %d, want 600000", cogs)
	}
}

func TestPlanFIFOConsumption_NoLayersAllFallback(t *testing.T) {
	cogs, updates, uncovered := planFIFOConsumption(nil, 100, 2_000)

	if cogs != 0 {
		t.Errorf("cogs = %d, want 0", cogs)
	}
	if uncovered != 100 {
		t.Errorf("uncovered = %v, want 100", uncovered)
	}
	if len(updates) != 0 {
		t.Errorf("updates = %+v, want none", updates)
	}
}

func TestPlanFIFOConsumption_EmptyLayerSkipped(t *testing.T) {
	layers := []fifoLayer{
		{ID: 1, QtyRemaining: 0, UnitCostCents: 10_000},
		{ID: 2, QtyRemaining: 50, UnitCostCents: 12_000},
	}
	cogs, updates, uncovered := planFIFOConsumption(layers, 50, 0)

	if cogs != 600_000 {
		t.Errorf("cogs = %d, want 600000", cogs)
	}
	if uncovered != 0 {
		t.Errorf("uncovered = %v, want 0", uncovered)
	}
	// The drained layer must not receive an UPDATE.
	if len(updates) != 1 || updates[0].LayerID != 2 {
		t.Errorf("updates = %+v, want only layer 2", updates)
	}
}

func TestPlanFIFOConsumption_StopsAtRequestedQty(t *testing.T) {
	// More open layers than needed: consumption must stop once qty is met.
	layers := []fifoLayer{
		{ID: 1, QtyRemaining: 10, UnitCostCents: 10_000},
		{ID: 2, QtyRemaining: 10, UnitCostCents: 20_000},
	}
	cogs, updates, uncovered := planFIFOConsumption(layers, 10, 0)

	if cogs != 100_000 {
		t.Errorf("cogs = %d, want 100000", cogs)
	}
	if uncovered != 0 {
		t.Errorf("uncovered = %v, want 0", uncovered)
	}
	if len(updates) != 1 || updates[0].LayerID != 1 {
		t.Errorf("updates = %+v, want only layer 1", updates)
	}
}

func TestPlanFIFOConsumption_RoundingToNearestCent(t *testing.T) {
	layers := []fifoLayer{{ID: 1, QtyRemaining: 10, UnitCostCents: 3_333}}
	cogs, _, _ := planFIFOConsumption(layers, 3, 0)

	// 3 * 3333 = 9999 exactly; rounding must not drift.
	if cogs != 9_999 {
		t.Errorf("cogs = %d, want 9999", cogs)
	}
}

func TestPlanFIFOConsumption_FractionalQty(t *testing.T) {
	layers := []fifoLayer{{ID: 1, QtyRemaining: 10.5, UnitCostCents: 10_000}}
	cogs, updates, uncovered := planFIFOConsumption(layers, 0.5, 0)

	if cogs != 5_000 {
		t.Errorf("cogs = %d, want 5000", cogs)
	}
	if uncovered != 0 {
		t.Errorf("uncovered = %v, want 0", uncovered)
	}
	if len(updates) != 1 || updates[0].NewRemaining != 10 {
		t.Errorf("updates = %+v, want 10 remaining", updates)
	}
}

func TestPlanFIFOConsumption_UpdateTargetsDistinct(t *testing.T) {
	layers := []fifoLayer{
		{ID: 1, QtyRemaining: 5, UnitCostCents: 1_000},
		{ID: 2, QtyRemaining: 5, UnitCostCents: 2_000},
	}
	_, updates, _ := planFIFOConsumption(layers, 8, 0)

	byID := layersByID(updates)
	if len(byID) != len(updates) {
		t.Fatalf("duplicate layer updates: %+v", updates)
	}
	if byID[1] != 0 {
		t.Errorf("layer 1 remaining = %v, want 0 (close)", byID[1])
	}
	if byID[2] != 2 {
		t.Errorf("layer 2 remaining = %v, want 2", byID[2])
	}
}
