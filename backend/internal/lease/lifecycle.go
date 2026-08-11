package lease

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// m-014: Lease modification and termination.
//
// Termination derecognises the remaining liability and the RoU carrying
// amount; any difference settles to gain/loss on disposal (4903 / 5903).
// Modification re-measures the lease to the new present value and posts an
// adjustment moving both the RoU asset and the liability by the delta.

// TerminateLeaseContract derecognises an ACTIVE lease.
func (service *Service) TerminateLeaseContract(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	leaseID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "lease id is required")
		return
	}
	var req struct {
		TerminationDate string `json:"termination_date"`
		Description     string `json:"description"`
	}
	_ = decodeJSON(request, &req)
	uid := userID(request)

	var result map[string]any
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID: tenant, IdempotencyKey: uuidValue(idem),
		}); err == nil {
			result = map[string]any{"journal_entry_id": existing.ID, "number": existing.Number, "replayed": true}
			return nil
		} else if !isNoRows(err) {
			return err
		}

		lease, err := fetchLeaseByID(request.Context(), tx, tenant, leaseID)
		if err != nil {
			return fmt.Errorf("lease not found: %w", err)
		}
		if lease.Status != statusActive {
			return fmt.Errorf("lease %s is %s; only ACTIVE leases can be terminated", lease.Number, lease.Status)
		}
		termDate := req.TerminationDate
		if termDate == "" {
			return fmt.Errorf("termination_date is required")
		}
		if _, err := parseDate(termDate); err != nil {
			return fmt.Errorf("invalid termination_date: %w", err)
		}

		remainingLiability := currentLiabilityCents(request.Context(), tx, tenant, leaseID, lease)
		accumDep := accumulatedRouDepCents(request.Context(), tx, tenant, leaseID)
		rouCost := lease.InitialROUCents

		accumDepAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, accumRouDepAccountCode)
		if err != nil {
			return err
		}

		lines := []accounting.Line{
			{AccountID: lease.LeaseLiabilityAccountID, DebitCents: remainingLiability, SourceLineRef: "liability"},
			{AccountID: accumDepAccountID, DebitCents: accumDep, SourceLineRef: "accum-dep"},
			{AccountID: lease.ROUAssetAccountID, CreditCents: rouCost, SourceLineRef: "rou"},
		}
		// Balancing figure: debits remove liability+accum, credit removes RoU
		// cost. The difference settles to gain (Cr 4903) or loss (Dr 5903).
		debits := remainingLiability + accumDep
		credits := rouCost
		if debits > credits {
			gainAcct, err := resolveAccountByCode(request.Context(), tx, tenant, "4903")
			if err != nil {
				return err
			}
			lines = append(lines, accounting.Line{AccountID: gainAcct, CreditCents: debits - credits, SourceLineRef: "gain"})
		} else if credits > debits {
			lossAcct, err := resolveAccountByCode(request.Context(), tx, tenant, "5903")
			if err != nil {
				return err
			}
			lines = append(lines, accounting.Line{AccountID: lossAcct, DebitCents: credits - debits, SourceLineRef: "loss"})
		}

		if err := accounting.BalanceCheck(lines); err != nil {
			return fmt.Errorf("termination journal not balanced: %w", err)
		}

		entryID, number, err := service.postLeaseJournal(request.Context(), tx, tenant, uid, leaseID, idem, termDate,
			fmt.Sprintf("LEASE-TERM-%d", leaseID), accounting.IntentType("LEASE_TERMINATION"),
			"Lease termination: "+lease.Number, lines)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE lease_contracts SET status = $3, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, leaseID, statusTerminated); err != nil {
			return err
		}
		result = map[string]any{
			"lease_id":            leaseID,
			"number":              lease.Number,
			"status":              statusTerminated,
			"journal_entry_id":    entryID,
			"journal_number":      number,
			"remaining_liability": remainingLiability,
			"accumulated_dep":     accumDep,
			"rou_cost":            rouCost,
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "termination already posted with this idempotency key")
			return
		}
		writeError(writer, http.StatusBadRequest, "TERMINATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ModifyLeaseContract re-measures an ACTIVE lease after a change in term or
// payment amount, posting an adjustment to move RoU asset and liability to the
// new present value.
func (service *Service) ModifyLeaseContract(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	leaseID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "lease id is required")
		return
	}
	var req struct {
		NewPaymentAmountCents int64  `json:"new_payment_amount_cents"`
		NewTotalPayments      int    `json:"new_total_payments"`
		EffectiveDate         string `json:"effective_date"`
		Description           string `json:"description"`
	}
	_ = decodeJSON(request, &req)
	uid := userID(request)

	var result map[string]any
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		if existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID: tenant, IdempotencyKey: uuidValue(idem),
		}); err == nil {
			result = map[string]any{"journal_entry_id": existing.ID, "number": existing.Number, "replayed": true}
			return nil
		} else if !isNoRows(err) {
			return err
		}

		lease, err := fetchLeaseByID(request.Context(), tx, tenant, leaseID)
		if err != nil {
			return fmt.Errorf("lease not found: %w", err)
		}
		if lease.Status != statusActive {
			return fmt.Errorf("lease %s is %s; only ACTIVE leases can be modified", lease.Number, lease.Status)
		}
		if req.NewPaymentAmountCents <= 0 || req.NewTotalPayments <= 0 {
			return fmt.Errorf("new_payment_amount_cents and new_total_payments must be > 0")
		}
		if req.EffectiveDate == "" {
			return fmt.Errorf("effective_date is required")
		}
		if _, err := parseDate(req.EffectiveDate); err != nil {
			return fmt.Errorf("invalid effective_date: %w", err)
		}

		rateF, err := parseRate(lease.DiscountRate)
		if err != nil {
			return fmt.Errorf("invalid discount rate %q: %w", lease.DiscountRate, err)
		}
		newPV := presentValueCents(req.NewPaymentAmountCents, rateF, req.NewTotalPayments)
		if newPV <= 0 {
			return fmt.Errorf("new present value must be > 0")
		}

		currentLiability := currentLiabilityCents(request.Context(), tx, tenant, leaseID, lease)
		currentROU := lease.InitialROUCents

		delta := newPV - currentLiability
		if delta == 0 {
			return fmt.Errorf("modification does not change the present value; nothing to adjust")
		}
		var lines []accounting.Line
		if delta > 0 {
			lines = []accounting.Line{
				{AccountID: lease.ROUAssetAccountID, DebitCents: delta, SourceLineRef: "rou-incr"},
				{AccountID: lease.LeaseLiabilityAccountID, CreditCents: delta, SourceLineRef: "liab-incr"},
			}
		} else {
			absDelta := -delta
			if absDelta > currentROU {
				return fmt.Errorf("modification decrease %d exceeds RoU carrying amount %d", absDelta, currentROU)
			}
			lines = []accounting.Line{
				{AccountID: lease.LeaseLiabilityAccountID, DebitCents: absDelta, SourceLineRef: "liab-decr"},
				{AccountID: lease.ROUAssetAccountID, CreditCents: absDelta, SourceLineRef: "rou-decr"},
			}
		}
		if err := accounting.BalanceCheck(lines); err != nil {
			return fmt.Errorf("modification journal not balanced: %w", err)
		}

		entryID, number, err := service.postLeaseJournal(request.Context(), tx, tenant, uid, leaseID, idem, req.EffectiveDate,
			fmt.Sprintf("LEASE-MOD-%d", leaseID), accounting.IntentType("LEASE_MODIFICATION"),
			"Lease modification: "+lease.Number, lines)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE lease_contracts
			SET payment_amount_cents = $3, total_payments = $4, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, leaseID, req.NewPaymentAmountCents, req.NewTotalPayments); err != nil {
			return err
		}
		result = map[string]any{
			"lease_id":           leaseID,
			"number":             lease.Number,
			"journal_entry_id":   entryID,
			"journal_number":     number,
			"delta_cents":        delta,
			"new_pv_cents":       newPV,
			"new_payment_cents":  req.NewPaymentAmountCents,
			"new_total_payments": req.NewTotalPayments,
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "modification already posted with this idempotency key")
			return
		}
		writeError(writer, http.StatusBadRequest, "MODIFY_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// postLeaseJournal builds, hashes, and inserts a lease journal entry + lines,
// advancing the hash chain. Returns the new entry id and journal number.
func (service *Service) postLeaseJournal(ctx context.Context, tx pgx.Tx, tenant, uid, leaseID int64, idem, entryDate, sourceRef string, intent accounting.IntentType, description string, lines []accounting.Line) (int64, string, error) {
	head, err := lockOrSeedHead(ctx, tx, tenant)
	if err != nil {
		return 0, "", err
	}
	journal := accounting.Journal{
		TenantID:     tenant,
		SourceRef:    sourceRef,
		IntentType:   intent,
		EntryDate:    entryDate,
		Description:  description,
		PreviousHash: head.LastHash,
		Lines:        lines,
	}
	journal.Hash = hashJournal(journal)

	periodID, err := resolvePeriod(ctx, tx, tenant, entryDate)
	if err != nil {
		return 0, "", err
	}
	number, err := nextJournalNumber(ctx, tx, tenant)
	if err != nil {
		return 0, "", err
	}
	var entryID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, tenant, number, entryDate, periodID, description,
		sourceRef, string(intent), idem, journal.Hash, journal.PreviousHash, int8Value(uid)).Scan(&entryID)
	if err != nil {
		return 0, "", err
	}
	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, tenant, entryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
			return 0, "", err
		}
	}
	if err := upsertHead(ctx, tx, tenant, entryID, journal.Hash); err != nil {
		return 0, "", err
	}
	return entryID, number, nil
}

// currentLiabilityCents reads the remaining lease liability: the last posted
// payment's remaining balance, or the initial liability if none posted.
func currentLiabilityCents(ctx context.Context, tx pgx.Tx, tenant, leaseID int64, lease *leaseContractResponse) int64 {
	var remaining pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT remaining_liability_cents
		FROM lease_payments
		WHERE tenant_id = $1 AND lease_id = $2 AND posted = true
		ORDER BY payment_no DESC LIMIT 1
	`, tenant, leaseID).Scan(&remaining)
	if err != nil || !remaining.Valid {
		return lease.InitialLiabilityCents
	}
	return remaining.Int64
}

// accumulatedRouDepCents reads the accumulated RoU depreciation from the
// depreciation log (sum of depreciation_cents).
func accumulatedRouDepCents(ctx context.Context, tx pgx.Tx, tenant, leaseID int64) int64 {
	var total pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(depreciation_cents), 0)
		FROM lease_depreciation_log
		WHERE tenant_id = $1 AND lease_id = $2
	`, tenant, leaseID).Scan(&total)
	if err != nil || !total.Valid {
		return 0
	}
	return total.Int64
}

// parseRate parses a decimal rate string ("0.01") into float64.
func parseRate(raw string) (float64, error) {
	return strconv.ParseFloat(raw, 64)
}
