package lease

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// US-110: Konsolidasi Multi-Entitas (PSAK 65)
//   Entity hierarchy: parent-child tenant relationships for consolidation.
//   Consolidated reports: aggregate journal lines across parent + children,
//   then eliminate inter-company transactions (matched SALE/PURCHASE pairs net to zero).
// ---------------------------------------------------------------------------

type CreateEntityHierarchyRequest struct {
	ChildTenantID    int64   `json:"child_tenant_id"`
	ParentTenantID   int64   `json:"parent_tenant_id"`
	ConsolidationPct float64 `json:"consolidation_pct"`
}

type entityHierarchyResponse struct {
	ID               int64   `json:"id"`
	TenantID         int64   `json:"tenant_id"`
	ParentTenantID   int64   `json:"parent_tenant_id"`
	Relationship     string  `json:"relationship"`
	ConsolidationPct float64 `json:"consolidation_pct"`
	TenantName       string  `json:"tenant_name,omitempty"`
	ParentTenantName string  `json:"parent_tenant_name,omitempty"`
	CreatedAt        string  `json:"created_at,omitempty"`
}

func (service *Service) CreateEntityHierarchy(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateEntityHierarchyRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.ChildTenantID <= 0 || req.ParentTenantID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "child_tenant_id and parent_tenant_id are required")
		return
	}
	if req.ChildTenantID == req.ParentTenantID {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "child and parent must be different tenants")
		return
	}
	pct := req.ConsolidationPct
	if pct == 0 {
		pct = 1.0
	}
	if pct <= 0 || pct > 1.0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "consolidation_pct must be between 0 and 1")
		return
	}

	var result entityHierarchyResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var id int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO entity_hierarchy (tenant_id, parent_tenant_id, relationship, consolidation_pct)
			VALUES ($1, $2, 'CHILD', $3)
			ON CONFLICT (tenant_id, parent_tenant_id) DO UPDATE
			SET consolidation_pct = EXCLUDED.consolidation_pct
			RETURNING id
		`, req.ChildTenantID, req.ParentTenantID, pct).Scan(&id)
		if err != nil {
			return err
		}
		result = entityHierarchyResponse{
			ID:               id,
			TenantID:         req.ChildTenantID,
			ParentTenantID:   req.ParentTenantID,
			Relationship:     "CHILD",
			ConsolidationPct: pct,
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "ENTITY_HIERARCHY_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) ListEntityHierarchy(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT eh.id, eh.tenant_id, eh.parent_tenant_id, eh.relationship, eh.consolidation_pct, eh.created_at,
		       ct.name, pt.name
		FROM entity_hierarchy eh
		JOIN tenants ct ON ct.id = eh.tenant_id
		JOIN tenants pt ON pt.id = eh.parent_tenant_id
		WHERE eh.tenant_id = $1 OR eh.parent_tenant_id = $1
		ORDER BY eh.created_at DESC
	`, tenant)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	items := make([]entityHierarchyResponse, 0)
	for rows.Next() {
		var e entityHierarchyResponse
		var createdAt pgtype.Timestamptz
		var pct pgtype.Numeric
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ParentTenantID, &e.Relationship,
			&pct, &createdAt, &e.TenantName, &e.ParentTenantName); err != nil {
			writeError(writer, http.StatusInternalServerError, "LIST_FAILED", err.Error())
			return
		}
		e.ConsolidationPct = numericToFloat(pct)
		if createdAt.Valid {
			e.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		items = append(items, e)
	}
	writeJSON(writer, http.StatusOK, items)
}

// ---------------------------------------------------------------------------
// Consolidated Trial Balance
// ---------------------------------------------------------------------------

type consolidatedTrialBalanceRow struct {
	AccountID   int64  `json:"account_id"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	ReportGroup string `json:"report_group"`
	DebitCents  int64  `json:"debit_cents"`
	CreditCents int64  `json:"credit_cents"`
	TenantName  string `json:"tenant_name,omitempty"`
}

type consolidatedTrialBalanceResult struct {
	Rows                []consolidatedTrialBalanceRow `json:"rows"`
	TotalDebitCents     int64                         `json:"total_debit_cents"`
	TotalCreditCents    int64                         `json:"total_credit_cents"`
	EliminationCents    int64                         `json:"elimination_cents"`
	Balanced            bool                          `json:"balanced"`
	ConsolidatedTenants []int64                       `json:"consolidated_tenant_ids"`
}

func (service *Service) ConsolidatedTrialBalance(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	tenantIDs := service.collectConsolidationTenants(request.Context(), tenant)
	result, err := service.fetchConsolidatedTrialBalance(request.Context(), tenant, tenantIDs)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) fetchConsolidatedTrialBalance(ctx context.Context, parentTenantID int64, tenantIDs []int64) (consolidatedTrialBalanceResult, error) {
	result := consolidatedTrialBalanceResult{
		Rows:                []consolidatedTrialBalanceRow{},
		ConsolidatedTenants: tenantIDs,
	}
	if len(tenantIDs) == 0 {
		return result, nil
	}

	// A-29: child balances are weighted by consolidation_pct from the entity
	// hierarchy (parent = 100%). The pct scales debit and credit equally, so
	// each tenant's books stay internally balanced after weighting.
	rows, err := service.pool.Query(ctx, `
		SELECT a.id, a.code, a.name, a.report_group,
		       COALESCE(SUM(ROUND(jl.debit_cents  * COALESCE(eh.consolidation_pct, 1.0))), 0)::bigint,
		       COALESCE(SUM(ROUND(jl.credit_cents * COALESCE(eh.consolidation_pct, 1.0))), 0)::bigint
		FROM accounts a
		LEFT JOIN journal_lines jl
		       ON jl.tenant_id = ANY($1) AND jl.account_id = a.id
		LEFT JOIN journal_entries je
		       ON je.tenant_id = ANY($1) AND je.id = jl.entry_id AND je.status = 'POSTED'
		LEFT JOIN entity_hierarchy eh
		       ON eh.tenant_id = jl.tenant_id AND eh.parent_tenant_id = $2
		WHERE a.tenant_id = ANY($1) AND a.is_group = false
		GROUP BY a.id, a.code, a.name, a.report_group
		ORDER BY a.code
	`, tenantIDs, parentTenantID)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var r consolidatedTrialBalanceRow
		if err := rows.Scan(&r.AccountID, &r.AccountCode, &r.AccountName, &r.ReportGroup,
			&r.DebitCents, &r.CreditCents); err != nil {
			return result, err
		}
		result.Rows = append(result.Rows, r)
		result.TotalDebitCents += r.DebitCents
		result.TotalCreditCents += r.CreditCents
	}

	// Eliminate inter-company transactions: for each matched pair, subtract the
	// journal lines of both entries from the consolidated totals.
	elim, err := service.computeEliminations(ctx, tenantIDs)
	if err != nil {
		return result, err
	}
	for _, e := range elim {
		for i := range result.Rows {
			if result.Rows[i].AccountID == e.accountID {
				result.Rows[i].DebitCents -= e.debit
				result.Rows[i].CreditCents -= e.credit
				result.TotalDebitCents -= e.debit
				result.TotalCreditCents -= e.credit
				result.EliminationCents += e.debit + e.credit
				break
			}
		}
	}

	result.Balanced = result.TotalDebitCents == result.TotalCreditCents
	return result, nil
}

// ---------------------------------------------------------------------------
// Consolidated Profit & Loss
// ---------------------------------------------------------------------------

type consolidatedProfitLossResult struct {
	RevenueCents        int64   `json:"revenue_cents"`
	ExpenseCents        int64   `json:"expense_cents"`
	ProfitCents         int64   `json:"profit_cents"`
	EliminationCents    int64   `json:"elimination_cents"`
	ConsolidatedTenants []int64 `json:"consolidated_tenant_ids"`
}

func (service *Service) ConsolidatedProfitLoss(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	tenantIDs := service.collectConsolidationTenants(request.Context(), tenant)
	result, err := service.fetchConsolidatedProfitLoss(request.Context(), tenant, tenantIDs)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REPORT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) fetchConsolidatedProfitLoss(ctx context.Context, parentTenantID int64, tenantIDs []int64) (consolidatedProfitLossResult, error) {
	result := consolidatedProfitLossResult{ConsolidatedTenants: tenantIDs}
	if len(tenantIDs) == 0 {
		return result, nil
	}

	// A-29: child revenue/expense weighted by consolidation_pct (parent = 100%).
	rows, err := service.pool.Query(ctx, `
		SELECT a.report_group, COALESCE(SUM(ROUND((CASE
			WHEN a.report_group IN ('revenue') THEN jl.credit_cents - jl.debit_cents
			ELSE jl.debit_cents - jl.credit_cents END)
			* COALESCE(eh.consolidation_pct, 1.0))), 0)::bigint
		FROM journal_lines jl
		JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
		JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
		LEFT JOIN entity_hierarchy eh ON eh.tenant_id = jl.tenant_id AND eh.parent_tenant_id = $2
		WHERE jl.tenant_id = ANY($1) AND je.status = 'POSTED'
		  AND a.report_group IN ('revenue', 'expense')
		GROUP BY a.report_group
	`, tenantIDs, parentTenantID)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var group string
		var amount int64
		if err := rows.Scan(&group, &amount); err != nil {
			return result, err
		}
		switch group {
		case "revenue":
			result.RevenueCents = amount
		case "expense":
			result.ExpenseCents = amount
		}
	}

	// Eliminate inter-company revenue/expense from matched pairs.
	elim, err := service.computeEliminations(ctx, tenantIDs)
	if err != nil {
		return result, err
	}
	for _, e := range elim {
		// Look up the account's report_group to determine if it's revenue or expense.
		var reportGroup string
		err := service.pool.QueryRow(ctx, `SELECT report_group FROM accounts WHERE id = $1`, e.accountID).Scan(&reportGroup)
		if err != nil {
			continue
		}
		net := e.debit - e.credit
		switch reportGroup {
		case "revenue":
			// Revenue is credit-normal: eliminating reduces revenue.
			result.RevenueCents -= -net // net is debit-credit; for revenue, credit is positive
		case "expense":
			result.ExpenseCents -= net
		}
		result.EliminationCents += e.debit + e.credit
	}

	result.ProfitCents = result.RevenueCents - result.ExpenseCents
	return result, nil
}

// ---------------------------------------------------------------------------
// Elimination helper
// ---------------------------------------------------------------------------

type eliminationEntry struct {
	accountID int64
	debit     int64
	credit    int64
}

// computeEliminations finds matched inter-company transaction pairs (SALE from
// A to B + PURCHASE from B to A, same amount) and returns the journal lines of
// both entries so they can be subtracted from the consolidated totals.
func (service *Service) computeEliminations(ctx context.Context, tenantIDs []int64) ([]eliminationEntry, error) {
	// Find all inter-company transactions within the consolidation group.
	rows, err := service.pool.Query(ctx, `
		SELECT id, tenant_id, counterparty_tenant_id, tx_type, journal_entry_id, amount_cents, eliminated
		FROM inter_company_transactions
		WHERE tenant_id = ANY($1) AND counterparty_tenant_id = ANY($1)
		  AND journal_entry_id IS NOT NULL AND eliminated = false
		ORDER BY amount_cents DESC, tx_date
	`, tenantIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []icTx
	for rows.Next() {
		var t icTx
		if err := rows.Scan(&t.id, &t.tenantID, &t.counterpartyTenantID, &t.txType,
			&t.journalEntryID, &t.amountCents, &t.used); err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	pairs := matchEliminationPairs(txs)
	if len(pairs) == 0 {
		return nil, nil
	}

	var elimEntries []eliminationEntry
	var matchedIDs []int64
	for _, p := range pairs {
		matchedIDs = append(matchedIDs, p[0].id, p[1].id)
		// Collect journal lines from both entries.
		for _, entryID := range []int64{p[0].journalEntryID, p[1].journalEntryID} {
			lines, err := service.pool.Query(ctx, `
				SELECT account_id, debit_cents, credit_cents
				FROM journal_lines
				WHERE entry_id = $1 AND (debit_cents > 0 OR credit_cents > 0)
			`, entryID)
			if err != nil {
				return nil, err
			}
			for lines.Next() {
				var e eliminationEntry
				if err := lines.Scan(&e.accountID, &e.debit, &e.credit); err != nil {
					lines.Close()
					return nil, err
				}
				elimEntries = append(elimEntries, e)
			}
			lines.Close()
		}
	}
	// A-29: persist the eliminated flag so each matched pair is eliminated
	// exactly once. Best-effort — a failure to update must not break the
	// report, so the error is swallowed (the next run retries the match).
	_, _ = service.pool.Exec(ctx, `
		UPDATE inter_company_transactions
		SET eliminated = true
		WHERE id = ANY($1)
	`, matchedIDs)
	return elimEntries, nil
}

// icTx is one inter-company transaction row loaded for elimination matching.
type icTx struct {
	id                   int64
	tenantID             int64
	counterpartyTenantID int64
	txType               string
	journalEntryID       int64
	amountCents          int64
	used                 bool
}

// eliminationPairType maps each tx_type to the tx_type of its mirror
// counterparty leg (i-006): SALE(A→B)↔PURCHASE(B→A), LOAN↔LOAN,
// INTEREST↔INTEREST, DIVIDEND↔DIVIDEND. MANAGEMENT_FEE has no mirror and is
// never auto-eliminated.
var eliminationPairType = map[string]string{
	"SALE":     "PURCHASE",
	"LOAN":     "LOAN",
	"INTEREST": "INTEREST",
	"DIVIDEND": "DIVIDEND",
}

// matchEliminationPairs pairs each transaction with its counterparty mirror:
// opposite directions between the same two tenants and the same amount. Each
// transaction participates in at most one pair (strict 1:1). It is a pure
// function over the loaded rows so it can be unit-tested without a database.
func matchEliminationPairs(txs []icTx) [][2]icTx {
	var pairs [][2]icTx
	for i := range txs {
		if txs[i].used {
			continue
		}
		wantType, ok := eliminationPairType[txs[i].txType]
		if !ok {
			continue
		}
		for j := range txs {
			if i == j || txs[j].used || txs[j].txType != wantType {
				continue
			}
			if txs[j].tenantID == txs[i].counterpartyTenantID &&
				txs[j].counterpartyTenantID == txs[i].tenantID &&
				txs[j].amountCents == txs[i].amountCents {
				txs[i].used = true
				txs[j].used = true
				pairs = append(pairs, [2]icTx{txs[i], txs[j]})
				break
			}
		}
	}
	return pairs
}

// collectConsolidationTenants returns the parent tenant + all its children.
// If no hierarchy exists, returns just the requesting tenant.
func (service *Service) collectConsolidationTenants(ctx context.Context, parentTenantID int64) []int64 {
	tenantIDs := []int64{parentTenantID}
	rows, err := service.pool.Query(ctx, `
		SELECT tenant_id FROM entity_hierarchy WHERE parent_tenant_id = $1
	`, parentTenantID)
	if err != nil {
		return tenantIDs
	}
	defer rows.Close()
	for rows.Next() {
		var childID int64
		if err := rows.Scan(&childID); err == nil {
			tenantIDs = append(tenantIDs, childID)
		}
	}
	return tenantIDs
}

// formatIntSlice is a small helper for debugging/logging (unused but kept for
// potential future use in error messages).
func formatIntSlice(ids []int64) string {
	s := "["
	for i, id := range ids {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%d", id)
	}
	return s + "]"
}
