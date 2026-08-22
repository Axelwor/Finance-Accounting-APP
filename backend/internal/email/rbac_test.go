package email

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"finance-accounting-app/backend/internal/auth"
)

// withRole injects a role into the request context the same way the auth
// middleware does, so RequireRole sees it.
func withRole(request *http.Request, role string) *http.Request {
	ctx := context.WithValue(request.Context(), auth.ContextKeyRole(), role)
	return request.WithContext(ctx)
}

// F-03: email template routes are mounted inside the Owner/Admin group in
// cmd/api/main.go because bodyHTML is rendered as HTML in the UI. These
// tests pin the RBAC chain: staff/viewer (and unauthenticated-role) requests
// must be rejected by RequireRole before any handler — and therefore any
// database access — runs.
func TestEmailRoutes_AdminOnlyRBAC(t *testing.T) {
	service := &Service{}
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleOwner, auth.RoleAdmin))
		service.Routes(r)
	})

	cases := []struct {
		name   string
		role   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "staff cannot create template", role: auth.RoleStaff, method: http.MethodPost, path: "/email/templates", body: `{}`, want: http.StatusForbidden},
		{name: "staff cannot enqueue", role: auth.RoleStaff, method: http.MethodPost, path: "/email/queue", body: `{}`, want: http.StatusForbidden},
		{name: "staff cannot send queued", role: auth.RoleStaff, method: http.MethodPost, path: "/email/queue/1/send", want: http.StatusForbidden},
		{name: "staff cannot list templates", role: auth.RoleStaff, method: http.MethodGet, path: "/email/templates", want: http.StatusForbidden},
		{name: "viewer cannot create template", role: auth.RoleViewer, method: http.MethodPost, path: "/email/templates", body: `{}`, want: http.StatusForbidden},
		{name: "accountant cannot update template", role: auth.RoleAccountant, method: http.MethodPut, path: "/email/templates/1", body: `{}`, want: http.StatusForbidden},
		{name: "missing role rejected", role: "", method: http.MethodGet, path: "/email/templates", want: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			if tc.role != "" {
				req = withRole(req, tc.role)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Errorf("%s %s as %q: status = %d, want %d", tc.method, tc.path, tc.role, rr.Code, tc.want)
			}
		})
	}
}

// The Owner path must pass RequireRole (reaching the handler); we assert on
// the FORBIDDEN error shape for rejections to make sure it is RequireRole
// answering, not a panic or different middleware.
func TestEmailRoutes_ForbiddenBodyShape(t *testing.T) {
	service := &Service{}
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleOwner, auth.RoleAdmin))
		service.Routes(r)
	})

	req := withRole(httptest.NewRequest(http.MethodGet, "/email/templates", nil), auth.RoleStaff)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `"code":"FORBIDDEN"`) {
		t.Errorf("body = %s, want FORBIDDEN code", body)
	}
}
