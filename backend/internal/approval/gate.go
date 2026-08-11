package approval

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Gate is the enforcement helper embedded in posting flows (F-03).
// It checks the tenant's active approval_workflows for the entity type; if a
// workflow matches (amount >= min_amount_cents) the posting requires an
// APPROVED, unconsumed approval_request for that entity.
//
// Backward compatible: when no workflow rule matches, posting proceeds
// unchanged.

type Gate struct {
	pool *pgxpool.Pool
}

func NewGate(pool *pgxpool.Pool) *Gate {
	return &Gate{pool: pool}
}

// ErrApprovalRequired is returned when a workflow matches but no valid
// approval exists. Handlers should translate it to 409 APPROVAL_REQUIRED.
var ErrApprovalRequired = fmt.Errorf("approval required before posting")

// RequiresApproval reports whether the tenant has an active workflow for the
// entity type whose min_amount_cents is <= amount. It returns the approver
// role and true when approval is mandatory.
func (g *Gate) RequiresApproval(ctx context.Context, tx pgx.Tx, tenantID int64, entityType string, amountCents int64) (approverRole string, required bool, err error) {
	err = tx.QueryRow(ctx, `
		SELECT approver_role
		FROM approval_workflows
		WHERE tenant_id = $1 AND entity_type = $2 AND is_active = true
		  AND min_amount_cents <= $3
		LIMIT 1
	`, tenantID, entityType, amountCents).Scan(&approverRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return approverRole, true, nil
}

// HasValidApproval reports whether an APPROVED, unconsumed approval_request
// exists for the entity type + entity id (or, when entityID is 0, for the
// entity number).
func (g *Gate) HasValidApproval(ctx context.Context, tx pgx.Tx, tenantID int64, entityType string, entityID int64, entityNumber string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM approval_requests
			WHERE tenant_id = $1 AND entity_type = $2 AND status = 'APPROVED'
			  AND consumed_at IS NULL
			  AND ($3::bigint IS NULL OR entity_id = $3)
			  AND ($4::text IS NULL OR entity_number = $4)
		)`
	var exists bool
	err := tx.QueryRow(ctx, query, tenantID, entityType, nullableID(entityID), nullableStr(entityNumber)).Scan(&exists)
	return exists, err
}

// ConsumeApproval marks the matching APPROVED request as consumed so it
// cannot be reused by another posting. Called inside the posting transaction
// after the gate passes.
func (g *Gate) ConsumeApproval(ctx context.Context, tx pgx.Tx, tenantID int64, entityType string, entityID int64, entityNumber string) error {
	_, err := tx.Exec(ctx, `
		UPDATE approval_requests
		SET consumed_at = now()
		WHERE tenant_id = $1 AND entity_type = $2 AND status = 'APPROVED'
		  AND consumed_at IS NULL
		  AND ($3::bigint IS NULL OR entity_id = $3)
		  AND ($4::text IS NULL OR entity_number = $4)
	`, tenantID, entityType, nullableID(entityID), nullableStr(entityNumber))
	return err
}

// Check gates a posting: returns ErrApprovalRequired when a workflow matches
// and no valid approval exists. Callers should invoke this before inserting
// the document; after a successful insert they should call ConsumeApproval.
func (g *Gate) Check(ctx context.Context, tx pgx.Tx, tenantID int64, entityType string, amountCents int64, entityID int64, entityNumber string) error {
	_, required, err := g.RequiresApproval(ctx, tx, tenantID, entityType, amountCents)
	if err != nil {
		return err
	}
	if !required {
		return nil
	}
	ok, err := g.HasValidApproval(ctx, tx, tenantID, entityType, entityID, entityNumber)
	if err != nil {
		return err
	}
	if !ok {
		return ErrApprovalRequired
	}
	return nil
}

// HasValidApprovalByAmount reports whether an APPROVED, unconsumed approval
// exists for the entity type whose approved amount covers amountCents. Used by
// invoices where the approval is submitted with a planned amount before the
// invoice is created.
func (g *Gate) HasValidApprovalByAmount(ctx context.Context, tx pgx.Tx, tenantID int64, entityType string, amountCents int64) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM approval_requests
			WHERE tenant_id = $1 AND entity_type = $2 AND status = 'APPROVED'
			  AND consumed_at IS NULL AND COALESCE(amount_cents, 0) >= $3
		)`, tenantID, entityType, amountCents).Scan(&exists)
	return exists, err
}

// CheckAmount gates an amount-based posting (e.g. invoice): returns
// ErrApprovalRequired when a workflow matches and no valid approval with a
// covering amount exists. After a successful posting, call ConsumeApprovalByAmount.
func (g *Gate) CheckAmount(ctx context.Context, tx pgx.Tx, tenantID int64, entityType string, amountCents int64) error {
	_, required, err := g.RequiresApproval(ctx, tx, tenantID, entityType, amountCents)
	if err != nil {
		return err
	}
	if !required {
		return nil
	}
	ok, err := g.HasValidApprovalByAmount(ctx, tx, tenantID, entityType, amountCents)
	if err != nil {
		return err
	}
	if !ok {
		return ErrApprovalRequired
	}
	return nil
}

// ConsumeApprovalByAmount consumes one APPROVED, unconsumed approval whose
// amount covers amountCents. Called inside the posting transaction.
func (g *Gate) ConsumeApprovalByAmount(ctx context.Context, tx pgx.Tx, tenantID int64, entityType string, amountCents int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE approval_requests
		SET consumed_at = now()
		WHERE id = (
			SELECT id FROM approval_requests
			WHERE tenant_id = $1 AND entity_type = $2 AND status = 'APPROVED'
			  AND consumed_at IS NULL AND COALESCE(amount_cents, 0) >= $3
			ORDER BY id LIMIT 1
			FOR UPDATE
		)
	`, tenantID, entityType, amountCents)
	return err
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
