package costing

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PostGRN records a stock receipt: it updates stock_balances (qty_on_hand and,
// for moving average, avg_unit_cost_cents) and, for FIFO items, creates a new
// inventory_cost_layers row. It is called by the GRN handler after inserting
// inventory_movements. costingMethod should be read from items.costing_method;
// if empty the method is read from the items table.
// PostGRN records a stock receipt: it updates stock_balances (qty_on_hand and,
// for moving-average items, avg_unit_cost_cents) and, for FIFO items, creates a
// new cost layer. warehouseID scopes the balance to a specific warehouse (0 =
// unspecified/legacy). F-02: multi-warehouse support.
func PostGRN(ctx context.Context, tx pgx.Tx, tenantID, itemID int64, warehouseID int64, qty float64, unitCostCents int64, costingMethod string) error {
	if qty <= 0 {
		return fmt.Errorf("costing: PostGRN qty must be > 0 (got %v)", qty)
	}
	if unitCostCents < 0 {
		return fmt.Errorf("costing: PostGRN unit_cost_cents must be >= 0 (got %d)", unitCostCents)
	}
	method := costingMethod
	if method == "" {
		var err error
		method, err = itemCostingMethod(ctx, tx, tenantID, itemID)
		if err != nil {
			return err
		}
	}
	if !validMethod(method) {
		return fmt.Errorf("%w: %s", ErrUnknownCostingMethod, method)
	}

	// Upsert stock_balances: add qty to on-hand. For moving average we also
	// recompute avg_unit_cost_cents atomically inside the UPDATE using the
	// pre-update qty_on_hand and avg_unit_cost_cents.
	switch method {
	case MethodMovingAverage:
		// avg = (old_qty*old_avg + new_qty*new_cost) / (old_qty + new_qty).
		// Done in SQL so the read and write are atomic against the row lock
		// taken by the UPSERT.
		_, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, item_id, warehouse_id, qty_on_hand, avg_unit_cost_cents, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, item_id, warehouse_id) DO UPDATE
			SET avg_unit_cost_cents = CASE
			        WHEN stock_balances.qty_on_hand + $4 = 0 THEN stock_balances.avg_unit_cost_cents
			        ELSE (
			          (stock_balances.qty_on_hand * stock_balances.avg_unit_cost_cents
			           + $4 * $5)
			          / (stock_balances.qty_on_hand + $4)
			        )
			      END,
			    qty_on_hand = stock_balances.qty_on_hand + $4,
			    updated_at = now()
		`, tenantID, itemID, warehouseID, qty, unitCostCents)
		if err != nil {
			return fmt.Errorf("costing: PostGRN stock_balances upsert: %w", err)
		}
	case MethodFIFO:
		// FIFO: upsert the balance (qty only — avg_unit_cost_cents is not
		// authoritative for FIFO but is kept as a fallback for legacy data)
		// and create a new open cost layer for this receipt.
		_, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, item_id, warehouse_id, qty_on_hand, avg_unit_cost_cents, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, item_id, warehouse_id) DO UPDATE
			SET qty_on_hand = stock_balances.qty_on_hand + $4,
			    updated_at = now()
		`, tenantID, itemID, warehouseID, qty, unitCostCents)
		if err != nil {
			return fmt.Errorf("costing: PostGRN stock_balances upsert: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_cost_layers
			    (tenant_id, item_id, qty_original, qty_remaining, unit_cost_cents)
			VALUES ($1, $2, $3, $3, $4)
		`, tenantID, itemID, qty, unitCostCents); err != nil {
			return fmt.Errorf("costing: PostGRN layer insert: %w", err)
		}
	case MethodSpecific:
		// Specific identification: just track the balance. The caller-
		// supplied unit cost is recorded on the movement; no layer is needed.
		_, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, item_id, warehouse_id, qty_on_hand, avg_unit_cost_cents, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, item_id, warehouse_id) DO UPDATE
			SET qty_on_hand = stock_balances.qty_on_hand + $4,
			    updated_at = now()
		`, tenantID, itemID, warehouseID, qty, unitCostCents)
		if err != nil {
			return fmt.Errorf("costing: PostGRN stock_balances upsert: %w", err)
		}
	}
	return nil
}

// ResolveCOGS computes the COGS for issuing qty units of itemID and consumes
// FIFO layers / adjusts the moving average accordingly. It returns the total
// COGS in cents. It is called by the DO (and stock-opname shortage) handler
// BEFORE posting the COGS journal so the journal uses the resolved cost rather
// than a caller-supplied unit cost. Negative stock is rejected.
//
// costingMethod should be read from items.costing_method; if empty the method
// is read from the items table.
// ResolveCOGS computes the COGS for a stock issue and reduces the on-hand
// balance. warehouseID scopes the balance to a specific warehouse (0 =
// unspecified/legacy). F-02: multi-warehouse support.
func ResolveCOGS(ctx context.Context, tx pgx.Tx, tenantID, itemID int64, warehouseID int64, qty float64, costingMethod string) (totalCOGSCents int64, err error) {
	if qty <= 0 {
		return 0, fmt.Errorf("costing: ResolveCOGS qty must be > 0 (got %v)", qty)
	}
	method := costingMethod
	if method == "" {
		method, err = itemCostingMethod(ctx, tx, tenantID, itemID)
		if err != nil {
			return 0, err
		}
	}
	if !validMethod(method) {
		return 0, fmt.Errorf("%w: %s", ErrUnknownCostingMethod, method)
	}

	// Lock the balance row for the duration of the transaction so concurrent
	// issues cannot race on qty_on_hand / avg_unit_cost_cents.
	qoh, avgCost, err := lockBalance(ctx, tx, tenantID, itemID, warehouseID)
	if err != nil {
		return 0, err
	}
	if qoh < qty {
		return 0, fmt.Errorf("%w: item %d on_hand=%v need=%v", ErrInsufficientStock, itemID, qoh, qty)
	}

	switch method {
	case MethodFIFO:
		return consumeFIFO(ctx, tx, tenantID, itemID, warehouseID, qty, qoh, float64(avgCost))
	case MethodMovingAverage:
		cogs := int64(math.Round(qty * float64(avgCost)))
		// Reduce on-hand. avg_unit_cost_cents is unchanged (the average does
		// not move when stock leaves at the average cost).
		if _, err := tx.Exec(ctx, `
			UPDATE stock_balances
			SET qty_on_hand = qty_on_hand - $4, updated_at = now()
			WHERE tenant_id = $1 AND item_id = $2 AND warehouse_id = $3
		`, tenantID, itemID, warehouseID, qty); err != nil {
			return 0, fmt.Errorf("costing: ResolveCOGS balance update: %w", err)
		}
		return cogs, nil
	case MethodSpecific:
		// Specific identification: the caller-supplied cost is authoritative,
		// but we still reduce the balance here. The actual unit cost used for
		// the journal is the caller's; ResolveCOGS only ensures the balance
		// moves and stock does not go negative. COGS is returned as 0 — the
		// caller computes its own COGS from its supplied unit cost.
		if _, err := tx.Exec(ctx, `
			UPDATE stock_balances
			SET qty_on_hand = qty_on_hand - $4, updated_at = now()
			WHERE tenant_id = $1 AND item_id = $2 AND warehouse_id = $3
		`, tenantID, itemID, warehouseID, qty); err != nil {
			return 0, fmt.Errorf("costing: ResolveCOGS balance update: %w", err)
		}
		return 0, nil
	}
	return 0, fmt.Errorf("%w: %s", ErrUnknownCostingMethod, method)
}

// consumeFIFO consumes qty from the oldest open layers (ORDER BY created_at),
// closing layers as they are drained, and returns the total COGS. It also
// reduces stock_balances.qty_on_hand by qty. If open layers do not cover the
// full qty (e.g. legacy stock received before migration 000017), the shortfall
// is valued at the balance's avg_unit_cost_cents as a fallback so the issue
// still succeeds.
func consumeFIFO(ctx context.Context, tx pgx.Tx, tenantID, itemID int64, warehouseID int64, qty, qoh, avgCost float64) (int64, error) {
	// Materialize the open layers and drain the result set BEFORE executing
	// any UPDATE: pgx v5 forbids running tx.Exec while a tx.Query cursor is
	// still open on the same connection ("conn busy" — QA-03).
	layers, err := loadOpenFIFOLayers(ctx, tx, tenantID, itemID, warehouseID)
	if err != nil {
		return 0, err
	}

	cogs, updates, uncovered := planFIFOConsumption(layers, qty, avgCost)

	for _, update := range updates {
		if update.NewRemaining <= 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE inventory_cost_layers
				SET qty_remaining = 0, closed_at = now()
				WHERE tenant_id = $1 AND id = $2 AND warehouse_id = $3
			`, tenantID, update.LayerID, warehouseID); err != nil {
				return 0, fmt.Errorf("costing: ResolveCOGS layer close: %w", err)
			}
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE inventory_cost_layers
			SET qty_remaining = $4
			WHERE tenant_id = $1 AND id = $2 AND warehouse_id = $3
		`, tenantID, update.LayerID, warehouseID, update.NewRemaining); err != nil {
			return 0, fmt.Errorf("costing: ResolveCOGS layer update: %w", err)
		}
	}

	// Fallback for legacy stock with no layers: value the shortfall at the
	// balance's average cost so the issue still succeeds.
	if uncovered > 0 {
		cogs += int64(math.Round(uncovered * avgCost))
	}

	// Reduce the balance.
	if _, err := tx.Exec(ctx, `
		UPDATE stock_balances
		SET qty_on_hand = qty_on_hand - $4, updated_at = now()
		WHERE tenant_id = $1 AND item_id = $2 AND warehouse_id = $3
	`, tenantID, itemID, warehouseID, qty); err != nil {
		return 0, fmt.Errorf("costing: ResolveCOGS balance update: %w", err)
	}
	return cogs, nil
}

// fifoLayer is a materialized open inventory_cost_layers row.
type fifoLayer struct {
	ID            int64
	QtyRemaining  float64
	UnitCostCents int64
}

// fifoLayerUpdate is a pending balance update for one consumed layer.
// NewRemaining <= 0 means the layer is fully consumed and must be closed
// (qty_remaining = 0, closed_at = now()).
type fifoLayerUpdate struct {
	LayerID      int64
	NewRemaining float64
}

// loadOpenFIFOLayers reads every open cost layer for an item/warehouse, oldest
// first, locking them FOR UPDATE. Rows are fully materialized (and the result
// set drained, including the rows.Err check) before the caller issues any
// further statement on the connection.
func loadOpenFIFOLayers(ctx context.Context, tx pgx.Tx, tenantID, itemID, warehouseID int64) ([]fifoLayer, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, qty_remaining, unit_cost_cents
		FROM inventory_cost_layers
		WHERE tenant_id = $1 AND item_id = $2 AND warehouse_id = $3 AND closed_at IS NULL
		ORDER BY created_at, id
		FOR UPDATE
	`, tenantID, itemID, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("costing: ResolveCOGS layer select: %w", err)
	}
	defer rows.Close()

	var layers []fifoLayer
	for rows.Next() {
		var layer fifoLayer
		var qtyRemaining pgtype.Numeric
		if err := rows.Scan(&layer.ID, &qtyRemaining, &layer.UnitCostCents); err != nil {
			return nil, fmt.Errorf("costing: ResolveCOGS layer scan: %w", err)
		}
		layer.QtyRemaining = numericToFloat(qtyRemaining)
		layers = append(layers, layer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("costing: ResolveCOGS rows: %w", err)
	}
	return layers, nil
}

// planFIFOConsumption walks materialized layers oldest-first and returns the
// FIFO COGS in cents, the per-layer balance updates to apply, and the qty not
// covered by any open layer (the caller values it at the fallback average
// cost). It is pure so the FIFO semantics stay unit-testable without a DB.
func planFIFOConsumption(layers []fifoLayer, qty, avgCost float64) (cogsCents int64, updates []fifoLayerUpdate, uncoveredQty float64) {
	remaining := qty
	for _, layer := range layers {
		if remaining <= 0 {
			break
		}
		if layer.QtyRemaining <= 0 {
			continue
		}
		take := math.Min(remaining, layer.QtyRemaining)
		cogsCents += int64(math.Round(take * float64(layer.UnitCostCents)))
		updates = append(updates, fifoLayerUpdate{
			LayerID:      layer.ID,
			NewRemaining: layer.QtyRemaining - take,
		})
		remaining -= take
	}
	return cogsCents, updates, remaining
}

// ReverseCOGS reverses a prior COGS posting (for sales returns / credit notes
// / purchase returns). It restores FIFO layers (a new layer at the original
// cost) or adjusts the moving-average cost, and increases qty_on_hand.
// unitCostCents is the original unit cost that was used for the COGS posting
// being reversed.
//
// costingMethod should be read from items.costing_method; if empty the method
// is read from the items table.
func ReverseCOGS(ctx context.Context, tx pgx.Tx, tenantID, itemID int64, warehouseID int64, qty float64, unitCostCents int64, costingMethod string) error {
	if qty <= 0 {
		return fmt.Errorf("costing: ReverseCOGS qty must be > 0 (got %v)", qty)
	}
	if unitCostCents < 0 {
		return fmt.Errorf("costing: ReverseCOGS unit_cost_cents must be >= 0 (got %d)", unitCostCents)
	}
	method := costingMethod
	if method == "" {
		var err error
		method, err = itemCostingMethod(ctx, tx, tenantID, itemID)
		if err != nil {
			return err
		}
	}
	if !validMethod(method) {
		return fmt.Errorf("%w: %s", ErrUnknownCostingMethod, method)
	}

	switch method {
	case MethodFIFO:
		// Create a new open layer at the original cost so the reversed qty is
		// available for the next issue (FIFO by created_at puts it after any
		// pre-existing open layers, which is the conservative choice).
		_, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, item_id, warehouse_id, qty_on_hand, avg_unit_cost_cents, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, item_id, warehouse_id) DO UPDATE
			SET qty_on_hand = stock_balances.qty_on_hand + $4,
			    updated_at = now()
		`, tenantID, itemID, warehouseID, qty, unitCostCents)
		if err != nil {
			return fmt.Errorf("costing: ReverseCOGS balance upsert: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_cost_layers
			    (tenant_id, item_id, warehouse_id, qty_original, qty_remaining, unit_cost_cents)
			VALUES ($1, $2, $3, $4, $4, $5)
		`, tenantID, itemID, warehouseID, qty, unitCostCents); err != nil {
			return fmt.Errorf("costing: ReverseCOGS layer insert: %w", err)
		}
	case MethodMovingAverage:
		// qty_on_hand += qty and recalculate the average as if the qty had
		// been received at the original cost.
		_, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, item_id, warehouse_id, qty_on_hand, avg_unit_cost_cents, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, item_id, warehouse_id) DO UPDATE
			SET avg_unit_cost_cents = CASE
			        WHEN stock_balances.qty_on_hand + $4 = 0 THEN stock_balances.avg_unit_cost_cents
			        ELSE (
			          (stock_balances.qty_on_hand * stock_balances.avg_unit_cost_cents
			           + $4 * $5)
			          / (stock_balances.qty_on_hand + $4)
			        )
			      END,
			    qty_on_hand = stock_balances.qty_on_hand + $4,
			    updated_at = now()
		`, tenantID, itemID, warehouseID, qty, unitCostCents)
		if err != nil {
			return fmt.Errorf("costing: ReverseCOGS balance upsert: %w", err)
		}
	case MethodSpecific:
		_, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, item_id, warehouse_id, qty_on_hand, avg_unit_cost_cents, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, item_id, warehouse_id) DO UPDATE
			SET qty_on_hand = stock_balances.qty_on_hand + $4,
			    updated_at = now()
		`, tenantID, itemID, warehouseID, qty, unitCostCents)
		if err != nil {
			return fmt.Errorf("costing: ReverseCOGS balance upsert: %w", err)
		}
	}
	return nil
}

// GetStockBalance returns the current on-hand qty and average unit cost for an
// item. If no stock_balances row exists yet (item never received), it returns
// zeros.
func GetStockBalance(ctx context.Context, tx pgx.Tx, tenantID, itemID int64, warehouseID int64) (qtyOnHand float64, avgCostCents int64, err error) {
	return lockBalance(ctx, tx, tenantID, itemID, warehouseID)
}

// lockBalance reads and locks the stock_balances row for an item (FOR UPDATE)
// so concurrent costing operations serialize on the row. Returns zeros (no
// error) when the item has no balance row yet.
func lockBalance(ctx context.Context, tx pgx.Tx, tenantID, itemID int64, warehouseID int64) (float64, int64, error) {
	var qoh pgtype.Numeric
	var avgCost int64
	err := tx.QueryRow(ctx, `
		SELECT qty_on_hand, avg_unit_cost_cents
		FROM stock_balances
		WHERE tenant_id = $1 AND item_id = $2 AND warehouse_id = $3
		FOR UPDATE
	`, tenantID, itemID, warehouseID).Scan(&qoh, &avgCost)
	if err != nil {
		if isNoRows(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("costing: lockBalance: %w", err)
	}
	return numericToFloat(qoh), avgCost, nil
}

// isNoRows reports whether err is pgx.ErrNoRows.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// numericToFloat converts a pgtype.Numeric to float64 (0 when not set).
func numericToFloat(value pgtype.Numeric) float64 {
	if !value.Valid {
		return 0
	}
	f, err := value.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}
