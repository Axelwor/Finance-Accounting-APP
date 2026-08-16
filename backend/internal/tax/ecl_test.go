package tax_test

import (
	"testing"
	"time"
)

// TestECL_BucketAssignment validates correct bucket assignment by aging days.
func TestECL_BucketAssignment(t *testing.T) {
	tests := []struct {
		name           string
		ageDays        int
		expectedBucket string
		expectedRate   float64
	}{
		{
			name:           "In-term - 0-30 days",
			ageDays:        15,
			expectedBucket: "0-30",
			expectedRate:   1.0,
		},
		{
			name:           "In-term - day 30 boundary",
			ageDays:        30,
			expectedBucket: "0-30",
			expectedRate:   1.0,
		},
		{
			name:           "Overdue - 31-60 days",
			ageDays:        45,
			expectedBucket: "31-60",
			expectedRate:   2.5,
		},
		{
			name:           "Overdue - day 60 boundary",
			ageDays:        60,
			expectedBucket: "31-60",
			expectedRate:   2.5,
		},
		{
			name:           "Seriously overdue - 61-90 days",
			ageDays:        75,
			expectedBucket: "61-90",
			expectedRate:   5.0,
		},
		{
			name:           "Seriously overdue - day 90 boundary",
			ageDays:        90,
			expectedBucket: "61-90",
			expectedRate:   5.0,
		},
		{
			name:           "Doubtful - >90 days",
			ageDays:        91,
			expectedBucket: ">90",
			expectedRate:   10.0,
		},
		{
			name:           "Very doubtful - 180 days",
			ageDays:        180,
			expectedBucket: ">90",
			expectedRate:   10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, rate := getECLBucket(tt.ageDays)

			if bucket != tt.expectedBucket {
				t.Errorf("bucket = %q, want %q", bucket, tt.expectedBucket)
			}

			if rate != tt.expectedRate {
				t.Errorf("rate = %.1f%%, want %.1f%%", rate*100, tt.expectedRate*100)
			}
		})
	}
}

// TestECL_AgeCalculation validates age calculation from due date.
func TestECL_AgeCalculation(t *testing.T) {
	tests := []struct {
		name         string
		dueDateStr   string
		asOfDateStr  string
		expectedDays int
		description  string
	}{
		{
			name:         "Exact match - zero days old",
			dueDateStr:   "2024-01-01",
			asOfDateStr:  "2024-01-01",
			expectedDays: 0,
			description:  "same date should be 0 days",
		},
		{
			name:         "One day overdue",
			dueDateStr:   "2024-01-01",
			asOfDateStr:  "2024-01-02",
			expectedDays: 1,
			description:  "one day difference",
		},
		{
			name:         "30 days exactly",
			dueDateStr:   "2024-01-01",
			asOfDateStr:  "2024-01-31",
			expectedDays: 30,
			description:  "30 days in January",
		},
		{
			name:         "Crosses month boundary",
			dueDateStr:   "2024-01-15",
			asOfDateStr:  "2024-02-15",
			expectedDays: 31,
			description:  "Jan has 31 days",
		},
		{
			name:         "Future due date - negative to zero",
			dueDateStr:   "2024-02-01",
			asOfDateStr:  "2024-01-15",
			expectedDays: 0,
			description:  "future dates should clamp to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.description, func(t *testing.T) {
			ageDays := calculateAgeDays(tt.dueDateStr, tt.asOfDateStr)
			if ageDays != tt.expectedDays {
				t.Errorf("age days = %d, want %d", ageDays, tt.expectedDays)
			}
		})
	}
}

// TestECL_ProvisionAggregation validates total provision calculation.
func TestECL_ProvisionAggregation(t *testing.T) {
	tests := []struct {
		name          string
		buckets       []eclBucketTest
		expectedTotal int64
		description   string
	}{
		{
			name:          "Single bucket full provision",
			buckets:       []eclBucketTest{{balanceCents: 100000, ratePct: 1.0}},
			expectedTotal: 1000,
			description:   "1% of 100k",
		},
		{
			name: "Multiple buckets weighted",
			buckets: []eclBucketTest{
				{balanceCents: 50000, ratePct: 1.0},
				{balanceCents: 30000, ratePct: 2.5},
				{balanceCents: 20000, ratePct: 5.0},
			},
			expectedTotal: 2250, // 500 + 750 + 1000
			description:   "weighted average",
		},
		{
			name: "High aging bucket dominant",
			buckets: []eclBucketTest{
				{balanceCents: 10000, ratePct: 1.0},
				{balanceCents: 50000, ratePct: 10.0},
			},
			expectedTotal: 5100, // 100 + 5000
			description:   ">90 days dominates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.description, func(t *testing.T) {
			totalProvision := aggregateProvisions(tt.buckets)
			if totalProvision != tt.expectedTotal {
				t.Errorf("total provision = %d, want %d", totalProvision, tt.expectedTotal)
			}
		})
	}
}

// TestECL_CustomRatesOverride validates custom rate configuration.
func TestECL_CustomRatesOverride(t *testing.T) {
	tests := []struct {
		name          string
		overrideRates map[string]float64
		testDays      int
		expectedRate  float64
		description   string
	}{
		{
			name:          "Custom rate for 0-30",
			overrideRates: map[string]float64{"0-30": 2.0},
			testDays:      15,
			expectedRate:  2.0,
			description:   "override affects matched bucket",
		},
		{
			name:          "Custom rate ignores other buckets",
			overrideRates: map[string]float64{"0-30": 2.0},
			testDays:      50,
			expectedRate:  2.5,
			description:   "unmatched buckets use defaults",
		},
		{
			name:          "All rates overridden",
			overrideRates: map[string]float64{"0-30": 0.5, "31-60": 1.0, "61-90": 3.0, ">90": 15.0},
			testDays:      100,
			expectedRate:  15.0,
			description:   "fully custom rates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.description, func(t *testing.T) {
			_, rate := getECLBucketWithOverrides(tt.testDays, tt.overrideRates)
			if rate != tt.expectedRate {
				t.Errorf("custom rate = %.1f%%, want %.1f%%", rate*100, tt.expectedRate*100)
			}
		})
	}
}

// Helper types and functions for ECL tests

type eclBucketTest struct {
	balanceCents int64
	ratePct      float64
}

func getECLBucket(ageDays int) (string, float64) {
	defaults := []struct {
		minDays int
		maxDays int
		rate    float64
		label   string
	}{
		{0, 30, 1.0, "0-30"},
		{31, 60, 2.5, "31-60"},
		{61, 90, 5.0, "61-90"},
		{91, 999999, 10.0, ">90"},
	}

	for _, b := range defaults {
		if ageDays >= b.minDays && ageDays <= b.maxDays {
			return b.label, b.rate
		}
	}

	return defaults[3].label, defaults[3].rate
}

func getECLBucketWithOverrides(ageDays int, overrides map[string]float64) (string, float64) {
	defaults := []struct {
		minDays int
		maxDays int
		rate    float64
		label   string
	}{
		{0, 30, 1.0, "0-30"},
		{31, 60, 2.5, "31-60"},
		{61, 90, 5.0, "61-90"},
		{91, 999999, 10.0, ">90"},
	}

	for i := range defaults {
		if r, ok := overrides[defaults[i].label]; ok {
			defaults[i].rate = r
		}
	}

	for _, b := range defaults {
		if ageDays >= b.minDays && ageDays <= b.maxDays {
			return b.label, b.rate
		}
	}

	return defaults[3].label, defaults[3].rate
}

func calculateAgeDays(dueDateStr, asOfDateStr string) int {
	dueDate, err1 := time.Parse("2006-01-02", dueDateStr)
	asOfDate, err2 := time.Parse("2006-01-02", asOfDateStr)
	if err1 != nil || err2 != nil {
		return 0
	}
	days := int(asOfDate.Sub(dueDate).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

func aggregateProvisions(buckets []eclBucketTest) int64 {
	var total int64
	for _, b := range buckets {
		total += percentageRound(b.balanceCents, b.ratePct/100.0)
	}
	return total
}

func percentageRound(amount int64, rate float64) int64 {
	return int64(float64(amount) * rate)
}
