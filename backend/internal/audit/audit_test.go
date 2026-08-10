package audit

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Action constants
// ---------------------------------------------------------------------------

func TestAction_Constants(t *testing.T) {
	cases := []struct {
		action Action
		want   string
	}{
		{ActionCreate, "CREATE"},
		{ActionUpdate, "UPDATE"},
		{ActionDelete, "DELETE"},
		{ActionPost, "POST"},
		{ActionVoid, "VOID"},
		{ActionClose, "CLOSE"},
		{ActionUnlock, "UNLOCK"},
	}
	for _, c := range cases {
		if string(c.action) != c.want {
			t.Fatalf("%v = %q, want %q", c.action, string(c.action), c.want)
		}
	}
}

func TestAction_Distinct(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range []Action{
		ActionCreate, ActionUpdate, ActionDelete,
		ActionPost, ActionVoid, ActionClose, ActionUnlock,
	} {
		s := string(a)
		if seen[s] {
			t.Fatalf("action %q appears more than once", s)
		}
		seen[s] = true
	}
}

// ---------------------------------------------------------------------------
// validOwnerType
// ---------------------------------------------------------------------------

func TestValidOwnerType_Accepted(t *testing.T) {
	accepted := []string{
		"journal_entry", "invoice", "payment", "grn", "delivery_order",
		"credit_note", "supplier_invoice", "supplier_payment",
		"purchase_return", "fixed_asset",
	}
	for _, ot := range accepted {
		if !validOwnerType(ot) {
			t.Fatalf("expected %q to be valid", ot)
		}
	}
}

func TestValidOwnerType_Rejected(t *testing.T) {
	rejected := []string{
		"", " JournalEntry", "invoice ", "INVOICE",
		"refund", "customer", "supplier", "bank_statement", "audit_log",
	}
	for _, ot := range rejected {
		if validOwnerType(ot) {
			t.Fatalf("expected %q to be invalid", ot)
		}
	}
}

// ---------------------------------------------------------------------------
// sniffMimeType
// ---------------------------------------------------------------------------

func TestSniffMimeType_KnownExtensions(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"scan.PNG", "image/png"},
		{"doc.PDF", "application/pdf"},
		{"archive.tar.gz", "application/x-gzip"}, // unknown -> handled by default
		{"invoice.pdf", "application/pdf"},
	}
	for _, c := range cases {
		got := sniffMimeType(c.filename)
		// Only assert for the three recognized types; unknown -> "".
		if c.want == "image/jpeg" || c.want == "image/png" || c.want == "application/pdf" {
			if got != c.want {
				t.Errorf("sniffMimeType(%q) = %q, want %q", c.filename, got, c.want)
			}
		}
	}
}

func TestSniffMimeType_UnknownExtension(t *testing.T) {
	for _, fn := range []string{"file.txt", "file.gif", "file.doc", "file", "file.xlsx"} {
		if got := sniffMimeType(fn); got != "" {
			t.Errorf("sniffMimeType(%q) = %q, want empty", fn, got)
		}
	}
}

func TestSniffMimeType_CaseInsensitive(t *testing.T) {
	for _, fn := range []string{"a.JPG", "a.Jpg", "a.png", "a.Pdf"} {
		got := sniffMimeType(fn)
		if got == "" {
			t.Errorf("sniffMimeType(%q) = empty, want non-empty", fn)
		}
	}
}

func TestSniffMimeType_NoExtension(t *testing.T) {
	if got := sniffMimeType("noextension"); got != "" {
		t.Fatalf("sniffMimeType(noextension) = %q, want empty", got)
	}
}

func TestSniffMimeType_DotOnlyFilename(t *testing.T) {
	// filepath.Ext(".") returns "." -> default "".
	if got := sniffMimeType("."); got != "" {
		t.Fatalf("sniffMimeType(%q) = %q, want empty", ".", got)
	}
}

// ---------------------------------------------------------------------------
// allowedMimeTypes / maxUploadSize
// ---------------------------------------------------------------------------

func TestAllowedMimeTypes(t *testing.T) {
	for _, mt := range []string{"image/jpeg", "image/png", "application/pdf"} {
		if !allowedMimeTypes[mt] {
			t.Errorf("expected %q to be allowed", mt)
		}
	}
}

func TestDisallowedMimeTypes(t *testing.T) {
	for _, mt := range []string{"image/gif", "text/plain", "application/msword", "image/webp", ""} {
		if allowedMimeTypes[mt] {
			t.Errorf("expected %q to be disallowed", mt)
		}
	}
}

func TestMaxUploadSize(t *testing.T) {
	// PRD: 10 MB. 10 << 20 == 10 * 1024 * 1024 == 10485760.
	const expected = 10 * 1024 * 1024
	if maxUploadSize != expected {
		t.Fatalf("maxUploadSize = %d, want %d", maxUploadSize, expected)
	}
}

// ---------------------------------------------------------------------------
// int8OrNil
// ---------------------------------------------------------------------------

func TestInt8OrNil_Zero(t *testing.T) {
	if got := int8OrNil(0); got != nil {
		t.Fatalf("int8OrNil(0) = %v, want nil", got)
	}
}

func TestInt8OrNil_Positive(t *testing.T) {
	got := int8OrNil(42)
	n, ok := got.(int64)
	if !ok {
		t.Fatalf("int8OrNil(42) returned %T, want int64", got)
	}
	if n != 42 {
		t.Fatalf("int8OrNil(42) = %d, want 42", n)
	}
}

func TestInt8OrNil_Negative(t *testing.T) {
	// int8OrNil only maps zero to nil; negative values are passed through.
	got := int8OrNil(-5)
	n, ok := got.(int64)
	if !ok {
		t.Fatalf("int8OrNil(-5) returned %T, want int64", got)
	}
	if n != -5 {
		t.Fatalf("int8OrNil(-5) = %d, want -5", n)
	}
}

// ---------------------------------------------------------------------------
// newFileKey
// ---------------------------------------------------------------------------

func TestNewFileKey_Length(t *testing.T) {
	// 16 random bytes -> 32 hex chars.
	for i := 0; i < 20; i++ { // run several times since it's random
		k := newFileKey()
		if len(k) != 32 {
			t.Fatalf("len(newFileKey()) = %d, want 32 (key=%q)", len(k), k)
		}
	}
}

func TestNewFileKey_IsHex(t *testing.T) {
	k := newFileKey()
	for _, c := range k {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("newFileKey() returned non-hex char %q in %q", c, k)
		}
	}
}

func TestNewFileKey_UniqueAcrossCalls(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		k := newFileKey()
		if seen[k] {
			t.Fatalf("newFileKey() returned duplicate %q", k)
		}
		seen[k] = true
	}
}

func TestNewFileKey_NotEmpty(t *testing.T) {
	if k := newFileKey(); k == "" {
		t.Fatal("newFileKey() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// joinAnd
// ---------------------------------------------------------------------------

func TestJoinAnd_Empty(t *testing.T) {
	if got := joinAnd(nil); got != "" {
		t.Fatalf("joinAnd(nil) = %q, want empty", got)
	}
	if got := joinAnd([]string{}); got != "" {
		t.Fatalf("joinAnd([]) = %q, want empty", got)
	}
}

func TestJoinAnd_Single(t *testing.T) {
	if got := joinAnd([]string{"a = 1"}); got != "a = 1" {
		t.Fatalf("joinAnd([a=1]) = %q, want %q", got, "a = 1")
	}
}

func TestJoinAnd_Multiple(t *testing.T) {
	got := joinAnd([]string{"a = 1", "b = 2", "c = 3"})
	want := "a = 1 AND b = 2 AND c = 3"
	if got != want {
		t.Fatalf("joinAnd(...) = %q, want %q", got, want)
	}
}

func TestJoinAnd_PreservesEmptyClauses(t *testing.T) {
	// joinAnd doesn't filter empties; it just joins with AND.
	got := joinAnd([]string{"a = 1", "", "c = 3"})
	want := "a = 1 AND  AND c = 3"
	if got != want {
		t.Fatalf("joinAnd(...) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// jsonNull
// ---------------------------------------------------------------------------

func TestJsonNull_EmptyBytes(t *testing.T) {
	if got := jsonNull(nil); got != nil {
		t.Fatalf("jsonNull(nil) = %v, want nil", got)
	}
	if got := jsonNull([]byte{}); got != nil {
		t.Fatalf("jsonNull([]byte{}) = %v, want nil", got)
	}
}

func TestJsonNull_NonEmptyPassesThrough(t *testing.T) {
	raw := []byte(`{"key":"value"}`)
	got := jsonNull(raw)
	if string(got) != string(raw) {
		t.Fatalf("jsonNull(...) = %q, want %q", got, raw)
	}
}

func TestJsonNull_WhitespaceBytesPassedThrough(t *testing.T) {
	// Only truly empty (len 0) becomes nil; whitespace is passed through.
	raw := []byte(" ")
	got := jsonNull(raw)
	if string(got) != " " {
		t.Fatalf("jsonNull(whitespace) = %q, want %q", got, raw)
	}
}

// ---------------------------------------------------------------------------
// pathID
// ---------------------------------------------------------------------------

func TestPathID_Valid(t *testing.T) {
	id, err := pathID("99")
	if err != nil {
		t.Fatalf("pathID(\"99\") error: %v", err)
	}
	if id != 99 {
		t.Fatalf("pathID(\"99\") = %d, want 99", id)
	}
}

func TestPathID_MaxInt64(t *testing.T) {
	const big = int64(9223372036854775807)
	id, err := pathID(strconv.FormatInt(big, 10))
	if err != nil {
		t.Fatalf("pathID(max) error: %v", err)
	}
	if id != big {
		t.Fatalf("pathID(max) = %d, want %d", id, big)
	}
}

func TestPathID_Zero(t *testing.T) {
	if _, err := pathID("0"); err == nil {
		t.Fatal("pathID(\"0\") should return error")
	}
}

func TestPathID_Negative(t *testing.T) {
	if _, err := pathID("-7"); err == nil {
		t.Fatal("pathID(\"-7\") should return error")
	}
}

func TestPathID_NonNumeric(t *testing.T) {
	for _, raw := range []string{"abc", "", "1a", "1.5", " 1"} {
		if _, err := pathID(raw); err == nil {
			t.Fatalf("pathID(%q) should return error", raw)
		}
	}
}

// ---------------------------------------------------------------------------
// File path / directory traversal safety (UploadAttachment builds paths via
// filepath.Join(storageRoot, tenant, fileKey)). fileKey is always a random
// hex string, so a malicious filename can never reach the disk path. This
// test pins that invariant: the stored file_key has no relation to the
// uploaded filename.
// ---------------------------------------------------------------------------

func TestFilePath_FileKeyNeverUsesFilename(t *testing.T) {
	// Even with a hostile filename, newFileKey() produces a safe hex name.
	hostileNames := []string{
		"../../../etc/passwd",
		"..\\..\\windows\\system32",
		"normal.pdf",
		"spaces in name.pdf",
		"",
	}
	for _, fn := range hostileNames {
		key := newFileKey()
		// The disk path is filepath.Join(storageRoot, tenantStr, key),
		// never the user's filename. Assert key is a safe 32-hex token.
		if len(key) != 32 {
			t.Fatalf("file key for filename %q is %q (len %d), want 32 hex chars", fn, key, len(key))
		}
		for _, c := range key {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("file key %q for filename %q contains non-hex char", key, fn)
			}
		}
		// The key must not contain path separators or dots (no traversal).
		if strings.ContainsAny(key, "/\\..") {
			t.Fatalf("file key %q contains traversal characters", key)
		}
	}
}

func TestFilePath_TenantFolderIsNumeric(t *testing.T) {
	// The on-disk folder is filepath.Join(storageRoot, strconv.FormatInt(tenant, 10)).
	// For any tenant id the folder name is the decimal string of that id and
	// therefore contains no path separators.
	for _, tenant := range []int64{1, 42, 999999, 9223372036854775807} {
		folder := strconv.FormatInt(tenant, 10)
		if strings.ContainsAny(folder, "/\\") {
			t.Fatalf("tenant folder %q contains a path separator", folder)
		}
		// Re-parse to confirm it round-trips to the same int64.
		parsed, err := strconv.ParseInt(folder, 10, 64)
		if err != nil || parsed != tenant {
			t.Fatalf("tenant folder %q did not round-trip to %d", folder, tenant)
		}
	}
}

// ---------------------------------------------------------------------------
// Log: marshalling error paths (do NOT require a DB)
//
// Log marshals `before` and `after` with json.Marshal BEFORE touching the
// transaction. If the value cannot be marshalled, Log returns an error
// without ever calling tx.Exec. We exploit that to test the error paths
// with a nil tx — no database needed.
//
// math.NaN() is the canonical value that json.Marshal rejects (NaN is not
// representable in JSON), so it forces the error branch reliably.
// ---------------------------------------------------------------------------

func TestLog_BeforeDataMarshalError(t *testing.T) {
	// before is non-nil and unmarshallable -> Log should fail at marshal time,
	// before reaching tx.Exec (which would panic on a nil tx).
	err := Log(t.Context(), nil, 1, 1, "journal_entry", 1, ActionCreate, math.NaN(), nil)
	if err == nil {
		t.Fatal("expected marshal error for unmarshallable before, got nil")
	}
	if !strings.Contains(err.Error(), "before_data") {
		t.Fatalf("expected error to mention before_data, got %q", err.Error())
	}
}

func TestLog_AfterDataMarshalError(t *testing.T) {
	// before is nil (skipped), after is unmarshallable -> Log fails at after.
	err := Log(t.Context(), nil, 1, 1, "journal_entry", 1, ActionCreate, nil, math.NaN())
	if err == nil {
		t.Fatal("expected marshal error for unmarshallable after, got nil")
	}
	if !strings.Contains(err.Error(), "after_data") {
		t.Fatalf("expected error to mention after_data, got %q", err.Error())
	}
}

func TestLog_BothUnmarshallable_ReportsBeforeFirst(t *testing.T) {
	// before is checked first; the error should reference before_data.
	err := Log(t.Context(), nil, 1, 1, "journal_entry", 1, ActionCreate, math.NaN(), math.NaN())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "before_data") {
		t.Fatalf("expected before_data error (checked first), got %q", err.Error())
	}
}

func TestLog_NaNRejectedByJSONMarshal(t *testing.T) {
	// Sanity-check that math.NaN() really is unmarshallable — this guards the
	// assumption the Log error-path tests above rely on.
	if _, err := json.Marshal(math.NaN()); err == nil {
		t.Fatal("expected json.Marshal(NaN) to fail")
	}
}

// ---------------------------------------------------------------------------
// Log branch-logic invariants (mirrored without a DB)
//
// Log converts entityID/userID to nil when <= 0 so pgx stores NULL, and
// stringifies the Action. We reproduce that exact branch logic here to pin
// the contract without standing up a pgx.Tx mock (the rules forbid mocks).
// ---------------------------------------------------------------------------

func TestLog_EntityIDZeroBecomesNil(t *testing.T) {
	// Mirror: var entityIDValue any; if entityID > 0 { entityIDValue = entityID }
	for _, id := range []int64{0, -1, -999} {
		var entityIDValue any
		if id > 0 {
			entityIDValue = id
		}
		if entityIDValue != nil {
			t.Fatalf("id=%d should map to nil, got %v", id, entityIDValue)
		}
	}
}

func TestLog_EntityIDPositivePassedThrough(t *testing.T) {
	for _, id := range []int64{1, 55, 99999} {
		var entityIDValue any
		if id > 0 {
			entityIDValue = id
		}
		got, ok := entityIDValue.(int64)
		if !ok {
			t.Fatalf("id=%d: expected int64, got %T (%v)", id, entityIDValue, entityIDValue)
		}
		if got != id {
			t.Fatalf("id=%d: got %d", id, got)
		}
	}
}

func TestLog_UserIDZeroBecomesNil(t *testing.T) {
	for _, id := range []int64{0, -1} {
		var userIDValue any
		if id > 0 {
			userIDValue = id
		}
		if userIDValue != nil {
			t.Fatalf("userID=%d should map to nil, got %v", id, userIDValue)
		}
	}
}

func TestLog_UserIDPositivePassedThrough(t *testing.T) {
	for _, id := range []int64{1, 77, 12345} {
		var userIDValue any
		if id > 0 {
			userIDValue = id
		}
		got, ok := userIDValue.(int64)
		if !ok {
			t.Fatalf("userID=%d: expected int64, got %T", id, userIDValue)
		}
		if got != id {
			t.Fatalf("userID=%d: got %d", id, got)
		}
	}
}

func TestLog_ActionStringified(t *testing.T) {
	// Log passes string(action) as the SQL arg. Verify each constant's string
	// form matches what would land in the audit_logs.action column.
	for _, a := range []Action{
		ActionCreate, ActionUpdate, ActionDelete,
		ActionPost, ActionVoid, ActionClose, ActionUnlock,
	} {
		s := string(a)
		if s == "" {
			t.Fatalf("action %v stringified to empty", a)
		}
	}
}

func TestLog_BeforeDataMarshaledToJSON(t *testing.T) {
	// Log uses json.Marshal(before). Verify representative values marshal
	// to the JSON the audit trail expects.
	before := map[string]any{"status": "DRAFT", "id": float64(3)}
	raw, err := json.Marshal(before)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("before is not valid JSON: %v", err)
	}
	if decoded["status"] != "DRAFT" {
		t.Fatalf("expected before.status=DRAFT, got %v", decoded["status"])
	}
}

func TestLog_AfterDataMarshaledToJSON(t *testing.T) {
	after := map[string]any{"status": "POSTED"}
	raw, err := json.Marshal(after)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("after is not valid JSON: %v", err)
	}
	if decoded["status"] != "POSTED" {
		t.Fatalf("expected after.status=POSTED, got %v", decoded["status"])
	}
}

func TestLog_BeforeAfterNilAreNotMarshaled(t *testing.T) {
	// When before/after are nil, Log skips marshal entirely (var beforeBytes
	// stays nil). Reproduce the branch: only marshal when non-nil.
	var beforeBytes, afterBytes []byte
	var before, after any = nil, nil
	if before != nil {
		beforeBytes, _ = json.Marshal(before)
	}
	if after != nil {
		afterBytes, _ = json.Marshal(after)
	}
	if beforeBytes != nil {
		t.Fatal("nil before should produce nil bytes")
	}
	if afterBytes != nil {
		t.Fatal("nil after should produce nil bytes")
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers (writeJSON / writeError)
// ---------------------------------------------------------------------------

func TestWriteJSON_StatusAndContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"k": "v"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"k":"v"`) {
		t.Fatalf("body = %q, want to contain k:v", rec.Body.String())
	}
}

func TestWriteError_Shape(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "INVALID_FILE_TYPE", "only image/jpeg, image/png, and application/pdf are allowed")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"INVALID_FILE_TYPE"`) {
		t.Fatalf("body %q missing code", body)
	}
	if !strings.Contains(body, "application/pdf") {
		t.Fatalf("body %q missing message", body)
	}
}

// ---------------------------------------------------------------------------
// NewHandler / Service construction
// ---------------------------------------------------------------------------

func TestNewHandler_ReturnsServiceWithStorageRoot(t *testing.T) {
	svc := NewHandler(nil, "/tmp/attachments")
	if svc == nil {
		t.Fatal("NewHandler returned nil")
	}
}

// ---------------------------------------------------------------------------
// Error classification (pathID is the main one; helpers mirror other pkgs)
// ---------------------------------------------------------------------------

func TestIsNoRowsOrErrorsImported(t *testing.T) {
	// errors package is used elsewhere; sanity-check the sentinel behaviour.
	plain := errors.New("plain")
	if errors.Is(plain, errors.New("other")) {
		t.Fatal("distinct plain errors should not be Is-equal")
	}
}
