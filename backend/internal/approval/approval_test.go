package approval

import (
	"strings"
	"testing"
)

func TestValidateWorkflow(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateWorkflowRequest
		wantError string // error code; empty means valid
	}{
		{
			name: "valid admin role",
			request: CreateWorkflowRequest{
				EntityType:     "INVOICE",
				MinAmountCents: 5000000,
				ApproverRole:   "admin",
			},
		},
		{
			name: "valid accountant role",
			request: CreateWorkflowRequest{
				EntityType:     "PURCHASE_ORDER",
				MinAmountCents: 1000000,
				ApproverRole:   "accountant",
			},
		},
		{
			name: "valid manager role",
			request: CreateWorkflowRequest{
				EntityType:   "JOURNAL",
				ApproverRole: "manager",
			},
		},
		{
			name: "valid role case-insensitive uppercase",
			request: CreateWorkflowRequest{
				EntityType:   "CN",
				ApproverRole: "ADMIN",
			},
		},
		{
			name: "valid role case-insensitive mixed case",
			request: CreateWorkflowRequest{
				EntityType:   "INVOICE",
				ApproverRole: "AcCoUnTaNt",
			},
		},
		{
			name: "valid role with surrounding whitespace (trimmed by ToLower path)",
			request: CreateWorkflowRequest{
				EntityType:   "INVOICE",
				ApproverRole: " manager ",
			},
			// NOTE: validateWorkflow lowercases but does not TrimSpace, so a
			// role with surrounding whitespace is NOT matched and is rejected.
			// This test documents that current behaviour.
			wantError: "INVALID_REQUEST",
		},
		{
			name: "valid entity type with surrounding whitespace",
			request: CreateWorkflowRequest{
				EntityType:   "  INVOICE  ",
				ApproverRole: "admin",
			},
		},
		{
			name: "valid with zero min amount (threshold not validated)",
			request: CreateWorkflowRequest{
				EntityType:   "INVOICE",
				ApproverRole: "admin",
			},
		},
		{
			name: "missing entity type",
			request: CreateWorkflowRequest{
				ApproverRole: "admin",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "empty entity type",
			request: CreateWorkflowRequest{
				EntityType:   "",
				ApproverRole: "admin",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "whitespace-only entity type",
			request: CreateWorkflowRequest{
				EntityType:   "   ",
				ApproverRole: "admin",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "unknown approver role",
			request: CreateWorkflowRequest{
				EntityType:   "INVOICE",
				ApproverRole: "superuser",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "empty approver role",
			request: CreateWorkflowRequest{
				EntityType:   "INVOICE",
				ApproverRole: "",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "whitespace-only approver role",
			request: CreateWorkflowRequest{
				EntityType:   "INVOICE",
				ApproverRole: "   ",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "both missing",
			request:   CreateWorkflowRequest{},
			wantError: "INVALID_REQUEST",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _ := validateWorkflow(test.request)
			if test.wantError == "" && code != "" {
				t.Fatalf("expected valid request, got error code %q", code)
			}
			if test.wantError != "" && code != test.wantError {
				t.Fatalf("expected error code %q, got %q", test.wantError, code)
			}
		})
	}
}

func TestValidateWorkflowEntityTypeCheckedBeforeRole(t *testing.T) {
	// When both fields are invalid, entity_type is validated first.
	code, msg := validateWorkflow(CreateWorkflowRequest{
		EntityType:   "",
		ApproverRole: "nobody",
	})
	if code == "" {
		t.Fatal("expected an error when entity_type is empty")
	}
	if !strings.Contains(msg, "entity_type") {
		t.Fatalf("expected message about entity_type, got %q", msg)
	}
}

func TestValidateWorkflowRoleErrorMessage(t *testing.T) {
	code, msg := validateWorkflow(CreateWorkflowRequest{
		EntityType:   "INVOICE",
		ApproverRole: "ceo",
	})
	if code != "INVALID_REQUEST" {
		t.Fatalf("expected error code INVALID_REQUEST, got %q", code)
	}
	// Message must list all three accepted roles so callers can self-correct.
	for _, role := range []string{"admin", "accountant", "manager"} {
		if !strings.Contains(msg, role) {
			t.Fatalf("expected error message to mention role %q, got %q", role, msg)
		}
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
