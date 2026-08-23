package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// QA-18: /auth/logout is mounted behind the auth middleware (see
// cmd/api/main.go), so a missing or garbage bearer token must be rejected
// with 401 by the middleware instead of the handler answering a
// false-success 200 while revoking nothing. A valid bearer with an empty or
// malformed body answers 400 INVALID_REQUEST — both paths run before any DB
// access, so this test uses a nil pool.
func TestLogoutRoute_AuthRequired(t *testing.T) {
	service := NewService(nil, "test-secret")
	router := chi.NewRouter()
	router.Group(func(gr chi.Router) {
		gr.Use(service.Middleware)
		gr.Post("/auth/logout", service.Logout)
	})

	post := func(body, authz string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", strings.NewReader(body))
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	if rr := post(`{"refresh_token":"x"}`, ""); rr.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", rr.Code)
	}
	if rr := post(`{"refresh_token":"x"}`, "Bearer garbage.token.here"); rr.Code != http.StatusUnauthorized {
		t.Errorf("garbage token: status = %d, want 401", rr.Code)
	}
	if rr := post(`{"refresh_token":"x"}`, "Basic dXNlcjpwYXNz"); rr.Code != http.StatusUnauthorized {
		t.Errorf("non-bearer scheme: status = %d, want 401", rr.Code)
	}

	token, err := service.issueToken(1, 2, RoleOwner, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	bearer := "Bearer " + token

	if rr := post(`not-json`, bearer); rr.Code != http.StatusBadRequest {
		t.Errorf("valid bearer + malformed body: status = %d, want 400", rr.Code)
	}
	rr := post(`{"refresh_token":""}`, bearer)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("valid bearer + empty refresh_token: status = %d, want 400", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"INVALID_REQUEST"`) {
		t.Errorf("body = %s, want INVALID_REQUEST code", body)
	}
}
