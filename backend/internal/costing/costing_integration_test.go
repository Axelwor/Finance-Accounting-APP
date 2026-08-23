package costing_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/costing"
)

// TestResolveCOGS_FIFO_NoConnBusy is the QA-03 regression test: consumeFIFO
// used to run UPDATEs while the layer SELECT cursor was still open on the same
// pgx connection, which pgx v5 rejects with "conn busy". It exercises the real
// driver against a real database (GRN layers in, ResolveCOGS out) and asserts
// FIFO valuation plus the resulting layer/balance state.
func TestResolveCOGS_FIFO_NoConnBusy(t *testing.T) {
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

	slug := fmt.Sprintf("qa-fifo-%d", time.Now().UnixNano())
	var tenantID int64
	err = conn.QueryRow(ctx, `
		INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id
	`, "QA FIFO "+slug[len(slug)-6:], slug).Scan(&tenantID)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	defer func() {
		cleanup := `
			DELETE FROM inventory_cost_layers WHERE tenant_id = $1;
			DELETE FROM stock_balances WHERE tenant_id = $1;
			DELETE FROM items WHERE tenant_id = $1;
			DELETE FROM accounts WHERE tenant_id = $1;
			DELETE FROM tenants WHERE id = $1;
		`
		_, _ = conn.Exec(context.Background(), cleanup, tenantID)
	}()

	var itemID int64
	// Session-scoped GUC: the RLS policies cast current_setting('app.tenant_id')
	// to bigint, and after any transaction-local set_config(true) commit the
	// session-level value would be '' which breaks that cast.
	if _, err := conn.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false)`, fmt.Sprintf("%d", tenantID)); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.QueryRow(ctx, `
		WITH a AS (
			INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
			VALUES ($1, '1301', 'Persediaan', 'asset', 'INVENTORY') RETURNING id
		), i AS (
			INSERT INTO items (tenant_id, code, name, item_type, costing_method, inventory_account_id, is_tracked_stock)
			SELECT $1, 'ITM-FIFO', 'FIFO Item', 'goods', 'fifo', a.id, true FROM a RETURNING id
		)
		SELECT id FROM i
	`, tenantID).Scan(&itemID)
	if err != nil {
		t.Fatalf("seed account/item: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// GRN 50 pcs @ 80.000 as two FIFO layers (30 @ 80.000 then 20 @ 90.000).
	tx, err = conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprintf("%d", tenantID)); err != nil {
		t.Fatal(err)
	}
	if err := costing.PostGRN(ctx, tx, tenantID, itemID, 0, 30, 80_000, costing.MethodFIFO); err != nil {
		t.Fatalf("PostGRN #1: %v", err)
	}
	if err := costing.PostGRN(ctx, tx, tenantID, itemID, 0, 20, 90_000, costing.MethodFIFO); err != nil {
		t.Fatalf("PostGRN #2: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// DO qty 10 — this exact path returned "conn busy" before the fix.
	tx, err = conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprintf("%d", tenantID)); err != nil {
		t.Fatal(err)
	}
	cogs, err := costing.ResolveCOGS(ctx, tx, tenantID, itemID, 0, 10, costing.MethodFIFO)
	if err != nil {
		t.Fatalf("ResolveCOGS: %v", err)
	}
	if cogs != 800_000 { // 10 pcs from the oldest layer @ 80.000.
		t.Errorf("cogs = %d, want 800000", cogs)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var qoh float64
	var oldestRemaining, newestRemaining float64
	var closedCount int
	err = conn.QueryRow(ctx, `
		SELECT sb.qty_on_hand::float8,
		       (SELECT l.qty_remaining::float8 FROM inventory_cost_layers l WHERE l.tenant_id = $1 AND l.unit_cost_cents = 80000),
		       (SELECT l.qty_remaining::float8 FROM inventory_cost_layers l WHERE l.tenant_id = $1 AND l.unit_cost_cents = 90000),
		       (SELECT count(*) FROM inventory_cost_layers l WHERE l.tenant_id = $1 AND l.closed_at IS NOT NULL)
		FROM stock_balances sb WHERE sb.tenant_id = $1 AND sb.item_id = $2 AND sb.warehouse_id = 0
	`, tenantID, itemID).Scan(&qoh, &oldestRemaining, &newestRemaining, &closedCount)
	if err != nil {
		t.Fatalf("verify state: %v", err)
	}
	if qoh != 40 {
		t.Errorf("qty_on_hand = %v, want 40", qoh)
	}
	if oldestRemaining != 20 {
		t.Errorf("oldest layer remaining = %v, want 20", oldestRemaining)
	}
	if newestRemaining != 20 {
		t.Errorf("newest layer remaining = %v, want 20", newestRemaining)
	}
	if closedCount != 0 {
		t.Errorf("closed layers = %d, want 0", closedCount)
	}
}
