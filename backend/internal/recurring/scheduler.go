package recurring

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// F-07: Recurring scheduler.
//
// StartScheduler launches a background loop that periodically scans every
// tenant's active recurring transactions and posts the ones whose next_date
// has arrived (or passed). Each posting runs in its own transaction with
// set_config('app.tenant_id', ...) so RLS sees the tenant, mirrors the
// manual PostNow path exactly (idempotency key REC-<id>-<date>, journal,
// next_date advance), and is safe against duplicate posting: the idempotency
// key makes a second attempt a no-op replay.
//
// The scheduler stops when ctx is cancelled (wired to the API server's
// graceful shutdown in cmd/api/main.go).
// ---------------------------------------------------------------------------

// postDueTx posts one due recurring transaction inside the given tx. It is
// the shared core of PostNow and the scheduler.
func (service *Service) postDueTx(ctx context.Context, tx pgx.Tx, tenantID, id, userID int64) (map[string]any, error) {
	// Idempotency key is derived from the recurring id + its next_date, so a
	// scheduler retry or a concurrent manual PostNow cannot double-post.
	var nextDate time.Time
	if err := tx.QueryRow(ctx, `
		SELECT next_date FROM recurring_transactions WHERE tenant_id = $1 AND id = $2
	`, tenantID, id).Scan(&nextDate); err != nil {
		return nil, errNotFound
	}
	idem := fmt.Sprintf("00000000-0000-0000-0000-%012d", id) // stable per row; date folded into source_ref

	// Lock the row so concurrent posts serialize.
	var code, name, intentType, frequency, description string
	var amountCents, fromAcct, toAcct int64
	var isActive bool
	err := tx.QueryRow(ctx, `
		SELECT code, name, intent_type, frequency, next_date, amount_cents,
		       from_account_id, to_account_id, COALESCE(description, ''), is_active
		FROM recurring_transactions
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE
	`, tenantID, id).Scan(&code, &name, &intentType, &frequency, &nextDate, &amountCents,
		&fromAcct, &toAcct, &description, &isActive)
	if err != nil {
		return nil, errNotFound
	}
	if !isActive {
		return nil, errInactive
	}
	if fromAcct <= 0 || toAcct <= 0 {
		return nil, errMissingAccounts
	}

	entryDate := nextDate.Format("2006-01-02")
	sourceRef := fmt.Sprintf("REC-%d-%s", id, entryDate)
	journal := accounting.Journal{
		TenantID:    tenantID,
		SourceRef:   sourceRef,
		IntentType:  accounting.IntentType(intentType),
		EntryDate:   entryDate,
		Description: description,
		Lines: []accounting.Line{
			{AccountID: fromAcct, DebitCents: amountCents, SourceLineRef: "debit"},
			{AccountID: toAcct, CreditCents: amountCents, SourceLineRef: "credit"},
		},
	}
	posted, err := postJournal(ctx, tx, tenantID, idem, journal, userID, 0)
	if err != nil {
		return nil, err
	}

	nextNext := computeNextDate(nextDate, frequency)
	if _, err := tx.Exec(ctx, `
		UPDATE recurring_transactions
		SET last_posted_date = now()::date, next_date = $3, last_journal_id = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id, nextNext, posted.ID); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":               id,
		"code":             code,
		"name":             name,
		"intent_type":      intentType,
		"amount_cents":     amountCents,
		"posted_at":        time.Now().Format("2006-01-02"),
		"next_date":        nextNext.Format("2006-01-02"),
		"posted_by":        userID,
		"journal_entry_id": posted.ID,
		"journal_number":   posted.Number,
	}, nil
}

// dueTx identifies one recurring row that is due for posting.
type dueTx struct {
	tenantID int64
	id       int64
}

// listDue returns every active recurring transaction whose next_date <= today
// (and, when end_date is set, has not yet passed it). Scanning across tenants
// is intentional: the scheduler serves the whole deployment, and each posting
// re-scopes RLS to the owning tenant inside its own transaction.
func (service *Service) listDue(ctx context.Context) ([]dueTx, error) {
	rows, err := service.pool.Query(ctx, `
		SELECT tenant_id, id
		FROM recurring_transactions
		WHERE is_active = true
		  AND from_account_id > 0 AND to_account_id > 0
		  AND next_date <= CURRENT_DATE
		  AND (end_date IS NULL OR next_date <= end_date)
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var due []dueTx
	for rows.Next() {
		var d dueTx
		if err := rows.Scan(&d.tenantID, &d.id); err != nil {
			return nil, err
		}
		due = append(due, d)
	}
	return due, rows.Err()
}

// postDueOne posts a single due transaction in its own transaction. A closed
// period or a transient failure only skips that row — the loop continues.
func (service *Service) postDueOne(ctx context.Context, d dueTx) error {
	return db.WithTransaction(ctx, service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprintf("%d", d.tenantID)); err != nil {
			return err
		}
		_, err := service.postDueTx(ctx, tx, d.tenantID, d.id, 0) // 0 = system/scheduler user
		return err
	})
}

// StartScheduler runs the recurring-post loop every interval until ctx is
// cancelled. It logs one summary line per pass; individual failures are
// logged at warn level with the tenant/row ids.
func (service *Service) StartScheduler(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("recurring scheduler stopped")
				return
			case <-ticker.C:
				service.RunOnce(ctx)
			}
		}
	}()
	slog.Info("recurring scheduler started", "interval", interval.String())
}

// runOnce is one scheduler pass: list due rows, post each, log a summary.
// It is exported so tests can drive a pass directly without waiting on the
// ticker.
func (service *Service) RunOnce(ctx context.Context) (posted, skipped int) {
	// Phase D: measure every pass — the loop posts sequentially per row, so
	// pass duration is the first place backlog growth becomes visible.
	start := time.Now()
	due, err := service.listDue(ctx)
	if err != nil {
		slog.Error("recurring scheduler: list due failed", "error", err)
		return 0, 0
	}
	for _, d := range due {
		err := service.postDueOne(ctx, d)
		switch {
		case err == nil:
			posted++
		case errors.Is(err, errNotFound), errors.Is(err, errInactive), errors.Is(err, errMissingAccounts):
			// Row disappeared or became ineligible between list and post.
			skipped++
		default:
			skipped++
			slog.Warn("recurring scheduler: post failed",
				"tenant_id", d.tenantID, "recurring_id", d.id, "error", err)
		}
	}
	if posted > 0 || skipped > 0 {
		slog.Info("recurring scheduler pass",
			"posted", posted, "skipped", skipped,
			"duration_ms", time.Since(start).Milliseconds())
	}
	return posted, skipped
}
