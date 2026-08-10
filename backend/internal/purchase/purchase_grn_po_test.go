package purchase

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
)

// ---------------------------------------------------------------------------
// GRN line total calculation
// ---------------------------------------------------------------------------

func TestGRNLineTotal(t *testing.T) {
	tests := []struct {
		name          string
		qty           float64
		unitCostCents int64
		want          int64
	}{
		{name: "whole qty", qty: 2, unitCostCents: 1000, want: 2000},
		{name: "fractional qty", qty: 2.5, unitCostCents: 1000, want: 2500},
		{name: "single unit", qty: 1, unitCostCents: 1500, want: 1500},
		{name: "zero cost", qty: 3, unitCostCents: 0, want: 0},
		{name: "zero qty", qty: 0, unitCostCents: 5000, want: 0},
		{name: "large values", qty: 100, unitCostCents: 500000, want: 50000000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := grnLineTotal(tc.qty, tc.unitCostCents)
			if got != tc.want {
				t.Errorf("grnLineTotal(%v, %d) = %d, want %d", tc.qty, tc.unitCostCents, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PO status transitions after GRN
// ---------------------------------------------------------------------------

func TestPOStatusAfterGRN(t *testing.T) {
	tests := []struct {
		name    string
		current string
		want    string
	}{
		{name: "CONFIRMED becomes PARTIALLY_RECEIVED", current: poStatusConfirmed, want: poStatusPartiallyReceived},
		{name: "PARTIALLY_RECEIVED stays PARTIALLY_RECEIVED", current: poStatusPartiallyReceived, want: poStatusPartiallyReceived},
		{name: "RECEIVED stays RECEIVED", current: poStatusReceived, want: poStatusReceived},
		{name: "unknown status falls back to PARTIALLY_RECEIVED", current: "SOMETHING_ELSE", want: poStatusPartiallyReceived},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := poStatusAfterGRN(tc.current)
			if got != tc.want {
				t.Errorf("poStatusAfterGRN(%q) = %q, want %q", tc.current, got, tc.want)
			}
		})
	}
}

func TestPOStatusConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "confirmed", value: poStatusConfirmed, want: "CONFIRMED"},
		{name: "partially received", value: poStatusPartiallyReceived, want: "PARTIALLY_RECEIVED"},
		{name: "received", value: poStatusReceived, want: "RECEIVED"},
		{name: "cancelled", value: poStatusCancelled, want: "CANCELLED"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value != tc.want {
				t.Errorf("PO status constant = %q, want %q", tc.value, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GRN request validation
// ---------------------------------------------------------------------------

func validGRN() CreateGRNRequest {
	return CreateGRNRequest{
		PurchaseOrderID: 1,
		GRNDate:         "2026-08-09",
		Lines: []GRNLineRequest{
			{ItemID: 1, POLineID: 1, Qty: 2, UnitCostCents: 1000},
		},
	}
}

func TestValidateGRNRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateGRNRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateGRNRequest) {}},
		{name: "missing purchase order", mutate: func(r *CreateGRNRequest) { r.PurchaseOrderID = 0 }, wantError: true},
		{name: "negative purchase order", mutate: func(r *CreateGRNRequest) { r.PurchaseOrderID = -1 }, wantError: true},
		{name: "empty grn_date", mutate: func(r *CreateGRNRequest) { r.GRNDate = "" }, wantError: true},
		{name: "bad grn_date", mutate: func(r *CreateGRNRequest) { r.GRNDate = "not-a-date" }, wantError: true},
		{name: "wrong date format", mutate: func(r *CreateGRNRequest) { r.GRNDate = "08-09-2026" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateGRNRequest) { r.Lines = nil }, wantError: true},
		{name: "missing item", mutate: func(r *CreateGRNRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateGRNRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative qty", mutate: func(r *CreateGRNRequest) { r.Lines[0].Qty = -1 }, wantError: true},
		{name: "negative cost", mutate: func(r *CreateGRNRequest) { r.Lines[0].UnitCostCents = -1 }, wantError: true},
		{name: "zero cost allowed", mutate: func(r *CreateGRNRequest) { r.Lines[0].UnitCostCents = 0 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validGRN()
			tc.mutate(&req)
			code, _ := validateGRNRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateGRNRequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PO request validation
// ---------------------------------------------------------------------------

func validPO() CreatePurchaseOrderRequest {
	return CreatePurchaseOrderRequest{
		SupplierID: 1,
		OrderDate:  "2026-08-09",
		Lines: []PurchaseOrderLineRequest{
			{ItemID: 1, Qty: 2, UnitPriceCents: 500000, DiscountCents: 0, TaxRate: 11},
		},
	}
}

func TestValidatePORequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreatePurchaseOrderRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreatePurchaseOrderRequest) {}},
		{name: "missing supplier", mutate: func(r *CreatePurchaseOrderRequest) { r.SupplierID = 0 }, wantError: true},
		{name: "negative supplier", mutate: func(r *CreatePurchaseOrderRequest) { r.SupplierID = -1 }, wantError: true},
		{name: "empty order_date", mutate: func(r *CreatePurchaseOrderRequest) { r.OrderDate = "" }, wantError: true},
		{name: "bad order_date", mutate: func(r *CreatePurchaseOrderRequest) { r.OrderDate = "2026-13-45" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines = nil }, wantError: true},
		{name: "missing item", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
		{name: "zero qty", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative price", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
		{name: "negative discount", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].DiscountCents = -1 }, wantError: true},
		{name: "tax above 100", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].TaxRate = 101 }, wantError: true},
		{name: "negative tax", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].TaxRate = -1 }, wantError: true},
		{name: "tax boundary 100 allowed", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].TaxRate = 100 }},
		{name: "tax boundary 0 allowed", mutate: func(r *CreatePurchaseOrderRequest) { r.Lines[0].TaxRate = 0 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validPO()
			tc.mutate(&req)
			code, _ := validatePORequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validatePORequest code=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PO line preparation (totals)
// ---------------------------------------------------------------------------

func TestPreparePOLines(t *testing.T) {
	prepared, total, err := preparePOLines([]PurchaseOrderLineRequest{
		{ItemID: 1, Qty: 2, UnitPriceCents: 500000, DiscountCents: 0},
		{ItemID: 2, Qty: 1, UnitPriceCents: 1000000, DiscountCents: 50000},
	})
	if err != nil {
		t.Fatalf("preparePOLines error: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("expected 2 prepared lines, got %d", len(prepared))
	}
	// Line 1: 2 × 500000 = 1000000; line 2: 1 × 1000000 − 50000 = 950000.
	if prepared[0].LineTotalCents != 1000000 {
		t.Errorf("line 1 total = %d, want 1000000", prepared[0].LineTotalCents)
	}
	if prepared[1].LineTotalCents != 950000 {
		t.Errorf("line 2 total = %d, want 950000", prepared[1].LineTotalCents)
	}
	if total != 1950000 {
		t.Errorf("total = %d, want 1950000", total)
	}
}

func TestPreparePOLinesDiscountExceedsGross(t *testing.T) {
	_, _, err := preparePOLines([]PurchaseOrderLineRequest{
		{ItemID: 1, Qty: 1, UnitPriceCents: 100, DiscountCents: 150},
	})
	if err == nil {
		t.Fatal("expected error when discount exceeds gross, got nil")
	}
}

func TestPreparePOLinesEmpty(t *testing.T) {
	prepared, total, err := preparePOLines(nil)
	if err != nil {
		t.Fatalf("preparePOLines(nil) error: %v", err)
	}
	if len(prepared) != 0 || total != 0 {
		t.Errorf("preparePOLines(nil) = %d lines, total %d; want 0, 0", len(prepared), total)
	}
}

func TestPreparePOLinesFractionalQty(t *testing.T) {
	prepared, total, err := preparePOLines([]PurchaseOrderLineRequest{
		{ItemID: 1, Qty: 2.5, UnitPriceCents: 1000, DiscountCents: 0},
	})
	if err != nil {
		t.Fatalf("preparePOLines error: %v", err)
	}
	if prepared[0].LineTotalCents != 2500 || total != 2500 {
		t.Errorf("fractional line total = %d, total = %d; want 2500, 2500", prepared[0].LineTotalCents, total)
	}
}

// ---------------------------------------------------------------------------
// Supplier validation
// ---------------------------------------------------------------------------

func TestValidateSupplierRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateSupplierRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateSupplierRequest) {}},
		{name: "empty code", mutate: func(r *CreateSupplierRequest) { r.Code = "" }, wantError: true},
		{name: "whitespace code", mutate: func(r *CreateSupplierRequest) { r.Code = "   " }, wantError: true},
		{name: "empty name", mutate: func(r *CreateSupplierRequest) { r.Name = "" }, wantError: true},
		{name: "whitespace name", mutate: func(r *CreateSupplierRequest) { r.Name = "  \t " }, wantError: true},
		{name: "optional fields may be empty", mutate: func(r *CreateSupplierRequest) {
			r.NPWP, r.Email, r.Phone, r.Address = "", "", "", ""
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateSupplierRequest{Code: "SUP-001", Name: "PT Sumber Jaya"}
			tc.mutate(&req)
			code, msg := validateSupplierRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateSupplierRequest code=%q msg=%q, wantError=%v", code, msg, tc.wantError)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

func TestLeftPad6(t *testing.T) {
	tests := []struct {
		seq  int64
		want string
	}{
		{1, "000001"},
		{42, "000042"},
		{123456, "123456"},
		{1234567, "1234567"},
		{0, "000000"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := leftPad6(tc.seq); got != tc.want {
				t.Errorf("leftPad6(%d) = %q, want %q", tc.seq, got, tc.want)
			}
		})
	}
}

func TestPathID(t *testing.T) {
	tests := []struct {
		raw     string
		want    int64
		wantErr bool
	}{
		{raw: "1", want: 1},
		{raw: "42", want: 42},
		{raw: "0", wantErr: true},
		{raw: "-5", wantErr: true},
		{raw: "abc", wantErr: true},
		{raw: "", wantErr: true},
		{raw: "1.5", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := pathID(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Errorf("pathID(%q) expected error, got %d", tc.raw, got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("pathID(%q) = %d, %v; want %d", tc.raw, got, err, tc.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "valid", raw: "2026-08-09"},
		{name: "empty", raw: "", wantErr: true},
		{name: "whitespace only", raw: "   ", wantErr: true},
		{name: "wrong format", raw: "09-08-2026", wantErr: true},
		{name: "invalid day", raw: "2026-02-30", wantErr: true},
		{name: "garbage", raw: "not-a-date", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDate(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseDate(%q) expected error, got %+v", tc.raw, got)
				}
				return
			}
			if err != nil || !got.Valid {
				t.Errorf("parseDate(%q) error=%v valid=%v; want valid date", tc.raw, err, got.Valid)
			}
		})
	}
}

func TestOptionalDate(t *testing.T) {
	got, err := optionalDate("")
	if err != nil || got.Valid {
		t.Errorf("optionalDate(\"\") = valid=%v err=%v; want invalid, nil error", got.Valid, err)
	}
	got, err = optionalDate("2026-08-09")
	if err != nil || !got.Valid {
		t.Errorf("optionalDate(\"2026-08-09\") = valid=%v err=%v; want valid", got.Valid, err)
	}
	if _, err := optionalDate("bad"); err == nil {
		t.Error("optionalDate(\"bad\") expected error, got nil")
	}
}

func TestValidDate(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"2026-08-09", true},
		{"", false},
		{"  ", false},
		{"2026-13-01", false},
		{"09-08-2026", false},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			if got := validDate(tc.raw); got != tc.want {
				t.Errorf("validDate(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestDateStringRoundTrip(t *testing.T) {
	parsed, err := parseDate("2026-08-09")
	if err != nil {
		t.Fatalf("parseDate error: %v", err)
	}
	if got := dateString(parsed); got != "2026-08-09" {
		t.Errorf("dateString = %q, want \"2026-08-09\"", got)
	}
	if got := dateString(pgtype.Date{}); got != "" {
		t.Errorf("dateString(invalid) = %q, want \"\"", got)
	}
}

func TestTextHelpers(t *testing.T) {
	if got := textValueOptional(""); got.Valid {
		t.Error("textValueOptional(\"\") should be invalid")
	}
	if got := textValueOptional("  "); got.Valid {
		t.Error("textValueOptional(\"  \") should be invalid")
	}
	if got := textValueOptional("abc"); !got.Valid || got.String != "abc" {
		t.Errorf("textValueOptional(\"abc\") = %+v; want valid \"abc\"", got)
	}
	if got := textValue(pgtype.Text{}); got != "" {
		t.Errorf("textValue(invalid) = %q, want \"\"", got)
	}
	if got := textValue(pgtype.Text{String: "  x  ", Valid: true}); got != "x" {
		t.Errorf("textValue = %q, want trimmed \"x\"", got)
	}
}

func TestInt8Helpers(t *testing.T) {
	if got := optionalInt8(0); got.Valid {
		t.Error("optionalInt8(0) should be invalid")
	}
	if got := optionalInt8(7); !got.Valid || got.Int64 != 7 {
		t.Errorf("optionalInt8(7) = %+v; want valid 7", got)
	}
	if got := int8Value(0); got.Valid {
		t.Error("int8Value(0) should be invalid")
	}
	if got := int8Value(-3); !got.Valid || got.Int64 != -3 {
		t.Errorf("int8Value(-3) = %+v; want valid -3", got)
	}
}

func TestIdempotencyKey(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		wantErr bool
	}{
		{name: "valid uuid", header: "550e8400-e29b-41d4-a716-446655440000"},
		{name: "missing header", header: "", wantErr: true},
		{name: "whitespace header", header: "   ", wantErr: true},
		{name: "not a uuid", header: "not-a-uuid", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tc.header != "" {
				req.Header.Set("Idempotency-Key", tc.header)
			}
			got, err := idempotencyKey(req)
			if tc.wantErr {
				if err == nil {
					t.Errorf("idempotencyKey expected error, got %q", got)
				}
				return
			}
			if err != nil || got != tc.header {
				t.Errorf("idempotencyKey = %q, %v; want %q", got, err, tc.header)
			}
		})
	}
}

func TestMustJSON(t *testing.T) {
	if got := string(mustJSON(map[string]int{"a": 1})); got != `{"a":1}` {
		t.Errorf("mustJSON = %s, want {\"a\":1}", got)
	}
	// Unmarshalable value falls back to "{}" instead of panicking.
	if got := string(mustJSON(make(chan int))); got != "{}" {
		t.Errorf("mustJSON(chan) = %s, want {}", got)
	}
}

func TestPgtypeFloat(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{1, 1},
		{2.5, 2.5},
		{0, 0},
		{123456.789, 123456.789},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			n := pgtypeFloat(tc.in)
			if !n.Valid {
				t.Fatalf("pgtypeFloat(%v) not valid", tc.in)
			}
			got := numericToFloat(n)
			if got != tc.want {
				t.Errorf("numericToFloat(pgtypeFloat(%v)) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestHashJournalDeterministic(t *testing.T) {
	journal := accounting.Journal{
		TenantID:     1,
		SourceRef:    "GRN-1",
		IntentType:   accounting.IntentType("PURCHASE_RECEIPT"),
		EntryDate:    "2026-08-09",
		PreviousHash: "prev",
		Lines: []accounting.Line{
			{AccountID: 1, DebitCents: 1000},
			{AccountID: 2, CreditCents: 1000},
		},
	}
	first := hashJournal(journal)
	if first == "" {
		t.Fatal("hashJournal returned empty string")
	}
	if second := hashJournal(journal); second != first {
		t.Errorf("hashJournal not deterministic: %q != %q", first, second)
	}
	// Changing content changes the hash.
	altered := journal
	altered.SourceRef = "GRN-2"
	if hashJournal(altered) == first {
		t.Error("hashJournal should change when journal content changes")
	}
}
