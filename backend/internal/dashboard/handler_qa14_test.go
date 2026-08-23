package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

// QA-14 unit: an omitted/empty config must normalize to {} so the INSERT no
// longer binds []byte(nil), which overrode the column's '{}'::jsonb DEFAULT
// with NULL and turned minimal payloads into a 500 not-null violation.
func TestNormalizeWidgetConfig(t *testing.T) {
	for _, in := range []json.RawMessage{
		nil,
		json.RawMessage(""),
		json.RawMessage("   \n\t "),
	} {
		got, err := normalizeWidgetConfig(in)
		if err != nil {
			t.Fatalf("normalizeWidgetConfig(%q) unexpected error: %v", string(in), err)
		}
		if string(got) != "{}" {
			t.Errorf("normalizeWidgetConfig(%q) = %q, want {}", string(in), got)
		}
	}

	got, err := normalizeWidgetConfig(json.RawMessage(`  {"refresh":60} `))
	if err != nil {
		t.Fatalf("unexpected error for valid config: %v", err)
	}
	if !json.Valid(got) {
		t.Errorf("valid config mangled: %q", got)
	}
	if strings.TrimSpace(string(got)) != `{"refresh":60}` {
		t.Errorf("valid config = %q, want trimmed passthrough", got)
	}

	if _, err := normalizeWidgetConfig(json.RawMessage(`{not-json`)); err == nil {
		t.Error("invalid JSON must return an error, not reach the DB cast")
	}
}

// QA-14 integration: POST /dashboard/widgets WITHOUT a config field must
// answer 201 (previously 500 WIDGET_CREATE_FAILED from the NULL override)
// and persist the column default {}.
func TestAddWidget_EmptyConfigCreates201(t *testing.T) {
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

	var tenantID int64
	slug := "qa-fx5-dash"
	err = pool.QueryRow(context.Background(),
		`INSERT INTO tenants (name, slug) VALUES ('QA F5 Dashboard', $1) RETURNING id`, slug).Scan(&tenantID)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	defer func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM dashboard_widgets WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(bg, `DELETE FROM dashboard_layouts WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(bg, `DELETE FROM tenants WHERE id=$1`, tenantID)
	}()

	const userID int64 = 990001 // positive; dashboard_layouts.user_id has no FK
	handler := NewHandler(pool)
	router := chi.NewRouter()
	router.Post("/dashboard/widgets", handler.AddWidget)

	body := `{"widget_type":"kpi_cash","title":"QA minimal","position":0}` // no "config" field
	req := httptest.NewRequest(http.MethodPost, "/dashboard/widgets", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.ContextKeyUserID(), userID))
	req = req.WithContext(context.WithValue(req.Context(), auth.ContextKeyTenantID(), tenantID))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || created.ID <= 0 {
		t.Fatalf("response = %s, want created id; err=%v", rr.Body.String(), err)
	}

	var stored string
	if err := pool.QueryRow(context.Background(),
		`SELECT config_json::text FROM dashboard_widgets WHERE id=$1`, created.ID).Scan(&stored); err != nil {
		t.Fatalf("read back widget: %v", err)
	}
	if stored != "{}" {
		t.Errorf("config_json = %s, want {}", stored)
	}
}
