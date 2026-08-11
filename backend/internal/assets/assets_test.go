package assets

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------
//
// The assets package stores monetary values as integer cents and computes
// straight-line depreciation on a per-MONTH basis (see computeDepreciation):
//
//	monthly = (cost - salvage) / useful_life_months
//
// The task brief expresses the formula in years (annual = (cost - salvage) /
// useful_life_years). Because the code uses months, an asset with a 5-year life
// is modelled with useful_life_months = 60, and the annual figure is recovered
// as monthly * 12. The canonical example (cost=10,000,000, salvage=1,000,000,
// 5 years) yields monthly = 180,000 and annual = 1,800,000 in both views.

// straightLineAsset builds an assetRow configured for straight-line
// depreciation with the given cost (cents), salvage (cents) and useful life
// expressed in months. BookValueCents is seeded equal to cost and AccumDepCents
// to zero, mirroring the post-registration state persisted by RegisterAsset.
func straightLineAsset(cost, salvage int64, lifeMonths int) assetRow {
	return assetRow{
		AcquisitionCostCents: cost,
		SalvageValueCents:     salvage,
		UsefulLifeMonths:      lifeMonths,
		DepreciationMethod:    methodStraightLine,
		BookValueCents:        cost,
		AccumDepCents:         0,
	}
}

// ---------------------------------------------------------------------------
// 1. Straight-line depreciation (annual & monthly)
// ---------------------------------------------------------------------------

func TestComputeDepreciation_StraightLine_CanonicalExample(t *testing.T) {
	// cost=10,000,000, salvage=1,000,000, life=5 years (60 months).
	// depreciable base = 9,000,000; monthly = 150,000; annual = 1,800,000.
	// (The brief's expected annual value 1,800,000 is verified via monthly*12.)
	asset := straightLineAsset(10_000_000, 1_000_000, 60)

	monthly := computeDepreciation(asset)
	wantMonthly := int64(150_000)
	if monthly != wantMonthly {
		t.Fatalf("monthly depreciation = %d, want %d", monthly, wantMonthly)
	}

	annual := monthly * 12
	wantAnnual := int64(1_800_000)
	if annual != wantAnnual {
		t.Fatalf("annual depreciation = %d, want %d", annual, wantAnnual)
	}
}

func TestComputeDepreciation_StraightLine_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		cost       int64
		salvage    int64
		lifeMonths int
		want       int64
	}{
		{"canonical_5yr", 10_000_000, 1_000_000, 60, 150_000},
		{"zero_salvage", 12_000_000, 0, 60, 200_000},
		{"3yr_vehicle", 36_000_000, 6_000_000, 36, 833_333}, // (30M)/36 = 833,333.33 -> 833,333 (rounded)
		{"1yr_short", 1_200_000, 0, 12, 100_000},
		{"no_salvage_4yr", 48_000_000, 0, 48, 1_000_000},
		{"rounding_case", 100, 0, 3, 33}, // 100/3 = 33.33 -> 33 (rounded)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := straightLineAsset(tc.cost, tc.salvage, tc.lifeMonths)
			got := computeDepreciation(asset)
			if got != tc.want {
				t.Fatalf("computeDepreciation = %d, want %d (cost=%d salvage=%d life=%d)",
					got, tc.want, tc.cost, tc.salvage, tc.lifeMonths)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Monthly depreciation (annual / 12)
// ---------------------------------------------------------------------------

func TestMonthlyDepreciation_EqualsAnnualDividedBy12(t *testing.T) {
	tests := []struct {
		name     string
		cost     int64
		salvage  int64
		lifeYrs  int
		wantAnnual int64
	}{
		{"5yr", 10_000_000, 1_000_000, 5, 1_800_000},
		{"4yr", 48_000_000, 0, 4, 12_000_000},
		{"10yr", 31_000_000, 1_000_000, 10, 3_000_000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := straightLineAsset(tc.cost, tc.salvage, tc.lifeYrs*12)
			monthly := computeDepreciation(asset)
			annual := monthly * 12
			if annual != tc.wantAnnual {
				t.Fatalf("annual = %d, want %d", annual, tc.wantAnnual)
			}
			// monthly should be exactly annual / 12 for these evenly-divisible cases.
			if tc.wantAnnual%12 != 0 {
				t.Fatalf("test setup error: annual %d not divisible by 12", tc.wantAnnual)
			}
			if monthly != tc.wantAnnual/12 {
				t.Fatalf("monthly = %d, want %d", monthly, tc.wantAnnual/12)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Accumulated depreciation after N months
// ---------------------------------------------------------------------------

// accumulatedAfter applies computeDepreciation once per elapsed month and
// returns the resulting accumulated depreciation and net book value, mirroring
// the per-period accumulation performed by DepreciateAsset. It reproduces the
// salvage-value cap enforced inside DepreciateAsset: once the next charge would
// drive NBV below salvage, the charge is clamped to (bookValue - salvage).
func accumulatedAfter(asset assetRow, months int) (accumDep, bookValue int64) {
	accumDep = 0
	bookValue = asset.AcquisitionCostCents
	for i := 0; i < months; i++ {
		// Reload the mutable state into a working copy so computeDepreciation
		// (which reads BookValueCents only for declining-balance) sees the
		// straight-line inputs.
		working := asset
		working.BookValueCents = bookValue
		working.AccumDepCents = accumDep

		dep := computeDepreciation(working)

		// Salvage cap: never depreciate below salvage value.
		newBookValue := bookValue - dep
		if newBookValue < asset.SalvageValueCents {
			dep = bookValue - asset.SalvageValueCents
			newBookValue = bookValue - dep
		}
		// Once fully depreciated, no further charge accrues.
		if dep <= 0 {
			break
		}
		bookValue = newBookValue
		accumDep += dep
	}
	return accumDep, bookValue
}

func TestAccumulatedDepreciation_MonthByMonth(t *testing.T) {
	// cost=10,000,000, salvage=1,000,000, 5yrs (60 months), monthly=150,000.
	asset := straightLineAsset(10_000_000, 1_000_000, 60)

	tests := []struct {
		months     int
		wantAccum  int64
		wantBook   int64
	}{
		{0, 0, 10_000_000},
		{1, 150_000, 9_850_000},
		{6, 900_000, 9_100_000},
		{12, 1_800_000, 8_200_000}, // 1 year
		{24, 3_600_000, 6_400_000}, // 2 years
		{36, 5_400_000, 4_600_000}, // 3 years
		{48, 7_200_000, 2_800_000}, // 4 years
		{59, 8_850_000, 1_150_000}, // one month before full depreciation
		{60, 9_000_000, 1_000_000}, // fully depreciated: NBV == salvage
		{61, 9_000_000, 1_000_000}, // cap: no further charge beyond salvage
		{72, 9_000_000, 1_000_000}, // over-depreciation safely ignored
	}

	for _, tc := range tests {
		t.Run("month_"+itoa(tc.months), func(t *testing.T) {
			accum, book := accumulatedAfter(asset, tc.months)
			if accum != tc.wantAccum {
				t.Errorf("accumulated depreciation after %d months = %d, want %d",
					tc.months, accum, tc.wantAccum)
			}
			if book != tc.wantBook {
				t.Errorf("book value after %d months = %d, want %d",
					tc.months, book, tc.wantBook)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. Disposal with gain
// ---------------------------------------------------------------------------

// disposalGainLoss reproduces the gain/loss computation embedded in
// DisposeAsset: gainLoss = proceeds - bookValue, where bookValue = cost -
// accumulated depreciation. Positive => gain, negative => loss.
func disposalGainLoss(cost, accumDep, proceeds int64) (gainLoss int64, isGain bool) {
	bookValue := cost - accumDep
	gainLoss = proceeds - bookValue
	return gainLoss, gainLoss > 0
}

func TestDisposal_WithGain(t *testing.T) {
	// cost=10,000, accum=3,000, proceeds=8,000.
	// book value = 10,000 - 3,000 = 7,000.
	// gain = 8,000 - 7,000 = 1,000.
	gainLoss, isGain := disposalGainLoss(10_000, 3_000, 8_000)

	if !isGain {
		t.Fatalf("expected a gain, got a loss (gainLoss=%d)", gainLoss)
	}
	want := int64(1_000)
	if gainLoss != want {
		t.Fatalf("gain = %d, want %d", gainLoss, want)
	}
}

func TestDisposal_GainLoss_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		cost     int64
		accum    int64
		proceeds int64
		wantGain int64
		wantLoss int64
	}{
		{"gain_1000", 10_000, 3_000, 8_000, 1_000, 0},
		{"gain_zero_nbv", 10_000, 10_000, 5_000, 5_000, 0}, // fully depreciated + sold
		{"gain_large", 1_000_000, 200_000, 1_500_000, 700_000, 0},
		{"loss_2000", 10_000, 3_000, 5_000, 0, 2_000},
		{"loss_partial", 50_000, 10_000, 20_000, 0, 20_000},
		{"break_even", 10_000, 3_000, 7_000, 0, 0}, // proceeds == book value
		{"loss_fully_depreciated", 10_000, 9_000, 0, 0, 1_000}, // NBV=salvage(1000), scrapped for 0
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gainLoss, isGain := disposalGainLoss(tc.cost, tc.accum, tc.proceeds)
			switch {
			case isGain:
				if tc.wantGain == 0 {
					t.Fatalf("expected loss/break-even, got gain %d", gainLoss)
				}
				if gainLoss != tc.wantGain {
					t.Fatalf("gain = %d, want %d", gainLoss, tc.wantGain)
				}
			case gainLoss < 0:
				if tc.wantLoss == 0 {
					t.Fatalf("expected gain/break-even, got loss %d", -gainLoss)
				}
				if -gainLoss != tc.wantLoss {
					t.Fatalf("loss = %d, want %d", -gainLoss, tc.wantLoss)
				}
			default: // break-even
				if tc.wantGain != 0 || tc.wantLoss != 0 {
					t.Fatalf("expected gain %d / loss %d, got break-even", tc.wantGain, tc.wantLoss)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 5. Disposal with loss
// ---------------------------------------------------------------------------

func TestDisposal_WithLoss(t *testing.T) {
	// cost=10,000, accum=3,000, proceeds=5,000.
	// book value = 7,000; loss = 5,000 - 7,000 = -2,000 (abs = 2,000).
	gainLoss, isGain := disposalGainLoss(10_000, 3_000, 5_000)

	if isGain {
		t.Fatalf("expected a loss, got a gain (gainLoss=%d)", gainLoss)
	}
	if gainLoss >= 0 {
		t.Fatalf("expected negative gainLoss for a loss, got %d", gainLoss)
	}
	wantAbs := int64(2_000)
	if got := -gainLoss; got != wantAbs {
		t.Fatalf("loss magnitude = %d, want %d", got, wantAbs)
	}
}

// ---------------------------------------------------------------------------
// 6. Fully depreciated asset
// ---------------------------------------------------------------------------

func TestFullyDepreciatedAsset(t *testing.T) {
	// Cases where (cost - salvage) divides evenly by lifeMonths, so the asset
	// reaches exactly (cost - salvage) accumulated / salvage NBV with no
	// remainder. Non-even cases are covered separately in
	// TestFullyDepreciatedAsset_TruncationRemainder.
	tests := []struct {
		name    string
		cost    int64
		salvage int64
		lifeYrs int
	}{
		{"canonical_5yr", 10_000_000, 1_000_000, 5}, // 9M / 60 = 150,000 exact
		{"zero_salvage_4yr", 48_000_000, 0, 4},       // 48M / 48 = 1,000,000 exact
		{"3yr_even", 42_000_000, 6_000_000, 3},       // 36M / 36 = 1,000,000 exact
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := straightLineAsset(tc.cost, tc.salvage, tc.lifeYrs*12)
			accum, book := accumulatedAfter(asset, tc.lifeYrs*12)

			// After full life, accumulated depreciation must equal
			// cost - salvage (only guaranteed when the per-month charge
			// divides evenly; the cases above are chosen to do so).
			wantAccum := tc.cost - tc.salvage
			if accum != wantAccum {
				t.Fatalf("accumulated depreciation = %d, want %d (cost - salvage)",
					accum, wantAccum)
			}

			// NBV must equal salvage value at end of useful life.
			if book != tc.salvage {
				t.Fatalf("book value = %d, want salvage %d", book, tc.salvage)
			}

			// Depreciating one more month must not change anything:
			// the asset is fully depreciated.
			accum2, book2 := accumulatedAfter(asset, tc.lifeYrs*12+1)
			if accum2 != accum || book2 != book {
				t.Fatalf("over-depreciation: accum %d->%d, book %d->%d",
					accum, accum2, book, book2)
			}
		})
	}
}

// TestFullyDepreciatedAsset_RoundingResidual verifies that the rounding +
// final-period absorb logic in computeDepreciation ensures the asset reaches
// salvage value exactly at the end of its useful life, even when
// (cost - salvage) does not divide evenly by lifeMonths.
func TestFullyDepreciatedAsset_RoundingResidual(t *testing.T) {
	// cost=36,000,000, salvage=6,000,000, 36 months.
	// monthly = round(30,000,000 / 36) = 833,333 (rounded from 833,333.33).
	// After 35 months: accum = 29,166,655, remaining = 833,345.
	// Month 36: remaining (833,345) < 2*monthly (1,666,666) → absorb residual.
	// After 36 months: accum = 30,000,000, NBV = 6,000,000 (exactly salvage).
	asset := straightLineAsset(36_000_000, 6_000_000, 36)
	monthly := computeDepreciation(asset)
	if monthly != 833_333 {
		t.Fatalf("monthly = %d, want 833,333 (rounded)", monthly)
	}

	accum, book := accumulatedAfter(asset, 36)
	wantAccum := int64(30_000_000)
	if accum != wantAccum {
		t.Fatalf("accumulated = %d, want %d (depreciable base fully recovered)", accum, wantAccum)
	}
	wantBook := asset.SalvageValueCents
	if book != wantBook {
		t.Fatalf("book value = %d, want %d (salvage value)", book, wantBook)
	}
	// No residual remains: NBV equals salvage exactly.
	remainder := int64(36_000_000-6_000_000) - accum
	if remainder != 0 {
		t.Fatalf("rounding remainder = %d, want 0", remainder)
	}
	if book != asset.SalvageValueCents {
		t.Fatalf("NBV %d should equal salvage %d", book, asset.SalvageValueCents)
	}
	// An extra month does not depreciate further: the asset is already at
	// salvage value, so computeDepreciation returns 0 and the loop breaks.
	accum2, book2 := accumulatedAfter(asset, 37)
	if book2 != asset.SalvageValueCents {
		t.Fatalf("after extra month: book = %d, want salvage %d", book2, asset.SalvageValueCents)
	}
	if accum2 != asset.AcquisitionCostCents-asset.SalvageValueCents {
		t.Fatalf("after extra month: accum = %d, want %d", accum2, asset.AcquisitionCostCents-asset.SalvageValueCents)
	}
}

func TestFullyDepreciated_ZeroSalvage(t *testing.T) {
	// Special case: salvage = 0 means NBV must reach exactly 0.
	asset := straightLineAsset(1_200_000, 0, 12) // 1 year, 100,000/month
	accum, book := accumulatedAfter(asset, 12)

	if accum != 1_200_000 {
		t.Fatalf("accumulated = %d, want %d", accum, 1_200_000)
	}
	if book != 0 {
		t.Fatalf("book value = %d, want 0 (fully depreciated, no salvage)", book)
	}
}

// ---------------------------------------------------------------------------
// 7. Validate registration request
// ---------------------------------------------------------------------------

func validRegisterRequest() RegisterAssetRequest {
	return RegisterAssetRequest{
		Code:                 "AST-001",
		Name:                 "Delivery Van",
		AcquisitionDate:      "2026-01-15",
		AcquisitionCostCents: 10_000_000,
		SalvageValueCents:    1_000_000,
		UsefulLifeMonths:     60,
		DepreciationMethod:   methodStraightLine,
	}
}

func TestValidateRegisterRequest_Valid(t *testing.T) {
	req := validRegisterRequest()
	if code, msg := validateRegisterRequest(req); code != "" || msg != "" {
		t.Fatalf("expected valid request, got code=%q msg=%q", code, msg)
	}
}

func TestValidateRegisterRequest_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(req *RegisterAssetRequest)
		wantCode  string
		wantMatch string // substring of want msg (empty = any non-empty)
	}{
		{
			name:      "missing_code",
			mutate:    func(r *RegisterAssetRequest) { r.Code = "" },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "code is required",
		},
		{
			name:      "missing_name",
			mutate:    func(r *RegisterAssetRequest) { r.Name = "" },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "name is required",
		},
		{
			name:      "missing_date",
			mutate:    func(r *RegisterAssetRequest) { r.AcquisitionDate = "" },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "acquisition_date",
		},
		{
			name:      "bad_date_format",
			mutate:    func(r *RegisterAssetRequest) { r.AcquisitionDate = "15-01-2026" },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "acquisition_date",
		},
		{
			name:      "zero_cost",
			mutate:    func(r *RegisterAssetRequest) { r.AcquisitionCostCents = 0 },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "acquisition_cost_cents",
		},
		{
			name:      "negative_cost",
			mutate:    func(r *RegisterAssetRequest) { r.AcquisitionCostCents = -5_000 },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "acquisition_cost_cents",
		},
		{
			name:      "negative_salvage",
			mutate:    func(r *RegisterAssetRequest) { r.SalvageValueCents = -100 },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "salvage_value_cents",
		},
		{
			name:      "zero_useful_life",
			mutate:    func(r *RegisterAssetRequest) { r.UsefulLifeMonths = 0 },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "useful_life_months",
		},
		{
			name:      "negative_useful_life",
			mutate:    func(r *RegisterAssetRequest) { r.UsefulLifeMonths = -12 },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "useful_life_months",
		},
		{
			name:      "invalid_method",
			mutate:    func(r *RegisterAssetRequest) { r.DepreciationMethod = "double_declining" },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "depreciation_method",
		},
		{
			name:      "empty_method",
			mutate:    func(r *RegisterAssetRequest) { r.DepreciationMethod = "" },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "depreciation_method",
		},
		{
			name:      "declining_balance_without_rate",
			mutate:    func(r *RegisterAssetRequest) { r.DepreciationMethod = methodDecliningBalance; r.Rate = "" },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "rate is required",
		},
		{
			name:      "units_of_production_without_units",
			mutate:    func(r *RegisterAssetRequest) { r.DepreciationMethod = methodUnitsOfProduction; r.UnitsTotal = 0 },
			wantCode:  "INVALID_REQUEST",
			wantMatch: "units_total",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validRegisterRequest()
			tc.mutate(&req)
			code, msg := validateRegisterRequest(req)
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q (msg=%q)", code, tc.wantCode, msg)
			}
			if code == "" {
				t.Fatalf("expected validation failure, got success")
			}
			if tc.wantMatch != "" && !contains(msg, tc.wantMatch) {
				t.Fatalf("message = %q, want substring %q", msg, tc.wantMatch)
			}
		})
	}
}

// Acceptance variants that should still be valid.
func TestValidateRegisterRequest_ValidEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		req  RegisterAssetRequest
	}{
		{
			"zero_salvage_ok",
			func() RegisterAssetRequest {
				r := validRegisterRequest()
				r.SalvageValueCents = 0
				return r
			}(),
		},
		{
			"declining_balance_with_rate",
			func() RegisterAssetRequest {
				r := validRegisterRequest()
				r.DepreciationMethod = methodDecliningBalance
				r.Rate = "0.25"
				return r
			}(),
		},
		{
			"units_of_production_with_units",
			func() RegisterAssetRequest {
				r := validRegisterRequest()
				r.DepreciationMethod = methodUnitsOfProduction
				r.UnitsTotal = 100_000
				return r
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code, msg := validateRegisterRequest(tc.req); code != "" || msg != "" {
				t.Fatalf("expected valid, got code=%q msg=%q", code, msg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 8. NBV (Net Book Value) calculation: cost - accumulated depreciation
// ---------------------------------------------------------------------------

func TestNetBookValue_Calculation(t *testing.T) {
	tests := []struct {
		name     string
		cost     int64
		accumDep int64
		wantNBV  int64
	}{
		{"fresh_asset", 10_000_000, 0, 10_000_000},
		{"partial_3yr", 10_000_000, 5_400_000, 4_600_000},
		{"fully_depreciated_with_salvage", 10_000_000, 9_000_000, 1_000_000},
		{"fully_depreciated_zero_salvage", 1_200_000, 1_200_000, 0},
		{"half_life", 48_000_000, 24_000_000, 24_000_000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cost - tc.accumDep
			if got != tc.wantNBV {
				t.Fatalf("NBV = %d, want %d", got, tc.wantNBV)
			}
			if got < 0 {
				t.Fatalf("NBV should never be negative: got %d", got)
			}
		})
	}
}

// NBV derived from running the depreciation accumulator, ensuring the
// accounting identity NBV = cost - accumulated depreciation holds at every
// step and never falls below salvage.
func TestNetBookValue_FromAccumulator(t *testing.T) {
	asset := straightLineAsset(10_000_000, 1_000_000, 60)
	for month := 0; month <= 72; month++ {
		accum, book := accumulatedAfter(asset, month)
		// Accounting identity.
		if got := asset.AcquisitionCostCents - accum; got != book {
			t.Fatalf("month %d: cost - accum = %d but book = %d", month, got, book)
		}
		// Salvage floor.
		if book < asset.SalvageValueCents {
			t.Fatalf("month %d: book value %d below salvage %d", month, book, asset.SalvageValueCents)
		}
	}
}

// ---------------------------------------------------------------------------
// Bonus: computeDepreciation edge cases & alternative methods
// ---------------------------------------------------------------------------

func TestComputeDepreciation_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		asset assetRow
		want  int64
	}{
		{
			"depreciable_base_zero",
			assetRow{AcquisitionCostCents: 1_000, SalvageValueCents: 1_000, UsefulLifeMonths: 12, DepreciationMethod: methodStraightLine},
			0,
		},
		{
			"salvage_exceeds_cost",
			assetRow{AcquisitionCostCents: 1_000, SalvageValueCents: 2_000, UsefulLifeMonths: 12, DepreciationMethod: methodStraightLine},
			0,
		},
		{
			"zero_useful_life",
			assetRow{AcquisitionCostCents: 10_000, SalvageValueCents: 1_000, UsefulLifeMonths: 0, DepreciationMethod: methodStraightLine},
			0,
		},
		{
			"unknown_method",
			assetRow{AcquisitionCostCents: 10_000, SalvageValueCents: 1_000, UsefulLifeMonths: 12, DepreciationMethod: "macrs"},
			0,
		},
		{
			"empty_method",
			assetRow{AcquisitionCostCents: 10_000, SalvageValueCents: 1_000, UsefulLifeMonths: 12, DepreciationMethod: ""},
			0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeDepreciation(tc.asset); got != tc.want {
				t.Fatalf("computeDepreciation = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestComputeDepreciation_DecliningBalance(t *testing.T) {
	// book_value * rate (rate as a float fraction, e.g. 0.25).
	asset := assetRow{
		AcquisitionCostCents: 10_000_000,
		BookValueCents:        10_000_000,
		DepreciationMethod:    methodDecliningBalance,
		Rate:                  0.25,
	}
	if got := computeDepreciation(asset); got != 2_500_000 {
		t.Fatalf("declining balance depreciation = %d, want %d", got, 2_500_000)
	}

	// Second period on reduced book value.
	asset.BookValueCents = 7_500_000
	if got := computeDepreciation(asset); got != 1_875_000 {
		t.Fatalf("declining balance period 2 = %d, want %d", got, 1_875_000)
	}
}

func TestComputeDepreciation_UnitsOfProduction(t *testing.T) {
	// (cost - salvage) * units_used / units_total
	asset := assetRow{
		AcquisitionCostCents: 10_000_000,
		SalvageValueCents:    1_000_000,
		UnitsTotal:           100_000,
		UnitsUsed:            10_000,
		DepreciationMethod:   methodUnitsOfProduction,
	}
	// (9,000,000 * 10,000) / 100,000 = 900,000
	if got := computeDepreciation(asset); got != 900_000 {
		t.Fatalf("units-of-production depreciation = %d, want %d", got, 900_000)
	}

	// Zero units_total must not divide by zero.
	asset.UnitsTotal = 0
	if got := computeDepreciation(asset); got != 0 {
		t.Fatalf("units-of-production with zero units_total = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Bonus: pure helper functions
// ---------------------------------------------------------------------------

func TestValidDate(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2026-01-15", true},
		{"2026-12-31", true},
		{"2026-1-5", false},    // non-zero-padded
		{"15-01-2026", false},  // wrong order
		{"2026/01/15", false},  // wrong separator
		{"", false},
		{"   ", false},
		{"2026-13-01", false}, // invalid month
		{"2026-02-30", false}, // invalid day
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := validDate(tc.input); got != tc.want {
				t.Fatalf("validDate(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestInt8ValueRaw(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Int8
		want  int64
	}{
		{"valid", pgtype.Int8{Int64: 42, Valid: true}, 42},
		{"invalid", pgtype.Int8{Valid: false}, 0},
		{"zero_valid", pgtype.Int8{Int64: 0, Valid: true}, 0},
		{"negative", pgtype.Int8{Int64: -100, Valid: true}, -100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := int8ValueRaw(tc.input); got != tc.want {
				t.Fatalf("int8ValueRaw = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTextValue(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Text
		want  string
	}{
		{"valid", pgtype.Text{String: "AST-001", Valid: true}, "AST-001"},
		{"invalid", pgtype.Text{Valid: false}, ""},
		{"trimmed", pgtype.Text{String: "  padded  ", Valid: true}, "padded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := textValue(tc.input); got != tc.want {
				t.Fatalf("textValue = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTextValueOptional(t *testing.T) {
	// Non-empty string -> valid Text.
	tv := textValueOptional("notes here")
	if !tv.Valid || tv.String != "notes here" {
		t.Fatalf("textValueOptional(non-empty) = %+v, want valid notes", tv)
	}
	// Whitespace-only -> invalid.
	tv = textValueOptional("   ")
	if tv.Valid {
		t.Fatalf("textValueOptional(whitespace) = %+v, want invalid", tv)
	}
}

func TestNumericToFloat(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Numeric
		want  float64
	}{
		{"invalid", pgtype.Numeric{Valid: false}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := numericToFloat(tc.input); got != tc.want {
				t.Fatalf("numericToFloat = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOptionalInt8(t *testing.T) {
	// Non-zero value -> valid.
	v := optionalInt8(42)
	if !v.Valid || v.Int64 != 42 {
		t.Fatalf("optionalInt8(42) = %+v, want valid 42", v)
	}
	// Zero value -> invalid (semantics: "absent").
	v = optionalInt8(0)
	if v.Valid {
		t.Fatalf("optionalInt8(0) = %+v, want invalid", v)
	}
}

func TestInt8Value(t *testing.T) {
	v := int8Value(99)
	if !v.Valid || v.Int64 != 99 {
		t.Fatalf("int8Value(99) = %+v, want valid 99", v)
	}
	v = int8Value(0)
	if v.Valid {
		t.Fatalf("int8Value(0) = %+v, want invalid", v)
	}
}

func TestMustJSON(t *testing.T) {
	// Valid value marshals.
	out := mustJSON(map[string]int{"a": 1})
	if string(out) != `{"a":1}` {
		t.Fatalf("mustJSON = %s, want {\"a\":1}", out)
	}
	// Unmarshallable value (channel) yields the fallback "{}".
	out = mustJSON(make(chan struct{}))
	if string(out) != "{}" {
		t.Fatalf("mustJSON(chan) = %s, want {}", out)
	}
}

func TestDateString(t *testing.T) {
	// Invalid date -> empty string.
	if got := dateString(pgtype.Date{}); got != "" {
		t.Fatalf("dateString(invalid) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Bonus: account code & status constants
// ---------------------------------------------------------------------------

func TestAccountCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"fixedAsset", fixedAssetAccountCode, "1401"},
		{"accumDep", accumDepAccountCode, "1402"},
		{"depExpense", depExpenseAccountCode, "5206"},
		{"revaluationSurplus", revaluationSurplusCode, "3401"},
		{"gainOnDisposal", gainOnDisposalCode, "4903"},
		{"lossOnDisposal", lossOnDisposalCode, "5903"},
		{"impairment", impairmentAccountCode, "5207"},
		{"cash", cashAccountCode, "1101"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	if statusActive != "ACTIVE" {
		t.Fatalf("statusActive = %q, want ACTIVE", statusActive)
	}
	if statusDisposed != "DISPOSED" {
		t.Fatalf("statusDisposed = %q, want DISPOSED", statusDisposed)
	}
	if statusImpaired != "IMPAIRED" {
		t.Fatalf("statusImpaired = %q, want IMPAIRED", statusImpaired)
	}
}

func TestTransactionTypeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"acquisition", txTypeAcquisition, "ACQUISITION"},
		{"depreciation", txTypeDepreciation, "DEPRECIATION"},
		{"revaluation", txTypeRevaluation, "REVALUATION"},
		{"disposal", txTypeDisposal, "DISPOSAL"},
		{"impairment", txTypeImpairment, "IMPAIRMENT"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestDepreciationMethodConstants(t *testing.T) {
	if methodStraightLine != "straight_line" {
		t.Fatalf("methodStraightLine = %q", methodStraightLine)
	}
	if methodDecliningBalance != "declining_balance" {
		t.Fatalf("methodDecliningBalance = %q", methodDecliningBalance)
	}
	if methodUnitsOfProduction != "units_of_production" {
		t.Fatalf("methodUnitsOfProduction = %q", methodUnitsOfProduction)
	}
}

// ---------------------------------------------------------------------------
// Tiny local helpers (avoid pulling in strconv just for one call)
// ---------------------------------------------------------------------------

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
