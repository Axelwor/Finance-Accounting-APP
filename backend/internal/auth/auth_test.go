package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIssueAndParseToken(t *testing.T) {
	service := NewService(nil, "test-secret")

	token, err := service.issueToken(1, 2, RoleAdmin, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := service.parseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 1 || claims.TenantID != 2 || claims.Role != RoleAdmin {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseRejectsInvalidToken(t *testing.T) {
	service := NewService(nil, "test-secret")
	if _, err := service.parseToken("not-a-token"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}

// F-13: a token signed with a different algorithm must be rejected even when
// the signature is valid for its own key (alg confusion / none-attack guard).
func TestParseRejectsWrongAlgorithm(t *testing.T) {
	service := NewService(nil, "test-secret")

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	claims := Claims{
		UserID:   1,
		TenantID: 2,
		Role:     RoleOwner,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	rsToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.parseToken(rsToken); err == nil {
		t.Fatal("expected RS256 token to be rejected, got valid claims")
	}

	noneToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.parseToken(noneToken); err == nil {
		t.Fatal("expected alg=none token to be rejected, got valid claims")
	}
}

func TestHashTokenIsDeterministic(t *testing.T) {
	first := hashToken("abc")
	second := hashToken("abc")
	if first != second || first == "abc" {
		t.Fatalf("hash is not deterministic or leaks plaintext: %q", first)
	}
}

func TestRandomTokenIsHex(t *testing.T) {
	token := randomToken()
	if len(token) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(token))
	}
	if _, err := hex.DecodeString(token); err != nil {
		t.Fatalf("token is not valid hex: %v", err)
	}
}

func TestRandomUUIDFormat(t *testing.T) {
	uuid := randomUUID()
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Fatalf("expected 5 parts, got %q", uuid)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Acme Corp", "acme-corp"},
		{"  Acme   Corp  ", "acme-corp"},
		{"PT. Maju Jaya & Sons", "pt-maju-jaya-sons"},
		{"Café Über", "caf-ber"},
		{"---", "tenant"},
		{"", "tenant"},
		{"Toko123", "toko123"},
	}
	for _, tc := range cases {
		if got := slugify(tc.name); got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRandomSuffixIsHex(t *testing.T) {
	suffix := randomSuffix()
	if len(suffix) != 4 {
		t.Fatalf("expected 4 hex chars, got %q", suffix)
	}
	if _, err := hex.DecodeString(suffix); err != nil {
		t.Fatalf("suffix is not valid hex: %v", err)
	}
}

// TestIsUniqueViolation covers FIX-MINOR-004: only PostgreSQL 23505 counts as
// a duplicate-key insert so permission/schema failures are not disguised as
// EMAIL_EXISTS during registration.
func TestIsUniqueViolation(t *testing.T) {
	dup := &pgconn.PgError{Code: "23505"}
	wrapped := fmt.Errorf("insert failed: %w", dup)
	fk := &pgconn.PgError{Code: "23503"}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unique-violation", dup, true},
		{"wrapped-unique", wrapped, true},
		{"foreign-key", fk, false},
		{"permission", &pgconn.PgError{Code: "42501"}, false},
		{"plain-error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		if got := isUniqueViolation(tc.err); got != tc.want {
			t.Errorf("%s: isUniqueViolation = %v, want %v", tc.name, got, tc.want)
		}
	}
}
