package reports

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// N7 integration: PUT/DELETE /reports/templates/{id} against an id owned by
// ANOTHER tenant (and by extension the RLS/write-protected global tenant_id=0
// rows) must answer 404 NOT_FOUND — previously both returned a false-success
// 200 because RowsAffected was never checked. Needs a live Postgres.
func TestTemplateUpdateDelete_ForeignTenantReturns404(t *testing.T) {
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	poolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	stamp := time.Now().UnixNano()
	seedTenant := func(name string) int64 {
		t.Helper()
		var id int64
		err := pool.QueryRow(context.Background(),
			`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`, name, name+"-"+fmt.Sprint(stamp)).Scan(&id)
		if err != nil {
			t.Fatalf("seed tenant %s: %v", name, err)
		}
		return id
	}
	tenantA := seedTenant("qa-fx5-tmpl-a")
	tenantB := seedTenant("qa-fx5-tmpl-b")
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM report_templates WHERE tenant_id IN ($1,$2)`, tenantA, tenantB)
		_, _ = pool.Exec(cleanup, `DELETE FROM tenants WHERE id IN ($1,$2)`, tenantA, tenantB)
	}()

	var foreignID int64
	err = db.WithTenantData(context.Background(), pool, tenantB, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `
			INSERT INTO report_templates (tenant_id, code, name, document_type, template_yaml)
			VALUES ($1, 'QA-FX5', 'QA F5 Template', 'invoice', 'title: x')
			RETURNING id
		`, tenantB).Scan(&foreignID)
	})
	if err != nil {
		t.Fatalf("seed template: %v", err)
	}

	handler := NewHandler(pool)
	router := chi.NewRouter()
	router.Put("/reports/templates/{id}", handler.UpdateTemplate)
	router.Delete("/reports/templates/{id}", handler.DeleteTemplate)

	asTenant := func(r *http.Request, tid int64) *http.Request {
		return r.WithContext(context.WithValue(r.Context(), auth.ContextKeyTenantID(), tid))
	}

	updateBody := `{"code":"HACK","name":"Hacked","document_type":"invoice","template_yaml":"evil: true"}`
	putReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/reports/templates/%d", foreignID), strings.NewReader(updateBody))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, asTenant(putReq, tenantA))
	if rr.Code != http.StatusNotFound {
		t.Errorf("PUT foreign template: status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	} else if !strings.Contains(rr.Body.String(), `"NOT_FOUND"`) {
		t.Errorf("PUT foreign template body = %s, want NOT_FOUND", rr.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/reports/templates/%d", foreignID), nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, asTenant(delReq, tenantA))
	if rr.Code != http.StatusNotFound {
		t.Errorf("DELETE foreign template: status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}

	// The foreign row must be untouched after both attempts.
	var stillThere bool
	err = db.WithTenantData(context.Background(), pool, tenantB, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM report_templates WHERE id=$1 AND code='QA-FX5')`, foreignID).Scan(&stillThere)
	})
	if err != nil || !stillThere {
		t.Errorf("foreign template damaged by cross-tenant attempts (exists=%v, err=%v)", stillThere, err)
	}

	// Sanity: updating/deleting your OWN template still works (200).
	ownReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/reports/templates/%d", foreignID), strings.NewReader(updateBody))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, asTenant(ownReq, tenantB))
	if rr.Code != http.StatusOK {
		t.Errorf("PUT own template: status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	delOwn := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/reports/templates/%d", foreignID), nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, asTenant(delOwn, tenantB))
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE own template: status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}
