package cash

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"finance-accounting-app/backend/internal/auth"
)

// ---------------------------------------------------------------------------
// Pure validation unit tests (no database)
// ---------------------------------------------------------------------------

func TestValidateCashRequest(t *testing.T) {
	base := CashRequest{
		SourceRef:        "BK-1",
		EntryDate:        "2026-08-06",
		CashAccountID:    1101,
		CounterAccountID: 4101,
		AmountCents:      500000,
	}

	valid := base
	if code, message := validateCashRequest(valid); code != "" {
		t.Fatalf("expected valid request, got %s: %s", code, message)
	}

	cases := []struct {
		name   string
		mutate func(*CashRequest)
	}{
		{"missing source_ref", func(r *CashRequest) { r.SourceRef = "" }},
		{"missing entry_date", func(r *CashRequest) { r.EntryDate = "" }},
		{"malformed entry_date", func(r *CashRequest) { r.EntryDate = "06-08-2026" }},
		{"zero cash account", func(r *CashRequest) { r.CashAccountID = 0 }},
		{"zero counter account", func(r *CashRequest) { r.CounterAccountID = 0 }},
		{"zero amount", func(r *CashRequest) { r.AmountCents = 0 }},
		{"negative amount", func(r *CashRequest) { r.AmountCents = -100 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			if code, _ := validateCashRequest(req); code != "INVALID_REQUEST" {
				t.Fatalf("expected INVALID_REQUEST, got %q", code)
			}
		})
	}
}

func TestValidateTransferRequest(t *testing.T) {
	base := TransferRequest{
		SourceRef:   "TRF-1",
		EntryDate:   "2026-08-06",
		FromAccount: 1101,
		ToAccount:   1102,
		AmountCents: 100000,
	}
	if code, _ := validateTransferRequest(base); code != "" {
		t.Fatalf("expected valid transfer, got %q", code)
	}
	if code, _ := validateTransferRequest(TransferRequest{
		SourceRef:   "",
		EntryDate:   "2026-08-06",
		FromAccount: 1101,
		ToAccount:   1102,
		AmountCents: 100000,
	}); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST for missing source_ref")
	}
	if code, _ := validateTransferRequest(TransferRequest{
		SourceRef:   "TRF-2",
		EntryDate:   "not-a-date",
		FromAccount: 1101,
		ToAccount:   1102,
		AmountCents: 100000,
	}); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST for malformed date")
	}
}

func TestValidateOpeningBalanceRequest(t *testing.T) {
	base := OpeningBalanceRequest{
		SourceRef:       "OPEN-1",
		EntryDate:       "2026-08-01",
		EquityAccountID: 3101,
		Balances: []OpeningBalanceLineRequest{
			{AccountID: 1101, DebitCents: 2000000},
			{AccountID: 2101, CreditCents: 500000},
		},
	}
	if code, _ := validateOpeningBalanceRequest(base); code != "" {
		t.Fatalf("expected valid opening balance, got %q", code)
	}

	cases := []struct {
		name   string
		mutate func(*OpeningBalanceRequest)
	}{
		{"missing source_ref", func(r *OpeningBalanceRequest) { r.SourceRef = "" }},
		{"no balances", func(r *OpeningBalanceRequest) { r.Balances = nil }},
		{"zero equity account", func(r *OpeningBalanceRequest) { r.EquityAccountID = 0 }},
		{"zero account id in line", func(r *OpeningBalanceRequest) { r.Balances[0].AccountID = 0 }},
		{"debit and credit on same line", func(r *OpeningBalanceRequest) {
			r.Balances[0].CreditCents = 100
		}},
		{"zero value line", func(r *OpeningBalanceRequest) {
			r.Balances = append(r.Balances, OpeningBalanceLineRequest{AccountID: 999})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			if code, _ := validateOpeningBalanceRequest(req); code != "INVALID_REQUEST" {
				t.Fatalf("expected INVALID_REQUEST, got %q", code)
			}
		})
	}
}

func TestValidateReverseRequest(t *testing.T) {
	valid := ReverseRequest{SourceRef: "REV-1", EntryDate: "2026-08-07"}
	if code, _ := validateReverseRequest(valid); code != "" {
		t.Fatalf("expected valid reverse, got %q", code)
	}
	if code, _ := validateReverseRequest(ReverseRequest{SourceRef: "", EntryDate: "2026-08-07"}); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST for missing source_ref")
	}
	if code, _ := validateReverseRequest(ReverseRequest{SourceRef: "REV-2", EntryDate: ""}); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST for missing entry_date")
	}
}

func TestEntryDateNormalizes(t *testing.T) {
	got, err := entryDate(" 2026-08-06 ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-08-06" {
		t.Fatalf("expected normalized date, got %q", got)
	}
	if _, err := entryDate("2026/08/06"); err == nil {
		t.Fatal("expected error for non-ISO date")
	}
}

func TestIdempotencyKeyValidation(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cash-in", nil)
	if _, err := idempotencyKey(request); err == nil {
		t.Fatal("expected error when header is missing")
	}
	request.Header.Set("Idempotency-Key", "not-a-uuid")
	if _, err := idempotencyKey(request); err == nil {
		t.Fatal("expected error for invalid UUID")
	}
	request.Header.Set("Idempotency-Key", "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50")
	key, err := idempotencyKey(request)
	if err != nil {
		t.Fatal(err)
	}
	if key != "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50" {
		t.Fatalf("unexpected key %q", key)
	}
}

// withTenant injects the tenant id into the request context the same way the
// auth middleware does, so handlers read it from JWT claims context.
func withTenant(request *http.Request, tenantID int64) *http.Request {
	ctx := context.WithValue(request.Context(), auth.ContextKeyTenantID(), tenantID)
	return request.WithContext(ctx)
}

func TestTenantIDValidation(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cash-in", nil)
	if _, err := tenantID(request); err == nil {
		t.Fatal("expected error when tenant context is missing")
	}
	request = withTenant(request, 0)
	if _, err := tenantID(request); err == nil {
		t.Fatal("expected error for non-positive tenant")
	}
	request = withTenant(request, 7)
	id, err := tenantID(request)
	if err != nil {
		t.Fatal(err)
	}
	if id != 7 {
		t.Fatalf("unexpected tenant %d", id)
	}
}

func TestPathIDValidation(t *testing.T) {
	if _, err := pathID("abc"); err == nil {
		t.Fatal("expected error for non-numeric id")
	}
	if _, err := pathID("0"); err == nil {
		t.Fatal("expected error for zero id")
	}
	id, err := pathID("42")
	if err != nil || id != 42 {
		t.Fatalf("expected 42, got %d (err %v)", id, err)
	}
}

// ---------------------------------------------------------------------------
// Handler-level validation tests (no database — validations short-circuit
// before any pool access, so a nil Service is safe here).
// ---------------------------------------------------------------------------

func newTestService() *Service {
	// No pool is needed: every test below fails validation before touching
	// the database.
	return &Service{}
}

func TestCashInHandlerRejectsMissingHeaders(t *testing.T) {
	service := newTestService()

	body := strings.NewReader(`{"source_ref":"BK-1","entry_date":"2026-08-06","cash_account_id":1101,"counter_account_id":4101,"amount_cents":500000}`)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/cash-in", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	service.CashIn(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without headers, got %d", recorder.Code)
	}
	if code := errorCode(t, recorder); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %q", code)
	}
}

func TestCashInHandlerRejectsMissingIdempotencyKey(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"source_ref":"BK-1","entry_date":"2026-08-06","cash_account_id":1101,"counter_account_id":4101,"amount_cents":500000}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cash-in", body)
	request = withTenant(request, 1)
	recorder := httptest.NewRecorder()

	service.CashIn(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestCashInHandlerRejectsInvalidBody(t *testing.T) {
	service := newTestService()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cash-in", strings.NewReader(`{invalid json`))
	request = withTenant(request, 1)
	request.Header.Set("Idempotency-Key", "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50")
	recorder := httptest.NewRecorder()

	service.CashIn(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestCashInHandlerRejectsInvalidPayload(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"source_ref":"","entry_date":"2026-08-06","cash_account_id":1101,"counter_account_id":4101,"amount_cents":500000}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cash-in", body)
	request = withTenant(request, 1)
	request.Header.Set("Idempotency-Key", "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50")
	recorder := httptest.NewRecorder()

	service.CashIn(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if code := errorCode(t, recorder); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %q", code)
	}
}

func TestReverseHandlerRejectsInvalidPathID(t *testing.T) {
	service := newTestService()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/journal-entries/abc/reverse", strings.NewReader(`{"source_ref":"REV-1","entry_date":"2026-08-07"}`))
	request = withTenant(request, 1)
	request.Header.Set("Idempotency-Key", "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50")
	recorder := httptest.NewRecorder()

	service.Reverse(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("cannot parse error body: %v", err)
	}
	return payload.Code
}

// Ensure the exported handler set compiles and matches the required routes.
func TestRoutesRegistered(t *testing.T) {
	service := newTestService()
	router := chi.NewRouter()
	router.Route("/api/v1", service.Routes)

	var routes []string
	_ = chi.Walk(router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	})

	expected := map[string]bool{
		"POST /api/v1/cash-in":                      false,
		"POST /api/v1/cash-out":                     false,
		"POST /api/v1/transfers":                    false,
		"POST /api/v1/opening-balances":             false,
		"POST /api/v1/journal-entries/{id}/reverse": false,
	}
	for _, route := range routes {
		if _, ok := expected[route]; ok {
			expected[route] = true
		}
	}
	for route, found := range expected {
		if !found {
			t.Errorf("expected route %s to be registered", route)
		}
	}
}
