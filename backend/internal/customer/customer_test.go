package customer

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

// withTenant injects the tenant id into the request context the same way the
// auth middleware does, so handlers read it from JWT claims context.
func withTenant(request *http.Request, tenantID int64) *http.Request {
	ctx := context.WithValue(request.Context(), auth.ContextKeyTenantID(), tenantID)
	return request.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// Pure validation unit tests (no database)
// ---------------------------------------------------------------------------

func TestValidateCreateCustomer(t *testing.T) {
	base := CreateCustomerRequest{
		Code: "CUST-001",
		Name: "PT Contoh",
	}
	if code, message := validateCreateCustomer(base); code != "" {
		t.Fatalf("expected valid request, got %s: %s", code, message)
	}

	tests := []struct {
		name string
		req  CreateCustomerRequest
	}{
		{"missing code", CreateCustomerRequest{Name: "PT Contoh"}},
		{"whitespace code", CreateCustomerRequest{Code: "   ", Name: "PT Contoh"}},
		{"missing name", CreateCustomerRequest{Code: "CUST-001"}},
		{"whitespace name", CreateCustomerRequest{Code: "CUST-001", Name: "\t"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _ := validateCreateCustomer(tt.req); code != "INVALID_REQUEST" {
				t.Fatalf("expected INVALID_REQUEST, got %q", code)
			}
		})
	}
}

func TestValidateCreatePaymentTerm(t *testing.T) {
	base := CreatePaymentTermRequest{
		Code: "NET-30",
		Name: "Net 30",
	}
	if code, message := validateCreatePaymentTerm(base); code != "" {
		t.Fatalf("expected valid request, got %s: %s", code, message)
	}

	tests := []struct {
		name string
		req  CreatePaymentTermRequest
	}{
		{"missing code", CreatePaymentTermRequest{Name: "Net 30"}},
		{"missing name", CreatePaymentTermRequest{Code: "NET-30"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _ := validateCreatePaymentTerm(tt.req); code != "INVALID_REQUEST" {
				t.Fatalf("expected INVALID_REQUEST, got %q", code)
			}
		})
	}
}

func TestValidateCreateCustomerAllowsEmptyReferences(t *testing.T) {
	req := CreateCustomerRequest{
		Code: "CUST-002",
		Name: "Toko Maju",
	}
	// References are optional; omitting them must pass pure validation.
	if code, _ := validateCreateCustomer(req); code != "" {
		t.Fatalf("expected valid request with empty references, got %q", code)
	}
}

// ---------------------------------------------------------------------------
// Handler-level validation tests (no database — validation short-circuits
// before any pool access, so a nil Service is safe here).
// ---------------------------------------------------------------------------

func newTestService() *Service {
	return &Service{}
}

func TestTenantIDValidation(t *testing.T) {
	service := newTestService()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
	recorder := httptest.NewRecorder()

	service.ListCustomers(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without tenant, got %d", recorder.Code)
	}
	if code := errorCode(t, recorder); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %q", code)
	}
}

func TestCreateCustomerHandlesInvalidPathTenant(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"code":"CUST-1","name":"PT A"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/customers", body)
	request = withTenant(request, 0)
	recorder := httptest.NewRecorder()

	service.CreateCustomer(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-positive tenant, got %d", recorder.Code)
	}
}

func TestCreateCustomerRejectsInvalidJSON(t *testing.T) {
	service := newTestService()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/customers", strings.NewReader(`{invalid`))
	request = withTenant(request, 1)
	recorder := httptest.NewRecorder()

	service.CreateCustomer(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestCreateCustomerRejectsMissingCode(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"name":"PT A"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/customers", body)
	request = withTenant(request, 1)
	recorder := httptest.NewRecorder()

	service.CreateCustomer(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if code := errorCode(t, recorder); code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %q", code)
	}
}

func TestCreateCustomerRejectsMissingName(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"code":"CUST-1"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/customers", body)
	request = withTenant(request, 1)
	recorder := httptest.NewRecorder()

	service.CreateCustomer(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestCreatePaymentTermRejectsMissingCode(t *testing.T) {
	service := newTestService()
	body := strings.NewReader(`{"name":"Net 30"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payment-terms", body)
	request = withTenant(request, 1)
	recorder := httptest.NewRecorder()

	service.CreatePaymentTerm(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestGetCustomerRejectsInvalidPathID(t *testing.T) {
	service := newTestService()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/customers/abc", nil)
	request = withTenant(request, 1)
	router := chi.NewRouter()
	router.Get("/api/v1/customers/{id}", service.GetCustomer)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid path id, got %d", recorder.Code)
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
		"GET /api/v1/customers":                  false,
		"POST /api/v1/customers":                 false,
		"GET /api/v1/customers/{id}":             false,
		"POST /api/v1/customers/{id}/deactivate": false,
		"GET /api/v1/payment-terms":              false,
		"POST /api/v1/payment-terms":             false,
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
