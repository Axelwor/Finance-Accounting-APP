package settings

import (
	"strings"
	"testing"
)

func TestValidatePreferences(t *testing.T) {
	cases := []struct {
		name      string
		req       PutPreferencesRequest
		wantCode  string
		wantMatch string
	}{
		{"empty ok", PutPreferencesRequest{}, "", ""},
		{"valid format", PutPreferencesRequest{DateFormat: strPtr("DD/MM/YYYY")}, "", ""},
		{"invalid format", PutPreferencesRequest{DateFormat: strPtr("DD-MMM-YY")}, "INVALID_REQUEST", "date_format"},
		{"separator too long", PutPreferencesRequest{ThousandSeparator: strPtr("..")}, "INVALID_REQUEST", "thousand_separator"},
		{"same separators", PutPreferencesRequest{ThousandSeparator: strPtr("."), DecimalSeparator: strPtr(".")}, "INVALID_REQUEST", "must differ"},
		{"negative places", PutPreferencesRequest{AmountDecimalPlaces: intPtrHelper(-1)}, "INVALID_REQUEST", "amount_decimal_places"},
		{"too many places", PutPreferencesRequest{QtyDecimalPlaces: intPtrHelper(5)}, "INVALID_REQUEST", "qty_decimal_places"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, message := validatePreferences(tc.req)
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q (message %q)", code, tc.wantCode, message)
			}
			if tc.wantMatch != "" && !strings.Contains(message, tc.wantMatch) {
				t.Fatalf("message %q does not contain %q", message, tc.wantMatch)
			}
		})
	}
}

func TestValidateRateRequest(t *testing.T) {
	cases := []struct {
		name     string
		req      ExchangeRateRequest
		wantCode string
	}{
		{"valid", ExchangeRateRequest{FromCurrency: "USD", ToCurrency: "IDR", Rate: 15750, EffectiveDate: "2026-08-28"}, ""},
		{"short code", ExchangeRateRequest{FromCurrency: "US", ToCurrency: "IDR", Rate: 1, EffectiveDate: "2026-08-28"}, "INVALID_REQUEST"},
		{"same pair", ExchangeRateRequest{FromCurrency: "USD", ToCurrency: "USD", Rate: 1, EffectiveDate: "2026-08-28"}, "INVALID_REQUEST"},
		{"zero rate", ExchangeRateRequest{FromCurrency: "USD", ToCurrency: "IDR", Rate: 0, EffectiveDate: "2026-08-28"}, "INVALID_REQUEST"},
		{"bad date", ExchangeRateRequest{FromCurrency: "USD", ToCurrency: "IDR", Rate: 1, EffectiveDate: "2026/08/28"}, "INVALID_REQUEST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _ := validateRateRequest(tc.req)
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
		})
	}
}

func strPtr(v string) *string { return &v }

func intPtrHelper(v int) *int { return &v }
