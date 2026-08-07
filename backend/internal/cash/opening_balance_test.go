package cash

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Opening-balances validation (pure — no database access)
// ---------------------------------------------------------------------------

func TestValidateOpeningBalanceValid(t *testing.T) {
	req := OpeningBalanceRequest{
		SourceRef:       "OPEN-1",
		EntryDate:       "2026-08-01",
		EquityAccountID: 3101,
		Balances: []OpeningBalanceLineRequest{
			{AccountID: 1101, DebitCents: 2000000},
			{AccountID: 2101, CreditCents: 500000},
		},
		Description: "Opening balance",
	}
	if code, message := validateOpeningBalanceRequest(req); code != "" {
		t.Fatalf("expected valid opening balance, got %s: %s", code, message)
	}
}

// The seeded equity account (code 3101) is resolved server-side when the
// client omits equity_account_id, so zero must be accepted by validation.
func TestValidateOpeningBalanceDefaultsEquityAccount(t *testing.T) {
	req := OpeningBalanceRequest{
		SourceRef: "OPEN-2",
		EntryDate: "2026-08-01",
		Balances: []OpeningBalanceLineRequest{
			{AccountID: 1101, DebitCents: 1000000},
		},
	}
	if code, message := validateOpeningBalanceRequest(req); code != "" {
		t.Fatalf("expected omitted equity_account_id to be valid, got %s: %s", code, message)
	}
}

func TestValidateOpeningBalanceRejects(t *testing.T) {
	base := OpeningBalanceRequest{
		SourceRef:       "OPEN-3",
		EntryDate:       "2026-08-01",
		EquityAccountID: 3101,
		Balances: []OpeningBalanceLineRequest{
			{AccountID: 1101, DebitCents: 2000000},
		},
	}

	cases := []struct {
		name   string
		mutate func(*OpeningBalanceRequest)
	}{
		{"missing source_ref", func(r *OpeningBalanceRequest) { r.SourceRef = "" }},
		{"malformed entry_date", func(r *OpeningBalanceRequest) { r.EntryDate = "01-08-2026" }},
		{"negative equity_account_id", func(r *OpeningBalanceRequest) { r.EquityAccountID = -1 }},
		{"empty balances", func(r *OpeningBalanceRequest) { r.Balances = []OpeningBalanceLineRequest{} }},
		{"negative debit_cents", func(r *OpeningBalanceRequest) {
			r.Balances[0].DebitCents = -100
		}},
		{"negative credit_cents", func(r *OpeningBalanceRequest) {
			r.Balances[0].DebitCents = 0
			r.Balances[0].CreditCents = -100
		}},
		{"both debit and credit positive", func(r *OpeningBalanceRequest) {
			r.Balances[0].DebitCents = 100
			r.Balances[0].CreditCents = 200
		}},
		{"neither debit nor credit", func(r *OpeningBalanceRequest) {
			r.Balances[0].DebitCents = 0
			r.Balances[0].CreditCents = 0
		}},
		{"zero account id in line", func(r *OpeningBalanceRequest) {
			r.Balances[0].AccountID = 0
		}},
		{"negative account id in line", func(r *OpeningBalanceRequest) {
			r.Balances[0].AccountID = -5
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

// ---------------------------------------------------------------------------
// Handler-level (no database — validation short-circuits before pool access,
// so a nil Service is safe, matching the newTestService pattern).
// ---------------------------------------------------------------------------

func TestOpeningBalanceHandlerRejectsMissingHeaders(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"source_ref":"OPEN-1","entry_date":"2026-08-01","equity_account_id":3101,"balances":[{"account_id":1101,"debit_cents":2000000}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/opening-balances", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	service.OpeningBalance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without tenant, got %d", recorder.Code)
	}
	if code := errorCode(t, recorder); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %q", code)
	}
}

func TestOpeningBalanceHandlerRejectsMissingIdempotencyKey(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"source_ref":"OPEN-1","entry_date":"2026-08-01","balances":[{"account_id":1101,"debit_cents":2000000}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/opening-balances", body)
	request = withTenant(request, 1)
	recorder := httptest.NewRecorder()

	service.OpeningBalance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestOpeningBalanceHandlerRejectsInvalidBody(t *testing.T) {
	service := newTestService()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/opening-balances", strings.NewReader(`{invalid json`))
	request = withTenant(request, 1)
	request.Header.Set("Idempotency-Key", "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50")
	recorder := httptest.NewRecorder()

	service.OpeningBalance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

// A payload with both debit and credit on one line must be rejected by
// validation before any database access.
func TestOpeningBalanceHandlerRejectsDebitAndCreditOnSameLine(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"source_ref":"OPEN-2","entry_date":"2026-08-01","balances":[{"account_id":1101,"debit_cents":100,"credit_cents":200}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/opening-balances", body)
	request = withTenant(request, 1)
	request.Header.Set("Idempotency-Key", "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50")
	recorder := httptest.NewRecorder()

	service.OpeningBalance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if code := errorCode(t, recorder); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %q", code)
	}
}

// Missing balances must be rejected before the handler touches the database.
func TestOpeningBalanceHandlerRejectsEmptyBalances(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"source_ref":"OPEN-3","entry_date":"2026-08-01","balances":[]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/opening-balances", body)
	request = withTenant(request, 1)
	request.Header.Set("Idempotency-Key", "1d1b9a44-9c3a-4f2f-9a54-4d4f4d4e4f50")
	recorder := httptest.NewRecorder()

	service.OpeningBalance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if code := errorCode(t, recorder); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %q", code)
	}
}
