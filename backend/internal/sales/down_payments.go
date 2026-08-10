package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// depositAccountCode is the seeded "Customer Deposit" account (2201) used as
// the credit side of a down payment journal. The seed provisions it on
// registration; migration 000006 adds it for existing tenants.
const depositAccountCode = "2201"

// CreateDownPaymentRequest is the POST /sales-orders/{id}/down-payments body.
type CreateDownPaymentRequest struct {
	CashAccountID int64  `json:"cash_account_id"`
	AmountCents   int64  `json:"amount_cents"`
	DPDate        string `json:"dp_date"`
	Description   string `json:"description"`
}

type dpResponse struct {
	ID             int64  `json:"id"`
	Number         string `json:"number"`
	OrderID        int64  `json:"order_id"`
	JournalEntryID int64  `json:"journal_entry_id,omitempty"`
	AmountCents    int64  `json:"amount_cents"`
	CashAccountID  int64  `json:"cash_account_id"`
	DPDate         string `json:"dp_date"`
	Description    string `json:"description"`
	Status         string `json:"status"`
}

// CreateDP posts a down payment for a sales order. DP posts a journal:
// Dr Cash/Bank / Cr 2201 Customer Deposit. DP must not exceed SO total minus
// existing DPs. Idempotent via the Idempotency-Key header.
func (service *Service) CreateDP(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	orderID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateDownPaymentRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateDPRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	userID := userID(request)

	var result dpResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			dp, err := service.findDPByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = dp
			return nil
		} else if !isNoRows(err) {
			return err
		}

		var soTotal, dpReceived int64
		var soStatus string
		// FOR UPDATE serializes concurrent DP postings for the same order so
		// the accumulation check below cannot be raced (M-006).
		if err := tx.QueryRow(request.Context(), `
			SELECT total_cents, dp_received_cents, status
			FROM sales_orders WHERE tenant_id = $1 AND id = $2
			FOR UPDATE
		`, tenant, orderID).Scan(&soTotal, &dpReceived, &soStatus); err != nil {
			return err
		}
		if soStatus != soConfirmed {
			return fmt.Errorf("order is %s, not CONFIRMED", soStatus)
		}
		if req.AmountCents <= 0 {
			return fmt.Errorf("amount_cents must be > 0")
		}
		if dpReceived+req.AmountCents > soTotal {
			return dpOverflowError{max: soTotal - dpReceived}
		}

		cashAccount, err := loadSalesAccount(request.Context(), tx, tenant, req.CashAccountID)
		if err != nil {
			return err
		}
		depositAccountID, err := resolveDepositAccount(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		depositAccount, err := loadSalesAccount(request.Context(), tx, tenant, depositAccountID)
		if err != nil {
			return err
		}

		lines := []accounting.Line{
			{AccountID: cashAccount.ID, DebitCents: req.AmountCents, SourceLineRef: "cash"},
			{AccountID: depositAccount.ID, CreditCents: req.AmountCents, SourceLineRef: "deposit"},
		}
		if err := accounting.BalanceCheck(lines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("DP-%d-%d", orderID, dpReceived+req.AmountCents)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("SALES_DOWN_PAYMENT"),
			EntryDate:   req.DPDate,
			Description: req.Description,
			Lines:       lines,
		}

		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashDP(journal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, journal.EntryDate)
		if err != nil {
			return err
		}
		number, err := nextJournalNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}

		var entryID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, tenant, number, journal.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem,
			journal.Hash, journal.PreviousHash, int8Value(userID)).Scan(&entryID)
		if err != nil {
			return err
		}
		for _, line := range journal.Lines {
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
		if err := insertOutbox(request.Context(), tx, tenant, "dp.posted", dpPayload(journal, entryID, number)); err != nil {
			return err
		}

		dpNumber, err := nextDPNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		dpDate, err := parseDate(req.DPDate)
		if err != nil {
			return err
		}
		var dpID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO sales_down_payments
				(tenant_id, number, order_id, journal_entry_id, amount_cents,
				 cash_account_id, deposit_account_id, dp_date, description,
				 status, idempotency_key, source_ref, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'RECEIVED', $10, $11, $12)
			RETURNING id
		`, tenant, dpNumber, orderID, entryID, req.AmountCents,
			req.CashAccountID, depositAccountID, dpDate,
			textValueOptional(req.Description), idem, sourceRef,
			int8Value(userID)).Scan(&dpID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE sales_orders SET dp_received_cents = dp_received_cents + $1, updated_at = now()
			WHERE tenant_id = $2 AND id = $3
		`, req.AmountCents, tenant, orderID); err != nil {
			return err
		}
		result = dpResponse{
			ID:             dpID,
			Number:         dpNumber,
			OrderID:        orderID,
			JournalEntryID: entryID,
			AmountCents:    req.AmountCents,
			CashAccountID:  req.CashAccountID,
			DPDate:         req.DPDate,
			Description:    req.Description,
			Status:         "RECEIVED",
		}
		return nil
	})
	if err != nil {
		status, code, message := dpErrorFor(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListDPs returns the down payments for a sales order.
func (service *Service) ListDPs(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	orderID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var results []dpResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		rows, err := tx.Query(request.Context(), `
			SELECT id, number, order_id, journal_entry_id, amount_cents,
			       cash_account_id, dp_date, description, status
			FROM sales_down_payments
			WHERE tenant_id = $1 AND order_id = $2
			ORDER BY dp_date, id
		`, tenant, orderID)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []dpResponse{}
		for rows.Next() {
			var dp dpResponse
			var dpDate pgtype.Date
			var journalID pgtype.Int8
			var desc pgtype.Text
			if err := rows.Scan(&dp.ID, &dp.Number, &dp.OrderID, &journalID,
				&dp.AmountCents, &dp.CashAccountID, &dpDate, &desc, &dp.Status); err != nil {
				return err
			}
			dp.DPDate = dateString(dpDate)
			if journalID.Valid {
				dp.JournalEntryID = journalID.Int64
			}
			dp.Description = textValue(desc)
			results = append(results, dp)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DP_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// RefundDP reverses a down payment: Dr 2201 / Cr Cash (reversal of the
// original DP journal). The DP status becomes REFUNDED and the SO
// dp_received_cents is reduced.
func (service *Service) RefundDP(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	dpID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	userID := userID(request)

	var result map[string]any
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var dpNumber string
		var orderID, journalEntryID, amountCents int64
		var dpStatus string
		err := tx.QueryRow(request.Context(), `
			SELECT number, order_id, journal_entry_id, amount_cents, status
			FROM sales_down_payments WHERE tenant_id = $1 AND id = $2
		`, tenant, dpID).Scan(&dpNumber, &orderID, &journalEntryID, &amountCents, &dpStatus)
		if err != nil {
			return err
		}
		if dpStatus != "RECEIVED" {
			return fmt.Errorf("down payment is %s, not RECEIVED", dpStatus)
		}
		if journalEntryID == 0 {
			return fmt.Errorf("down payment has no linked journal")
		}

		lines, err := loadJournalLines(request.Context(), tx, tenant, journalEntryID)
		if err != nil {
			return err
		}
		reversed := make([]accounting.Line, len(lines))
		for i, line := range lines {
			reversed[i] = accounting.Line{
				AccountID:     line.AccountID,
				DebitCents:    line.CreditCents,
				CreditCents:   line.DebitCents,
				SourceLineRef: "rev-" + line.SourceLineRef,
			}
		}
		if err := accounting.BalanceCheck(reversed); err != nil {
			return err
		}
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   fmt.Sprintf("DP-REFUND-%d", dpID),
			IntentType:  accounting.IntentType("SALES_DP_REFUND"),
			EntryDate:   time.Now().Format("2006-01-02"),
			Description: "Down payment refund: " + dpNumber,
			Lines:       reversed,
		}
		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashDP(journal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, journal.EntryDate)
		if err != nil {
			return err
		}
		number, err := nextJournalNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(request.Context(), `SELECT set_config('app.void_context', '1', true)`); err != nil {
			return err
		}
		var reversalID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by, reversal_of_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id
		`, tenant, number, journal.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem,
			journal.Hash, journal.PreviousHash, int8Value(userID), journalEntryID).Scan(&reversalID)
		if err != nil {
			return err
		}
		for _, line := range journal.Lines {
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, tenant, reversalID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE journal_entries SET status = 'VOID', void_reason = 'refunded', voided_by = $1, voided_at = now()
			WHERE tenant_id = $2 AND id = $3
		`, userID, tenant, journalEntryID); err != nil {
			return err
		}
		if err := upsertHead(request.Context(), tx, tenant, reversalID, journal.Hash); err != nil {
			return err
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE sales_down_payments SET status = 'REFUNDED', updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, dpID); err != nil {
			return err
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE sales_orders SET dp_received_cents = GREATEST(dp_received_cents - $1, 0), updated_at = now()
			WHERE tenant_id = $2 AND id = $3
		`, amountCents, tenant, orderID); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "dp.refunded", mustJSON(map[string]any{"dp_id": dpID, "journal_id": reversalID, "number": number})); err != nil {
			return err
		}
		result = map[string]any{
			"dp_id":           dpID,
			"status":          "REFUNDED",
			"journal_id":      reversalID,
			"voided_entry_id": journalEntryID,
			"number":          number,
		}
		return nil
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "DP_NOT_FOUND", "down payment not found")
			return
		}
		writeError(writer, http.StatusBadRequest, "DP_REFUND_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// DP helpers
// ---------------------------------------------------------------------------

func validateDPRequest(req CreateDownPaymentRequest) (string, string) {
	if req.CashAccountID <= 0 {
		return "INVALID_REQUEST", "cash_account_id is required"
	}
	if req.AmountCents <= 0 {
		return "INVALID_REQUEST", "amount_cents must be positive"
	}
	if !validDate(req.DPDate) {
		return "INVALID_REQUEST", "dp_date must be a valid date in YYYY-MM-DD format"
	}
	return "", ""
}

type dpOverflowError struct{ max int64 }

func (e dpOverflowError) Error() string {
	return fmt.Sprintf("down payment exceeds remaining order total (%d cents)", e.max)
}

func dpErrorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "ORDER_NOT_FOUND", "sales order not found"
	}
	var overflow dpOverflowError
	if errors.As(err, &overflow) {
		return http.StatusConflict, "DP_EXCEEDS_ORDER", overflow.Error()
	}
	return http.StatusInternalServerError, "DP_CREATE_FAILED", err.Error()
}

func loadSalesAccount(ctx context.Context, tx pgx.Tx, tenantID, accountID int64) (db.Account, error) {
	var row db.Account
	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, code, name, report_group, account_type,
		       parent_id, is_group, is_active, valid_from, valid_to
		FROM accounts
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, accountID).Scan(
		&row.ID, &row.TenantID, &row.Code, &row.Name, &row.ReportGroup, &row.AccountType,
		&row.ParentID, &row.IsGroup, &row.IsActive, &row.ValidFrom, &row.ValidTo)
	return row, err
}

func resolveDepositAccount(ctx context.Context, tx pgx.Tx, tenantID int64) (int64, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2
	`, tenantID, depositAccountCode).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("customer deposit account %s not found: %w", depositAccountCode, err)
	}
	return accountID, nil
}

func (service *Service) findDPByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (dpResponse, error) {
	var dp dpResponse
	var dpDate pgtype.Date
	var journalIDOut pgtype.Int8
	var desc pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, number, order_id, journal_entry_id, amount_cents,
		       cash_account_id, dp_date, description, status
		FROM sales_down_payments
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&dp.ID, &dp.Number, &dp.OrderID, &journalIDOut,
		&dp.AmountCents, &dp.CashAccountID, &dpDate, &desc, &dp.Status)
	if err != nil {
		return dpResponse{}, err
	}
	dp.DPDate = dateString(dpDate)
	if journalIDOut.Valid {
		dp.JournalEntryID = journalIDOut.Int64
	}
	dp.Description = textValue(desc)
	return dp, nil
}

func nextDPNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'DP', 'DP', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}

func loadJournalLines(ctx context.Context, tx pgx.Tx, tenantID, entryID int64) ([]accounting.Line, error) {
	rows, err := tx.Query(ctx, `
		SELECT account_id, debit_cents, credit_cents, source_line_ref
		FROM journal_lines
		WHERE tenant_id = $1 AND entry_id = $2
	`, tenantID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []accounting.Line
	for rows.Next() {
		var line accounting.Line
		if err := rows.Scan(&line.AccountID, &line.DebitCents, &line.CreditCents, &line.SourceLineRef); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func lockOrSeedHead(ctx context.Context, tx pgx.Tx, tenantID int64) (db.LedgerChainHead, error) {
	head, err := db.New(tx).LockLedgerChainHead(ctx, tenantID)
	if err == nil {
		return head, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, NULL, 'genesis') ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID); err != nil {
		return db.LedgerChainHead{}, err
	}
	return db.New(tx).LockLedgerChainHead(ctx, tenantID)
}

func resolvePeriod(ctx context.Context, tx pgx.Tx, tenantID int64, date string) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounting_periods
		WHERE tenant_id = $1 AND $2::date BETWEEN period_start AND period_end
		  AND status IN ('OPEN', 'REOPENED')
		ORDER BY period_start DESC LIMIT 1
	`, tenantID, date).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("entry date is outside an open period: %w", err)
	}
	return periodID, nil
}

func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'JRN', 'JRN', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}

func upsertHead(ctx context.Context, tx pgx.Tx, tenantID, lastJournalID int64, lastHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE
		SET last_journal_id = EXCLUDED.last_journal_id, last_hash = EXCLUDED.last_hash, updated_at = now()
	`, tenantID, lastJournalID, lastHash)
	return err
}

func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID int64, topic string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, topic, payload)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, topic, payload)
	return err
}

func dpPayload(journal accounting.Journal, entryID int64, number string) []byte {
	return mustJSON(map[string]any{
		"journal_id":  entryID,
		"number":      number,
		"source_ref":  journal.SourceRef,
		"intent_type": string(journal.IntentType),
		"entry_date":  journal.EntryDate,
		"hash":        journal.Hash,
	})
}

func hashDP(journal accounting.Journal) string {
	return accounting.HashJournal(journal)
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func int8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func uuidValue(raw string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(raw)
	return value
}

// idempotencyKey validates the required Idempotency-Key header and returns it
// as a string only if it parses as a valid UUID.
func idempotencyKey(request *http.Request) (string, error) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", errors.New("Idempotency-Key header is required")
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(key); err != nil {
		return "", errors.New("Idempotency-Key must be a UUID")
	}
	return key, nil
}
