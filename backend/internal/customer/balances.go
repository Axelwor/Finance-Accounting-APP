package customer

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// M-007: AR sub-ledger reporting. The customer_balances table is maintained
// on every invoice/payment/credit-note posting; these endpoints expose the
// per-customer outstanding AR and reconcile the sub-ledger total against the
// GL AR account so drift is detectable.

type customerBalanceRow struct {
	CustomerID   int64  `json:"customer_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	ARCents      int64  `json:"ar_cents"`
	OverdueCents int64  `json:"overdue_cents"`
}

type arBalancesResponse struct {
	Balances     []customerBalanceRow `json:"balances"`
	TotalARCents int64                `json:"total_ar_cents"`
	GLARCents    int64                `json:"gl_ar_cents"`
	DiffCents    int64                `json:"diff_cents"`
	Reconciled   bool                 `json:"reconciled"`
}

const arAccountCode = "1201"

// ARBalances returns per-customer outstanding AR from the sub-ledger and
// reconciles the total against the GL AR account (code 1201).
// GET /customers/ar-balances
func (service *Service) ARBalances(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Per-customer balances from the sub-ledger, joined with overdue amounts
	// derived from invoices past their due date.
	rows, err := service.pool.Query(request.Context(), `
		SELECT b.customer_id, c.code, c.name, b.ar_cents,
		       COALESCE((
		           SELECT SUM(i.receivable_cents)
		           FROM invoices i
		           WHERE i.tenant_id = b.tenant_id AND i.customer_id = b.customer_id
		             AND i.status IN ('ISSUED','PARTIALLY_PAID')
		             AND i.due_date < CURRENT_DATE
		       ), 0)
		FROM customer_balances b
		JOIN customers c ON c.tenant_id = b.tenant_id AND c.id = b.customer_id
		WHERE b.tenant_id = $1 AND b.ar_cents > 0
		ORDER BY b.ar_cents DESC, c.code
	`, tenant)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "AR_BALANCES_FAILED", err.Error())
		return
	}
	defer rows.Close()

	resp := arBalancesResponse{Balances: []customerBalanceRow{}}
	for rows.Next() {
		var row customerBalanceRow
		if err := rows.Scan(&row.CustomerID, &row.Code, &row.Name, &row.ARCents, &row.OverdueCents); err != nil {
			writeError(writer, http.StatusInternalServerError, "AR_BALANCES_FAILED", err.Error())
			return
		}
		resp.TotalARCents += row.ARCents
		resp.Balances = append(resp.Balances, row)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "AR_BALANCES_FAILED", err.Error())
		return
	}

	// GL AR balance (account code 1201): debit - credit across posted journals.
	if err := service.pool.QueryRow(request.Context(), `
		SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		WHERE jl.tenant_id = $1 AND a.code = $2 AND je.status = 'POSTED'
	`, tenant, arAccountCode).Scan(&resp.GLARCents); err != nil {
		writeError(writer, http.StatusInternalServerError, "AR_BALANCES_FAILED", err.Error())
		return
	}

	resp.DiffCents = resp.GLARCents - resp.TotalARCents
	resp.Reconciled = resp.DiffCents == 0
	writeJSON(writer, http.StatusOK, resp)
}

// ARBalance returns the outstanding AR for a single customer.
// GET /customers/{id}/ar-balance
func (service *Service) ARBalance(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(request, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var arCents int64
	err = service.pool.QueryRow(request.Context(), `
		SELECT COALESCE(ar_cents, 0) FROM customer_balances
		WHERE tenant_id = $1 AND customer_id = $2
	`, tenant, id).Scan(&arCents)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "AR_BALANCE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"customer_id": id,
		"ar_cents":    arCents,
	})
}
