package reporting

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// F-02 regression: fetchTrialBalance must aggregate only POSTED entries.
// Before the fix the journal_entries join was a LEFT JOIN with the POSTED
// filter inside the ON clause, so lines belonging to non-POSTED (VOID /
// unposted) entries still contributed to the sums when no date filter was
// supplied. These paths need a live Postgres; they are skipped unless
// TEST_DATABASE_URL is set (same convention as internal/db integration
// tests). Seed data is tenant-scoped and removed on cleanup, so the test is
// safe to run against a shared test database.
func TestFetchTrialBalance_PostedOnly(t *testing.T) {
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	conn, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	const userEmailBase = "tb-posted-only-test"
	// Unique per run: POSTED journals are undeletable when triggers are
	// active, so a previous run's cleanup may have left residue behind.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	tenantName := "tb-posted-only-" + suffix
	userEmail := userEmailBase + "+" + suffix + "@test.local"
	cleanup := func() {
		// journal_entries_immutable blocks deleting POSTED rows; disable
		// triggers when the role permits (superuser/table owner). Best
		// effort — on failure the unique-per-run names avoid collisions.
		_, _ = conn.Exec(context.Background(), "SET session_replication_role = replica")
		_, _ = conn.Exec(context.Background(), `
			DELETE FROM journal_lines WHERE tenant_id IN (SELECT id FROM tenants WHERE name = $1);
			DELETE FROM journal_entries WHERE tenant_id IN (SELECT id FROM tenants WHERE name = $1);
			DELETE FROM accounting_periods WHERE tenant_id IN (SELECT id FROM tenants WHERE name = $1);
			DELETE FROM accounts WHERE tenant_id IN (SELECT id FROM tenants WHERE name = $1);
			DELETE FROM user_tenants WHERE tenant_id IN (SELECT id FROM tenants WHERE name = $1);
			DELETE FROM users WHERE email = $2;
			DELETE FROM tenants WHERE name = $1;
		`, tenantName, userEmail)
		_, _ = conn.Exec(context.Background(), "RESET session_replication_role")
	}
	cleanup()
	defer cleanup()

	var tenantID, userID, periodID, cashID, revenueID int64
	if err := conn.QueryRow(ctx, `
		INSERT INTO tenants (name, slug) VALUES ($1, $1) RETURNING id
	`, tenantName).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `
		INSERT INTO users (email, full_name) VALUES ($1, 'Trial Balance Test') RETURNING id
	`, userEmail).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO user_tenants (user_id, tenant_id, role) VALUES ($1, $2, 'owner')
	`, userID, tenantID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `
		INSERT INTO accounting_periods (tenant_id, period_start, period_end, status)
		VALUES ($1, DATE '2026-01-01', DATE '2026-12-31', 'OPEN') RETURNING id
	`, tenantID).Scan(&periodID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `
		INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
		VALUES ($1, '1101', 'TB Test Cash', 'asset', 'CASH') RETURNING id
	`, tenantID).Scan(&cashID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `
		INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
		VALUES ($1, '4101', 'TB Test Revenue', 'revenue', 'REVENUE') RETURNING id
	`, tenantID).Scan(&revenueID); err != nil {
		t.Fatal(err)
	}

	// The balance constraint trigger is deferred to commit, so each entry
	// and its lines must be inserted inside one transaction.
	insertEntry := func(status string, lines []struct {
		accountID     int64
		debit, credit int64
	}) int64 {
		t.Helper()
		var entryID int64
		voidCols := ", NULL, NULL, NULL"
		if status == "VOID" {
			voidCols = fmt.Sprintf(", 'test void', %d, now()", userID)
		}
		query := fmt.Sprintf(`
			INSERT INTO journal_entries
				(tenant_id, number, entry_date, period_id, status, source_ref, intent_type,
				 idempotency_key, hash, prev_hash, created_by, void_reason, voided_by, voided_at)
			VALUES ($1, $2, DATE '2026-08-06', $3, $4, $5, 'CASH_IN',
				 gen_random_uuid(), 'hash-tb', 'genesis', $6%s)
			RETURNING id
		`, voidCols)

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := tx.QueryRow(ctx, query,
			tenantID, "JRN-TB-"+status, periodID, status,
			"BK-TB-TEST-"+status, userID).Scan(&entryID); err != nil {
			t.Fatalf("insert %s entry: %v", status, err)
		}
		for _, l := range lines {
			if _, err := tx.Exec(ctx, `
				INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
				VALUES ($1, $2, $3, $4, $5, 'tb-test')
			`, tenantID, entryID, l.accountID, l.debit, l.credit); err != nil {
				t.Fatalf("insert line: %v", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit %s entry: %v", status, err)
		}
		return entryID
	}

	// Posted: Dr 100000 cash / Cr 100000 revenue — must be counted.
	insertEntry("POSTED", []struct {
		accountID     int64
		debit, credit int64
	}{
		{cashID, 100_000, 0},
		{revenueID, 0, 100_000},
	})
	// Void: Dr 5000 cash / Cr 5000 revenue — must be excluded entirely.
	insertEntry("VOID", []struct {
		accountID     int64
		debit, credit int64
	}{
		{cashID, 5_000, 0},
		{revenueID, 0, 5_000},
	})

	service := NewHandler(pool)
	result, err := service.fetchTrialBalance(ctx, tenantID, reportFilter{})
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalDebitCents != 100_000 || result.TotalCreditCents != 100_000 {
		t.Fatalf("totals = (dr=%d, cr=%d), want (dr=100000, cr=100000) — void entry leaked into trial balance",
			result.TotalDebitCents, result.TotalCreditCents)
	}
	for _, row := range result.Rows {
		if row.AccountID == cashID && row.DebitCents != 100_000 {
			t.Errorf("cash debit = %d, want 100000", row.DebitCents)
		}
		if row.AccountID == revenueID && row.CreditCents != 100_000 {
			t.Errorf("revenue credit = %d, want 100000", row.CreditCents)
		}
	}
	if len(result.Rows) != 2 {
		t.Errorf("rows = %d, want 2 (cash + revenue)", len(result.Rows))
	}
	if !result.Balanced {
		t.Error("result.Balanced = false, want true")
	}
}
