// Package costing implements PSAK 14 inventory costing: FIFO, moving average,
// and specific identification. It is the single source of truth for stock
// balances and COGS resolution — handlers call PostGRN on receipt and
// ResolveCOGS / ReverseCOGS on issue / reversal, and never trust caller-
// supplied unit costs for FIFO or moving-average items.
//
// The package persists two tables (migration 000017):
//   - stock_balances: one row per (tenant, item) with qty_on_hand and the
//     running moving-average unit cost (avg_unit_cost_cents).
//   - inventory_cost_layers: one row per GRN receipt for FIFO items; each
//     stock-out consumes the oldest open layer first.
//
// All functions run inside the caller's transaction (pgx.Tx) so costing
// updates are atomic with the journal entry and inventory movement.
package costing

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Costing methods (mirror items.costing_method CHECK constraint).
const (
	MethodFIFO         = "fifo"
	MethodMovingAverage = "moving_average"
	MethodSpecific     = "specific"
)

// ErrInsufficientStock is returned when a stock-out would drive qty_on_hand
// below zero (negative stock is rejected per PSAK 14).
var ErrInsufficientStock = errors.New("insufficient stock on hand")

// ErrUnknownCostingMethod is returned when an item's costing_method is not one
// of the supported values.
var ErrUnknownCostingMethod = errors.New("unknown costing method")

// validMethod reports whether method is one of the supported costing methods.
func validMethod(method string) bool {
	switch method {
	case MethodFIFO, MethodMovingAverage, MethodSpecific:
		return true
	}
	return false
}

// itemCostingMethod reads the costing_method for an item. The caller has
// already set app.tenant_id via withTenant, so the RLS-scoped SELECT is safe.
func itemCostingMethod(ctx context.Context, tx pgx.Tx, tenantID, itemID int64) (string, error) {
	var method pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT costing_method FROM items
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, itemID).Scan(&method)
	if err != nil {
		return "", fmt.Errorf("costing: item %d not found: %w", itemID, err)
	}
	if !method.Valid {
		return "", fmt.Errorf("costing: item %d has no costing_method", itemID)
	}
	return method.String, nil
}
