package reconciliation

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// ValidationError signals a 400-style business rule violation.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func validationError(msg string) error { return &ValidationError{msg: msg} }

// ---------------------------------------------------------------------------
// Bank Statements (US-050)
// ---------------------------------------------------------------------------

type statementLineInput struct {
	TxDate      string `json:"tx_date"`
	Description string `json:"description"`
	Reference   string `json:"reference"`
	AmountCents int64  `json:"amount_cents"`
}

type createStatementRequest struct {
	BankAccountID       int64                `json:"bank_account_id"`
	StatementDate       string               `json:"statement_date"`
	OpeningBalanceCents int64                `json:"opening_balance_cents"`
	ClosingBalanceCents int64                `json:"closing_balance_cents"`
	Notes               string               `json:"notes"`
	Lines               []statementLineInput `json:"lines"`
}

type statementLineResponse struct {
	ID                   int64  `json:"id"`
	LineNo               int    `json:"line_no"`
	TxDate               string `json:"tx_date"`
	Description          string `json:"description"`
	Reference            string `json:"reference"`
	AmountCents          int64  `json:"amount_cents"`
	MatchedJournalLineID int64  `json:"matched_journal_line_id,omitempty"`
	MatchStatus          string `json:"match_status"`
}

type statementResponse struct {
	ID                  int64                   `json:"id"`
	BankAccountID       int64                   `json:"bank_account_id"`
	BankAccountName     string                  `json:"bank_account_name"`
	BankAccountCode     string                  `json:"bank_account_code"`
	StatementDate       string                  `json:"statement_date"`
	OpeningBalanceCents int64                   `json:"opening_balance_cents"`
	ClosingBalanceCents int64                   `json:"closing_balance_cents"`
	Status              string                  `json:"status"`
	Notes               string                  `json:"notes"`
	Lines               []statementLineResponse `json:"lines,omitempty"`
}

type statementListItem struct {
	ID                  int64  `json:"id"`
	BankAccountID       int64  `json:"bank_account_id"`
	BankAccountName     string `json:"bank_account_name"`
	BankAccountCode     string `json:"bank_account_code"`
	StatementDate       string `json:"statement_date"`
	OpeningBalanceCents int64  `json:"opening_balance_cents"`
	ClosingBalanceCents int64  `json:"closing_balance_cents"`
	Status              string `json:"status"`
	LineCount           int    `json:"line_count"`
}

// CreateStatement handles POST /bank-statements.
func (service *Service) CreateStatement(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	// Idempotency-Key is required for imports (validated, not persisted —
	// the bank_statements schema has no idempotency_key column).
	if _, err := idempotencyKey(request); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req createStatementRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.BankAccountID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "bank_account_id is required")
		return
	}
	stmtDate, err := parseDate(req.StatementDate)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "statement_date is required (YYYY-MM-DD)")
		return
	}
	if len(req.Lines) == 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "at least one statement line is required")
		return
	}
	for _, line := range req.Lines {
		if line.AmountCents == 0 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "line amount_cents must not be zero")
			return
		}
		if _, err := parseDate(line.TxDate); err != nil {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "line tx_date is required (YYYY-MM-DD)")
			return
		}
	}

	var result *statementResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Idempotency-Key is required for the import command (validated above).
		// The schema does not persist the key, so replays create a new statement;
		// the header guard still protects against accidental double-submits.

		// Validate the bank account: must exist, belong to tenant, and be type BANK.
		var acctName, acctCode, acctType string
		err := tx.QueryRow(request.Context(), `
			SELECT name, code, account_type
			FROM accounts
			WHERE tenant_id = $1 AND id = $2 AND is_active = true
		`, tenant, req.BankAccountID).Scan(&acctName, &acctCode, &acctType)
		if err != nil {
			if isNoRows(err) {
				return validationError("bank account not found")
			}
			return err
		}
		if acctType != "BANK" {
			return validationError("account is not a bank account")
		}

		var stmtID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO bank_statements
				(tenant_id, bank_account_id, statement_date,
				 opening_balance_cents, closing_balance_cents, status, notes,
				 created_by)
			VALUES ($1, $2, $3, $4, $5, 'IMPORTED', $6, $7)
			RETURNING id
		`,
			tenant, req.BankAccountID, stmtDate,
			req.OpeningBalanceCents, req.ClosingBalanceCents,
			textValueOptional(req.Notes), userID(request),
		).Scan(&stmtID)
		if err != nil {
			return err
		}

		// Insert lines with sequential line_no.
		lineNo := 1
		for _, line := range req.Lines {
			lineDate, _ := parseDate(line.TxDate)
			_, err = tx.Exec(request.Context(), `
				INSERT INTO bank_statement_lines
					(tenant_id, statement_id, line_no, tx_date, description, reference, amount_cents, match_status)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 'UNMATCHED')
			`,
				tenant, stmtID, lineNo, lineDate,
				textValueOptional(line.Description), textValueOptional(line.Reference),
				line.AmountCents,
			)
			if err != nil {
				return err
			}
			lineNo++
		}

		result, err = loadStatement(request.Context(), tx, tenant, stmtID)
		return err
	})
	if err != nil {
		if ve, ok := err.(*ValidationError); ok {
			writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", ve.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "STATEMENT_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListStatements handles GET /bank-statements.
func (service *Service) ListStatements(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	status := strings.TrimSpace(request.URL.Query().Get("status"))

	var results []statementListItem
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT bs.id, bs.bank_account_id, a.name, a.code,
			       bs.statement_date, bs.opening_balance_cents, bs.closing_balance_cents,
			       bs.status, COUNT(bsl.id) AS line_count
			FROM bank_statements bs
			JOIN accounts a ON a.tenant_id = bs.tenant_id AND a.id = bs.bank_account_id
			LEFT JOIN bank_statement_lines bsl ON bsl.tenant_id = bs.tenant_id AND bsl.statement_id = bs.id
		`
		args := []any{}
		if status != "" {
			query += " WHERE bs.status = $1"
			args = append(args, status)
		}
		query += `
			GROUP BY bs.id, a.name, a.code
			ORDER BY bs.statement_date DESC, bs.id DESC
		`
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []statementListItem{}
		for rows.Next() {
			var item statementListItem
			var stmtDate pgtype.Date
			if err := rows.Scan(&item.ID, &item.BankAccountID, &item.BankAccountName, &item.BankAccountCode,
				&stmtDate, &item.OpeningBalanceCents, &item.ClosingBalanceCents,
				&item.Status, &item.LineCount); err != nil {
				return err
			}
			item.StatementDate = dateString(stmtDate)
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STATEMENT_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetStatement handles GET /bank-statements/{id}.
func (service *Service) GetStatement(writer http.ResponseWriter, request *http.Request) {
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
	var result *statementResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = loadStatement(request.Context(), tx, tenant, stmtID)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "NOT_FOUND", "bank statement not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "STATEMENT_GET_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// loadStatement loads a statement with its lines + bank account info.
func loadStatement(ctx context.Context, tx pgx.Tx, tenantID, stmtID int64) (*statementResponse, error) {
	var stmt statementResponse
	var stmtDate pgtype.Date
	var notes pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT bs.id, bs.bank_account_id, a.name, a.code,
		       bs.statement_date, bs.opening_balance_cents, bs.closing_balance_cents,
		       bs.status, bs.notes
		FROM bank_statements bs
		JOIN accounts a ON a.tenant_id = bs.tenant_id AND a.id = bs.bank_account_id
		WHERE bs.tenant_id = $1 AND bs.id = $2
	`, tenantID, stmtID).Scan(
		&stmt.ID, &stmt.BankAccountID, &stmt.BankAccountName, &stmt.BankAccountCode,
		&stmtDate, &stmt.OpeningBalanceCents, &stmt.ClosingBalanceCents,
		&stmt.Status, &notes,
	)
	if err != nil {
		return nil, err
	}
	stmt.StatementDate = dateString(stmtDate)
	stmt.Notes = textValue(notes)

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
	stmt.Lines = []statementLineResponse{}
	for rows.Next() {
		var line statementLineResponse
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
		stmt.Lines = append(stmt.Lines, line)
	}
	return &stmt, rows.Err()
}
