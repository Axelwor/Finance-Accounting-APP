package auth

import (
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
