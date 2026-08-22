// Package db provides shared database access helpers.
package db

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTransaction runs fn inside a plain transaction (no tenant GUC). Use it
// for non-tenant-scoped work; tenant-scoped reads/writes must go through
// WithTenantData so RLS-restricted roles see the right rows.
func WithTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Ensure rollback runs even if fn panics. After a successful Commit the
	// Rollback is a safe no-op (returns pgx.ErrTxClosed).
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// WithTenantData runs fn inside a transaction with app.tenant_id set
// (transaction-scoped), so RLS-restricted roles see exactly this tenant's
// rows. For read paths whose queries go through the pool directly — every
// RLS policy is fail-closed (current_setting('app.tenant_id', true) is NULL
// when unset), so a restricted role sees zero rows without this wrapper.
//
// Write paths that already open their own transactions should keep using the
// package-local withTenant helpers; this function exists so handlers can wrap
// read-only fetchers uniformly:
//
//	err := db.WithTenantData(ctx, service.pool, tenantID, func(tx pgx.Tx) error {
//	    result, err = service.fetchX(ctx, tx, tenantID)
//	    return err
//	})
func WithTenantData(ctx context.Context, pool *pgxpool.Pool, tenantID int64, fn func(tx pgx.Tx) error) error {
	return WithTransaction(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`,
			strconv.FormatInt(tenantID, 10)); err != nil {
			return err
		}
		return fn(tx)
	})
}
