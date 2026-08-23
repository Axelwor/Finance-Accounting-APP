package period

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIdempotencyKeyValidation covers the header guard shared by Close and
// Unlock (QA-09): a missing or malformed key must be rejected before any SQL
// runs, so no uuid cast error can leak into the response.
func TestIdempotencyKeyValidation(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantErr  bool
		wantText string
	}{
		{
			name:     "missing key rejected",
			key:      "",
			wantErr:  true,
			wantText: "Idempotency-Key header is required",
		},
		{
			name:     "whitespace-only key rejected",
			key:      "   ",
			wantErr:  true,
			wantText: "Idempotency-Key header is required",
		},
		{
			name:     "non-uuid key rejected",
			key:      "not-a-uuid",
			wantErr:  true,
			wantText: "Idempotency-Key must be a UUID",
		},
		{
			name:    "valid uuid accepted",
			key:     "6f1e6b3a-0d5e-4a7c-9c2f-3b8a1d2e4f50",
			wantErr: false,
		},
		{
			name:    "padded valid uuid trimmed and accepted",
			key:     "  6f1e6b3a-0d5e-4a7c-9c2f-3b8a1d2e4f50  ",
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/api/v1/periods/close", nil)
			if test.key != "" {
				request.Header.Set("Idempotency-Key", test.key)
			}
			key, err := idempotencyKey(request)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got key %q", key)
				}
				if !strings.Contains(err.Error(), test.wantText) {
					t.Fatalf("error %q does not contain %q", err.Error(), test.wantText)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected success, got %v", err)
			}
			if key != strings.TrimSpace(test.key) {
				t.Fatalf("key = %q, want trimmed %q", key, strings.TrimSpace(test.key))
			}
		})
	}
}
