package httperr

import (
	"strings"
	"testing"
)

// TestSanitizeMessage5xx verifies that internal (5xx) error messages are
// replaced with a generic client-safe message. The raw message can contain
// SQL fragments, connection strings, or stack details — none of which may
// reach the API client.
func TestSanitizeMessage5xx(t *testing.T) {
	raw := `pq: duplicate key value violates unique constraint "journal_entries_number_key" (SQLSTATE 23505)`
	got := SanitizeMessage(500, "DB_ERROR", raw)
	if got != "An internal error occurred. Please try again or contact support." {
		t.Errorf("5xx message not sanitized: %q", got)
	}
	if strings.Contains(got, "SQLSTATE") {
		t.Error("sanitized message must not contain the raw SQL error")
	}
}

// TestSanitizeMessage5xxBoundaries checks every status at and above 500 is
// sanitized, including non-canonical codes like 599.
func TestSanitizeMessage5xxBoundaries(t *testing.T) {
	for _, status := range []int{500, 502, 503, 599} {
		if got := SanitizeMessage(status, "X", "secret detail"); got == "secret detail" {
			t.Errorf("status %d must sanitize the message", status)
		}
	}
}

// TestSanitizeMessage4xxPreserved verifies validation (4xx) messages pass
// through untouched — they are intentional, human-readable, and required
// for the form UX.
func TestSanitizeMessage4xxPreserved(t *testing.T) {
	msg := "code, name, and imprest_amount_cents are required"
	if got := SanitizeMessage(400, "INVALID_REQUEST", msg); got != msg {
		t.Errorf("4xx message must be preserved, got %q", got)
	}
	if got := SanitizeMessage(409, "CONFLICT", "duplicate code"); got != "duplicate code" {
		t.Errorf("409 message must be preserved, got %q", got)
	}
	if got := SanitizeMessage(404, "NOT_FOUND", "cost center not found"); got != "cost center not found" {
		t.Errorf("404 message must be preserved, got %q", got)
	}
}

// TestSanitizeMessage499Boundary: 499 is the highest 4xx — still preserved.
func TestSanitizeMessage499Boundary(t *testing.T) {
	if got := SanitizeMessage(499, "X", "client closed request"); got != "client closed request" {
		t.Errorf("499 message must be preserved, got %q", got)
	}
}
