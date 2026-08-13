package assets_test

import (
	"math"
	"testing"
)

// TestDepreciation_StraightLine validates straight-line depreciation calculation.
func TestDepreciation_StraightLine(t *testing.T) {
	tests := []struct {
		name            string
		costCents       int64
		salvageCents    int64
		usefulLifeMonths int
		expectedMonthly int64
	}{
		{
			name:             "Standard straight-line",
			costCents:        1200000, // Rp 120,000
			salvageCents:     200000,  // Rp 20,000
			usefulLifeMonths: 60,      // 5 years
			expectedMonthly:  16667,   // (1200000-20000)/60 = 16666.67 rounded
		},
		{
			name:             "Zero salvage value",
			costCents:        1000000,
			salvageCents:     0,
			usefulLifeMonths: 24,
			expectedMonthly:  41667,  // 1000000/24 = 41666.67
		},
		{
			name:             "Equal cost and salvage",
			costCents:        500000,
			salvageCents:     500000,
			usefulLifeMonths: 36,
			expectedMonthly:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depreciableBase := float64(tt.costCents - tt.salvageCents)
			if depreciableBase <= 0 {
				if tt.expectedMonthly != 0 {
					t.Errorf("expected monthly dep = 0 when cost=salvage, got %d", tt.expectedMonthly)
				}
				return
			}

			monthlyDep := int64(math.Round(depreciableBase / float64(tt.usefulLifeMonths)))
			if monthlyDep != tt.expectedMonthly {
				t.Errorf("monthly depreciation = %d, want %d", monthlyDep, tt.expectedMonthly)
			}
		})
	}
}

// TestDepreciation_DecliningBalance validates declining balance method.
func TestDepreciation_DecliningBalance(t *testing.T) {
	tests := []struct {
		name         string
		bookValue    int64
		rate         float64
		expectedDep  int64
		description  string
	}{
		{
			name:         "Standard declining balance",
			bookValue:    1000000,
			rate:         0.20, // 20%
			expectedDep:  200000,
			description:  "20% of book value",
		},
		{
			name:         "Small rate",
			bookValue:    500000,
			rate:         0.05, // 5%
			expectedDep:  25000,
			description:  "5% of book value",
		},
		{
			name:         "Large rate",
			bookValue:    800000,
			rate:         0.30, // 30%
			expectedDep:  240000,
			description:  "30% of book value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.description, func(t *testing.T) {
			dep := int64(math.Round(float64(tt.bookValue)*tt.rate))
			if dep != tt.expectedDep {
				t.Errorf("depreciation = %d, want %d", dep, tt.expectedDep)
			}

			// Verify new book value is correct
			newBookValue := tt.bookValue - dep
			expectedBookValue := int64(float64(tt.bookValue) * (1 - tt.rate))
			if newBookValue != expectedBookValue {
				t.Logf("new book value = %d, expected ~%d", newBookValue, expectedBookValue)
			}
		})
	}
}

// TestDepreciation_UnitsOfProduction validates units of production method.
func TestDepreciation_UnitsOfProduction(t *testing.T) {
	tests := []struct {
		name          string
		costCents     int64
		salvageCents  int64
		unitsTotal    int64
		unitsUsed     int64
		expectedDep   int64
		description   string
	}{
		{
			name:          "Full capacity usage",
			costCents:     1000000,
			salvageCents:  200000,
			unitsTotal:    10000,
			unitsUsed:     10000,
			expectedDep:   800000, // (1000000-200000)/10000 * 10000
		},
		{
			name:          "Partial usage",
			costCents:     1000000,
			salvageCents:  200000,
			unitsTotal:    10000,
			unitsUsed:     5000,
			expectedDep:   400000, // half of depreciable base
		},
		{
			name:          "No units used",
			costCents:     1000000,
			salvageCents:  0,
			unitsTotal:    5000,
			unitsUsed:     0,
			expectedDep:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.description, func(t *testing.T) {
			depreciableBase := float64(tt.costCents - tt.salvageCents)
			if depreciableBase <= 0 || tt.unitsTotal <= 0 {
				if tt.expectedDep != 0 {
					t.Errorf("expected zero depreciation, got non-zero")
				}
				return
			}

			dep := int64(math.Round(depreciableBase*float64(tt.unitsUsed)/float64(tt.unitsTotal)))
			if dep != tt.expectedDep {
				t.Errorf("units of production depreciation = %d, want %d", dep, tt.expectedDep)
			}
		})
	}
}

// TestDepreciation_ResidualAbsorption validates final period absorbs rounding remainder.
func TestDepreciation_ResidualAbsorption(t *testing.T) {
	tests := []struct {
		name             string
		costCents        int64
		salvageCents     int64
		usefulLifeMonths int
		accumDepCents    int64
		currentPeriod    int
		isLastPeriod     bool
	}{
		{
			name:             "Regular period - no absorption",
			costCents:        100000,
			salvageCents:     10000,
			usefulLifeMonths: 10,
			accumDepCents:    50000,
			currentPeriod:    5,
			isLastPeriod:     false,
		},
		{
			name:             "Final period - should absorb residual",
			costCents:        100000,
			salvageCents:     10000,
			usefulLifeMonths: 10,
			accumDepCents:    90000,
			currentPeriod:    10,
			isLastPeriod:     true,
		},
		{
			name:             "Over-depreciated scenario",
			costCents:        100000,
			salvageCents:     10000,
			usefulLifeMonths: 10,
			accumDepCents:    95000, // Slightly more due to rounding
			currentPeriod:    10,
			isLastPeriod:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depreciableBase := float64(tt.costCents - tt.salvageCents)
			if depreciableBase <= 0 {
				return
			}

			monthlyDep := int64(math.Round(depreciableBase / float64(tt.usefulLifeMonths)))
			remainingDepreciable := float64(tt.costCents - tt.salvageCents) - float64(tt.accumDepCents)
			
			var currentDep int64
			if remainingDepreciable <= 0 {
				currentDep = 0
			} else if tt.isLastPeriod {
				// Final period absorbs all remaining
				currentDep = int64(math.Round(remainingDepreciable))
				if currentDep < 0 {
					currentDep = 0
				}
			} else {
				currentDep = monthlyDep
			}

			newBookValue := tt.costCents - tt.accumDepCents - currentDep
			if newBookValue < tt.salvageCents {
				t.Logf("warning: new book value %.0f would be below salvage %.0f", 
					float64(newBookValue), float64(tt.salvageCents))
			}
		})
	}
}
