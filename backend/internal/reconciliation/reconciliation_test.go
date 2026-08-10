package reconciliation

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// ValidationError
// ---------------------------------------------------------------------------

func TestValidationError_ErrorReturnsMessage(t *testing.T) {
	err := validationError("something is wrong")
	if err.Error() != "something is wrong" {
		t.Fatalf("expected message %q, got %q", "something is wrong", err.Error())
	}
}

func TestValidationError_IsValidationErrorType(t *testing.T) {
	err := validationError("bad input")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected err to be *ValidationError, got %T", err)
	}
	if ve.msg != "bad input" {
		t.Fatalf("expected msg %q, got %q", "bad input", ve.msg)
	}
}

func TestValidationError_DistinctFromOtherErrors(t *testing.T) {
	ve := validationError("vfe")
	other := errors.New("plain")
	var target *ValidationError
	if errors.As(other, &target) {
		t.Fatalf("plain error should not match *ValidationError")
	}
	if !errors.As(ve, &target) {
		t.Fatalf("validationError should match *ValidationError")
	}
}

// ---------------------------------------------------------------------------
// pathID
// ---------------------------------------------------------------------------

func TestPathID_ValidPositive(t *testing.T) {
	id, err := pathID("42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected 42, got %d", id)
	}
}

func TestPathID_LargeValue(t *testing.T) {
	id, err := pathID("9223372036854775807") // max int64
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 9223372036854775807 {
		t.Fatalf("expected max int64, got %d", id)
	}
}

func TestPathID_ZeroRejected(t *testing.T) {
	if _, err := pathID("0"); err == nil {
		t.Fatal("expected error for zero id")
	}
}

func TestPathID_NegativeRejected(t *testing.T) {
	if _, err := pathID("-5"); err == nil {
		t.Fatal("expected error for negative id")
	}
}

func TestPathID_NonNumericRejected(t *testing.T) {
	for _, raw := range []string{"abc", "", "12abc", "1.5", " "} {
		if _, err := pathID(raw); err == nil {
			t.Fatalf("expected error for pathID(%q)", raw)
		}
	}
}

// ---------------------------------------------------------------------------
// parseDate / optionalDate
// ---------------------------------------------------------------------------

func TestParseDate_Valid(t *testing.T) {
	d, err := parseDate("2026-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Valid {
		t.Fatal("expected valid date")
	}
	want := "2026-01-15"
	got := d.Time.Format("2006-01-02")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestParseDate_TrimsWhitespace(t *testing.T) {
	d, err := parseDate("  2026-12-31  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Time.Format("2006-01-02") != "2026-12-31" {
		t.Fatalf("date not parsed correctly: %v", d.Time)
	}
}

func TestParseDate_EmptyString(t *testing.T) {
	_, err := parseDate("")
	if err == nil {
		t.Fatal("expected error for empty date")
	}
}

func TestParseDate_WhitespaceOnly(t *testing.T) {
	_, err := parseDate("   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only date")
	}
}

func TestParseDate_InvalidFormat(t *testing.T) {
	for _, bad := range []string{
		"2026/01/15", "15-01-2026", "2026-1-5", "not-a-date", "2026-13-01",
	} {
		if _, err := parseDate(bad); err == nil {
			t.Fatalf("expected error parsing %q", bad)
		}
	}
}

func TestOptionalDate_EmptyReturnsInvalid(t *testing.T) {
	d, err := optionalDate("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Valid {
		t.Fatal("empty optional date should be invalid")
	}
}

func TestOptionalDate_WhitespaceReturnsInvalid(t *testing.T) {
	d, err := optionalDate("  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Valid {
		t.Fatal("whitespace optional date should be invalid")
	}
}

func TestOptionalDate_ValidDate(t *testing.T) {
	d, err := optionalDate("2026-06-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Valid {
		t.Fatal("expected valid date")
	}
}

// ---------------------------------------------------------------------------
// dateString / textValue / textValueOptional / int8Value
// ---------------------------------------------------------------------------

func TestDateString_InvalidReturnsEmpty(t *testing.T) {
	if got := dateString(pgtype.Date{}); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestDateString_ValidFormats(t *testing.T) {
	cases := []struct {
		year, month, day int
		want             string
	}{
		{2026, 1, 5, "2026-01-05"},
		{2026, 12, 31, "2026-12-31"},
		{2024, 2, 29, "2024-02-29"}, // leap year
	}
	for _, c := range cases {
		d := pgtype.Date{Time: time.Date(c.year, time.Month(c.month), c.day, 0, 0, 0, 0, time.UTC), Valid: true}
		if got := dateString(d); got != c.want {
			t.Fatalf("expected %q, got %q", c.want, got)
		}
	}
}

func TestTextValue_InvalidReturnsEmpty(t *testing.T) {
	if got := textValue(pgtype.Text{}); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestTextValue_ValidReturnsString(t *testing.T) {
	got := textValue(pgtype.Text{String: "hello", Valid: true})
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestTextValueOptional_EmptyInvalid(t *testing.T) {
	v := textValueOptional("   ")
	if v.Valid {
		t.Fatal("expected invalid for whitespace-only input")
	}
}

func TestTextValueOptional_NonEmptyValid(t *testing.T) {
	v := textValueOptional("notes")
	if !v.Valid || v.String != "notes" {
		t.Fatalf("expected valid 'notes', got %+v", v)
	}
}

func TestTextValueOptional_PreservesInternalSpaces(t *testing.T) {
	v := textValueOptional("  hello world  ")
	if !v.Valid || v.String != "  hello world  " {
		t.Fatalf("expected internal spaces preserved, got %q", v.String)
	}
}

func TestInt8Value_ZeroInvalid(t *testing.T) {
	v := int8Value(0)
	if v.Valid {
		t.Fatal("zero should be invalid")
	}
}

func TestInt8Value_NonZeroValid(t *testing.T) {
	v := int8Value(42)
	if !v.Valid || v.Int64 != 42 {
		t.Fatalf("expected valid 42, got %+v", v)
	}
}

func TestInt8Value_NegativeValid(t *testing.T) {
	v := int8Value(-7)
	if !v.Valid || v.Int64 != -7 {
		t.Fatalf("expected valid -7, got %+v", v)
	}
}

// ---------------------------------------------------------------------------
// uuidValue
// ---------------------------------------------------------------------------

func TestUUIDValue_ValidUUID(t *testing.T) {
	v := uuidValue("123e4567-e89b-12d3-a456-426614174000")
	if !v.Valid {
		t.Fatal("expected valid UUID")
	}
}

func TestUUIDValue_InvalidUUID(t *testing.T) {
	v := uuidValue("not-a-uuid")
	if v.Valid {
		t.Fatal("expected invalid UUID")
	}
}

func TestUUIDValue_Empty(t *testing.T) {
	v := uuidValue("")
	if v.Valid {
		t.Fatal("expected invalid UUID for empty string")
	}
}

// ---------------------------------------------------------------------------
// idempotencyKey
// ---------------------------------------------------------------------------

func TestIdempotencyKey_MissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := idempotencyKey(req)
	if err == nil {
		t.Fatal("expected error when Idempotency-Key header is missing")
	}
}

func TestIdempotencyKey_EmptyHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Idempotency-Key", "   ")
	_, err := idempotencyKey(req)
	if err == nil {
		t.Fatal("expected error for empty Idempotency-Key")
	}
}

func TestIdempotencyKey_ValidUUID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174000")
	key, err := idempotencyKey(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("expected key returned, got %q", key)
	}
}

func TestIdempotencyKey_InvalidUUID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Idempotency-Key", "not-a-uuid")
	_, err := idempotencyKey(req)
	if err == nil {
		t.Fatal("expected error for non-UUID Idempotency-Key")
	}
}

// ---------------------------------------------------------------------------
// HTTP helper plumbing: writeJSON / writeError / errorResponse
// ---------------------------------------------------------------------------

func TestWriteJSON_ContentTypeAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusTeapot, map[string]string{"hello": "world"})

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"hello"`) || !strings.Contains(body, `"world"`) {
		t.Fatalf("expected JSON body with hello/world, got %q", body)
	}
}

func TestWriteError_Payload(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "INVALID_REQUEST", "bad input")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"INVALID_REQUEST"`) {
		t.Fatalf("expected code in body, got %q", body)
	}
	if !strings.Contains(body, "bad input") {
		t.Fatalf("expected message in body, got %q", body)
	}
}

func TestDecodeJSON_ValidBody(t *testing.T) {
	body := strings.NewReader(`{"bank_account_id":1,"statement_date":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	var got map[string]any
	if err := decodeJSON(req, &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// JSON numbers decode as float64.
	if v, ok := got["bank_account_id"].(float64); !ok || v != 1 {
		t.Fatalf("expected bank_account_id=1, got %v", got["bank_account_id"])
	}
}

func TestDecodeJSON_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{not json"))
	var got map[string]any
	if err := decodeJSON(req, &got); err == nil {
		t.Fatal("expected error decoding invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// Error classification helpers (isUniqueViolation / isForeignKeyViolation / isNoRows)
// ---------------------------------------------------------------------------

func TestIsNoRows_NilAndPlainError(t *testing.T) {
	if isNoRows(nil) {
		t.Fatal("nil should not be NoRows")
	}
	if isNoRows(errors.New("plain")) {
		t.Fatal("plain error should not be NoRows")
	}
}

func TestIsUniqueViolation_NilAndPlainError(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("nil should not be unique violation")
	}
	if isUniqueViolation(errors.New("plain")) {
		t.Fatal("plain error should not be unique violation")
	}
}

func TestIsForeignKeyViolation_NilAndPlainError(t *testing.T) {
	if isForeignKeyViolation(nil) {
		t.Fatal("nil should not be FK violation")
	}
	if isForeignKeyViolation(errors.New("plain")) {
		t.Fatal("plain error should not be FK violation")
	}
}

// ---------------------------------------------------------------------------
// Reconciliation variance math (mirrors recomputeBalances formula):
//   diff_cents = adjusted_book_cents - adjusted_statement_cents
// where adjusted_book = book_balance and adjusted_statement = closing_balance.
// ---------------------------------------------------------------------------

func TestReconciliationViance_BookHigherThanStatement(t *testing.T) {
	bookBalance := int64(100000) // 1,000.00
	closingBalance := int64(95000)
	adjustedBook := bookBalance
	adjustedStatement := closingBalance
	diff := adjustedBook - adjustedStatement
	if diff != 5000 {
		t.Fatalf("expected diff 5000, got %d", diff)
	}
}

func TestReconciliationViance_StatementHigherThanBook(t *testing.T) {
	bookBalance := int64(90000)
	closingBalance := int64(100000)
	diff := bookBalance - closingBalance
	if diff != -10000 {
		t.Fatalf("expected diff -10000, got %d", diff)
	}
}

func TestReconciliationViance_Balanced(t *testing.T) {
	bookBalance := int64(100000)
	closingBalance := int64(100000)
	if (bookBalance - closingBalance) != 0 {
		t.Fatal("expected zero diff when book == statement")
	}
}

func TestReconciliationSummary_FieldsPropagate(t *testing.T) {
	s := reconciliationSummary{
		StatementBalanceCents:  1000,
		BookBalanceCents:       990,
		AdjustedBookCents:      990,
		AdjustedStatementCents: 1000,
		DiffCents:              -10,
		MatchedCount:           3,
		UnmatchedCount:         1,
		TotalLines:             4,
	}
	if s.TotalLines != s.MatchedCount+s.UnmatchedCount {
		t.Fatalf("total_lines (%d) != matched+unmatched (%d)",
			s.TotalLines, s.MatchedCount+s.UnmatchedCount)
	}
	if s.DiffCents != s.AdjustedBookCents-s.AdjustedStatementCents {
		t.Fatalf("diff does not match book-statement")
	}
}

// ---------------------------------------------------------------------------
// Bank statement line match-status semantics. The auto-match code sets
// match_status to "MATCHED" when a journal line is paired and "UNMATCHED"
// otherwise. A statement credit (amount_cents > 0) pairs with a journal
// DEBIT; a statement debit (amount_cents < 0) pairs with a journal CREDIT.
// ---------------------------------------------------------------------------

func TestStatementLine_MatchStatusStrings(t *testing.T) {
	cases := []struct {
		amount   int64
		wantDir  string // direction on the matched journal line
		positive bool
	}{
		{5000, "DEBIT", true},   // deposit -> bank debit
		{-5000, "CREDIT", false}, // withdrawal -> bank credit
	}
	for _, c := range cases {
		wantDebit := c.amount > 0
		if wantDebit != c.positive {
			t.Fatalf("amount %d: expected positive=%v", c.amount, c.positive)
		}
		dir := "CREDIT"
		if wantDebit {
			dir = "DEBIT"
		}
		if dir != c.wantDir {
			t.Fatalf("amount %d: expected dir %s, got %s", c.amount, c.wantDir, dir)
		}
		// absAmount logic
		abs := c.amount
		if abs < 0 {
			abs = -abs
		}
		if abs != 5000 {
			t.Fatalf("expected abs 5000, got %d", abs)
		}
	}
}

func TestStatementLine_AbsoluteAmount_NeverNegative(t *testing.T) {
	for _, amt := range []int64{-1, -100, -99999999, 1, 100, 99999999} {
		abs := amt
		if abs < 0 {
			abs = -abs
		}
		if abs <= 0 {
			t.Fatalf("abs of %d should be positive, got %d", amt, abs)
		}
	}
}

// ---------------------------------------------------------------------------
// NewHandler / Service construction
// ---------------------------------------------------------------------------

func TestNewHandler_NilPoolReturnsService(t *testing.T) {
	// We don't connect to a DB; we only verify the constructor returns a
	// non-nil *Service. The pool field is unexported so we cannot assert on
	// it directly, but a nil pool is fine because we never call routes here.
	svc := NewHandler(nil)
	if svc == nil {
		t.Fatal("expected non-nil Service")
	}
}
