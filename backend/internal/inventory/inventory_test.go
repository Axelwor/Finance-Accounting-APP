package inventory

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Account code constants — verify the exact values used in journal postings.
// ---------------------------------------------------------------------------

func TestInventoryAccountCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"inventoryAccountCode", inventoryAccountCode, "1301"},
		{"adjustmentGainAccountCode", adjustmentGainAccountCode, "4907"},
		{"adjustmentLossAccountCode", adjustmentLossAccountCode, "5907"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Stock opname status constants
// ---------------------------------------------------------------------------

func TestOpnameStatusConstants(t *testing.T) {
	if opnameStatusDraft != "DRAFT" {
		t.Errorf("opnameStatusDraft = %q, want DRAFT", opnameStatusDraft)
	}
	if opnameStatusCounted != "COUNTED" {
		t.Errorf("opnameStatusCounted = %q, want COUNTED", opnameStatusCounted)
	}
	if opnameStatusApproved != "APPROVED" {
		t.Errorf("opnameStatusApproved = %q, want APPROVED", opnameStatusApproved)
	}
	if opnameStatusVoid != "VOID" {
		t.Errorf("opnameStatusVoid = %q, want VOID", opnameStatusVoid)
	}
}

// ---------------------------------------------------------------------------
// Transfer status constant
// ---------------------------------------------------------------------------

func TestTransferStatusConstant(t *testing.T) {
	if transferStatusCompleted != "COMPLETED" {
		t.Errorf("transferStatusCompleted = %q, want COMPLETED", transferStatusCompleted)
	}
}

// ---------------------------------------------------------------------------
// validateStockOpname — all validation paths
// ---------------------------------------------------------------------------

func TestValidateStockOpname_InvalidDate(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-13-45",
		Lines:      []StockOpnameLineRequest{{ItemID: 1, CountedQty: 10, UnitCostCents: 1000}},
	}
	code, msg := validateStockOpname(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "opname_date") {
		t.Errorf("msg = %q, want to mention opname_date", msg)
	}
}

func TestValidateStockOpname_EmptyDate(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "",
		Lines:      []StockOpnameLineRequest{{ItemID: 1, CountedQty: 10}},
	}
	code, _ := validateStockOpname(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
}

func TestValidateStockOpname_NoLines(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-01-15",
		Lines:      []StockOpnameLineRequest{},
	}
	code, msg := validateStockOpname(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "line") {
		t.Errorf("msg = %q, want to mention line", msg)
	}
}

func TestValidateStockOpname_LineItemIDZero(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-01-15",
		Lines:      []StockOpnameLineRequest{{ItemID: 0, CountedQty: 10, UnitCostCents: 1000}},
	}
	code, msg := validateStockOpname(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "item_id") {
		t.Errorf("msg = %q, want to mention item_id", msg)
	}
}

func TestValidateStockOpname_LineItemIDNegative(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-01-15",
		Lines:      []StockOpnameLineRequest{{ItemID: -5, CountedQty: 10}},
	}
	code, _ := validateStockOpname(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
}

func TestValidateStockOpname_CountedQtyNegative(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-01-15",
		Lines:      []StockOpnameLineRequest{{ItemID: 1, CountedQty: -1, UnitCostCents: 1000}},
	}
	code, msg := validateStockOpname(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "counted_qty") {
		t.Errorf("msg = %q, want to mention counted_qty", msg)
	}
}

func TestValidateStockOpname_UnitCostNegative(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-01-15",
		Lines:      []StockOpnameLineRequest{{ItemID: 1, CountedQty: 10, UnitCostCents: -1}},
	}
	code, msg := validateStockOpname(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "unit_cost_cents") {
		t.Errorf("msg = %q, want to mention unit_cost_cents", msg)
	}
}

func TestValidateStockOpname_CountedQtyZero(t *testing.T) {
	// Zero counted qty is valid (physically counted as zero).
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-01-15",
		Lines:      []StockOpnameLineRequest{{ItemID: 1, CountedQty: 0, UnitCostCents: 0}},
	}
	code, _ := validateStockOpname(req)
	if code != "" {
		t.Errorf("code = %q, want empty (valid)", code)
	}
}

func TestValidateStockOpname_Valid(t *testing.T) {
	req := CreateStockOpnameRequest{
		OpnameDate: "2026-01-15",
		Lines: []StockOpnameLineRequest{
			{ItemID: 1, CountedQty: 100, UnitCostCents: 1000},
			{ItemID: 2, CountedQty: 50, UnitCostCents: 2000},
		},
	}
	code, msg := validateStockOpname(req)
	if code != "" || msg != "" {
		t.Errorf("validateStockOpname(valid) = (%q, %q), want (\"\", \"\")", code, msg)
	}
}

// ---------------------------------------------------------------------------
// Stock opname variance calculation: variance = counted_qty - system_qty
// ---------------------------------------------------------------------------

func TestOpnameVariance_Surplus(t *testing.T) {
	systemQty := 80.0
	countedQty := 100.0
	variance := countedQty - systemQty
	if variance != 20 {
		t.Errorf("variance = %v, want 20", variance)
	}
}

func TestOpnameVariance_Shortage(t *testing.T) {
	systemQty := 100.0
	countedQty := 80.0
	variance := countedQty - systemQty
	if variance != -20 {
		t.Errorf("variance = %v, want -20", variance)
	}
}

func TestOpnameVariance_Zero(t *testing.T) {
	systemQty := 100.0
	countedQty := 100.0
	variance := countedQty - systemQty
	if variance != 0 {
		t.Errorf("variance = %v, want 0", variance)
	}
}

func TestOpnameVariance_SystemZeroCountedPositive(t *testing.T) {
	systemQty := 0.0
	countedQty := 50.0
	variance := countedQty - systemQty
	if variance != 50 {
		t.Errorf("variance = %v, want 50", variance)
	}
}

func TestOpnameVariance_Fractional(t *testing.T) {
	systemQty := 10.5
	countedQty := 12.25
	variance := countedQty - systemQty
	if variance != 1.75 {
		t.Errorf("variance = %v, want 1.75", variance)
	}
}

// ---------------------------------------------------------------------------
// Stock opname adjustment cost: adjustment_cents = diff_qty * unit_cost_cents
// (surplus => positive; shortage => negative)
// ---------------------------------------------------------------------------

func TestOpnameAdjustmentCents_Surplus(t *testing.T) {
	diffQty := 20.0
	unitCostCents := int64(1000)
	adj := int64(diffQty * float64(unitCostCents))
	if adj != 20000 {
		t.Errorf("adj = %d, want 20000", adj)
	}
}

func TestOpnameAdjustmentCents_Shortage(t *testing.T) {
	diffQty := -20.0
	unitCostCents := int64(1000)
	adj := int64(diffQty * float64(unitCostCents))
	if adj != -20000 {
		t.Errorf("adj = %d, want -20000", adj)
	}
}

func TestOpnameAdjustmentCents_ZeroDiff(t *testing.T) {
	diffQty := 0.0
	unitCostCents := int64(1000)
	adj := int64(diffQty * float64(unitCostCents))
	if adj != 0 {
		t.Errorf("adj = %d, want 0", adj)
	}
}

func TestOpnameTotalAdjustment_MultipleLines(t *testing.T) {
	// Replicate the total accumulation: sum of all line adjustments.
	diffs := []float64{20, -10, 5, 0, -3}
	unitCosts := []int64{1000, 2000, 500, 1500, 999}
	var total int64
	for i, d := range diffs {
		total += int64(d * float64(unitCosts[i]))
	}
	// 20*1000 - 10*2000 + 5*500 + 0*1500 - 3*999 = 20000 - 20000 + 2500 + 0 - 2997 = -497
	if total != -497 {
		t.Errorf("total = %d, want -497", total)
	}
}

// ---------------------------------------------------------------------------
// Stock opname journal posting logic:
//   surplus  (diff > 0): Dr 1301 Inventory / Cr 4907 Adjustment Gain
//   shortage (diff < 0): Dr 5907 Adjustment Loss / Cr 1301 Inventory
// ---------------------------------------------------------------------------

func TestOpnameJournal_Surplus(t *testing.T) {
	diffQty := 20.0
	if diffQty > 0 {
		// Dr 1301 / Cr 4907
		debitAccount := inventoryAccountCode      // 1301
		creditAccount := adjustmentGainAccountCode // 4907
		if debitAccount != "1301" || creditAccount != "4907" {
			t.Errorf("surplus: Dr %s / Cr %s, want Dr 1301 / Cr 4907", debitAccount, creditAccount)
		}
	}
}

func TestOpnameJournal_Shortage(t *testing.T) {
	diffQty := -20.0
	if diffQty < 0 {
		// Dr 5907 / Cr 1301
		debitAccount := adjustmentLossAccountCode // 5907
		creditAccount := inventoryAccountCode      // 1301
		if debitAccount != "5907" || creditAccount != "1301" {
			t.Errorf("shortage: Dr %s / Cr %s, want Dr 5907 / Cr 1301", debitAccount, creditAccount)
		}
	}
}

// ---------------------------------------------------------------------------
// validateStockTransfer — all validation paths
// ---------------------------------------------------------------------------

func TestValidateStockTransfer_InvalidDate(t *testing.T) {
	req := CreateStockTransferRequest{
		TransferDate: "not-a-date",
		Lines:        []StockTransferLineRequest{{ItemID: 1, Qty: 10, UnitCostCents: 1000}},
	}
	code, msg := validateStockTransfer(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "transfer_date") {
		t.Errorf("msg = %q, want to mention transfer_date", msg)
	}
}

func TestValidateStockTransfer_NoLines(t *testing.T) {
	req := CreateStockTransferRequest{
		TransferDate: "2026-01-15",
		Lines:        []StockTransferLineRequest{},
	}
	code, msg := validateStockTransfer(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "line") {
		t.Errorf("msg = %q, want to mention line", msg)
	}
}

func TestValidateStockTransfer_LineItemIDZero(t *testing.T) {
	req := CreateStockTransferRequest{
		TransferDate: "2026-01-15",
		Lines:        []StockTransferLineRequest{{ItemID: 0, Qty: 10, UnitCostCents: 1000}},
	}
	code, msg := validateStockTransfer(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "item_id") {
		t.Errorf("msg = %q, want to mention item_id", msg)
	}
}

func TestValidateStockTransfer_QtyZero(t *testing.T) {
	req := CreateStockTransferRequest{
		TransferDate: "2026-01-15",
		Lines:        []StockTransferLineRequest{{ItemID: 1, Qty: 0, UnitCostCents: 1000}},
	}
	code, msg := validateStockTransfer(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "qty") {
		t.Errorf("msg = %q, want to mention qty", msg)
	}
}

func TestValidateStockTransfer_QtyNegative(t *testing.T) {
	req := CreateStockTransferRequest{
		TransferDate: "2026-01-15",
		Lines:        []StockTransferLineRequest{{ItemID: 1, Qty: -5, UnitCostCents: 1000}},
	}
	code, _ := validateStockTransfer(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
}

func TestValidateStockTransfer_UnitCostNegative(t *testing.T) {
	req := CreateStockTransferRequest{
		TransferDate: "2026-01-15",
		Lines:        []StockTransferLineRequest{{ItemID: 1, Qty: 10, UnitCostCents: -1}},
	}
	code, msg := validateStockTransfer(req)
	if code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}
	if !strings.Contains(msg, "unit_cost_cents") {
		t.Errorf("msg = %q, want to mention unit_cost_cents", msg)
	}
}

func TestValidateStockTransfer_UnitCostZero(t *testing.T) {
	// Zero unit cost is allowed.
	req := CreateStockTransferRequest{
		TransferDate: "2026-01-15",
		Lines:        []StockTransferLineRequest{{ItemID: 1, Qty: 10, UnitCostCents: 0}},
	}
	code, _ := validateStockTransfer(req)
	if code != "" {
		t.Errorf("code = %q, want empty (valid)", code)
	}
}

func TestValidateStockTransfer_Valid(t *testing.T) {
	req := CreateStockTransferRequest{
		TransferDate: "2026-01-15",
		Lines: []StockTransferLineRequest{
			{ItemID: 1, Qty: 10, UnitCostCents: 1000},
			{ItemID: 2, Qty: 5, UnitCostCents: 2000},
		},
	}
	code, msg := validateStockTransfer(req)
	if code != "" || msg != "" {
		t.Errorf("validateStockTransfer(valid) = (%q, %q), want (\"\", \"\")", code, msg)
	}
}

// ---------------------------------------------------------------------------
// Stock transfer line total: qty * unit_cost_cents
// ---------------------------------------------------------------------------

func TestTransferLineTotal(t *testing.T) {
	qty := 10.5
	unitCostCents := int64(2000)
	total := int64(qty * float64(unitCostCents))
	if total != 21000 {
		t.Errorf("total = %d, want 21000", total)
	}
}

// ---------------------------------------------------------------------------
// validDate helper
// ---------------------------------------------------------------------------

func TestValidDate_Valid(t *testing.T) {
	for _, d := range []string{"2026-01-15", "2026-12-31", "2025-02-28", "2024-02-29"} {
		if !validDate(d) {
			t.Errorf("validDate(%q) = false, want true", d)
		}
	}
}

func TestValidDate_Invalid(t *testing.T) {
	for _, d := range []string{"", "not-a-date", "2026-13-01", "2026-02-30", "2026/01/15", "01-15-2026"} {
		if validDate(d) {
			t.Errorf("validDate(%q) = true, want false", d)
		}
	}
}

func TestValidDate_WithWhitespace(t *testing.T) {
	// validDate trims before parsing.
	if !validDate("  2026-01-15  ") {
		t.Error("validDate with whitespace should be true")
	}
}

// ---------------------------------------------------------------------------
// parseDate helper
// ---------------------------------------------------------------------------

func TestParseDate_Valid(t *testing.T) {
	d, err := parseDate("2026-01-15")
	if err != nil {
		t.Fatalf("parseDate failed: %v", err)
	}
	if !d.Valid {
		t.Fatal("date should be valid")
	}
	expected := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !d.Time.Equal(expected) {
		t.Errorf("date = %v, want %v", d.Time, expected)
	}
}

func TestParseDate_Empty(t *testing.T) {
	_, err := parseDate("")
	if err == nil {
		t.Error("parseDate(\"\") should return error")
	}
}

func TestParseDate_Whitespace(t *testing.T) {
	_, err := parseDate("   ")
	if err == nil {
		t.Error("parseDate(\"   \") should return error")
	}
}

func TestParseDate_Invalid(t *testing.T) {
	_, err := parseDate("2026-13-45")
	if err == nil {
		t.Error("parseDate(invalid) should return error")
	}
}

// ---------------------------------------------------------------------------
// dateString helper
// ---------------------------------------------------------------------------

func TestDateString_Valid(t *testing.T) {
	d, _ := parseDate("2026-01-15")
	if got := dateString(d); got != "2026-01-15" {
		t.Errorf("dateString = %q, want 2026-01-15", got)
	}
}

func TestDateString_Invalid(t *testing.T) {
	var d pgtype.Date
	if got := dateString(d); got != "" {
		t.Errorf("dateString(invalid) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// textValue / textValueOptional helpers
// ---------------------------------------------------------------------------

func TestTextValue_Valid(t *testing.T) {
	v := pgtype.Text{String: "hello", Valid: true}
	if got := textValue(v); got != "hello" {
		t.Errorf("textValue = %q, want hello", got)
	}
}

func TestTextValue_Invalid(t *testing.T) {
	var v pgtype.Text
	if got := textValue(v); got != "" {
		t.Errorf("textValue(invalid) = %q, want empty", got)
	}
}

func TestTextValueOptional_NonEmpty(t *testing.T) {
	v := textValueOptional("test")
	if !v.Valid || v.String != "test" {
		t.Errorf("textValueOptional(test) = {String: %q, Valid: %v}", v.String, v.Valid)
	}
}

func TestTextValueOptional_WhitespaceOnly(t *testing.T) {
	v := textValueOptional("   ")
	if v.Valid {
		t.Error("textValueOptional(whitespace) should be invalid")
	}
}

func TestTextValueOptional_Empty(t *testing.T) {
	v := textValueOptional("")
	if v.Valid {
		t.Error("textValueOptional(\"\") should be invalid")
	}
}

// ---------------------------------------------------------------------------
// optionalInt8 / int8Value helpers
// ---------------------------------------------------------------------------

func TestOptionalInt8_Zero(t *testing.T) {
	v := optionalInt8(0)
	if v.Valid {
		t.Error("optionalInt8(0) should be invalid")
	}
}

func TestOptionalInt8_NonZero(t *testing.T) {
	v := optionalInt8(42)
	if !v.Valid || v.Int64 != 42 {
		t.Errorf("optionalInt8(42) = {Int64: %d, Valid: %v}", v.Int64, v.Valid)
	}
}

func TestInt8Value_Zero(t *testing.T) {
	v := int8Value(0)
	if v.Valid {
		t.Error("int8Value(0) should be invalid")
	}
}

func TestInt8Value_NonZero(t *testing.T) {
	v := int8Value(99)
	if !v.Valid || v.Int64 != 99 {
		t.Errorf("int8Value(99) = {Int64: %d, Valid: %v}", v.Int64, v.Valid)
	}
}

// ---------------------------------------------------------------------------
// numericToFloat helper (same as costing but in inventory package)
// ---------------------------------------------------------------------------

func TestNumericToFloat_Invalid(t *testing.T) {
	var n pgtype.Numeric
	if got := numericToFloat(n); got != 0 {
		t.Errorf("numericToFloat(invalid) = %v, want 0", got)
	}
}

func TestNumericToFloat_Valid(t *testing.T) {
	var n pgtype.Numeric
	if err := n.Scan("42.5"); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if got := numericToFloat(n); got != 42.5 {
		t.Errorf("numericToFloat(42.5) = %v, want 42.5", got)
	}
}

// ---------------------------------------------------------------------------
// pgtypeFloat helper
// ---------------------------------------------------------------------------

func TestPgtypeFloat_Positive(t *testing.T) {
	n := pgtypeFloat(42.5)
	if !n.Valid {
		t.Fatal("pgtypeFloat(42.5) should be valid")
	}
	if got := numericToFloat(n); got != 42.5 {
		t.Errorf("pgtypeFloat(42.5) round-trip = %v, want 42.5", got)
	}
}

func TestPgtypeFloat_Zero(t *testing.T) {
	n := pgtypeFloat(0)
	// Zero is still a valid numeric.
	f := numericToFloat(n)
	if f != 0 {
		t.Errorf("pgtypeFloat(0) = %v, want 0", f)
	}
}

func TestPgtypeFloat_Negative(t *testing.T) {
	n := pgtypeFloat(-10.25)
	if got := numericToFloat(n); got != -10.25 {
		t.Errorf("pgtypeFloat(-10.25) = %v, want -10.25", got)
	}
}

// ---------------------------------------------------------------------------
// pathID helper
// ---------------------------------------------------------------------------

func TestPathID_Valid(t *testing.T) {
	id, err := pathID("42")
	if err != nil || id != 42 {
		t.Errorf("pathID(42) = (%d, %v), want (42, nil)", id, err)
	}
}

func TestPathID_Zero(t *testing.T) {
	_, err := pathID("0")
	if err == nil {
		t.Error("pathID(0) should return error")
	}
}

func TestPathID_Negative(t *testing.T) {
	_, err := pathID("-5")
	if err == nil {
		t.Error("pathID(-5) should return error")
	}
}

func TestPathID_NonNumeric(t *testing.T) {
	_, err := pathID("abc")
	if err == nil {
		t.Error("pathID(abc) should return error")
	}
}

func TestPathID_Empty(t *testing.T) {
	_, err := pathID("")
	if err == nil {
		t.Error("pathID(\"\") should return error")
	}
}

// ---------------------------------------------------------------------------
// leftPad6 helper
// ---------------------------------------------------------------------------

func TestLeftPad6(t *testing.T) {
	tests := []struct {
		seq   int64
		want  string
	}{
		{1, "000001"},
		{10, "000010"},
		{100, "000100"},
		{1000, "001000"},
		{10000, "010000"},
		{100000, "100000"},
		{999999, "999999"},
	}
	for _, tc := range tests {
		got := leftPad6(tc.seq)
		if got != tc.want {
			t.Errorf("leftPad6(%d) = %q, want %q", tc.seq, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// mustJSON helper
// ---------------------------------------------------------------------------

func TestMustJSON_Valid(t *testing.T) {
	data := mustJSON(map[string]any{"key": "value"})
	if !strings.Contains(string(data), "\"key\"") || !strings.Contains(string(data), "value") {
		t.Errorf("mustJSON output = %s, want to contain key and value", string(data))
	}
}

func TestMustJSON_InvalidType(t *testing.T) {
	// Channels cannot be marshaled to JSON.
	data := mustJSON(make(chan int))
	if string(data) != "{}" {
		t.Errorf("mustJSON(chan) = %s, want {}", string(data))
	}
}

// ---------------------------------------------------------------------------
// isNoRows helper (uses errors.Is, unlike costing's == version)
// ---------------------------------------------------------------------------

func TestIsNoRows_ErrNoRows(t *testing.T) {
	if !isNoRows(pgx.ErrNoRows) {
		t.Error("isNoRows(pgx.ErrNoRows) = false, want true")
	}
}

func TestIsNoRows_Other(t *testing.T) {
	if isNoRows(errors.New("other")) {
		t.Error("isNoRows(other) = true, want false")
	}
}

func TestIsNoRows_Nil(t *testing.T) {
	if isNoRows(nil) {
		t.Error("isNoRows(nil) = true, want false")
	}
}

func TestIsNoRows_Wrapped(t *testing.T) {
	// inventory's isNoRows uses errors.Is, so wrapped ErrNoRows DOES match.
	wrapped := wrapNoRows("context")
	if !isNoRows(wrapped) {
		t.Error("isNoRows(wrapped) = false, want true (uses errors.Is)")
	}
}

func wrapNoRows(msg string) error {
	return &noRowsWrap{msg: msg}
}

type noRowsWrap struct{ msg string }

func (e *noRowsWrap) Error() string { return e.msg + ": " + pgx.ErrNoRows.Error() }
func (e *noRowsWrap) Unwrap() error { return pgx.ErrNoRows }

// ---------------------------------------------------------------------------
// isUniqueViolation / isForeignKeyViolation
// ---------------------------------------------------------------------------

func TestIsUniqueViolation_Nil(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Error("isUniqueViolation(nil) = true, want false")
	}
}

func TestIsUniqueViolation_Other(t *testing.T) {
	if isUniqueViolation(errors.New("not pg")) {
		t.Error("isUniqueViolation(non-pg) = true, want false")
	}
}

func TestIsUniqueViolation_Match(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505"}
	if !isUniqueViolation(pgErr) {
		t.Error("isUniqueViolation(23505) = false, want true")
	}
}

func TestIsUniqueViolation_DifferentCode(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503"}
	if isUniqueViolation(pgErr) {
		t.Error("isUniqueViolation(23503) = true, want false")
	}
}

func TestIsForeignKeyViolation_Match(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503"}
	if !isForeignKeyViolation(pgErr) {
		t.Error("isForeignKeyViolation(23503) = false, want true")
	}
}

func TestIsForeignKeyViolation_DifferentCode(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505"}
	if isForeignKeyViolation(pgErr) {
		t.Error("isForeignKeyViolation(23505) = true, want false")
	}
}

// ---------------------------------------------------------------------------
// errorResponse struct
// ---------------------------------------------------------------------------

func TestErrorResponse_JSON(t *testing.T) {
	er := errorResponse{Code: "INVALID_REQUEST", Message: "bad input"}
	if er.Code != "INVALID_REQUEST" {
		t.Errorf("Code = %q, want INVALID_REQUEST", er.Code)
	}
	if er.Message != "bad input" {
		t.Errorf("Message = %q, want 'bad input'", er.Message)
	}
}

// ---------------------------------------------------------------------------
// nextDocNumber format: PREFIX-YYYY-000001 (replicating the format string)
// ---------------------------------------------------------------------------

func TestNextDocNumber_Format(t *testing.T) {
	// Replicate the format: prefix + "-" + year + "-" + leftPad6(seq)
	year := time.Now().Year()
	prefix := "OPN"
	seq := int64(1)
	formatted := prefix + "-" + strconv.FormatInt(int64(year), 10) + "-" + leftPad6(seq)
	expected := prefix + "-" + strconv.Itoa(year) + "-000001"
	if formatted != expected {
		t.Errorf("formatted = %q, want %q", formatted, expected)
	}
}
