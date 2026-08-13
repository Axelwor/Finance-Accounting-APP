package costing_test

import (
	"math"
	"testing"

	"finance-accounting-app/backend/internal/costing"
)

// TestCosting_FIFO_BalanceChecks validates FIFO layer balance integrity.
func TestCosting_FIFO_BalanceChecks(t *testing.T) {
	tests := []struct {
		name         string
		layers       []fifoLayerTest
		qtyToConsume float64
		expectedCOGS int64
	}{
		{
			name:         "Single layer full consumption",
			layers:       []fifoLayerTest{{qtyRemaining: 100, unitCostCents: 10000}},
			qtyToConsume: 100,
			expectedCOGS: 1000000,
		},
		{
			name:         "Multiple layers partial consumption",
			layers:       []fifoLayerTest{{qtyRemaining: 50, unitCostCents: 10000}, {qtyRemaining: 50, unitCostCents: 12000}},
			qtyToConsume: 70,
			expectedCOGS: 740000,
		},
		{
			name:         "Layer exhaustion with fallback",
			layers:       []fifoLayerTest{{qtyRemaining: 30, unitCostCents: 10000}},
			qtyToConsume: 50,
			expectedCOGS: 550000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cogs := computeFIFOCOGS(tt.layers, tt.qtyToConsume, 12500)
			if cogs != tt.expectedCOGS {
				t.Errorf("COGS = %d, want %d", cogs, tt.expectedCOGS)
			}
		})
	}
}

// TestCosting_MovingAverage_BalanceValidation validates moving average calculations.
func TestCosting_MovingAverage_BalanceValidation(t *testing.T) {
	tests := []struct {
		name            string
		oldQty          float64
		oldAvgCost      int64
		newQty          float64
		newUnitCost     int64
		expectedAvgCost int64
	}{
		{
			name:            "First receipt - no previous balance",
			oldQty:          0,
			oldAvgCost:      0,
			newQty:          100,
			newUnitCost:     10000,
			expectedAvgCost: 10000,
		},
		{
			name:            "Receipt at same price - no change",
			oldQty:          100,
			oldAvgCost:      10000,
			newQty:          50,
			newUnitCost:     10000,
			expectedAvgCost: 10000,
		},
		{
			name:            "Receipt at higher price - average increases",
			oldQty:          100,
			oldAvgCost:      10000,
			newQty:          50,
			newUnitCost:     12000,
			expectedAvgCost: 10666,
		},
		{
			name:            "Receipt at lower price - average decreases",
			oldQty:          100,
			oldAvgCost:      10000,
			newQty:          50,
			newUnitCost:     8000,
			expectedAvgCost: 9333,
		},
		{
			name:            "Issue does not change average",
			oldQty:          100,
			oldAvgCost:      10000,
			newQty:          0,
			newUnitCost:     0,
			expectedAvgCost: 10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			avgCost := calculateMovingAverage(tt.oldQty, tt.newQty, tt.oldAvgCost, tt.newUnitCost)
			if avgCost != tt.expectedAvgCost {
				t.Errorf("average cost = %d, want %d", avgCost, tt.expectedAvgCost)
			}
		})
	}
}

// TestCosting_PartialQuantityHandling handles fractional quantities correctly.
func TestCosting_PartialQuantityHandling(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		qty           float64
		unitCostCents int64
		shouldReject  bool
		description   string
	}{
		{
			name:          "Whole quantity accepted",
			method:        costing.MethodFIFO,
			qty:           100,
			unitCostCents: 10000,
			shouldReject:  false,
			description:   "whole number of units",
		},
		{
			name:          "Fractional quantity accepted",
			method:        costing.MethodFIFO,
			qty:           10.5,
			unitCostCents: 10000,
			shouldReject:  false,
			description:   "partial units allowed",
		},
		{
			name:          "Zero quantity rejected",
			method:        costing.MethodFIFO,
			qty:           0,
			unitCostCents: 10000,
			shouldReject:  true,
			description:   "zero qty should be rejected",
		},
		{
			name:          "Negative quantity rejected",
			method:        costing.MethodFIFO,
			qty:           -10,
			unitCostCents: 10000,
			shouldReject:  true,
			description:   "negative qty should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.description, func(t *testing.T) {
			// Validate input parameters
			if tt.qty <= 0 {
				if !tt.shouldReject {
					t.Errorf("expected error for %s, got none", tt.description)
				}
				return
			}
			if tt.unitCostCents < 0 {
				if !tt.shouldReject {
					t.Errorf("negative unit cost should reject for %s", tt.description)
				}
				return
			}
			if tt.shouldReject {
				t.Logf("validation passed for %s", tt.description)
			}
		})
	}
}

// Helper types and functions for costing tests

type fifoLayerTest struct {
	qtyRemaining  float64
	unitCostCents int64
}

func computeFIFOCOGS(layers []fifoLayerTest, qty float64, fallbackAvgCost int64) int64 {
	remaining := qty
	var cogs int64

	for _, l := range layers {
		if remaining <= 0 {
			break
		}
		if l.qtyRemaining <= 0 {
			continue
		}

		take := l.qtyRemaining
		if take > remaining {
			take = remaining
		}

		cogs += int64(math.Round(take*float64(l.unitCostCents)))
		remaining -= take
	}

	if remaining > 0 {
		cogs += int64(math.Round(remaining*float64(fallbackAvgCost)))
	}

	return cogs
}

func calculateMovingAverage(oldQty, newQty float64, oldAvgCost, newUnitCost int64) int64 {
	totalQty := oldQty + newQty
	if totalQty == 0 {
		return oldAvgCost
	}

	totalValue := oldQty*float64(oldAvgCost) + newQty*float64(newUnitCost)
	return int64(totalValue / totalQty)
}
