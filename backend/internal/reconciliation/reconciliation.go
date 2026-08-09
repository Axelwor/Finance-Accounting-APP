package reconciliation

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// Reconciliation (US-050)
// ---------------------------------------------------------------------------

// journalLineCandidate is a book-side cash/bank journal line eligible for
// matching against a statement line.
type journalLineCandidate struct {
	ID          int64  `json:"journal_line_id"`
	EntryID     int64  `json:"entry_id"`
	EntryNumber string `json:"entry_number"`
	EntryDate   string `json:"entry_date"`
	Description string `json:"description"`
	AmountCents int64  `json:"amount_cents"`
	Direction   string `json:"direction"` // "DEBIT" or "CREDIT"
	Matched     bool   `json:"is_matched"`
}

type statementLineRecon struct {
	ID                   int64  `json:"id"`
	LineNo               int    `json:"line_no"`
	TxDate               string `json:"tx_date"`
	Description          string `json:"description"`
	Reference            string `json:"reference"`
	AmountCents          int64  `json:"amount_cents"`
	MatchedJournalLineID int64  `json:"matched_journal_line_id,omitempty"`
	MatchStatus          string `json:"match_status"`
}

type reconciliationSummary struct {
	StatementBalanceCents  int64 `json:"statement_balance_cents"`
	BookBalanceCents       int64 `json:"book_balance_cents"`
	AdjustedBookCents      int64 `json:"adjusted_book_cents"`
	AdjustedStatementCents int64 `json:"adjusted_statement_cents"`
	DiffCents              int64 `json:"diff_cents"`
	MatchedCount           int   `json:"matched_count"`
	UnmatchedCount         int   `json:"unmatched_count"`
	TotalLines             int   `json:"total_lines"`
}

type reconciliationResponse struct {
	ID              int64                  `json:"id"`
	StatementID     int64                  `json:"statement_id"`
	BankAccountID   int64                  `json:"bank_account_id"`
	BankAccountName string                 `json:"bank_account_name"`
	ReconDate       string                 `json:"recon_date"`
	Status          string                 `json:"status"`
	Notes           string                 `json:"notes"`
	Summary         reconciliationSummary  `json:"summary"`
	StatementLines  []statementLineRecon   `json:"statement_lines"`
	BookCandidates  []journalLineCandidate `json:"book_candidates"`
}

type matchRequest struct {
	StatementLineID int64 `json:"statement_line_id"`
	JournalLineID   int64 `json:"journal_line_id"`
}

type unmatchRequest struct {
	StatementLineID int64 `json:"statement_line_id"`
}

// StartReconciliation handles POST /bank-statements/{id}/reconcile.
// It creates a DRAFT bank_reconciliations row for the statement (if none
// exists), runs auto-match by amount + date proximity (±3 days), and returns
// the reconciliation view.
func (service *Service) StartReconciliation(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	stmtID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var result *reconciliationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Load the statement + bank account.
		var bankAccountID int64
		var bankAccountName, stmtStatus string
		var stmtDate pgtype.Date
		var closingBalance int64
		err := tx.QueryRow(request.Context(), `
			SELECT bs.bank_account_id, a.name, bs.status, bs.statement_date, bs.closing_balance_cents
			FROM bank_statements bs
			JOIN accounts a ON a.tenant_id = bs.tenant_id AND a.id = bs.bank_account_id
			WHERE bs.tenant_id = $1 AND bs.id = $2
		`, tenant, stmtID).Scan(&bankAccountID, &bankAccountName, &stmtStatus, &stmtDate, &closingBalance)
		if err != nil {
			if isNoRows(err) {
				return validationError("bank statement not found")
			}
			return err
		}
		if stmtStatus == "VOID" {
			return validationError("cannot reconcile a voided statement")
		}

		// Find or create the DRAFT reconciliation for this statement.
		var reconID int64
		err = tx.QueryRow(request.Context(), `
			SELECT id FROM bank_reconciliations
			WHERE tenant_id = $1 AND statement_id = $2 AND status = 'DRAFT'
			ORDER BY id DESC LIMIT 1
		`, tenant, stmtID).Scan(&reconID)
		if err != nil {
			if !isNoRows(err) {
				return err
			}
			reconDateStr := dateString(stmtDate)
			if reconDateStr == "" {
				reconDateStr = time.Now().Format("2006-01-02")
			}
			rd, _ := parseDate(reconDateStr)
			err = tx.QueryRow(request.Context(), `
				INSERT INTO bank_reconciliations
					(tenant_id, statement_id, bank_account_id, recon_date,
					 book_balance_cents, statement_balance_cents,
					 adjusted_book_cents, adjusted_statement_cents, diff_cents, status, created_by)
				VALUES ($1, $2, $3, $4, 0, $5, 0, 0, 0, 'DRAFT', $6)
				RETURNING id
			`, tenant, stmtID, bankAccountID, rd, closingBalance, userID(request)).Scan(&reconID)
			if err != nil {
				return err
			}
		}

		// Mark the statement as RECONCILING.
		_, _ = tx.Exec(request.Context(), `
			UPDATE bank_statements SET status = 'RECONCILING', updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND status IN ('IMPORTED','RECONCILING')
		`, tenant, stmtID)

		// Auto-match: only currently-unmatched statement lines are considered,
		// and only journal lines not already matched to another statement line.
		if err := autoMatch(request.Context(), tx, tenant, bankAccountID); err != nil {
			return err
		}

		// Recompute balances + diff and persist them.
		if err := recomputeBalances(request.Context(), tx, tenant, reconID, stmtID, bankAccountID, closingBalance); err != nil {
			return err
		}

		result, err = loadReconciliation(request.Context(), tx, tenant, reconID)
		return err
	})
	if err != nil {
		if ve, ok := err.(*ValidationError); ok {
			writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", ve.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "RECONCILE_START_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// MatchLine handles POST /bank-reconciliations/{id}/match — manual match.
func (service *Service) MatchLine(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	reconID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req matchRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.StatementLineID <= 0 || req.JournalLineID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "statement_line_id and journal_line_id are required")
		return
	}

	var result *reconciliationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		stmtID, bankAccountID, closingBalance, status, err := loadReconHeader(request.Context(), tx, tenant, reconID)
		if err != nil {
			return err
		}
		if status != "DRAFT" {
			return validationError("reconciliation is not editable")
		}

		// Verify the journal line belongs to this tenant + bank account and is
		// not already matched to a different statement line.
		var owned int64
		err = tx.QueryRow(request.Context(), `
			SELECT COUNT(*) FROM journal_lines
			WHERE tenant_id = $1 AND id = $2 AND account_id = $3
		`, tenant, req.JournalLineID, bankAccountID).Scan(&owned)
		if err != nil {
			return err
		}
		if owned == 0 {
			return validationError("journal line does not belong to this bank account")
		}
		var alreadyMatched int64
		err = tx.QueryRow(request.Context(), `
			SELECT COUNT(*) FROM bank_statement_lines
			WHERE tenant_id = $1 AND matched_journal_line_id = $2
			  AND id <> $3
		`, tenant, req.JournalLineID, req.StatementLineID).Scan(&alreadyMatched)
		if err != nil {
			return err
		}
		if alreadyMatched > 0 {
			return validationError("journal line is already matched to another statement line")
		}

		_, err = tx.Exec(request.Context(), `
			UPDATE bank_statement_lines
			SET matched_journal_line_id = $1, match_status = 'MANUAL'
			WHERE tenant_id = $2 AND id = $3
		`, req.JournalLineID, tenant, req.StatementLineID)
		if err != nil {
			return err
		}

		if err := recomputeBalances(request.Context(), tx, tenant, reconID, stmtID, bankAccountID, closingBalance); err != nil {
			return err
		}
		result, err = loadReconciliation(request.Context(), tx, tenant, reconID)
		return err
	})
	if err != nil {
		if ve, ok := err.(*ValidationError); ok {
			writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", ve.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "RECONCILE_MATCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// UnmatchLine handles POST /bank-reconciliations/{id}/unmatch.
func (service *Service) UnmatchLine(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	reconID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req unmatchRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.StatementLineID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "statement_line_id is required")
		return
	}

	var result *reconciliationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		stmtID, bankAccountID, closingBalance, status, err := loadReconHeader(request.Context(), tx, tenant, reconID)
		if err != nil {
			return err
		}
		if status != "DRAFT" {
			return validationError("reconciliation is not editable")
		}
		_, err = tx.Exec(request.Context(), `
			UPDATE bank_statement_lines
			SET matched_journal_line_id = NULL, match_status = 'UNMATCHED'
			WHERE tenant_id = $1 AND id = $2
		`, tenant, req.StatementLineID)
		if err != nil {
			return err
		}
		if err := recomputeBalances(request.Context(), tx, tenant, reconID, stmtID, bankAccountID, closingBalance); err != nil {
			return err
		}
		result, err = loadReconciliation(request.Context(), tx, tenant, reconID)
		return err
	})
	if err != nil {
		if ve, ok := err.(*ValidationError); ok {
			writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", ve.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "RECONCILE_UNMATCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// CompleteReconciliation handles POST /bank-reconciliations/{id}/complete.
// Validates diff_cents == 0 (adjusted book = adjusted statement), then marks
// the reconciliation + statement RECONCILED.
func (service *Service) CompleteReconciliation(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	reconID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if _, err := idempotencyKey(request); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var result *reconciliationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		stmtID, bankAccountID, closingBalance, status, err := loadReconHeader(request.Context(), tx, tenant, reconID)
		if err != nil {
			return err
		}
		if status != "DRAFT" {
			return validationError("reconciliation is not editable")
		}

		if err := recomputeBalances(request.Context(), tx, tenant, reconID, stmtID, bankAccountID, closingBalance); err != nil {
			return err
		}

		var diff int64
		err = tx.QueryRow(request.Context(), `
			SELECT diff_cents FROM bank_reconciliations WHERE tenant_id = $1 AND id = $2
		`, tenant, reconID).Scan(&diff)
		if err != nil {
			return err
		}
		if diff != 0 {
			return validationError("cannot complete: adjusted book does not equal adjusted statement")
		}

		_, err = tx.Exec(request.Context(), `
			UPDATE bank_reconciliations
			SET status = 'RECONCILED', updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, reconID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(request.Context(), `
			UPDATE bank_statements
			SET status = 'RECONCILED', updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, stmtID)
		if err != nil {
			return err
		}

		result, err = loadReconciliation(request.Context(), tx, tenant, reconID)
		return err
	})
	if err != nil {
		if ve, ok := err.(*ValidationError); ok {
			writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", ve.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "RECONCILE_COMPLETE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// GetReconciliation handles GET /bank-reconciliations/{id}.
func (service *Service) GetReconciliation(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	reconID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *reconciliationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = loadReconciliation(request.Context(), tx, tenant, reconID)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "NOT_FOUND", "reconciliation not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "RECONCILE_GET_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Matching + balance computation
// ---------------------------------------------------------------------------

// autoMatch pairs unmatched statement lines with unmatched journal lines on
// the bank account by exact amount and date proximity (±3 days). The bank
// account is an asset, so a statement credit (deposit, amount_cents > 0)
// matches a journal debit on the bank line, and a statement debit
// (withdrawal, amount_cents < 0) matches a journal credit.
func autoMatch(ctx context.Context, tx pgx.Tx, tenantID, bankAccountID int64) error {
	rows, err := tx.Query(ctx, `
		SELECT id, tx_date, amount_cents
		FROM bank_statement_lines
		WHERE tenant_id = $1 AND match_status = 'UNMATCHED'
		ORDER BY line_no
	`, tenantID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pending struct {
		id     int64
		txDate time.Time
		amount int64
	}
	var lines []pending
	for rows.Next() {
		var p pending
		var d pgtype.Date
		if err := rows.Scan(&p.id, &d, &p.amount); err != nil {
			return err
		}
		if d.Valid {
			p.txDate = d.Time
		}
		lines = append(lines, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, line := range lines {
		// A statement line with amount_cents > 0 is a deposit -> bank debit.
		// A statement line with amount_cents < 0 is a withdrawal -> bank credit.
		wantDebit := line.amount > 0
		absAmount := line.amount
		if absAmount < 0 {
			absAmount = -absAmount
		}
		var journalLineID int64
		err := tx.QueryRow(ctx, `
			SELECT jl.id
			FROM journal_lines jl
			JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
			WHERE jl.tenant_id = $1
			  AND jl.account_id = $2
			  AND je.status = 'POSTED'
			  AND (
			        ( $3 AND jl.debit_cents = $4 )
			     OR ( NOT $3 AND jl.credit_cents = $4 )
			  )
			  AND NOT EXISTS (
			        SELECT 1 FROM bank_statement_lines bsl
			        WHERE bsl.tenant_id = jl.tenant_id
			          AND bsl.matched_journal_line_id = jl.id
			  )
			  AND ABS(je.entry_date - $5::date) <= 3
			ORDER BY ABS(je.entry_date - $5::date), jl.id
			LIMIT 1
		`, tenantID, bankAccountID, wantDebit, absAmount, line.txDate).Scan(&journalLineID)
		if err != nil {
			if isNoRows(err) {
				continue // no candidate for this line
			}
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE bank_statement_lines
			SET matched_journal_line_id = $1, match_status = 'MATCHED'
			WHERE tenant_id = $2 AND id = $3
		`, journalLineID, tenantID, line.id)
		if err != nil {
			return err
		}
	}
	return nil
}

// recomputeBalances recomputes the reconciliation's book/statement/adjusted
// balances and diff and persists them.
//
//   - statement_balance_cents = statement closing balance
//   - book_balance_cents      = sum of posted bank-account journal lines up to
//     and including the statement date
//   - adjusted_statement_cents = statement_balance_cents
//     (statement lines sum to the movement already in closing balance; we do
//     not double-count them)
//   - adjusted_book_cents      = book_balance_cents
//     (matched lines are already in the book; unmatched statement lines
//     represent timing differences the user resolves, not auto-adjustments)
//   - diff_cents = adjusted_book_cents - adjusted_statement_cents
func recomputeBalances(ctx context.Context, tx pgx.Tx, tenantID, reconID, stmtID, bankAccountID, closingBalance int64) error {
	// Book balance: running balance of the bank account up to the statement date.
	var stmtDate pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT statement_date FROM bank_statements WHERE tenant_id = $1 AND id = $2
	`, tenantID, stmtID).Scan(&stmtDate)
	if err != nil {
		return err
	}
	var bookBalance int64
	if stmtDate.Valid {
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
			FROM journal_lines jl
			JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
			WHERE jl.tenant_id = $1
			  AND jl.account_id = $2
			  AND je.status = 'POSTED'
			  AND je.entry_date <= $3::date
		`, tenantID, bankAccountID, stmtDate.Time).Scan(&bookBalance)
		if err != nil {
			return err
		}
	}

	adjustedBook := bookBalance
	adjustedStatement := closingBalance
	diff := adjustedBook - adjustedStatement

	_, err = tx.Exec(ctx, `
		UPDATE bank_reconciliations
		SET book_balance_cents = $1,
		    statement_balance_cents = $2,
		    adjusted_book_cents = $3,
		    adjusted_statement_cents = $4,
		    diff_cents = $5,
		    updated_at = now()
		WHERE tenant_id = $6 AND id = $7
	`, bookBalance, closingBalance, adjustedBook, adjustedStatement, diff, tenantID, reconID)
	return err
}

// loadReconHeader loads the statement_id, bank_account_id, closing balance,
// and status for a reconciliation.
func loadReconHeader(ctx context.Context, tx pgx.Tx, tenantID, reconID int64) (stmtID, bankAccountID, closingBalance int64, status string, err error) {
	err = tx.QueryRow(ctx, `
		SELECT r.statement_id, r.bank_account_id, bs.closing_balance_cents, r.status
		FROM bank_reconciliations r
		JOIN bank_statements bs ON bs.tenant_id = r.tenant_id AND bs.id = r.statement_id
		WHERE r.tenant_id = $1 AND r.id = $2
	`, tenantID, reconID).Scan(&stmtID, &bankAccountID, &closingBalance, &status)
	return
}

// loadReconciliation loads the full reconciliation view: header, summary,
// statement lines with candidate journal lines for unmatched rows.
func loadReconciliation(ctx context.Context, tx pgx.Tx, tenantID, reconID int64) (*reconciliationResponse, error) {
	var recon reconciliationResponse
	var reconDate pgtype.Date
	var notes pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT r.id, r.statement_id, r.bank_account_id, a.name,
		       r.recon_date, r.status, r.notes,
		       r.statement_balance_cents, r.book_balance_cents,
		       r.adjusted_book_cents, r.adjusted_statement_cents, r.diff_cents
		FROM bank_reconciliations r
		JOIN accounts a ON a.tenant_id = r.tenant_id AND a.id = r.bank_account_id
		WHERE r.tenant_id = $1 AND r.id = $2
	`, tenantID, reconID).Scan(
		&recon.ID, &recon.StatementID, &recon.BankAccountID, &recon.BankAccountName,
		&reconDate, &recon.Status, &notes,
		&recon.Summary.StatementBalanceCents, &recon.Summary.BookBalanceCents,
		&recon.Summary.AdjustedBookCents, &recon.Summary.AdjustedStatementCents,
		&recon.Summary.DiffCents,
	)
	if err != nil {
		return nil, err
	}
	recon.ReconDate = dateString(reconDate)
	recon.Notes = textValue(notes)

	// Load statement lines.
	stmtID := recon.StatementID
	bankAccountID := recon.BankAccountID
	rows, err := tx.Query(ctx, `
		SELECT id, line_no, tx_date, description, reference, amount_cents,
		       matched_journal_line_id, match_status
		FROM bank_statement_lines
		WHERE tenant_id = $1 AND statement_id = $2
		ORDER BY line_no
	`, tenantID, stmtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recon.StatementLines = []statementLineRecon{}
	for rows.Next() {
		var line statementLineRecon
		var txDate pgtype.Date
		var desc, ref pgtype.Text
		var matchedID pgtype.Int8
		if err := rows.Scan(&line.ID, &line.LineNo, &txDate, &desc, &ref,
			&line.AmountCents, &matchedID, &line.MatchStatus); err != nil {
			return nil, err
		}
		line.TxDate = dateString(txDate)
		line.Description = textValue(desc)
		line.Reference = textValue(ref)
		if matchedID.Valid {
			line.MatchedJournalLineID = matchedID.Int64
		}
		recon.StatementLines = append(recon.StatementLines, line)
		recon.Summary.TotalLines++
		if line.MatchStatus == "UNMATCHED" {
			recon.Summary.UnmatchedCount++
		} else {
			recon.Summary.MatchedCount++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load the flat book-candidates panel: all posted bank-account journal
	// lines on/around the statement date, each flagged matched/unmatched.
	bookCands, err := allBookCandidates(ctx, tx, tenantID, stmtID, bankAccountID)
	if err != nil {
		return nil, err
	}
	recon.BookCandidates = bookCands
	return &recon, nil
}

// allBookCandidates returns all posted journal lines on the bank account whose
// entry date falls within ±60 days of the statement date, each flagged with
// whether it is already matched to a statement line. The matched flag lets the
// UI disable already-paired book rows.
func allBookCandidates(ctx context.Context, tx pgx.Tx, tenantID, stmtID, bankAccountID int64) ([]journalLineCandidate, error) {
	var stmtDate pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT statement_date FROM bank_statements WHERE tenant_id = $1 AND id = $2
	`, tenantID, stmtID).Scan(&stmtDate)
	if err != nil {
		return nil, err
	}
	if !stmtDate.Valid {
		return []journalLineCandidate{}, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT jl.id, jl.entry_id, je.number, je.entry_date, jl.description,
		       CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE jl.credit_cents END AS amount,
		       CASE WHEN jl.debit_cents > 0 THEN 'DEBIT' ELSE 'CREDIT' END AS direction,
		       EXISTS (
		         SELECT 1 FROM bank_statement_lines bsl
		         WHERE bsl.tenant_id = jl.tenant_id
		           AND bsl.matched_journal_line_id = jl.id
		       ) AS is_matched
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		WHERE jl.tenant_id = $1
		  AND jl.account_id = $2
		  AND je.status = 'POSTED'
		  AND ABS(je.entry_date - $3::date) <= 60
		ORDER BY je.entry_date DESC, jl.id DESC
		LIMIT 200
	`, tenantID, bankAccountID, stmtDate.Time)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cands := []journalLineCandidate{}
	for rows.Next() {
		var c journalLineCandidate
		var entryDate pgtype.Date
		var desc pgtype.Text
		if err := rows.Scan(&c.ID, &c.EntryID, &c.EntryNumber, &entryDate, &desc, &c.AmountCents, &c.Direction, &c.Matched); err != nil {
			return nil, err
		}
		c.EntryDate = dateString(entryDate)
		c.Description = textValue(desc)
		cands = append(cands, c)
	}
	return cands, rows.Err()
}
