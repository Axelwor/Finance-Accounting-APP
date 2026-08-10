package production

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// PostOverheadVariance handles POST /overhead-variance (M-012).
//
// At period close the applied overhead credited to 4902 (Applied Overhead)
// during job costing is reconciled against the actual overhead debited to the
// real expense accounts. This endpoint posts the difference so 4902 nets to
// zero and the variance lands in the P&L:
//
//	Over-applied  (credit balance on 4902): Dr 4902 / Cr 4908 Variance Gain
//	Under-applied (debit balance on 4902):  Dr 5908 Variance Loss / Cr 4902
//
// Idempotent via Idempotency-Key; the caller should derive a deterministic
// key per period (e.g. UUID seeded from tenant + period).
func (service *Service) PostOverheadVariance(writer http.ResponseWriter, request *http.Request) {
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
	uid := userID(request)

	var req struct {
		EntryDate string `json:"entry_date"`
	}
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.EntryDate == "" {
		req.EntryDate = time.Now().UTC().Format("2006-01-02")
	}

	var result struct {
		JournalEntryID int64  `json:"journal_entry_id"`
		Number         string `json:"number"`
		VarianceCents  int64  `json:"variance_cents"`
		Direction      string `json:"direction"`
	}

	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Idempotent replay.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			result.JournalEntryID = existing.ID
			result.Number = existing.Number
			result.Direction = "REPLAY"
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Balance of the applied-overhead account: credits - debits.
		// A positive balance means over-applied (too much overhead absorbed),
		// a negative balance means under-applied.
		appliedAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, appliedOverheadCode)
		if err != nil {
			return err
		}
		var balance int64
		err = tx.QueryRow(request.Context(), `
			SELECT COALESCE(SUM(jl.credit_cents - jl.debit_cents), 0)
			FROM journal_lines jl
			JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
			WHERE jl.tenant_id = $1 AND jl.account_id = $2 AND je.status = 'POSTED'
		`, tenant, appliedAcctID).Scan(&balance)
		if err != nil {
			return err
		}
		if balance == 0 {
			return fmt.Errorf("no applied overhead to reconcile (4902 balance is 0)")
		}

		var lines []accounting.Line
		if balance > 0 {
			// Over-applied: clear 4902 by debiting it, credit the gain account.
			gainAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, varianceGainAccountCode)
			if err != nil {
				return err
			}
			lines = []accounting.Line{
				{AccountID: appliedAcctID, DebitCents: balance, SourceLineRef: "applied-1"},
				{AccountID: gainAcctID, CreditCents: balance, SourceLineRef: "vgain-1"},
			}
			result.Direction = "OVER_APPLIED"
		} else {
			// Under-applied: credit 4902 to clear it, debit the loss account.
			lossAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, varianceLossAccountCode)
			if err != nil {
				return err
			}
			magnitude := -balance
			lines = []accounting.Line{
				{AccountID: lossAcctID, DebitCents: magnitude, SourceLineRef: "vloss-1"},
				{AccountID: appliedAcctID, CreditCents: magnitude, SourceLineRef: "applied-1"},
			}
			result.Direction = "UNDER_APPLIED"
			result.VarianceCents = magnitude
		}
		if result.Direction == "OVER_APPLIED" {
			result.VarianceCents = balance
		}

		if err := accounting.BalanceCheck(lines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("OVERHEAD-VARIANCE-%s", req.EntryDate)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("OVERHEAD_VARIANCE"),
			EntryDate:   req.EntryDate,
			Description: fmt.Sprintf("Overhead variance reconciliation (%s)", result.Direction),
			Lines:       lines,
		}

		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashJobJournal(journal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, req.EntryDate)
		if err != nil {
			return err
		}
		jrnNumber, err := nextJournalNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		var entryID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, tenant, jrnNumber, req.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem,
			journal.Hash, journal.PreviousHash, int8Value(uid)).Scan(&entryID)
		if err != nil {
			return err
		}
		for _, line := range lines {
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, tenant, entryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
				return err
			}
		}
		if err := upsertHead(request.Context(), tx, tenant, entryID, journal.Hash); err != nil {
			return err
		}
		result.JournalEntryID = entryID
		result.Number = jrnNumber
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "overhead variance already posted with this idempotency key")
			return
		}
		if strings.Contains(err.Error(), "no applied overhead") {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "OVERHEAD_VARIANCE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}
