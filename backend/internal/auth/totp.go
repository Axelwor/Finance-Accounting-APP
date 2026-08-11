package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"time"
)

// m-006: Two-factor authentication via TOTP (RFC 6238). Implemented with
// only the standard library so no dependency is added.
//
// Setup flow:
//   1. POST /auth/2fa/setup   → generate secret, store unverified, return
//      secret + otpauth:// URI for authenticator apps.
//   2. POST /auth/2fa/verify  → user submits a 6-digit code; if valid,
//      totp_enabled is set to true.
//   3. Login for a user with totp_enabled requires a valid totp_code.

const (
	totpPeriod = 30
	totpDigits = 6
	// totpWindow allows ±1 period of clock drift on verification.
	totpWindow = 1
)

// GenerateTOTPSecret returns a fresh base32-encoded 20-byte secret.
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// TOTPUri builds an otpauth:// provisioning URI for authenticator apps.
func TOTPUri(secret, accountName, issuer string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=%d&period=%d",
		issuer, accountName, secret, issuer, totpDigits, totpPeriod)
}

// TOTPCode computes the RFC 6238 code for the given base32 secret at time t.
func TOTPCode(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("invalid TOTP secret: %w", err)
	}
	counter := uint64(t.Unix()) / totpPeriod

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	// Dynamic truncation (RFC 4226 §5.4).
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	code %= 1_000_000
	return fmt.Sprintf("%06d", code), nil
}

// ValidateTOTP reports whether the supplied 6-digit code matches the secret,
// tolerating ±totpWindow periods of clock drift.
func ValidateTOTP(secret, code string) bool {
	if len(code) != totpDigits {
		return false
	}
	now := time.Now()
	for i := -totpWindow; i <= totpWindow; i++ {
		want, err := TOTPCode(secret, now.Add(time.Duration(i)*totpPeriod*time.Second))
		if err != nil {
			return false
		}
		if hmac.Equal([]byte(want), []byte(code)) {
			return true
		}
	}
	return false
}
