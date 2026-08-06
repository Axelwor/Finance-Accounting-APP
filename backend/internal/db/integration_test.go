package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMVPDatabaseInvariants(t *testing.T) {
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000001_mvp_foundation.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	}()

	seedMVPDatabase(t, ctx, conn)
	t.Run("tenant isolation", func(t *testing.T) { testTenantIsolation(t, ctx, conn) })
	t.Run("balanced journal commits", func(t *testing.T) { testBalancedJournal(t, ctx, conn) })
	t.Run("unbalanced journal rolls back", func(t *testing.T) { testUnbalancedJournal(t, ctx, conn) })
	t.Run("posted journal is immutable", func(t *testing.T) { testPostedJournalImmutable(t, ctx, conn) })
	t.Run("idempotency is unique", func(t *testing.T) { testIdempotency(t, ctx, conn) })
}

// execAsTenant runs body inside one transaction with the tenant context set
// transaction-locally, matching how the application middleware scopes RLS.
func execAsTenant(t *testing.T, ctx context.Context, conn *pgx.Conn, tenantID int64, body string) {
	t.Helper()
	query := fmt.Sprintf("BEGIN; SELECT set_config('app.tenant_id', '%d', false); %s COMMIT;", tenantID, body)
	if _, err := conn.Exec(ctx, query); err != nil {
		t.Fatalf("execAsTenant failed: %v", err)
	}
}

func seedMVPDatabase(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	if _, err := conn.Exec(ctx, `
		INSERT INTO tenants (name, slug) VALUES ('Test Tenant', 'test-tenant');
		INSERT INTO users (email, full_name) VALUES ('owner@test.local', 'Owner');
		INSERT INTO user_tenants (user_id, tenant_id, role) VALUES (1, 1, 'owner');
	`); err != nil {
		t.Fatal(err)
	}
	execAsTenant(t, ctx, conn, 1, `
		INSERT INTO accounting_periods (tenant_id, period_start, period_end, status)
		VALUES (1, DATE '2026-01-01', DATE '2026-12-31', 'OPEN');
		INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
		VALUES (1, '1101', 'Kas', 'asset', 'CASH'),
		       (1, '4101', 'Pendapatan', 'revenue', 'REVENUE');
	`)
}

func testTenantIsolation(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	if _, err := conn.Exec(ctx, "SELECT set_config('app.tenant_id', '999', false)"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM accounts").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected tenant 999 to see no accounts, got %d", count)
	}
}

func testBalancedJournal(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	execAsTenant(t, ctx, conn, 1, `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES (1, 'JRN-TEST-1', DATE '2026-08-06', 1, 'BK-TEST-1', 'CASH_IN', gen_random_uuid(), 'hash-1', 'genesis', 1);
		INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, source_line_ref) VALUES (1, 1, 1, 500000, 'cash');
		INSERT INTO journal_lines (tenant_id, entry_id, account_id, credit_cents, source_line_ref) VALUES (1, 1, 2, 500000, 'revenue');
	`)
}

func testUnbalancedJournal(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	body := `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES (1, 'JRN-TEST-2', DATE '2026-08-06', 1, 'BK-TEST-2', 'CASH_IN', gen_random_uuid(), 'hash-2', 'hash-1', 1);
		INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, source_line_ref) VALUES (1, 2, 1, 100, 'cash');
	`
	query := fmt.Sprintf("BEGIN; SELECT set_config('app.tenant_id', '1', false); %s COMMIT;", body)
	_, err := conn.Exec(ctx, query)
	if err == nil || !strings.Contains(err.Error(), "not balanced") {
		t.Fatalf("expected balance rejection, got %v", err)
	}
}

func testPostedJournalImmutable(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	if _, err := conn.Exec(ctx, "UPDATE journal_entries SET description = 'tampered' WHERE id = 1"); err == nil ||
		!strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable rejection, got %v", err)
	}
}

func testIdempotency(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	execAsTenant(t, ctx, conn, 1, `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES (1, 'JRN-TEST-3', DATE '2026-08-06', 1, 'BK-TEST-3', 'CASH_IN', '00000000-0000-0000-0000-000000000003', 'hash-3', 'hash-1', 1);
	`)
	query := fmt.Sprintf("BEGIN; SELECT set_config('app.tenant_id', '1', false); %s COMMIT;", `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES (1, 'JRN-TEST-4', DATE '2026-08-06', 1, 'BK-TEST-4', 'CASH_IN', '00000000-0000-0000-0000-000000000003', 'hash-4', 'hash-3', 1);
	`)
	_, err := conn.Exec(ctx, query)
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected idempotency rejection, got %v", err)
	}
}
