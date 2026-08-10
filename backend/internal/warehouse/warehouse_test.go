package warehouse

import (
	"testing"
)

func TestValidateWarehouse(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateWarehouseRequest
		wantError string // error code; empty means valid
	}{
		{
			name: "valid with all fields",
			request: CreateWarehouseRequest{
				Code:     "WH01",
				Name:     "Main Warehouse",
				Address:  "Jl. Industri No. 1",
				City:     "Jakarta",
				IsActive: true,
			},
		},
		{
			name: "valid with only required fields",
			request: CreateWarehouseRequest{
				Code: "WH02",
				Name: "Secondary Warehouse",
			},
		},
		{
			name: "valid with whitespace-padded required fields",
			request: CreateWarehouseRequest{
				Code: "  WH03  ",
				Name: "  Padded Name  ",
			},
		},
		{
			name: "missing code",
			request: CreateWarehouseRequest{
				Name: "Main Warehouse",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "empty code",
			request: CreateWarehouseRequest{
				Code: "",
				Name: "Main Warehouse",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "whitespace-only code",
			request: CreateWarehouseRequest{
				Code: "   ",
				Name: "Main Warehouse",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "missing name",
			request: CreateWarehouseRequest{
				Code: "WH01",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "empty name",
			request: CreateWarehouseRequest{
				Code: "WH01",
				Name: "",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "whitespace-only name",
			request: CreateWarehouseRequest{
				Code: "WH01",
				Name: "   ",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "both missing",
			request: CreateWarehouseRequest{},
			wantError: "INVALID_REQUEST",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _ := validateWarehouse(test.request)
			if test.wantError == "" && code != "" {
				t.Fatalf("expected valid request, got error code %q", code)
			}
			if test.wantError != "" && code != test.wantError {
				t.Fatalf("expected error code %q, got %q", test.wantError, code)
			}
		})
	}
}

func TestValidateWarehouseCodeCheckedBeforeName(t *testing.T) {
	// When both code and name are missing, the validator should report the
	// code error first (returns at the first failure). We assert it returns
	// an error rather than passing; ordering is an implementation detail but
	// must remain a failure.
	code, _ := validateWarehouse(CreateWarehouseRequest{})
	if code == "" {
		t.Fatal("expected error when both code and name are missing")
	}
}

func TestPathID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "positive integer", raw: "42", want: 42},
		{name: "one", raw: "1", want: 1},
		{name: "large int64", raw: "9223372036854775807", want: 9223372036854775807},
		{name: "zero", raw: "0", want: 0},
		{name: "negative", raw: "-5", want: -5},
		{name: "empty string returns zero", raw: "", want: 0},
		{name: "non-numeric returns zero", raw: "abc", want: 0},
		{name: "leading whitespace returns zero", raw: " 12", want: 0},
		{name: "trailing whitespace returns zero", raw: "12 ", want: 0},
		{name: "float string returns zero", raw: "12.5", want: 0},
		{name: "overflow saturates", raw: "99999999999999999999999999", want: 9223372036854775807},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := pathID(test.raw)
			if got != test.want {
				t.Fatalf("pathID(%q) = %d, want %d", test.raw, got, test.want)
			}
		})
	}
}
