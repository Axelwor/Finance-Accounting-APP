package recurring

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests for pure functions in the recurring package:
//   - validateRecurring
//   - computeNextDate
//   - pathID
//   - nullIfEmpty
//   - nullIfZero
// ---------------------------------------------------------------------------

func TestValidateRecurring(t *testing.T) {
	baseReq := CreateRecurringRequest{
		Code:        "RENT-001",
		Name:        "Office Rent",
		IntentType:  "CASH_OUT",
		Frequency:   "monthly",
		NextDate:    "2026-01-01",
		AmountCents: 250000,
	}

	tests := []struct {
		name     string
		mutate   func(req *CreateRecurringRequest)
		wantCode string
		wantMsg  string
	}{
		{
			name:     "valid request passes",
			mutate:   func(r *CreateRecurringRequest) {},
			wantCode: "",
			wantMsg:  "",
		},
		{
			name:     "empty code rejected",
			mutate:   func(r *CreateRecurringRequest) { r.Code = "" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "code is required",
		},
		{
			name:     "whitespace-only code rejected",
			mutate:   func(r *CreateRecurringRequest) { r.Code = "   " },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "code is required",
		},
		{
			name:     "empty name rejected",
			mutate:   func(r *CreateRecurringRequest) { r.Name = "" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "name is required",
		},
		{
			name:     "whitespace-only name rejected",
			mutate:   func(r *CreateRecurringRequest) { r.Name = "\t\n" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "name is required",
		},
		{
			name:     "zero amount rejected",
			mutate:   func(r *CreateRecurringRequest) { r.AmountCents = 0 },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "amount_cents must be > 0",
		},
		{
			name:     "negative amount rejected",
			mutate:   func(r *CreateRecurringRequest) { r.AmountCents = -100 },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "amount_cents must be > 0",
		},
		{
			name:     "invalid intent type rejected",
			mutate:   func(r *CreateRecurringRequest) { r.IntentType = "WIRE" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "intent_type must be one of: CASH_IN, CASH_OUT, TRANSFER, MANUAL_JOURNAL",
		},
		{
			name:     "empty intent type rejected",
			mutate:   func(r *CreateRecurringRequest) { r.IntentType = "" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "intent_type must be one of: CASH_IN, CASH_OUT, TRANSFER, MANUAL_JOURNAL",
		},
		{
			name:     "invalid frequency rejected",
			mutate:   func(r *CreateRecurringRequest) { r.Frequency = "fortnightly" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "frequency must be one of: daily, weekly, monthly, quarterly, yearly",
		},
		{
			name:     "empty frequency rejected",
			mutate:   func(r *CreateRecurringRequest) { r.Frequency = "" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "frequency must be one of: daily, weekly, monthly, quarterly, yearly",
		},
		{
			name:     "empty next date rejected",
			mutate:   func(r *CreateRecurringRequest) { r.NextDate = "" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "next_date is required (YYYY-MM-DD)",
		},
		{
			name:     "malformed next date rejected",
			mutate:   func(r *CreateRecurringRequest) { r.NextDate = "01/15/2026" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "next_date must be YYYY-MM-DD",
		},
		{
			name:     "next date wrong format rejected",
			mutate:   func(r *CreateRecurringRequest) { r.NextDate = "2026-1-5" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "next_date must be YYYY-MM-DD",
		},
		{
			name:     "malformed end date rejected",
			mutate:   func(r *CreateRecurringRequest) { r.EndDate = "Dec 31 2026" },
			wantCode: "INVALID_REQUEST",
			wantMsg:  "end_date must be YYYY-MM-DD",
		},
		{
			name:     "valid end date passes",
			mutate:   func(r *CreateRecurringRequest) { r.EndDate = "2026-12-31" },
			wantCode: "",
			wantMsg:  "",
		},
		{
			name:     "empty end date is allowed",
			mutate:   func(r *CreateRecurringRequest) { r.EndDate = "" },
			wantCode: "",
			wantMsg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := baseReq
			tt.mutate(&req)
			code, msg := validateRecurring(req)
			if code != tt.wantCode {
				t.Errorf("validateRecurring code = %q, want %q", code, tt.wantCode)
			}
			if msg != tt.wantMsg {
				t.Errorf("validateRecurring msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestValidateRecurring_AllIntentTypes(t *testing.T) {
	validIntents := []string{"CASH_IN", "CASH_OUT", "TRANSFER", "MANUAL_JOURNAL"}
	for _, intent := range validIntents {
		t.Run(intent, func(t *testing.T) {
			req := CreateRecurringRequest{
				Code: "X", Name: "Y", IntentType: intent,
				Frequency: "daily", NextDate: "2026-01-01", AmountCents: 1,
			}
			code, _ := validateRecurring(req)
			if code != "" {
				t.Errorf("expected valid for intent %q, got code %q", intent, code)
			}
		})
	}
}

func TestValidateRecurring_AllFrequencies(t *testing.T) {
	validFreqs := []string{"daily", "weekly", "monthly", "quarterly", "yearly"}
	for _, freq := range validFreqs {
		t.Run(freq, func(t *testing.T) {
			req := CreateRecurringRequest{
				Code: "X", Name: "Y", IntentType: "CASH_IN",
				Frequency: freq, NextDate: "2026-01-01", AmountCents: 1,
			}
			code, _ := validateRecurring(req)
			if code != "" {
				t.Errorf("expected valid for frequency %q, got code %q", freq, code)
			}
		})
	}
}

func TestComputeNextDate(t *testing.T) {
	base := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		current   time.Time
		frequency string
		want      time.Time
	}{
		{"daily advances 1 day", base, "daily", base.AddDate(0, 0, 1)},
		{"weekly advances 7 days", base, "weekly", base.AddDate(0, 0, 7)},
		{"monthly advances 1 month", base, "monthly", base.AddDate(0, 1, 0)},
		{"quarterly advances 3 months", base, "quarterly", base.AddDate(0, 3, 0)},
		{"yearly advances 1 year", base, "yearly", base.AddDate(1, 0, 0)},
		{"unknown frequency defaults to monthly", base, "bimonthly", base.AddDate(0, 1, 0)},
		{"empty frequency defaults to monthly", base, "", base.AddDate(0, 1, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeNextDate(tt.current, tt.frequency)
			if !got.Equal(tt.want) {
				t.Errorf("computeNextDate(%q) = %v, want %v", tt.frequency, got, tt.want)
			}
		})
	}
}

func TestComputeNextDate_MonthBoundaryRollover(t *testing.T) {
	// Go's AddDate normalizes overflow: Jan 31 + 1 month → March 3 (non-leap)
	// or March 2 (leap year), because Feb has fewer days. This documents the
	// actual behavior the production code relies on.
	tests := []struct {
		name    string
		current time.Time
		want    time.Time
	}{
		{"Jan 31 -> Mar 3 (non-leap overflow)", time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)},
		{"Jan 31 -> Mar 2 (leap year 2024 overflow)", time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC)},
		{"Dec 31 -> Jan 31 next year", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)},
		{"Mar 31 -> May 1 (April has 30 days)", time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"Oct 31 -> Dec 1 (November has 30 days)", time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeNextDate(tt.current, "monthly")
			if !got.Equal(tt.want) {
				t.Errorf("computeNextDate(monthly) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{"simple positive", "123", 123},
		{"zero", "0", 0},
		{"large number", "9999999999", 9999999999},
		{"negative", "-42", -42},
		{"empty string", "", 0},
		{"non-numeric", "abc", 0},
		{"leading number then text", "42abc", 42}, // fmt.Sscanf stops at first non-match
		{"text then number", "abc42", 0},
		{"whitespace", "  17  ", 17},
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

func TestNullIfEmpty(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want any
	}{
		{"empty returns nil", "", nil},
		{"non-empty returns string", "hello", "hello"},
		{"single space returns string (not nil)", " ", " "},
		{"number-like string returns string", "0", "0"},
		{"whitespace tab returns string", "\t", "\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullIfEmpty(tt.s)
			if got != tt.want {
				t.Errorf("nullIfEmpty(%q) = %v (%T), want %v (%T)", tt.s, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestNullIfZero(t *testing.T) {
	tests := []struct {
		name string
		v    int64
		want any
	}{
		{"zero returns nil", 0, nil},
		{"positive returns value", 42, int64(42)},
		{"negative returns value", -1, int64(-1)},
		{"large positive", 9223372036854775807, int64(9223372036854775807)},
		{"large negative", -9223372036854775808, int64(-9223372036854775808)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullIfZero(tt.v)
			if got != tt.want {
				t.Errorf("nullIfZero(%d) = %v (%T), want %v (%T)", tt.v, got, got, tt.want, tt.want)
			}
		})
	}
}
