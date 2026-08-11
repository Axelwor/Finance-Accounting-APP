package auth

import (
	"testing"
	"time"
)

// m-006: TOTP (RFC 6238) tests.

func TestGenerateTOTPSecret(t *testing.T) {
	s1, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if len(s1) < 20 {
		t.Errorf("secret too short: %d chars", len(s1))
	}
	s2, _ := GenerateTOTPSecret()
	if s1 == s2 {
		t.Error("two generated secrets must differ")
	}
}

func TestTOTPCodeRoundtrip(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	now := time.Now()
	code, err := TOTPCode(secret, now)
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("code length = %d, want 6", len(code))
	}
	// Same secret + same 30s window → same code. Pin both instants inside the
	// same window (start of window + 1s/+29s) so the test cannot flake at a
	// window boundary.
	windowStart := time.Unix(now.Unix()/totpPeriod*totpPeriod, 0)
	code2, _ := TOTPCode(secret, windowStart.Add(1*time.Second))
	code3, _ := TOTPCode(secret, windowStart.Add(29*time.Second))
	if code2 != code3 {
		t.Errorf("same-window codes differ: %s vs %s", code2, code3)
	}
	if _, err := TOTPCode(secret, windowStart.Add(30*time.Second)); err != nil {
		t.Errorf("next-window code: %v", err)
	}
}

func TestTOTPCodeKnownVector(t *testing.T) {
	// RFC 6238 Appendix B uses ASCII "12345678901234567890" as the
	// base32 secret GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ (20 bytes).
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := []struct {
		ts   int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, tc := range cases {
		got, err := TOTPCode(secret, time.Unix(tc.ts, 0).UTC())
		if err != nil {
			t.Fatalf("TOTPCode(%d): %v", tc.ts, err)
		}
		if got != tc.want {
			t.Errorf("TOTPCode at %d = %s, want %s", tc.ts, got, tc.want)
		}
	}
}

func TestValidateTOTP(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	now := time.Now()
	current, _ := TOTPCode(secret, now)

	if !ValidateTOTP(secret, current) {
		t.Error("current code must validate")
	}
	// Code from the previous 30s window must validate (±1 drift).
	prev, _ := TOTPCode(secret, now.Add(-30*time.Second))
	if !ValidateTOTP(secret, prev) {
		t.Error("previous-window code must validate (clock drift tolerance)")
	}
	// A code two windows ago must fail.
	old, _ := TOTPCode(secret, now.Add(-90*time.Second))
	if ValidateTOTP(secret, old) {
		t.Error("stale code must not validate")
	}
	if ValidateTOTP(secret, "12345") {
		t.Error("short code must not validate")
	}
	if ValidateTOTP(secret, "") {
		t.Error("empty code must not validate")
	}
	if ValidateTOTP("INVALIDBASE32!!!", current) {
		t.Error("invalid base32 secret must not validate")
	}
}

func TestTOTPUri(t *testing.T) {
	uri := TOTPUri("ABCDEF234567", "user@example.com", "FinanceAccounting")
	want := "otpauth://totp/FinanceAccounting:user@example.com?secret=ABCDEF234567&issuer=FinanceAccounting&digits=6&period=30"
	if uri != want {
		t.Errorf("TOTPUri = %q, want %q", uri, want)
	}
}
