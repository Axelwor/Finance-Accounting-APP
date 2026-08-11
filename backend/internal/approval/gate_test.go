package approval

import (
	"testing"
)

// F-03: approval gate pure-function tests.

func TestErrApprovalRequired(t *testing.T) {
	if ErrApprovalRequired == nil {
		t.Fatal("ErrApprovalRequired must be defined")
	}
	if ErrApprovalRequired.Error() != "approval required before posting" {
		t.Errorf("unexpected error message: %s", ErrApprovalRequired.Error())
	}
}

func TestNewGate(t *testing.T) {
	// NewGate with nil pool is safe (gate defers DB use to method calls).
	g := NewGate(nil)
	if g == nil {
		t.Fatal("NewGate returned nil")
	}
}

func TestNullableID(t *testing.T) {
	if nullableID(0) != nil {
		t.Error("nullableID(0) should be nil")
	}
	if nullableID(5) == nil {
		t.Error("nullableID(5) should not be nil")
	}
	if got, ok := nullableID(7).(int64); !ok || got != 7 {
		t.Errorf("nullableID(7) = %v, want 7", nullableID(7))
	}
}

func TestNullableStr(t *testing.T) {
	if nullableStr("") != nil {
		t.Error("nullableStr(\"\") should be nil")
	}
	if got, ok := nullableStr("INV-001").(string); !ok || got != "INV-001" {
		t.Errorf("nullableStr = %v, want INV-001", nullableStr("INV-001"))
	}
}
