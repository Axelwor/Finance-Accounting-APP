package pph

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Rate constants — pin the values defined by Indonesian tax law (2026) so a
// silent edit to a rate is caught here before it reaches production.
// ---------------------------------------------------------------------------

func TestRateConstants(t *testing.T) {
	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{name: "PPh21 non-NPWP 20%", got: RatePPh21NonNPWP, want: 0.20},
		{name: "PPh22 import 2.5%", got: RatePPh22Import, want: 0.025},
		{name: "PPh23 service 2%", got: RatePPh23Service, want: 0.02},
		{name: "PPh23 rent 10%", got: RatePPh23Rent, want: 0.10},
		{name: "PPh23 royalty 15%", got: RatePPh23Royalty, want: 0.15},
		{name: "PPh26 non-resident 20%", got: RatePPh26NonRes, want: 0.20},
		{name: "PPh Final UMKM 0.5%", got: RatePPhFinalUMKM, want: 0.005},
		{name: "PPh Final UMKM 0.75%", got: RatePPhFinalUMKM075, want: 0.0075},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("%s = %v, want %v", test.name, test.got, test.want)
			}
		})
	}
}

func TestAccountConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "PPh21 payable account", got: AccountPPh21, want: "2107"},
		{name: "PPh22 payable account", got: AccountPPh22, want: "2108"},
		{name: "PPh23 payable account", got: AccountPPh23, want: "2109"},
		{name: "PPh26 payable account", got: AccountPPh26, want: "2110"},
		{name: "PPh UMKM payable account", got: AccountPPhUMKM, want: "2111"},
		{name: "Income tax expense account", got: AccountIncomeTax, want: "5203"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("%s = %q, want %q", test.name, test.got, test.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// calculatePPh — integer math. rateMilli = ratePercent * 1000, then
// dppCents * rateMilli / 100000. Truncation (floor toward zero) is the
// documented behaviour and is asserted below.
// ---------------------------------------------------------------------------

func TestCalculatePPh(t *testing.T) {
	tests := []struct {
		name        string
		dppCents    int64
		ratePercent float64
		want        int64
	}{
		// --- exact / clean cases ---
		{name: "PPh23 service 2% on 100M cents", dppCents: 100_000_00, ratePercent: 2.0, want: 2_000_00},
		{name: "PPh22 import 2.5% on 100M cents", dppCents: 100_000_00, ratePercent: 2.5, want: 2_500_00},
		{name: "PPh23 rent 10% on 100M cents", dppCents: 100_000_00, ratePercent: 10.0, want: 10_000_00},
		{name: "PPh23 royalty 15% on 100M cents", dppCents: 100_000_00, ratePercent: 15.0, want: 15_000_00},
		{name: "PPh21 non-NPWP 20% on 100M cents", dppCents: 100_000_00, ratePercent: 20.0, want: 20_000_00},
		{name: "PPh Final UMKM 0.5% on 100M cents", dppCents: 100_000_00, ratePercent: 0.5, want: 500_00},
		{name: "PPh Final UMKM 0.75% on 100M cents", dppCents: 100_000_00, ratePercent: 0.75, want: 750_00},

		// --- fractional-rate truncation (integer math rounds toward zero) ---
		// 0.75% of 1_000_001 cents = 7500.0075 milli-cents -> 7500 milli-cents
		// dppCents*rateMilli = 1_000_001 * 750 = 750_000_750; /100000 = 7500 (truncated)
		{name: "0.75% on 1M+1 cents truncates", dppCents: 1_000_001, ratePercent: 0.75, want: 7500},
		// 2% of 99_999 cents = 1999.98 cents -> 1999 cents (truncated)
		{name: "2% on 99999 cents truncates", dppCents: 99_999, ratePercent: 2.0, want: 1999},

		// --- small DPP ---
		{name: "1% on 100 cents", dppCents: 100, ratePercent: 1.0, want: 1},
		{name: "0.5% on 100 cents rounds down to 0", dppCents: 100, ratePercent: 0.5, want: 0},
		{name: "0.5% on 1000 cents", dppCents: 1000, ratePercent: 0.5, want: 5},

		// --- guard cases (return 0) ---
		{name: "zero dpp returns 0", dppCents: 0, ratePercent: 10.0, want: 0},
		{name: "negative dpp returns 0", dppCents: -1_000_00, ratePercent: 10.0, want: 0},
		{name: "zero rate returns 0", dppCents: 1_000_00, ratePercent: 0.0, want: 0},
		{name: "negative rate returns 0", dppCents: 1_000_00, ratePercent: -2.0, want: 0},

		// --- large DPP (no overflow at typical invoice sizes) ---
		{name: "2% on 10 billion cents", dppCents: 10_000_000_00, ratePercent: 2.0, want: 200_000_00},

		// --- upper boundary rate (100%) ---
		{name: "100% on 1M cents", dppCents: 1_000_00, ratePercent: 100.0, want: 1_000_00},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := calculatePPh(test.dppCents, test.ratePercent)
			if got != test.want {
				t.Fatalf("calculatePPh(dpp=%d, rate=%v) = %d, want %d",
					test.dppCents, test.ratePercent, got, test.want)
			}
		})
	}
}

func TestCalculatePPhTruncatesTowardZero(t *testing.T) {
	// The formula divides by 100000 using integer division, which for
	// positive operands floors toward zero. Verify two DPPs that produce a
	// remainder both map to the same (truncated) result.
	low := calculatePPh(1_000_001, 2.0)  // 2_000_002 / ... -> 20000.02 -> 20000
	high := calculatePPh(1_004_999, 2.0) // 2_009_998 / ... -> 20099.98 -> 20099
	if low == high {
		t.Fatalf("expected distinct truncated values, both were %d", low)
	}
	// And the true mathematical floor of each:
	if low != 20000 {
		t.Fatalf("low = %d, want 20000", low)
	}
	if high != 20099 {
		t.Fatalf("high = %d, want 20099", high)
	}
}

// ---------------------------------------------------------------------------
// pphAccountForType
// ---------------------------------------------------------------------------

func TestPphAccountForType(t *testing.T) {
	tests := []struct {
		name    string
		pphType string
		want    string
	}{
		{name: "PPH21", pphType: "PPH21", want: AccountPPh21},
		{name: "PPH22", pphType: "PPH22", want: AccountPPh22},
		{name: "PPH23", pphType: "PPH23", want: AccountPPh23},
		{name: "PPH26", pphType: "PPH26", want: AccountPPh26},
		{name: "PPH_FINAL_UMKM", pphType: "PPH_FINAL_UMKM", want: AccountPPhUMKM},

		// case-insensitive (ToUpper normalisation)
		{name: "lowercase pph21", pphType: "pph21", want: AccountPPh21},
		{name: "mixed case Pph23", pphType: "Pph23", want: AccountPPh23},
		{name: "lowercase pph_final_umkm", pphType: "pph_final_umkm", want: AccountPPhUMKM},

		// unknown / empty
		{name: "unknown type returns empty", pphType: "PPH99", want: ""},
		{name: "empty type returns empty", pphType: "", want: ""},
		{name: "whitespace type returns empty", pphType: "   ", want: ""},
		{name: "PPH15 not supported", pphType: "PPH15", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := pphAccountForType(test.pphType)
			if got != test.want {
				t.Fatalf("pphAccountForType(%q) = %q, want %q", test.pphType, got, test.want)
			}
		})
	}
}

func TestPphAccountForTypeEveryKnownType(t *testing.T) {
	// Every PPh type accepted by validatePPh must resolve to a non-empty
	// payable account code. This guards against a new type being added to
	// validation without a matching account mapping.
	for _, pphType := range []string{"PPH21", "PPH22", "PPH23", "PPH26", "PPH_FINAL_UMKM"} {
		if got := pphAccountForType(pphType); got == "" {
			t.Fatalf("pphAccountForType(%q) returned empty for a valid PPh type", pphType)
		}
	}
}

func TestPphAccountForTypeAccountsAreDistinct(t *testing.T) {
	// Each PPh type must map to a unique payable account so postings don't
	// collide in the ledger.
	seen := make(map[string]string) // account -> first type that used it
	for _, pphType := range []string{"PPH21", "PPH22", "PPH23", "PPH26", "PPH_FINAL_UMKM"} {
		account := pphAccountForType(pphType)
		if first, dup := seen[account]; dup {
			t.Fatalf("PPh types %s and %s share account %q", first, pphType, account)
		}
		seen[account] = pphType
	}
}

// ---------------------------------------------------------------------------
// validatePPh
// ---------------------------------------------------------------------------

func TestValidatePPh(t *testing.T) {
	validReq := CreatePPhRequest{
		PphType:         "PPH23",
		CalculationDate: "2026-03-15",
		DppCents:        50_000_00,
		RatePercent:     2.0,
		EntityName:      "PT Sumber Rezeki",
		EntityNPWP:      "01.234.567.8-091.000",
		Description:     "Consulting fee withholding",
	}

	tests := []struct {
		name      string
		mutate    func(req *CreatePPhRequest)
		wantError string // error code; empty means valid
	}{
		{
			name:   "valid request",
			mutate: func(req *CreatePPhRequest) {},
		},
		{
			name:      "missing pph_type",
			mutate:    func(req *CreatePPhRequest) { req.PphType = "" },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "unknown pph_type",
			mutate:    func(req *CreatePPhRequest) { req.PphType = "PPH99" },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "pph_type case-insensitive (lowercase accepted)",
			mutate:    func(req *CreatePPhRequest) { req.PphType = "pph23" },
			wantError: "",
		},
		{
			name:      "pph_type PPH21 accepted",
			mutate:    func(req *CreatePPhRequest) { req.PphType = "PPH21" },
		},
		{
			name:      "pph_type PPH22 accepted",
			mutate:    func(req *CreatePPhRequest) { req.PphType = "PPH22" },
		},
		{
			name:      "pph_type PPH26 accepted",
			mutate:    func(req *CreatePPhRequest) { req.PphType = "PPH26" },
		},
		{
			name:      "pph_type PPH_FINAL_UMKM accepted",
			mutate:    func(req *CreatePPhRequest) { req.PphType = "PPH_FINAL_UMKM" },
		},
		{
			name:      "zero dpp rejected",
			mutate:    func(req *CreatePPhRequest) { req.DppCents = 0 },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "negative dpp rejected",
			mutate:    func(req *CreatePPhRequest) { req.DppCents = -100 },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "zero rate rejected",
			mutate:    func(req *CreatePPhRequest) { req.RatePercent = 0 },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "negative rate rejected",
			mutate:    func(req *CreatePPhRequest) { req.RatePercent = -1.5 },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "rate above 100 rejected",
			mutate:    func(req *CreatePPhRequest) { req.RatePercent = 100.01 },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "rate exactly 100 accepted",
			mutate:    func(req *CreatePPhRequest) { req.RatePercent = 100.0 },
		},
		{
			name:      "rate just above zero accepted",
			mutate:    func(req *CreatePPhRequest) { req.RatePercent = 0.5 },
		},
		{
			name:      "missing calculation_date",
			mutate:    func(req *CreatePPhRequest) { req.CalculationDate = "" },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "malformed calculation_date (slashes)",
			mutate:    func(req *CreatePPhRequest) { req.CalculationDate = "2026/03/15" },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "malformed calculation_date (missing day)",
			mutate:    func(req *CreatePPhRequest) { req.CalculationDate = "2026-03" },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "malformed calculation_date (text)",
			mutate:    func(req *CreatePPhRequest) { req.CalculationDate = "March 15, 2026" },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "calculation_date with time component rejected",
			mutate:    func(req *CreatePPhRequest) { req.CalculationDate = "2026-03-15T00:00:00Z" },
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "invalid day for month (Feb 30)",
			mutate:    func(req *CreatePPhRequest) { req.CalculationDate = "2026-02-30" },
			wantError: "INVALID_REQUEST",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := validReq
			test.mutate(&req)
			code, _ := validatePPh(req)
			if test.wantError == "" && code != "" {
				t.Fatalf("expected valid request, got error code %q", code)
			}
			if test.wantError != "" && code != test.wantError {
				t.Fatalf("expected error code %q, got %q", test.wantError, code)
			}
		})
	}
}

func TestValidatePPhTypeErrorIncludesValue(t *testing.T) {
	// The unknown-type error message echoes the offending value so support
	// can trace which input triggered it.
	code, msg := validatePPh(CreatePPhRequest{
		PphType:         "PPH99",
		CalculationDate: "2026-03-15",
		DppCents:        1_000_00,
		RatePercent:     2.0,
	})
	if code != "INVALID_REQUEST" {
		t.Fatalf("expected error code INVALID_REQUEST, got %q", code)
	}
	if !strings.Contains(msg, "PPH99") {
		t.Fatalf("expected error message to contain the offending type %q, got %q", "PPH99", msg)
	}
	if !strings.Contains(msg, "PPH21") {
		t.Fatalf("expected error message to list accepted types, got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// pathID
// ---------------------------------------------------------------------------

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
