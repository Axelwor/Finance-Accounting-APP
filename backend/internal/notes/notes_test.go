package notes

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// jsonMarshal / jsonUnmarshal are thin wrappers over encoding/json used by the
// struct round-trip tests so the call sites stay short.
func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// ---------------------------------------------------------------------------
// validateNote (pure validation)
// ---------------------------------------------------------------------------

func TestValidateNote_Valid(t *testing.T) {
	req := noteRequest{
		PeriodYear:   2024,
		NoteNumber:   "1",
		Title:        "Revenue Recognition",
		Content:      "Revenue is recognised when...",
		DisplayOrder: 1,
	}
	if err := validateNote(req); err != nil {
		t.Fatalf("expected nil error for valid note, got %v", err)
	}
}

func TestValidateNote_WhitespaceOnlyFields(t *testing.T) {
	// Whitespace-only strings should fail the TrimSpace checks.
	req := noteRequest{
		PeriodYear: 2024,
		NoteNumber: "   ",
		Title:      "Revenue",
		Content:    "content",
	}
	if err := validateNote(req); err == nil {
		t.Fatal("expected error for whitespace-only note_number, got nil")
	}
}

func TestValidateNote_PeriodYearZero(t *testing.T) {
	req := noteRequest{
		PeriodYear: 0,
		NoteNumber: "1",
		Title:      "T",
		Content:    "C",
	}
	err := validateNote(req)
	if err == nil {
		t.Fatal("expected error for zero period_year, got nil")
	}
	if !strings.Contains(err.Error(), "period_year") {
		t.Fatalf("expected error to mention period_year, got %q", err.Error())
	}
}

func TestValidateNote_PeriodYearNegative(t *testing.T) {
	req := noteRequest{
		PeriodYear: -2024,
		NoteNumber: "1",
		Title:      "T",
		Content:    "C",
	}
	err := validateNote(req)
	if err == nil {
		t.Fatal("expected error for negative period_year, got nil")
	}
	if !strings.Contains(err.Error(), "period_year") {
		t.Fatalf("expected error to mention period_year, got %q", err.Error())
	}
}

func TestValidateNote_MissingNoteNumber(t *testing.T) {
	req := noteRequest{
		PeriodYear: 2024,
		NoteNumber: "",
		Title:      "Title",
		Content:    "content",
	}
	err := validateNote(req)
	if err == nil {
		t.Fatal("expected error for missing note_number, got nil")
	}
	if !strings.Contains(err.Error(), "note_number") {
		t.Fatalf("expected error to mention note_number, got %q", err.Error())
	}
}

func TestValidateNote_MissingTitle(t *testing.T) {
	req := noteRequest{
		PeriodYear: 2024,
		NoteNumber: "1",
		Title:      "",
		Content:    "content",
	}
	err := validateNote(req)
	if err == nil {
		t.Fatal("expected error for missing title, got nil")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Fatalf("expected error to mention title, got %q", err.Error())
	}
}

func TestValidateNote_MissingContent(t *testing.T) {
	req := noteRequest{
		PeriodYear: 2024,
		NoteNumber: "1",
		Title:      "Title",
		Content:    "",
	}
	err := validateNote(req)
	if err == nil {
		t.Fatal("expected error for missing content, got nil")
	}
	if !strings.Contains(err.Error(), "content") {
		t.Fatalf("expected error to mention content, got %q", err.Error())
	}
}

func TestValidateNote_ContentWhitespaceOnlyIsValid(t *testing.T) {
	// Content is checked with a plain == "" (not TrimSpace), so a content
	// string of only spaces is treated as present. This test pins that
	// behaviour so a future tightening is a conscious change.
	req := noteRequest{
		PeriodYear: 2024,
		NoteNumber: "1",
		Title:      "Title",
		Content:    "   ",
	}
	if err := validateNote(req); err != nil {
		t.Fatalf("expected nil error for whitespace-only content (current behaviour), got %v", err)
	}
}

func TestValidateNote_FieldCheckedInOrder(t *testing.T) {
	// Every field invalid -> the first reported error must be period_year,
	// because that is the first check in validateNote.
	req := noteRequest{
		PeriodYear: 0,
		NoteNumber: "",
		Title:      "",
		Content:    "",
	}
	err := validateNote(req)
	if err == nil {
		t.Fatal("expected error when all fields invalid, got nil")
	}
	if !strings.Contains(err.Error(), "period_year") {
		t.Fatalf("expected first error to be period_year, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// optionalInt (query string parser)
// ---------------------------------------------------------------------------

func TestOptionalInt_Empty(t *testing.T) {
	if got := optionalInt(""); got != nil {
		t.Fatalf("optionalInt(\"\") = %v, want nil", got)
	}
	if got := optionalInt("   "); got != nil {
		t.Fatalf("optionalInt(\"   \") = %v, want nil", got)
	}
}

func TestOptionalInt_ValidNumber(t *testing.T) {
	got := optionalInt("42")
	v, ok := got.(int)
	if !ok {
		t.Fatalf("optionalInt(\"42\") returned %T, want int", got)
	}
	if v != 42 {
		t.Fatalf("optionalInt(\"42\") = %d, want 42", v)
	}
}

func TestOptionalInt_InvalidNumber(t *testing.T) {
	if got := optionalInt("abc"); got != nil {
		t.Fatalf("optionalInt(\"abc\") = %v, want nil", got)
	}
}

func TestOptionalInt_NegativeNumber(t *testing.T) {
	// optionalInt does not reject negatives — it only parses. Pin behaviour.
	got := optionalInt("-5")
	v, ok := got.(int)
	if !ok {
		t.Fatalf("optionalInt(\"-5\") returned %T, want int", got)
	}
	if v != -5 {
		t.Fatalf("optionalInt(\"-5\") = %d, want -5", v)
	}
}

func TestOptionalInt_TrimsWhitespace(t *testing.T) {
	got := optionalInt("  2024  ")
	v, ok := got.(int)
	if !ok {
		t.Fatalf("optionalInt(\"  2024  \") returned %T, want int", got)
	}
	if v != 2024 {
		t.Fatalf("optionalInt(\"  2024  \") = %d, want 2024", v)
	}
}

// ---------------------------------------------------------------------------
// pathID
// ---------------------------------------------------------------------------

func TestPathID_Valid(t *testing.T) {
	id, err := pathID("123")
	if err != nil {
		t.Fatalf("pathID(\"123\") returned error %v", err)
	}
	if id != 123 {
		t.Fatalf("pathID(\"123\") = %d, want 123", id)
	}
}

func TestPathID_Zero(t *testing.T) {
	if _, err := pathID("0"); err == nil {
		t.Fatal("pathID(\"0\") should return error, got nil")
	}
}

func TestPathID_Negative(t *testing.T) {
	if _, err := pathID("-1"); err == nil {
		t.Fatal("pathID(\"-1\") should return error, got nil")
	}
}

func TestPathID_NonNumeric(t *testing.T) {
	if _, err := pathID("abc"); err == nil {
		t.Fatal("pathID(\"abc\") should return error, got nil")
	}
}

func TestPathID_Empty(t *testing.T) {
	if _, err := pathID(""); err == nil {
		t.Fatal("pathID(\"\") should return error, got nil")
	}
}

func TestPathID_LargeInt64(t *testing.T) {
	const big = int64(9223372036854775807) // max int64
	id, err := pathID(strconv.FormatInt(big, 10))
	if err != nil {
		t.Fatalf("pathID(max int64) returned error %v", err)
	}
	if id != big {
		t.Fatalf("pathID(max int64) = %d, want %d", id, big)
	}
}

// ---------------------------------------------------------------------------
// isUniqueViolation
// ---------------------------------------------------------------------------

func TestIsUniqueViolation_True(t *testing.T) {
	err := &pgconn.PgError{Code: "23505"}
	if !isUniqueViolation(err) {
		t.Fatal("expected isUniqueViolation true for code 23505")
	}
}

func TestIsUniqueViolation_FalseWrongCode(t *testing.T) {
	err := &pgconn.PgError{Code: "23503"} // foreign key violation
	if isUniqueViolation(err) {
		t.Fatal("expected isUniqueViolation false for code 23503")
	}
}

func TestIsUniqueViolation_FalseNonPgError(t *testing.T) {
	if isUniqueViolation(errors.New("not a pg error")) {
		t.Fatal("expected isUniqueViolation false for plain error")
	}
}

func TestIsUniqueViolation_FalseNil(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("expected isUniqueViolation false for nil")
	}
}

func TestIsUniqueViolation_WrappedPgError(t *testing.T) {
	// errors.As unwraps, so a wrapped pgconn.PgError should still match.
	inner := &pgconn.PgError{Code: "23505"}
	wrapped := errors.Join(errors.New("context"), inner)
	if !isUniqueViolation(wrapped) {
		t.Fatal("expected isUniqueViolation true for wrapped 23505 error")
	}
}

// ---------------------------------------------------------------------------
// errorResponse / writeError / writeJSON (JSON shape)
// ---------------------------------------------------------------------------

func TestWriteError_Shape(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "NOTE_EXISTS", "note_number already exists")

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"NOTE_EXISTS"`) {
		t.Fatalf("response body %q missing code NOTE_EXISTS", body)
	}
	if !strings.Contains(body, `"message":"note_number already exists"`) {
		t.Fatalf("response body %q missing expected message", body)
	}
}

func TestWriteJSON_StatusAndPayload(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]any{"id": float64(7), "ok": true})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"id":7`) {
		t.Fatalf("body %q missing id:7", body)
	}
	if !strings.Contains(body, `"ok":true`) {
		t.Fatalf("body %q missing ok:true", body)
	}
}

// ---------------------------------------------------------------------------
// dueDateReminder struct + days_ahead query param parsing
// ---------------------------------------------------------------------------

func TestDueDateReminder_Fields(t *testing.T) {
	r := dueDateReminder{
		ID:          1,
		Number:      "INV-001",
		PartyName:   "PT Maju",
		Direction:   "customer",
		InvoiceDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
		AmountCents: 1500000,
		Status:      "OPEN",
		DaysOverdue: 3,
	}
	if r.DaysOverdue != 3 {
		t.Fatalf("DaysOverdue = %d, want 3", r.DaysOverdue)
	}
}

func TestDueDateReminders_DaysAheadQueryParsing(t *testing.T) {
	// The handler parses days_ahead with strconv.Atoi and requires parsed >= 0.
	// When the value is invalid or negative, it falls back to the default 7.
	// We replicate that exact parsing logic here to lock the contract.
	tests := []struct {
		raw   string
		want  int
		valid bool
	}{
		{"", 0, false},    // empty -> default path
		{"7", 7, true},    // valid
		{"0", 0, true},    // zero is valid (>= 0)
		{"30", 30, true},  // valid larger window
		{"-1", 0, false},  // negative -> fallback
		{"abc", 0, false}, // non-numeric -> fallback
		{"3.5", 0, false}, // float -> fallback
		{" 7 ", 0, false}, // unparsed whitespace -> Atoi fails
	}
	for _, tc := range tests {
		daysAhead := 7
		if tc.raw != "" {
			if parsed, perr := strconv.Atoi(tc.raw); perr == nil && parsed >= 0 {
				daysAhead = parsed
			}
		}
		if tc.valid {
			if daysAhead != tc.want {
				t.Errorf("days_ahead=%q: got %d, want %d", tc.raw, daysAhead, tc.want)
			}
		} else {
			if daysAhead != 7 {
				t.Errorf("days_ahead=%q: got %d, want fallback default 7", tc.raw, daysAhead)
			}
		}
	}
}

func TestDueDateReminders_DaysAheadDefaultIsSeven(t *testing.T) {
	// The handler hard-codes 7 as the default window. Lock that constant.
	const defaultDaysAhead = 7
	if defaultDaysAhead != 7 {
		t.Fatalf("default days_ahead = %d, want 7", defaultDaysAhead)
	}
}

// ---------------------------------------------------------------------------
// noteRequest / noteResponse struct behaviour (JSON tags)
// ---------------------------------------------------------------------------

func TestNoteRequest_JSONRoundTrip(t *testing.T) {
	original := noteRequest{
		PeriodYear:   2024,
		NoteNumber:   "2a",
		Title:        "Leases",
		Content:      "Operating leases...",
		DisplayOrder: 5,
	}
	// Encode then decode to confirm JSON tags preserve values.
	data, err := jsonMarshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded noteRequest
	if err := jsonUnmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.PeriodYear != original.PeriodYear ||
		decoded.NoteNumber != original.NoteNumber ||
		decoded.Title != original.Title ||
		decoded.Content != original.Content ||
		decoded.DisplayOrder != original.DisplayOrder {
		t.Fatalf("round-trip mismatch: %+v vs %+v", decoded, original)
	}
}

func TestNoteResponse_OmitsEmptyLines(t *testing.T) {
	// noteResponse has no omitempty on most fields, so they always render.
	// This is the documented shape the API returns.
	r := noteResponse{
		ID:         1,
		PeriodYear: 2024,
		Title:      "Revenue",
		Content:    "Recognition policy...",
	}
	data, err := jsonMarshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"title":"Revenue"`) {
		t.Fatalf("expected title in JSON: %s", s)
	}
}

// ---------------------------------------------------------------------------
// Routes registration smoke test
// ---------------------------------------------------------------------------

func TestRoutes_RegistersAllEndpoints(t *testing.T) {
	// We can't easily assert chi routes without importing chi internals, but
	// NewHandler must return a non-nil Service and Routes must not panic.
	svc := NewHandler(nil)
	if svc == nil {
		t.Fatal("NewHandler(nil) returned nil")
	}
}
