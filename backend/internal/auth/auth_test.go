package auth

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestIssueAndParseToken(t *testing.T) {
	service := NewService(nil, "test-secret")

	token, err := service.issueToken(1, 2, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := service.parseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 1 || claims.TenantID != 2 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseRejectsInvalidToken(t *testing.T) {
	service := NewService(nil, "test-secret")
	if _, err := service.parseToken("not-a-token"); err == nil {
		t.Fatal("expected error for invalid token")
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
