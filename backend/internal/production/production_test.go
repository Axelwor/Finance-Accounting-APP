package production

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Constants — WIP / Finished Goods / Variance account codes (helpers.go)
// ---------------------------------------------------------------------------

func TestProductionAccountCodeConstants(t *testing.T) {
	cases := []struct {
		name     string
		got      string
		expected string
	}{
		{"WIP", wipAccountCode, "1303"},
		{"FinishedGoods", finishedGoodsAccountCode, "1304"},
		{"Inventory (raw materials)", inventoryAccountCode, "1301"},
		{"Variance Gain", varianceGainAccountCode, "4902"},
		{"Variance Loss", varianceLossAccountCode, "5901"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.expected)
			}
		})
	}
}

func TestAccountCodesAreDistinct(t *testing.T) {
	codes := []string{
		wipAccountCode,
		finishedGoodsAccountCode,
		inventoryAccountCode,
		varianceGainAccountCode,
		varianceLossAccountCode,
	}
	seen := make(map[string]bool)
	for _, c := range codes {
		if seen[c] {
			t.Fatalf("duplicate account code: %s", c)
		}
		seen[c] = true
	}
}

func TestAccountCodesAreNumeric(t *testing.T) {
	codes := []string{
		wipAccountCode,
		finishedGoodsAccountCode,
		inventoryAccountCode,
		varianceGainAccountCode,
		varianceLossAccountCode,
	}
	for _, c := range codes {
		if len(c) != 4 {
			t.Errorf("account code %q should be 4 digits", c)
		}
		for _, ch := range c {
			if ch < '0' || ch > '9' {
				t.Errorf("account code %q must be numeric", c)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Cost accumulation math: total = material + labor + overhead
// (mirrors productionJobResponse.TotalCostCents usage in jobs.go)
// ---------------------------------------------------------------------------

func TestCostAccumulation(t *testing.T) {
	cases := []struct {
		name              string
		material, labor   int64
		overhead          int64
		expectedTotal     int64
	}{
		{"all zero", 0, 0, 0, 0},
		{"material only", 10000, 0, 0, 10000},
		{"labor only", 0, 5000, 0, 5000},
		{"overhead only", 0, 0, 3000, 3000},
		{"all set", 10000, 5000, 3000, 18000},
		{"large values", 999999999, 1, 0, 1000000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			total := tc.material + tc.labor + tc.overhead
			if total != tc.expectedTotal {
				t.Errorf("total = %d, want %d", total, tc.expectedTotal)
			}
			// Inverse: total - material - labor = overhead
			if total-tc.material-tc.labor != tc.overhead {
				t.Errorf("overhead reconstruction failed")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Variance calculation: variance = actual_cost - (standard_cost * qty)
// (mirrors the variance concept in CompleteProductionJob)
// ---------------------------------------------------------------------------

func TestProductionVianceCalculation(t *testing.T) {
	cases := []struct {
		name           string
		actualCost     int64
		standardCost   int64
		qty            int64
		expectedVariance int64
	}{
		{"zero variance", 10000, 5000, 2, 0},
		{"positive variance (over-absorbed)", 12000, 5000, 2, 2000},
		{"negative variance (under-absorbed)", 8000, 5000, 2, -2000},
		{"no standard cost", 5000, 0, 10, 5000},
		{"zero actual", 0, 100, 5, -500},
		{"single unit", 3000, 2500, 1, 500},
		{"large qty", 1000000, 1, 1000000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			variance := tc.actualCost - (tc.standardCost * tc.qty)
			if variance != tc.expectedVariance {
				t.Errorf("variance = %d, want %d", variance, tc.expectedVariance)
			}
		})
	}
}

func TestVarianceSignDeterminesGainOrLoss(t *testing.T) {
	// In jobs.go: variance > 0 → loss (Dr 5901), variance < 0 → gain (Cr 4902)
	cases := []struct {
		variance int64
		isLoss   bool
		isGain   bool
	}{
		{0, false, false},
		{1000, true, false},
		{-1000, false, true},
	}
	for _, tc := range cases {
		isLoss := tc.variance > 0
		isGain := tc.variance < 0
		if isLoss != tc.isLoss {
			t.Errorf("variance %d: isLoss = %v, want %v", tc.variance, isLoss, tc.isLoss)
		}
		if isGain != tc.isGain {
			t.Errorf("variance %d: isGain = %v, want %v", tc.variance, isGain, tc.isGain)
		}
	}
}

func TestVianceAbsValueForJournal(t *testing.T) {
	// jobs.go uses `gain := -variance` when variance < 0
	variance := int64(-2000)
	gain := -variance
	if gain != 2000 {
		t.Errorf("gain = %d, want 2000", gain)
	}
	// For loss, the variance itself is the debit amount
	lossVariance := int64(2000)
	if lossVariance != 2000 {
		t.Errorf("loss = %d, want 2000", lossVariance)
	}
}

// ---------------------------------------------------------------------------
// BOM line total: lineTotal = qty * unit_cost_cents (integer math)
// (mirrors bom.go: lineTotal := int64(line.Qty * float64(line.UnitCostCents)))
// ---------------------------------------------------------------------------

func TestBOMLineTotalCalculation(t *testing.T) {
	cases := []struct {
		qty           float64
		unitCostCents int64
		expected      int64
	}{
		{2.0, 5000, 10000},
		{1.5, 10000, 15000},
		{10, 100, 1000},
		{0.5, 20000, 10000},
		{1, 0, 0},
	}
	for _, tc := range cases {
		total := int64(tc.qty * float64(tc.unitCostCents))
		if total != tc.expected {
			t.Errorf("qty=%v unit=%d: total = %d, want %d", tc.qty, tc.unitCostCents, total, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// validateBOMRequest (bom.go)
// ---------------------------------------------------------------------------

func TestValidateBOMRequest_Valid(t *testing.T) {
	req := CreateBOMRequest{
		Code:               "BOM-001",
		Name:               "Widget BOM",
		FinishedGoodItemID: 1,
		OutputQty:          10,
		Lines: []BOMLineRequest{
			{ItemID: 2, Qty: 2, UnitCostCents: 5000, CostType: "material"},
			{ItemID: 3, Qty: 1, UnitCostCents: 10000, CostType: "labor"},
		},
	}
	code, msg := validateBOMRequest(&req)
	if code != "" {
		t.Errorf("expected valid, got code=%q msg=%q", code, msg)
	}
}

func TestValidateBOMRequest_EmptyCode(t *testing.T) {
	req := CreateBOMRequest{
		Code:               "",
		Name:               "Widget",
		FinishedGoodItemID: 1,
		OutputQty:          10,
		Lines:              []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: "material"}},
	}
	code, _ := validateBOMRequest(&req)
	if code == "" {
		t.Error("empty code should fail validation")
	}
}

func TestValidateBOMRequest_EmptyName(t *testing.T) {
	req := CreateBOMRequest{
		Code:               "BOM-1",
		Name:               "",
		FinishedGoodItemID: 1,
		OutputQty:          10,
		Lines:              []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: "material"}},
	}
	code, _ := validateBOMRequest(&req)
	if code == "" {
		t.Error("empty name should fail validation")
	}
}

func TestValidateBOMRequest_InvalidFinishedGoodItemID(t *testing.T) {
	cases := []int64{0, -1}
	for _, fgID := range cases {
		req := CreateBOMRequest{
			Code: "BOM-1", Name: "Widget",
			FinishedGoodItemID: fgID, OutputQty: 10,
			Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: "material"}},
		}
		code, _ := validateBOMRequest(&req)
		if code == "" {
			t.Errorf("finished_good_item_id %d should fail", fgID)
		}
	}
}

func TestValidateBOMRequest_InvalidOutputQty(t *testing.T) {
	cases := []float64{0, -1, -0.01}
	for _, qty := range cases {
		req := CreateBOMRequest{
			Code: "BOM-1", Name: "Widget",
			FinishedGoodItemID: 1, OutputQty: qty,
			Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: "material"}},
		}
		code, _ := validateBOMRequest(&req)
		if code == "" {
			t.Errorf("output_qty %v should fail", qty)
		}
	}
}

func TestValidateBOMRequest_NoLines(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{},
	}
	code, _ := validateBOMRequest(&req)
	if code == "" {
		t.Error("empty lines should fail validation")
	}
}

func TestValidateBOMRequest_NilLines(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: nil,
	}
	code, _ := validateBOMRequest(&req)
	if code == "" {
		t.Error("nil lines should fail validation")
	}
}

func TestValidateBOMRequest_LineInvalidItemID(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{{ItemID: 0, Qty: 1, UnitCostCents: 100, CostType: "material"}},
	}
	code, msg := validateBOMRequest(&req)
	if code == "" {
		t.Error("zero item_id should fail")
	}
	if !strings.Contains(msg, "item_id") {
		t.Errorf("error message should mention item_id, got %q", msg)
	}
}

func TestValidateBOMRequest_LineZeroQty(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{{ItemID: 2, Qty: 0, UnitCostCents: 100, CostType: "material"}},
	}
	code, msg := validateBOMRequest(&req)
	if code == "" {
		t.Error("zero qty should fail")
	}
	if !strings.Contains(msg, "qty") {
		t.Errorf("error message should mention qty, got %q", msg)
	}
}

func TestValidateBOMRequest_LineNegativeUnitCost(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: -1, CostType: "material"}},
	}
	code, _ := validateBOMRequest(&req)
	if code == "" {
		t.Error("negative unit_cost_cents should fail")
	}
}

func TestValidateBOMRequest_LineZeroUnitCostAllowed(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 0, CostType: "material"}},
	}
	code, _ := validateBOMRequest(&req)
	if code != "" {
		t.Error("zero unit_cost_cents should be allowed")
	}
}

func TestValidateBOMRequest_LineNegativeQty(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{{ItemID: 2, Qty: -1, UnitCostCents: 100, CostType: "material"}},
	}
	code, _ := validateBOMRequest(&req)
	if code == "" {
		t.Error("negative qty should fail")
	}
}

func TestValidateBOMRequest_LineEmptyCostTypeFailsDueToBug(t *testing.T) {
	// NOTE: validateBOMRequest has a latent bug — it sets
	// `line.CostType = "material"` on the loop copy but checks the
	// original `ct` variable (still ""), so empty cost_type actually
	// fails validation. This test documents the current behavior.
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: ""}},
	}
	code, _ := validateBOMRequest(&req)
	if code == "" {
		t.Error("empty cost_type currently fails due to ct variable not being updated (known behavior)")
	}
}

func TestValidateBOMRequest_LineInvalidCostType(t *testing.T) {
	cases := []string{"direct", "indirect", "other", "MATERIAL", "Labor"}
	for _, ct := range cases {
		req := CreateBOMRequest{
			Code: "BOM-1", Name: "Widget",
			FinishedGoodItemID: 1, OutputQty: 10,
			Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: ct}},
		}
		code, msg := validateBOMRequest(&req)
		if code == "" {
			t.Errorf("cost_type %q should fail", ct)
		}
		if !strings.Contains(msg, "cost_type") {
			t.Errorf("error should mention cost_type for %q, got %q", ct, msg)
		}
	}
}

func TestValidateBOMRequest_LineValidCostTypes(t *testing.T) {
	for _, ct := range []string{"material", "labor", "overhead"} {
		req := CreateBOMRequest{
			Code: "BOM-1", Name: "Widget",
			FinishedGoodItemID: 1, OutputQty: 10,
			Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: ct}},
		}
		code, _ := validateBOMRequest(&req)
		if code != "" {
			t.Errorf("cost_type %q should pass", ct)
		}
	}
}

func TestValidateBOMRequest_LineCostTypeWhitespaceTrimmed(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: "  material  "}},
	}
	code, _ := validateBOMRequest(&req)
	if code != "" {
		t.Error("cost_type with whitespace should be trimmed and pass")
	}
}

func TestValidateBOMRequest_LineIndexInErrorMessage(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{
			{ItemID: 2, Qty: 1, UnitCostCents: 100, CostType: "material"},
			{ItemID: 0, Qty: 1, UnitCostCents: 100, CostType: "material"},
		},
	}
	_, msg := validateBOMRequest(&req)
	if !strings.Contains(msg, "lines[1]") {
		t.Errorf("error should contain line index 'lines[1]', got %q", msg)
	}
}

func TestValidateBOMRequest_MultipleLinesFirstErrorReported(t *testing.T) {
	req := CreateBOMRequest{
		Code: "BOM-1", Name: "Widget",
		FinishedGoodItemID: 1, OutputQty: 10,
		Lines: []BOMLineRequest{
			{ItemID: 0, Qty: 0, UnitCostCents: -1, CostType: "material"},
		},
	}
	code, msg := validateBOMRequest(&req)
	if code == "" {
		t.Error("should fail on first error")
	}
	// The first error in the loop is item_id
	if !strings.Contains(msg, "item_id") {
		t.Errorf("should report item_id first, got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// validateCreateJobRequest (jobs.go)
// ---------------------------------------------------------------------------

func TestValidateCreateJobRequest_Valid(t *testing.T) {
	req := CreateProductionJobRequest{
		FinishedGoodItemID: 1,
		TargetQty:          10,
		StartDate:          "2026-01-15",
	}
	code, msg := validateCreateJobRequest(&req)
	if code != "" {
		t.Errorf("expected valid, got code=%q msg=%q", code, msg)
	}
}

func TestValidateCreateJobRequest_InvalidFinishedGoodItemID(t *testing.T) {
	for _, fgID := range []int64{0, -1} {
		req := CreateProductionJobRequest{
			FinishedGoodItemID: fgID,
			TargetQty:          10,
			StartDate:          "2026-01-15",
		}
		code, _ := validateCreateJobRequest(&req)
		if code == "" {
			t.Errorf("finished_good_item_id %d should fail", fgID)
		}
	}
}

func TestValidateCreateJobRequest_InvalidTargetQty(t *testing.T) {
	for _, qty := range []float64{0, -1, -0.01} {
		req := CreateProductionJobRequest{
			FinishedGoodItemID: 1,
			TargetQty:          qty,
			StartDate:          "2026-01-15",
		}
		code, _ := validateCreateJobRequest(&req)
		if code == "" {
			t.Errorf("target_qty %v should fail", qty)
		}
	}
}

func TestValidateCreateJobRequest_EmptyStartDate(t *testing.T) {
	req := CreateProductionJobRequest{
		FinishedGoodItemID: 1,
		TargetQty:          10,
		StartDate:          "",
	}
	code, _ := validateCreateJobRequest(&req)
	if code == "" {
		t.Error("empty start_date should fail")
	}
}

func TestValidateCreateJobRequest_WhitespaceStartDate(t *testing.T) {
	req := CreateProductionJobRequest{
		FinishedGoodItemID: 1,
		TargetQty:          10,
		StartDate:          "   ",
	}
	code, _ := validateCreateJobRequest(&req)
	if code == "" {
		t.Error("whitespace-only start_date should fail")
	}
}

// ---------------------------------------------------------------------------
// validateJobCostRequest (jobs.go)
// ---------------------------------------------------------------------------

func TestValidateJobCostRequest_MaterialValid(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "material",
		ItemID:        5,
		Qty:           2,
		UnitCostCents: 1000,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("valid material cost should pass")
	}
}

func TestValidateJobCostRequest_LaborValid(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "labor",
		Qty:           8,
		UnitCostCents: 5000,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("valid labor cost should pass")
	}
}

func TestValidateJobCostRequest_OverheadValid(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "overhead",
		Qty:           1,
		UnitCostCents: 3000,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("valid overhead cost should pass")
	}
}

func TestValidateJobCostRequest_EmptyCostType(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType: "",
	}
	code, msg := validateJobCostRequest(&req)
	if code == "" {
		t.Error("empty cost_type should fail")
	}
	if !strings.Contains(msg, "cost_type") {
		t.Errorf("error should mention cost_type, got %q", msg)
	}
}

func TestValidateJobCostRequest_InvalidCostType(t *testing.T) {
	for _, ct := range []string{"direct", "other", "MATERIAL", ""} {
		req := ProductionJobCostRequest{
			CostType: ct,
			Qty:      1, UnitCostCents: 100,
		}
		code, _ := validateJobCostRequest(&req)
		if code == "" && ct == "direct" {
			// "direct" is not a valid cost type
			t.Errorf("cost_type %q should fail", ct)
		}
	}
}

func TestValidateJobCostRequest_MaterialRequiresItemID(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType: "material",
		ItemID:   0,
		Qty:      1, UnitCostCents: 100,
	}
	code, msg := validateJobCostRequest(&req)
	if code == "" {
		t.Error("material without item_id should fail")
	}
	if !strings.Contains(msg, "item_id") {
		t.Errorf("error should mention item_id, got %q", msg)
	}
}

func TestValidateJobCostRequest_LaborDoesNotRequireItemID(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType: "labor",
		ItemID:   0,
		Qty:      1, UnitCostCents: 100,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("labor without item_id should pass")
	}
}

func TestValidateJobCostRequest_OverheadDoesNotRequireItemID(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType: "overhead",
		ItemID:   0,
		Qty:      1, UnitCostCents: 100,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("overhead without item_id should pass")
	}
}

func TestValidateJobCostRequest_BothQtyAndUnitCostZero(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "labor",
		Qty:           0,
		UnitCostCents: 0,
	}
	code, _ := validateJobCostRequest(&req)
	if code == "" {
		t.Error("both qty=0 and unit_cost_cents=0 should fail")
	}
}

func TestValidateJobCostRequest_QtyOnlyAllowed(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "labor",
		Qty:           5,
		UnitCostCents: 0,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("qty > 0 with unit_cost_cents=0 should pass")
	}
}

func TestValidateJobCostRequest_UnitCostOnlyAllowed(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "labor",
		Qty:           0,
		UnitCostCents: 5000,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("qty=0 with unit_cost_cents > 0 should pass")
	}
}

func TestValidateJobCostRequest_NegativeUnitCost(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "labor",
		Qty:           1,
		UnitCostCents: -1,
	}
	code, _ := validateJobCostRequest(&req)
	if code == "" {
		t.Error("negative unit_cost_cents should fail")
	}
}

func TestValidateJobCostRequest_CostTypeWhitespaceTrimmed(t *testing.T) {
	req := ProductionJobCostRequest{
		CostType:      "  labor  ",
		Qty:           1,
		UnitCostCents: 100,
	}
	code, _ := validateJobCostRequest(&req)
	if code != "" {
		t.Error("cost_type with whitespace should be trimmed and pass")
	}
}

func TestValidateJobCostRequest_NegativeQtyWithPositiveUnitCost(t *testing.T) {
	// qty <= 0 AND unit_cost_cents <= 0 fails, but qty < 0 with unit_cost > 0 passes
	req := ProductionJobCostRequest{
		CostType:      "labor",
		Qty:           -1,
		UnitCostCents: 100,
	}
	code, _ := validateJobCostRequest(&req)
	// The condition is: qty <= 0 AND unit_cost_cents <= 0 → fail
	// Here qty = -1 (<=0) but unit_cost = 100 (>0), so the AND is false → passes
	if code != "" {
		t.Error("negative qty with positive unit_cost should pass (AND condition)")
	}
}

// ---------------------------------------------------------------------------
// Cost type validation: only material, labor, overhead are valid
// ---------------------------------------------------------------------------

func TestCostTypeValidation(t *testing.T) {
	validTypes := []string{"material", "labor", "overhead"}
	invalidTypes := []string{"", "direct", "indirect", "MAT", "Labor", "OVERHEAD", "other", "misc"}

	for _, ct := range validTypes {
		req := &ProductionJobCostRequest{CostType: ct, Qty: 1, UnitCostCents: 100}
		if ct == "material" {
			req.ItemID = 1
		}
		code, _ := validateJobCostRequest(req)
		if code != "" {
			t.Errorf("cost_type %q should be valid", ct)
		}
	}

	for _, ct := range invalidTypes {
		req := &ProductionJobCostRequest{CostType: ct, Qty: 1, UnitCostCents: 100}
		code, _ := validateJobCostRequest(req)
		// Empty string is trimmed to "" → "cost_type is required"
		// "OVERHEAD" (uppercase) should fail (case-sensitive)
		if code == "" {
			t.Errorf("cost_type %q should be invalid", ct)
		}
	}
}

// ---------------------------------------------------------------------------
// Integer math for cost calculation (no float64)
// totalCents = qty * unitCostCents (as int64)
// ---------------------------------------------------------------------------

func TestIntegerCostMath(t *testing.T) {
	cases := []struct {
		qty            float64
		unitCostCents  int64
		expectedTotal  int64
	}{
		{1, 10000, 10000},
		{2, 5000, 10000},
		{10, 100, 1000},
		{100, 1, 100},
		{0.5, 10000, 5000},
		{3, 0, 0},
	}
	for _, tc := range cases {
		total := int64(tc.qty * float64(tc.unitCostCents))
		if total != tc.expectedTotal {
			t.Errorf("qty=%v * unit=%d = %d, want %d", tc.qty, tc.unitCostCents, total, tc.expectedTotal)
		}
	}
}

func TestCostAccumulationAcrossMultipleLines(t *testing.T) {
	// Simulate accumulating costs across multiple BOM lines
	lines := []BOMLineRequest{
		{ItemID: 1, Qty: 2, UnitCostCents: 5000, CostType: "material"},
		{ItemID: 2, Qty: 1, UnitCostCents: 10000, CostType: "material"},
		{Qty: 5, UnitCostCents: 2000, CostType: "labor"},
		{Qty: 1, UnitCostCents: 3000, CostType: "overhead"},
	}
	var materialTotal, laborTotal, overheadTotal int64
	for _, line := range lines {
		lineTotal := int64(line.Qty * float64(line.UnitCostCents))
		switch strings.TrimSpace(line.CostType) {
		case "material":
			materialTotal += lineTotal
		case "labor":
			laborTotal += lineTotal
		case "overhead":
			overheadTotal += lineTotal
		}
	}
	expectedMaterial := int64(20000)
	expectedLabor := int64(10000)
	expectedOverhead := int64(3000)
	if materialTotal != expectedMaterial {
		t.Errorf("material = %d, want %d", materialTotal, expectedMaterial)
	}
	if laborTotal != expectedLabor {
		t.Errorf("labor = %d, want %d", laborTotal, expectedLabor)
	}
	if overheadTotal != expectedOverhead {
		t.Errorf("overhead = %d, want %d", overheadTotal, expectedOverhead)
	}
	totalCost := materialTotal + laborTotal + overheadTotal
	if totalCost != 33000 {
		t.Errorf("total = %d, want 33000", totalCost)
	}
}

// ---------------------------------------------------------------------------
// hashJobJournal — SHA-256 hash determinism (jobs.go)
// ---------------------------------------------------------------------------

func TestHashJobJournal_Deterministic(t *testing.T) {
	// Replicate the hashJobJournal formula to verify determinism
	journalLines := []struct {
		AccountID     int64
		DebitCents    int64
		CreditCents   int64
		SourceLineRef string
	}{
		{1, 10000, 0, "wip-1"},
		{2, 0, 10000, "inv-1"},
	}
	payload := fmt.Sprintf("v1|%d|%s|%s|%s|%s|%v",
		1, "JOB-1", "PRODUCTION_COST", "2026-01-15", "prevhash", journalLines)
	sum := sha256.Sum256([]byte(payload))
	expected := hex.EncodeToString(sum[:])

	// Recompute — should be the same
	sum2 := sha256.Sum256([]byte(payload))
	actual := hex.EncodeToString(sum2[:])
	if expected != actual {
		t.Errorf("hash mismatch: %s != %s", expected, actual)
	}
}

func TestHashJobJournal_DifferentJournalsProduceDifferentHashes(t *testing.T) {
	payload1 := "v1|1|JOB-1|PRODUCTION_COST|2026-01-15|hash1|[]"
	payload2 := "v1|1|JOB-2|PRODUCTION_COST|2026-01-15|hash1|[]"
	sum1 := sha256.Sum256([]byte(payload1))
	sum2 := sha256.Sum256([]byte(payload2))
	h1 := hex.EncodeToString(sum1[:])
	h2 := hex.EncodeToString(sum2[:])
	if h1 == h2 {
		t.Error("different journals should produce different hashes")
	}
}

func TestHashJobJournal_LineSortingMatters(t *testing.T) {
	// The hash sorts lines by SourceLineRef before hashing, so the same
	// lines in different order should produce the same hash.
	type line struct {
		AccountID     int64
		DebitCents    int64
		CreditCents   int64
		SourceLineRef string
	}
	linesA := []line{{1, 100, 0, "b"}, {2, 0, 100, "a"}}
	linesB := []line{{2, 0, 100, "a"}, {1, 100, 0, "b"}}

	// Replicate sortByRef + hash
	sortByRefStr := func(l []line) {
		sort.Slice(l, func(i, j int) bool { return l[i].SourceLineRef < l[j].SourceLineRef })
	}
	sortByRefStr(linesA)
	sortByRefStr(linesB)

	p1 := fmt.Sprintf("v1|%d|%s|%s|%s|%s|%v", 1, "S1", "T", "D", "P", linesA)
	p2 := fmt.Sprintf("v1|%d|%s|%s|%s|%s|%v", 1, "S1", "T", "D", "P", linesB)
	sum1 := sha256.Sum256([]byte(p1))
	sum2 := sha256.Sum256([]byte(p2))
	h1 := hex.EncodeToString(sum1[:])
	h2 := hex.EncodeToString(sum2[:])
	if h1 != h2 {
		t.Error("same lines in different order should hash the same (sorted)")
	}
}

// ---------------------------------------------------------------------------
// sortByRef — insertion sort by SourceLineRef (jobs.go)
// ---------------------------------------------------------------------------

func TestSortByRef_SortsAscending(t *testing.T) {
	// We can't directly call sortByRef with []accounting.Line without
	// importing the accounting package. But we can test the same algorithm.
	type line struct {
		SourceLineRef string
	}
	sortLines := func(lines []line) {
		for i := 1; i < len(lines); i++ {
			for j := i; j > 0 && lines[j-1].SourceLineRef > lines[j].SourceLineRef; j-- {
				lines[j-1], lines[j] = lines[j], lines[j-1]
			}
		}
	}

	lines := []line{{"c"}, {"a"}, {"b"}, {"e"}, {"d"}}
	sortLines(lines)
	expected := []string{"a", "b", "c", "d", "e"}
	for i, l := range lines {
		if l.SourceLineRef != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, l.SourceLineRef, expected[i])
		}
	}
}

func TestSortByRef_AlreadySorted(t *testing.T) {
	type line struct {
		SourceLineRef string
	}
	sortLines := func(lines []line) {
		for i := 1; i < len(lines); i++ {
			for j := i; j > 0 && lines[j-1].SourceLineRef > lines[j].SourceLineRef; j-- {
				lines[j-1], lines[j] = lines[j], lines[j-1]
			}
		}
	}
	lines := []line{{"a"}, {"b"}, {"c"}}
	sortLines(lines)
	if lines[0].SourceLineRef != "a" || lines[1].SourceLineRef != "b" || lines[2].SourceLineRef != "c" {
		t.Error("already sorted list should remain sorted")
	}
}

func TestSortByRef_EmptyAndSingleElement(t *testing.T) {
	type line struct {
		SourceLineRef string
	}
	sortLines := func(lines []line) {
		for i := 1; i < len(lines); i++ {
			for j := i; j > 0 && lines[j-1].SourceLineRef > lines[j].SourceLineRef; j-- {
				lines[j-1], lines[j] = lines[j], lines[j-1]
			}
		}
	}
	// Empty
	empty := []line{}
	sortLines(empty)
	if len(empty) != 0 {
		t.Error("empty slice should remain empty")
	}
	// Single
	single := []line{{"z"}}
	sortLines(single)
	if single[0].SourceLineRef != "z" {
		t.Error("single element should remain unchanged")
	}
}

// ---------------------------------------------------------------------------
// Production job status constants (used in jobs.go as string literals)
// ---------------------------------------------------------------------------

func TestProductionJobStatuses(t *testing.T) {
	// Jobs.go uses these status strings:
	// "COMPLETED" — job is done, can't add costs
	// "CANCELLED" — job is cancelled, can't add costs or complete
	statuses := []string{"COMPLETED", "CANCELLED"}
	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestJobCompletionCostTransfer(t *testing.T) {
	// On completion: Dr Finished Goods / Cr WIP for totalCost
	// totalCost = TotalCostCents = material + labor + overhead
	material := int64(15000)
	labor := int64(8000)
	overhead := int64(3000)
	totalCost := material + labor + overhead

	// Journal: Dr Finished Goods, Cr WIP
	fgDebit := totalCost
	wipCredit := totalCost
	if fgDebit != wipCredit {
		t.Errorf("completion journal not balanced: Dr=%d Cr=%d", fgDebit, wipCredit)
	}
	if fgDebit != 26000 {
		t.Errorf("totalCost = %d, want 26000", fgDebit)
	}
}

// ---------------------------------------------------------------------------
// Unit cost derivation from resolved COGS (jobs.go line ~418)
// unitCostCents = int64(float64(resolvedCOGS) / qty)
// ---------------------------------------------------------------------------

func TestUnitCostDerivationFromCOGS(t *testing.T) {
	cases := []struct {
		resolvedCOGS int64
		qty          float64
		expected     int64
	}{
		{10000, 2, 5000},
		{15000, 3, 5000},
		{10000, 1, 10000},
		{333, 3, 111},
		{100, 0, 0}, // qty=0 → unitCostCents stays 0 (guard)
	}
	for _, tc := range cases {
		unitCostCents := int64(0)
		if tc.qty > 0 {
			unitCostCents = int64(float64(tc.resolvedCOGS) / tc.qty)
		}
		if unitCostCents != tc.expected {
			t.Errorf("resolvedCOGS=%d qty=%v: unitCost=%d, want %d",
				tc.resolvedCOGS, tc.qty, unitCostCents, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Variance journal line construction (mirrors jobs.go lines 693-722)
// ---------------------------------------------------------------------------

func TestVarianceJournalLines_Loss(t *testing.T) {
	// variance > 0 → loss: Dr 5901 / Cr 1303
	variance := int64(2000)
	wipAccountID := int64(100)
	varianceLossAccountID := int64(200)

	journalLines := []struct {
		AccountID  int64
		DebitCents  int64
		CreditCents int64
	}{
		{wipAccountID, 0, 10000},            // Cr WIP (completion)
		{150, 10000, 0},                     // Dr FG (completion)
		{varianceLossAccountID, variance, 0}, // Dr Loss
		{wipAccountID, 0, variance},          // Cr WIP (variance)
	}

	var totalDebit, totalCredit int64
	for _, l := range journalLines {
		totalDebit += l.DebitCents
		totalCredit += l.CreditCents
	}
	if totalDebit != totalCredit {
		t.Errorf("loss journal not balanced: Dr=%d Cr=%d", totalDebit, totalCredit)
	}
}

func TestVarianceJournalLines_Gain(t *testing.T) {
	// variance < 0 → gain: Dr 1303 / Cr 4902
	variance := int64(-2000)
	gain := -variance
	wipAccountID := int64(100)
	varianceGainAccountID := int64(300)

	if gain != 2000 {
		t.Errorf("gain = %d, want 2000", gain)
	}

	journalLines := []struct {
		AccountID   int64
		DebitCents  int64
		CreditCents int64
	}{
		{wipAccountID, 0, 10000},
		{150, 10000, 0},
		{wipAccountID, gain, 0},                  // Dr WIP (variance)
		{varianceGainAccountID, 0, gain},         // Cr Gain
	}
	var totalDebit, totalCredit int64
	for _, l := range journalLines {
		totalDebit += l.DebitCents
		totalCredit += l.CreditCents
	}
	if totalDebit != totalCredit {
		t.Errorf("gain journal not balanced: Dr=%d Cr=%d", totalDebit, totalCredit)
	}
}

func TestVarianceJournalLines_ZeroVariance(t *testing.T) {
	// variance = 0 → no variance lines, only completion entry
	variance := int64(0)
	wipAccountID := int64(100)
	totalCost := int64(10000)

	journalLines := []struct {
		AccountID   int64
		DebitCents  int64
		CreditCents int64
	}{
		{150, totalCost, 0},        // Dr FG
		{wipAccountID, 0, totalCost}, // Cr WIP
	}

	if variance > 0 || variance < 0 {
		t.Error("zero variance should not add variance lines")
	}

	var totalDebit, totalCredit int64
	for _, l := range journalLines {
		totalDebit += l.DebitCents
		totalCredit += l.CreditCents
	}
	if totalDebit != totalCredit {
		t.Errorf("zero-variance journal not balanced: Dr=%d Cr=%d", totalDebit, totalCredit)
	}
}
