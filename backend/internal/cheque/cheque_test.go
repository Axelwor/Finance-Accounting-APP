package cheque

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Table-driven unit tests for the pure (non-DB) functions in cheque/handler.go.
//   - validateCheque        : request validation
//   - validateBounceReason  : bounce-reason validation
//   - canTransitionTo       : state-transition guard
//   - formatScannedDate     : date-string normalization
//   - pathID                : URL path ID parsing
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }

// ----------------------------------------------------------------------------
// validateCheque
// ----------------------------------------------------------------------------

func TestValidateCheque(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateChequeRequest
		wantCode string
		wantMsg  string
	}{
		{
			name:    "valid cheque received",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-001", ChequeType: "cheque", Direction: "received", AmountCents: 50000, IssueDate: "2026-01-15"},
			wantCode: "",
			wantMsg:  "",
		},
		{
			name:    "valid giro issued with whitespace and case",
			req:     CreateChequeRequest{ChequeNumber: "  GIRO-9  ", ChequeType: "  Giro ", Direction: " ISSUED ", AmountCents: 1, IssueDate: "2026-02-01"},
			wantCode: "",
			wantMsg:  "",
		},
		{
			name:    "missing cheque number",
			req:     CreateChequeRequest{ChequeNumber: "   ", ChequeType: "CHEQUE", Direction: "RECEIVED", AmountCents: 100, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "cheque_number is required",
		},
		{
			name:    "empty cheque number",
			req:     CreateChequeRequest{ChequeNumber: "", ChequeType: "CHEQUE", Direction: "RECEIVED", AmountCents: 100, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "cheque_number is required",
		},
		{
			name:    "invalid cheque type",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "WARRANT", Direction: "RECEIVED", AmountCents: 100, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "cheque_type must be CHEQUE or GIRO",
		},
		{
			name:    "empty cheque type",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "", Direction: "RECEIVED", AmountCents: 100, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "cheque_type must be CHEQUE or GIRO",
		},
		{
			name:    "invalid direction",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "CHEQUE", Direction: "SIDWAYS", AmountCents: 100, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "direction must be RECEIVED or ISSUED",
		},
		{
			name:    "empty direction",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "CHEQUE", Direction: "", AmountCents: 100, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "direction must be RECEIVED or ISSUED",
		},
		{
			name:    "zero amount cents",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "CHEQUE", Direction: "RECEIVED", AmountCents: 0, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "amount_cents must be positive",
		},
		{
			name:    "negative amount cents",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "CHEQUE", Direction: "RECEIVED", AmountCents: -500, IssueDate: "2026-01-15"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "amount_cents must be positive",
		},
		{
			name:    "missing issue date",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "CHEQUE", Direction: "RECEIVED", AmountCents: 100, IssueDate: ""},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "issue_date is required",
		},
		{
			name:    "whitespace issue date",
			req:     CreateChequeRequest{ChequeNumber: "CHQ-1", ChequeType: "CHEQUE", Direction: "RECEIVED", AmountCents: 100, IssueDate: "   "},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "issue_date is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotMsg := validateCheque(tt.req)
			if gotCode != tt.wantCode {
				t.Errorf("validateCheque() code = %q, want %q", gotCode, tt.wantCode)
			}
			if gotMsg != tt.wantMsg {
				t.Errorf("validateCheque() msg = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// validateBounceReason
// ----------------------------------------------------------------------------

func TestValidateBounceReason(t *testing.T) {
	tests := []struct {
		name     string
		reason   string
		wantCode string
		wantMsg  string
	}{
		{name: "non-empty reason", reason: "Insufficient funds", wantCode: "", wantMsg: ""},
		{name: "reason with surrounding whitespace", reason: "  NSF  ", wantCode: "", wantMsg: ""},
		{name: "empty reason", reason: "", wantCode: "INVALID_REQUEST", wantMsg: "reason is required"},
		{name: "whitespace-only reason", reason: "   ", wantCode: "INVALID_REQUEST", wantMsg: "reason is required"},
		{name: "single char reason", reason: "X", wantCode: "", wantMsg: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotMsg := validateBounceReason(tt.reason)
			if gotCode != tt.wantCode {
				t.Errorf("validateBounceReason() code = %q, want %q", gotCode, tt.wantCode)
			}
			if gotMsg != tt.wantMsg {
				t.Errorf("validateBounceReason() msg = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// canTransitionTo
// ----------------------------------------------------------------------------

func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		name    string
		current string
		target  string
		want    bool
	}{
		// Deposit: only from REGISTERED
		{name: "registered to deposited", current: "REGISTERED", target: "DEPOSITED", want: true},
		{name: "deposited to deposited", current: "DEPOSITED", target: "DEPOSITED", want: false},
		{name: "cleared to deposited", current: "CLEARED", target: "DEPOSITED", want: false},
		{name: "bounced to deposited", current: "BOUNCED", target: "DEPOSITED", want: false},

		// Clear: from REGISTERED or DEPOSITED
		{name: "registered to cleared", current: "REGISTERED", target: "CLEARED", want: true},
		{name: "deposited to cleared", current: "DEPOSITED", target: "CLEARED", want: true},
		{name: "cleared to cleared", current: "CLEARED", target: "CLEARED", want: false},
		{name: "bounced to cleared", current: "BOUNCED", target: "CLEARED", want: false},

		// Bounce: from REGISTERED or DEPOSITED
		{name: "registered to bounced", current: "REGISTERED", target: "BOUNCED", want: true},
		{name: "deposited to bounced", current: "DEPOSITED", target: "BOUNCED", want: true},
		{name: "cleared to bounced", current: "CLEARED", target: "BOUNCED", want: false},
		{name: "bounced to bounced", current: "BOUNCED", target: "BOUNCED", want: false},

		// Unknown target / source
		{name: "registered to unknown", current: "REGISTERED", target: "FROZEN", want: false},
		{name: "unknown to cleared", current: "FROZEN", target: "CLEARED", want: false},
		{name: "empty current", current: "", target: "CLEARED", want: false},
		{name: "empty target", current: "REGISTERED", target: "", want: false},

		// Case-insensitivity / trimming
		{name: "lowercase registered to deposited", current: "registered", target: "deposited", want: true},
		{name: "padded deposited to cleared", current: "  deposited ", target: " cleared ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canTransitionTo(tt.current, tt.target)
			if got != tt.want {
				t.Errorf("canTransitionTo(%q, %q) = %v, want %v", tt.current, tt.target, got, tt.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// formatScannedDate
// ----------------------------------------------------------------------------

func TestFormatScannedDate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "full ISO datetime trims to date", in: "2026-01-15T00:00:00Z", want: "2026-01-15"},
		{name: "date with time component", in: "2026-12-31T23:59:59Z", want: "2026-12-31"},
		{name: "exactly 10 chars passes through", in: "2026-01-15", want: "2026-01-15"},
		{name: "short string returned as-is", in: "2026", want: "2026"},
		{name: "empty string returned as-is", in: "", want: ""},
		{name: "nine char string returned as-is", in: "2026-01-1", want: "2026-01-1"},
		{name: "eleven chars trims to ten", in: "2026-01-15X", want: "2026-01-15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatScannedDate(tt.in)
			if got != tt.want {
				t.Errorf("formatScannedDate(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// pathID
// ----------------------------------------------------------------------------

func TestPathID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "positive integer", raw: "42", want: 42},
		{name: "one", raw: "1", want: 1},
		{name: "large number", raw: "9999999999", want: 9999999999},
		{name: "zero", raw: "0", want: 0},
		{name: "negative number", raw: "-5", want: -5},
		{name: "non-numeric string", raw: "abc", want: 0},
		{name: "empty string", raw: "", want: 0},
		{name: "mixed alphanumeric", raw: "12abc", want: 0},
		{name: "float string", raw: "3.14", want: 0},
		{name: "leading whitespace", raw: " 7", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathID(tt.raw)
			if got != tt.want {
				t.Errorf("pathID(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Status constants
// ----------------------------------------------------------------------------

func TestChequeStatusConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "StatusRegistered", got: StatusRegistered, want: "REGISTERED"},
		{name: "StatusDeposited", got: StatusDeposited, want: "DEPOSITED"},
		{name: "StatusCleared", got: StatusCleared, want: "CLEARED"},
		{name: "StatusBounced", got: StatusBounced, want: "BOUNCED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("constant %s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
