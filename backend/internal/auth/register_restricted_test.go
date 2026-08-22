package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// QA-01 regression: tenant registration must work under a RESTRICTED
// (NOSUPERUSER NOBYPASSRLS) application role. SeedDefaultCOA now sets
// app.tenant_id inside its transaction; without that the RLS WITH CHECK
// policies reject every seeded row ("could not provision chart of accounts").
//
// The test provisions a local shadow role (same shape as the planned prod
// `finance_app` cutover role), connects as it, and runs the full register
// flow. Skipped unless TEST_DATABASE_URL is set.
// ---------------------------------------------------------------------------

const (
	shadowRoleName     = "finance_app_test"
	shadowRolePassword = "shadow-test-only-NOT-prod"
)

// setupRestrictedRole ensures the NOSUPERUSER shadow role exists and has the
// standard C2 grant set, then returns a DATABASE_URL connecting as it.
func setupRestrictedRole(t *testing.T, ctx context.Context, adminURL string) string {
	t.Helper()
	adminCfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	statements := []string{
		fmt.Sprintf(`DO $$ BEGIN
			CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`, shadowRoleName, shadowRolePassword),
		fmt.Sprintf(`GRANT CONNECT ON DATABASE %s TO %s`, dbNameFromURL(t, adminURL), shadowRoleName),
		fmt.Sprintf(`GRANT USAGE ON SCHEMA public TO %s`, shadowRoleName),
		fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s`, shadowRoleName),
		fmt.Sprintf(`GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO %s`, shadowRoleName),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s`, shadowRoleName),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO %s`, shadowRoleName),
	}
	for _, stmt := range statements {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("grant %q: %v", strings.Fields(stmt)[1], err)
		}
	}

	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(shadowRoleName, shadowRolePassword)
	return u.String()
}

// dbNameFromURL extracts the database name from a postgres URL for GRANT
// statements (which require a plain identifier, not an expression).
func dbNameFromURL(t *testing.T, adminURL string) string {
	t.Helper()
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimPrefix(u.Path, "/")
}

func TestRegister_UnderRestrictedRole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	restrictedURL := setupRestrictedRole(t, ctx, adminURL)

	cfg, err := pgxpool.ParseConfig(restrictedURL)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect as %s: %v", shadowRoleName, err)
	}
	defer pool.Close()

	// Sanity: we really are the restricted role.
	var who string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&who); err != nil {
		t.Fatal(err)
	}
	if who != shadowRoleName {
		t.Fatalf("connected as %q, want %q", who, shadowRoleName)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := "restricted-register-" + suffix + "@test.local"
	tenantName := "Restricted Register " + suffix

	service := NewService(pool, "test-secret-that-is-long-enough-32ch")
	body, _ := json.Marshal(RegisterRequest{
		Email:      email,
		Password:   "OwnerPass!2026",
		FullName:   "Restricted Register Test",
		TenantName: tenantName,
	})
	rr := httptest.NewRecorder()
	service.Register(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("register = %d, want 201 (%s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		TenantID int64  `json:"tenant_id"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.TenantID <= 0 || resp.Role != RoleOwner {
		t.Errorf("tenant_id=%d role=%q, want positive id + owner", resp.TenantID, resp.Role)
	}

	// The seed must have provisioned the core COA rows visible under the new
	// tenant scope — count them through a fresh restricted-role transaction.
	var seeded int
	err = db.WithTenantData(ctx, pool, resp.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM accounts WHERE tenant_id = $1`, resp.TenantID).Scan(&seeded)
	})
	if err != nil {
		t.Fatal(err)
	}
	if seeded < 17 {
		t.Errorf("seeded accounts = %d, want >= 17 core accounts", seeded)
	}

	// Cleanup (best-effort): drop the freshly created tenant book.
	bg := context.Background()
	cleanup, cErr := pgx.Connect(bg, adminURL)
	if cErr == nil {
		defer cleanup.Close(bg)
		_, _ = cleanup.Exec(bg, `
			DELETE FROM journal_lines WHERE tenant_id = $1;
			DELETE FROM journal_entries WHERE tenant_id = $1;
			DELETE FROM ledger_chain_heads WHERE tenant_id = $1;
			DELETE FROM outbox_events WHERE tenant_id = $1;
			DELETE FROM accounts WHERE tenant_id = $1;
			DELETE FROM categories WHERE tenant_id = $1;
			DELETE FROM accounting_periods WHERE tenant_id = $1;
			DELETE FROM report_frameworks WHERE tenant_id = $1;
			DELETE FROM dimensions WHERE tenant_id = $1;
			DELETE FROM dashboard_widgets WHERE tenant_id = $1;
			DELETE FROM dashboard_layouts WHERE tenant_id = $1;
			DELETE FROM user_tenants WHERE tenant_id = $1;
			DELETE FROM tenants WHERE id = $1;
			DELETE FROM user_tokens WHERE user_id IN (SELECT id FROM users WHERE email = $2);
			DELETE FROM users WHERE email = $2;
		`, resp.TenantID, email)
	}
}
