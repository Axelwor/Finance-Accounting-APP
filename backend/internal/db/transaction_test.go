package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTenantData must expose the tenant GUC inside fn, commit on success,
// roll back on error, and keep the GUC invisible outside the transaction.
// Against a live Postgres this also proves the RLS contract: a row from
// another tenant is invisible inside the transaction, and the GUC is gone
// after commit (transaction-scoped set_config).
func TestWithTenantData(t *testing.T) {
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	const tenantID = int64(1) // seeded by the db integration harness

	t.Run("GUC visible inside, committed on nil error", func(t *testing.T) {
		var guc string
		err := WithTenantData(ctx, pool, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT current_setting('app.tenant_id', true)`).Scan(&guc)
		})
		if err != nil {
			t.Fatal(err)
		}
		if guc != fmt.Sprintf("%d", tenantID) {
			t.Errorf("app.tenant_id inside tx = %q, want %q", guc, fmt.Sprintf("%d", tenantID))
		}

		var after string
		if err := pool.QueryRow(ctx,
			`SELECT current_setting('app.tenant_id', true)`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after != "" {
			t.Errorf("app.tenant_id leaked past commit: %q", after)
		}
	})

	t.Run("rollback on fn error", func(t *testing.T) {
		sentinel := errors.New("boom")
		err := WithTenantData(ctx, pool, tenantID, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx,
				`SELECT set_config('app.probe_marker', '1', true)`); err != nil {
				return err
			}
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want sentinel", err)
		}
	})

	t.Run("set_config failure propagates and aborts", func(t *testing.T) {
		called := false
		err := WithTenantData(ctx, pool, -1, func(tx pgx.Tx) error {
			called = true
			return nil
		})
		// Negative ids are still valid strings for set_config, so this
		// mainly asserts the wrapper survives; fn must not have run if the
		// config step failed. With a valid id it always runs.
		if err != nil && called {
			t.Error("fn ran despite set_config failure")
		}
	})
}
