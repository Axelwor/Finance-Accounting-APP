package accounting

import (
	"fmt"

	"github.com/jackc/pgx/v5"

	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// General Ledger (Buku Besar)
// ---------------------------------------------------------------------------

// GeneralLedgerMovement is one posted line on the account ledger.
type GeneralLedgerMovement struct {
	EntryNumber         string `json:"entry_number"`
	EntryDate           string `json:"entry_date"`
	Description         string `json:"description"`
	DebitCents          int64  `json:"debit_cents"`
	CreditCents         int64  `json:"credit_cents"`
	RunningBalanceCents int64  `json:"running_balance_cents"`
}

// GeneralLedgerResult is the full ledger response: opening balance, the
// ordered movements inside the requested window, and the closing balance.
type GeneralLedgerResult struct {
	AccountID           int64                   `json:"account_id"`
	AccountCode         string                  `json:"account_code"`
	AccountName         string                  `json:"account_name"`
	OpeningBalanceCents int64                   `json:"opening_balance_cents"`
	Entries             []GeneralLedgerMovement `json:"entries"`
	ClosingBalanceCents int64                   `json:"closing_balance_cents"`
}

// GetGeneralLedger returns the per-account ledger (buku besar) for the
// requested account and date window. The opening balance is the signed
// debit-credit sum of all posted lines on the account before `from_date`;
// movements are the posted lines inside [from_date, to_date] in entry-date
// then number order, each carrying a running balance.
//
// Query params:
//
//	account_id (required) — the account to inspect
//	from_date (optional)  — inclusive lower bound (YYYY-MM-DD)
//	to_date   (optional)  — inclusive upper bound (YYYY-MM-DD)
func (service *Service) GetGeneralLedger(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	query := request.URL.Query()
	accountIDRaw := strings.TrimSpace(query.Get("account_id"))
	accountID, err := parsePositiveInt(accountIDRaw)
	if err != nil || accountID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "account_id is required")
		return
	}
	fromDate := normalizeDate(query.Get("from_date"))
	toDate := normalizeDate(query.Get("to_date"))

	ctx := request.Context()

	// Load the account (code + name) for display.
	var accountCode, accountName string
	err = db.WithTenantData(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT code, name FROM accounts WHERE tenant_id = $1 AND id = $2
		`, tenant, accountID).Scan(&accountCode, &accountName)
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "ACCOUNT_NOT_FOUND", "account not found")
		return
	}

	// Opening balance: signed sum of posted lines strictly before from_date.
	// When from_date is absent, the opening balance is zero (everything is a
	// movement).
	var opening int64
	if fromDate != "" {
		err = db.WithTenantData(ctx, service.pool, tenant, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)::bigint
				FROM journal_lines jl
				JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
				WHERE jl.tenant_id = $1 AND jl.account_id = $2
				  AND je.status = 'POSTED'
				  AND je.entry_date < $3
			`, tenant, accountID, dateValue(fromDate)).Scan(&opening)
		})
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "LEDGER_FAILED", err.Error())
			return
		}
	}

	// Movements inside the window. Date bounds are appended only when present
	// and always bound as pgtype.Date: passing the raw string makes Postgres
	// compare date >= text (SQLSTATE 42883).
	movements := make([]GeneralLedgerMovement, 0)
	err = db.WithTenantData(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		query := `
			SELECT je.number, to_char(je.entry_date, 'YYYY-MM-DD'),
			       COALESCE(je.description, ''),
			       jl.debit_cents, jl.credit_cents
			FROM journal_lines jl
			JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
			WHERE jl.tenant_id = $1 AND jl.account_id = $2
			  AND je.status = 'POSTED'`
		args := []any{tenant, accountID}
		if fromDate != "" {
			args = append(args, dateValue(fromDate))
			query += fmt.Sprintf(" AND je.entry_date >= $%d", len(args))
		}
		if toDate != "" {
			args = append(args, dateValue(toDate))
			query += fmt.Sprintf(" AND je.entry_date <= $%d", len(args))
		}
		query += " ORDER BY je.entry_date ASC, je.number ASC"
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		running := opening
		for rows.Next() {
			var mov GeneralLedgerMovement
			if err := rows.Scan(&mov.EntryNumber, &mov.EntryDate, &mov.Description, &mov.DebitCents, &mov.CreditCents); err != nil {
				return err
			}
			running += mov.DebitCents - mov.CreditCents
			mov.RunningBalanceCents = running
			movements = append(movements, mov)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LEDGER_FAILED", err.Error())
		return
	}

	closing := opening
	for _, m := range movements {
		closing += m.DebitCents - m.CreditCents
	}

	writeJSON(writer, http.StatusOK, GeneralLedgerResult{
		AccountID:           accountID,
		AccountCode:         accountCode,
		AccountName:         accountName,
		OpeningBalanceCents: opening,
		Entries:             movements,
		ClosingBalanceCents: closing,
	})
}

// normalizeDate trims and validates a YYYY-MM-DD date; returns "" when the
// input is blank or unparseable (treated as an open bound by callers).
func normalizeDate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", trimmed); err != nil {
		return ""
	}
	return trimmed
}

// parsePositiveInt parses a positive integer string (used for query params).
func parsePositiveInt(raw string) (int64, error) {
	return parseInt64(raw)
}

// dateValue converts a YYYY-MM-DD string to a pgtype.Date (zero when blank).
func dateValue(raw string) pgtype.Date {
	if raw == "" {
		return pgtype.Date{}
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

// ensure ctx import is used when this file grows.
