package sales

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/costing"
	"finance-accounting-app/backend/internal/db"
)

// returnAccountCode is the seeded "Sales Returns" contra-revenue account (4201).
const returnAccountCode = "4201"

// cogsAccountCode is the seeded "COGS" account (5101).
const cogsAccountCode = "5101"

// inventoryAccountCode is the seeded "Inventory" account (1301).
const inventoryAccountCode = "1301"

// CN statuses.
const (
	cnDraft   = "DRAFT"
	cnApplied = "APPLIED"
	cnVoid    = "VOID"
)

// CreditNoteLineRequest is one line of a create-CN request.
type CreditNoteLineRequest struct {
	ItemID         int64   `json:"item_id"`
	InvoiceLineID  int64   `json:"invoice_line_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	UnitCostCents  int64   `json:"unit_cost_cents"`
	Description    string  `json:"description"`
}

// CreateCreditNoteRequest is the POST /credit-notes body.
type CreateCreditNoteRequest struct {
	InvoiceID    int64                   `json:"invoice_id"`
	CustomerID   int64                   `json:"customer_id"`
	CNDate       string                  `json:"cn_date"`
	RefundMethod string                  `json:"refund_method"`
	Reason       string                  `json:"reason"`
	Lines        []CreditNoteLineRequest `json:"lines"`
}

type cnLineResponse struct {
	ID                int64   `json:"id"`
	ItemID            int64   `json:"item_id"`
	ItemCode          string  `json:"item_code"`
	ItemName          string  `json:"item_name"`
	InvoiceLineID     int64   `json:"invoice_line_id,omitempty"`
	LineNo            int     `json:"line_no"`
	Qty               float64 `json:"qty"`
	UnitPriceCents    int64   `json:"unit_price_cents"`
	UnitCostCents     int64   `json:"unit_cost_cents"`
	LineTotalCents    int64   `json:"line_total_cents"`
	COGSReversedCents int64   `json:"cogs_reversed_cents"`
	Description       string  `json:"description"`
}

type creditNoteResponse struct {
	ID                int64            `json:"id"`
	Number            string           `json:"number"`
	InvoiceID         int64            `json:"invoice_id"`
	CustomerID        int64            `json:"customer_id"`
	CustomerName      string           `json:"customer_name"`
	CNDate            string           `json:"cn_date"`
	RefundMethod      string           `json:"refund_method"`
	Reason            string           `json:"reason"`
	Status            string           `json:"status"`
	TotalCents        int64            `json:"total_cents"`
	ARDeductedCents   int64            `json:"ar_deducted_cents"`
	COGSReversedCents int64            `json:"cogs_reversed_cents"`
	Lines             []cnLineResponse `json:"lines,omitempty"`
}

// CreateCreditNote posts a sales return / credit note. It posts two journals:
// 1. Revenue reversal: Dr 4201 Sales Returns / Cr 1201 AR (intent SALES_RETURN)
// 2. COGS reversal: Dr 1301 Inventory / Cr 5101 COGS (intent COGS_REVERSAL)
// The invoice's receivable_cents is increased (AR deducted on CN).
func (service *Service) CreateCreditNote(writer http.ResponseWriter, request *http.Request) {
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
	var req CreateCreditNoteRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCNRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	userID := userID(request)

	var result creditNoteResponse
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
			cn, err := service.findCNByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = cn
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load the invoice.
		var invNumber string
		var customerID int64
		var invStatus string
		var receivable int64
		err = tx.QueryRow(request.Context(), `
			SELECT number, customer_id, status, receivable_cents
			FROM invoices WHERE tenant_id = $1 AND id = $2
		`, tenant, req.InvoiceID).Scan(&invNumber, &customerID, &invStatus, &receivable)
		if err != nil {
			return err
		}
		if invStatus == invVoid {
			return fmt.Errorf("invoice %s is VOID", invNumber)
		}
		// M-008: the invoice row must be locked for the whole CN transaction so
		// concurrent credit notes cannot both pass the receivable check.
		if _, err := tx.Exec(request.Context(), `
			SELECT id FROM invoices WHERE tenant_id = $1 AND id = $2 FOR UPDATE
		`, tenant, req.InvoiceID); err != nil {
			return err
		}
		// Use the invoice's customer if not explicitly provided.
		if req.CustomerID == 0 {
			req.CustomerID = customerID
		}
		var customerName string
		if err := tx.QueryRow(request.Context(), `
			SELECT name FROM customers WHERE tenant_id = $1 AND id = $2
		`, tenant, req.CustomerID).Scan(&customerName); err != nil {
			return err
		}

		// Prepare lines: compute totals and COGS reversal.
		type preparedCNLine struct {
			line          CreditNoteLineRequest
			lineTotal     int64
			cogsReversed  int64
			inventoryAcct int64
			cogsAcct      int64
			costingMethod string
		}
		prepared := make([]preparedCNLine, 0, len(req.Lines))
		var totalReturn int64
		var totalCOGSReversed int64
		for _, line := range req.Lines {
			var itemType, itemCode, itemName string
			var invAcct, cogsAcct pgtype.Int8
			var costingMethod pgtype.Text
			err := tx.QueryRow(request.Context(), `
				SELECT item_type, code, name, inventory_account_id, cogs_account_id, costing_method
				FROM items WHERE tenant_id = $1 AND id = $2
			`, tenant, line.ItemID).Scan(&itemType, &itemCode, &itemName, &invAcct, &cogsAcct, &costingMethod)
			if err != nil {
				return err
			}
			if !invAcct.Valid || !cogsAcct.Valid {
				return fmt.Errorf("item %s (%s) is missing inventory or cogs account", itemCode, itemName)
			}
			lineTotal := lineTotalCents(line.Qty, line.UnitPriceCents, 0)
			cogsReversed := roundQty(line.Qty) * line.UnitCostCents
			totalReturn += lineTotal
			totalCOGSReversed += cogsReversed
			prepared = append(prepared, preparedCNLine{
				line:          line,
				lineTotal:     lineTotal,
				cogsReversed:  cogsReversed,
				inventoryAcct: invAcct.Int64,
				cogsAcct:      cogsAcct.Int64,
				costingMethod: textValue(costingMethod),
			})
		}

		// Resolve accounts.
		returnAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, returnAccountCode)
		if err != nil {
			return err
		}
		arAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, arAccountCode)
		if err != nil {
			return err
		}

		// 1. Revenue reversal journal: Dr 4201 Sales Returns / Cr 1201 AR.
		revenueLines := []accounting.Line{
			{AccountID: returnAccountID, DebitCents: totalReturn, SourceLineRef: "returns"},
			{AccountID: arAccountID, CreditCents: totalReturn, SourceLineRef: "ar"},
		}
		if err := accounting.BalanceCheck(revenueLines); err != nil {
			return err
		}
		revenueSourceRef := fmt.Sprintf("CN-REV-%d", req.InvoiceID)
		revenueJournal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   revenueSourceRef,
			IntentType:  accounting.IntentType("SALES_RETURN"),
			EntryDate:   req.CNDate,
			Description: "Sales return: invoice " + invNumber,
			Lines:       revenueLines,
		}
		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		revenueJournal.PreviousHash = head.LastHash
		revenueJournal.Hash = hashDP(revenueJournal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, revenueJournal.EntryDate)
		if err != nil {
			return err
		}
		jrnNumber, err := nextJournalNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		var revenueEntryID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, tenant, jrnNumber, revenueJournal.EntryDate, periodID, revenueJournal.Description,
			revenueJournal.SourceRef, string(revenueJournal.IntentType), idem,
			revenueJournal.Hash, revenueJournal.PreviousHash, int8Value(userID)).Scan(&revenueEntryID)
		if err != nil {
			return err
		}
		for _, line := range revenueJournal.Lines {
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, tenant, revenueEntryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
				return err
			}
		}
		if err := upsertHead(request.Context(), tx, tenant, revenueEntryID, revenueJournal.Hash); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "cn.posted", mustJSON(map[string]any{
			"journal_id": revenueEntryID, "number": jrnNumber,
		})); err != nil {
			return err
		}

		// 2. COGS reversal journal: Dr 1301 Inventory / Cr 5101 COGS.
		var cogsEntryID int64
		if totalCOGSReversed > 0 {
			cogsLines := make([]accounting.Line, 0, len(prepared)*2)
			for i, p := range prepared {
				cogsLines = append(cogsLines,
					accounting.Line{AccountID: p.inventoryAcct, DebitCents: p.cogsReversed, SourceLineRef: fmt.Sprintf("inv-%d", i)},
					accounting.Line{AccountID: p.cogsAcct, CreditCents: p.cogsReversed, SourceLineRef: fmt.Sprintf("cogs-%d", i)},
				)
			}
			if err := accounting.BalanceCheck(cogsLines); err != nil {
				return err
			}
			cogsSourceRef := fmt.Sprintf("CN-COGS-%d", req.InvoiceID)
			cogsJournal := accounting.Journal{
				TenantID:    tenant,
				SourceRef:   cogsSourceRef,
				IntentType:  accounting.IntentType("COGS_REVERSAL"),
				EntryDate:   req.CNDate,
				Description: "COGS reversal: invoice " + invNumber,
				Lines:       cogsLines,
			}
			head2, err := lockOrSeedHead(request.Context(), tx, tenant)
			if err != nil {
				return err
			}
			cogsJournal.PreviousHash = head2.LastHash
			cogsJournal.Hash = hashDP(cogsJournal)
			cogsJrnNumber, err := nextJournalNumber(request.Context(), tx, tenant)
			if err != nil {
				return err
			}
			cogsIdemKey := idem + "-cogs"
			err = tx.QueryRow(request.Context(), `
				INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				RETURNING id
			`, tenant, cogsJrnNumber, cogsJournal.EntryDate, periodID, cogsJournal.Description,
				cogsJournal.SourceRef, string(cogsJournal.IntentType), cogsIdemKey,
				cogsJournal.Hash, cogsJournal.PreviousHash, int8Value(userID)).Scan(&cogsEntryID)
			if err != nil {
				return err
			}
			for _, line := range cogsJournal.Lines {
				if _, err := tx.Exec(request.Context(), `
					INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
					VALUES ($1, $2, $3, $4, $5, $6)
				`, tenant, cogsEntryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
					return err
				}
			}
			if err := upsertHead(request.Context(), tx, tenant, cogsEntryID, cogsJournal.Hash); err != nil {
				return err
			}
			if err := insertOutbox(request.Context(), tx, tenant, "cn.cogs_reversed", mustJSON(map[string]any{
				"journal_id": cogsEntryID, "number": cogsJrnNumber,
			})); err != nil {
				return err
			}
			// Record inventory movements (qty positive = stock in).
			for _, p := range prepared {
				var posQty pgtype.Numeric
				_ = posQty.Scan(fmt.Sprintf("%g", p.line.Qty))
				if _, err := tx.Exec(request.Context(), `
					INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
					VALUES ($1, $2, 'SALES_RETURN', $3, $4, $5, $6)
				`, tenant, p.line.ItemID, posQty, p.line.UnitCostCents,
					fmt.Sprintf("CN-%d", req.InvoiceID), 0); err != nil {
					return err
				}
				// Reverse the COGS posting: restore FIFO layers / adjust
				// the moving average (PSAK 14).
				if err := costing.ReverseCOGS(request.Context(), tx, tenant,
					p.line.ItemID, 0, p.line.Qty, p.line.UnitCostCents, p.costingMethod); err != nil {
					return err
				}
			}
		}

		// Update invoice: reduce receivable. The reversal journal credits AR,
		// so the invoice's receivable_cents must go DOWN (M-008: it previously
		// went up, which both overstated AR and allowed over-crediting).
		if totalReturn > receivable {
			return fmt.Errorf("credit note total %d exceeds the invoice's outstanding receivable %d", totalReturn, receivable)
		}
		newReceivable := receivable - totalReturn
		newStatus := invStatus
		if newReceivable <= 0 && newStatus != invPaid {
			newStatus = invPaid
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE invoices SET receivable_cents = $1, status = $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, newReceivable, newStatus, tenant, req.InvoiceID); err != nil {
			return err
		}

		// M-007: reduce the AR sub-ledger to mirror the AR credit.
		if totalReturn > 0 {
			if _, err := tx.Exec(request.Context(), `
				UPDATE customer_balances
				SET ar_cents = GREATEST(ar_cents - $1, 0), updated_at = now()
				WHERE tenant_id = $2 AND customer_id = $3
			`, totalReturn, tenant, req.CustomerID); err != nil {
				return err
			}
		}

		// Insert CN header + lines.
		cnNumber, err := nextCNNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		cnDate, err := parseDate(req.CNDate)
		if err != nil {
			return err
		}
		refundMethod := req.RefundMethod
		if refundMethod == "" {
			refundMethod = "deduct"
		}
		var cnID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO credit_notes
				(tenant_id, number, invoice_id, customer_id, cn_date, refund_method,
				 reason, status, total_cents, ar_deducted_cents, cogs_reversed_cents,
				 revenue_journal_entry_id, cogs_journal_entry_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'APPLIED', $8, $8, $9, $10, $11, $12)
			RETURNING id
		`, tenant, cnNumber, req.InvoiceID, req.CustomerID, cnDate, refundMethod,
			textValueOptional(req.Reason), totalReturn, totalCOGSReversed,
			int8Value(revenueEntryID), int8Value(cogsEntryID), int8Value(userID)).Scan(&cnID)
		if err != nil {
			return err
		}
		for position, p := range prepared {
			var qty pgtype.Numeric
			_ = qty.Scan(p.line.Qty)
			lineNo := position + 1
			var invLineID pgtype.Int8
			if p.line.InvoiceLineID > 0 {
				invLineID = pgtype.Int8{Int64: p.line.InvoiceLineID, Valid: true}
			}
			_, err := tx.Exec(request.Context(), `
				INSERT INTO credit_note_lines
					(tenant_id, credit_note_id, item_id, invoice_line_id, line_no, qty,
					 unit_price_cents, unit_cost_cents, line_total_cents, cogs_reversed_cents,
					 inventory_account_id, cogs_account_id, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			`, tenant, cnID, p.line.ItemID, invLineID, lineNo, qty,
				p.line.UnitPriceCents, p.line.UnitCostCents, p.lineTotal, p.cogsReversed,
				p.inventoryAcct, p.cogsAcct, textValueOptional(p.line.Description))
			if err != nil {
				return err
			}
		}

		fetched, err := service.fetchCN(request.Context(), tx, tenant, cnID)
		if err != nil {
			return err
		}
		result = *fetched
		return nil
	})
	if err != nil {
		status, code, message := cnErrorFor(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListCreditNotes returns the tenant's credit notes.
func (service *Service) ListCreditNotes(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	statusFilter := strings.TrimSpace(request.URL.Query().Get("status"))
	var results []creditNoteResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT cn.id, cn.number, cn.invoice_id, cn.customer_id, c.name AS customer_name,
			       cn.cn_date, cn.refund_method, cn.reason, cn.status,
			       cn.total_cents, cn.ar_deducted_cents, cn.cogs_reversed_cents
			FROM credit_notes cn
			JOIN customers c ON c.tenant_id = cn.tenant_id AND c.id = cn.customer_id
			WHERE cn.tenant_id = $1`
		args := []any{tenant}
		if statusFilter != "" {
			query += " AND cn.status = $2"
			args = append(args, statusFilter)
		}
		query += " ORDER BY cn.cn_date DESC, cn.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []creditNoteResponse{}
		for rows.Next() {
			var item creditNoteResponse
			var cnDate pgtype.Date
			var reason pgtype.Text
			if err := rows.Scan(&item.ID, &item.Number, &item.InvoiceID, &item.CustomerID, &item.CustomerName,
				&cnDate, &item.RefundMethod, &reason, &item.Status,
				&item.TotalCents, &item.ARDeductedCents, &item.COGSReversedCents); err != nil {
				return err
			}
			item.CNDate = dateString(cnDate)
			item.Reason = textValue(reason)
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "CN_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetCreditNote returns one credit note with its lines.
func (service *Service) GetCreditNote(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *creditNoteResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		result, err = service.fetchCN(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "CN_NOT_FOUND", "credit note not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "CN_FETCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// CN helpers
// ---------------------------------------------------------------------------

func validateCNRequest(req CreateCreditNoteRequest) (string, string) {
	if req.InvoiceID <= 0 {
		return "INVALID_REQUEST", "invoice_id is required"
	}
	if !validDate(req.CNDate) {
		return "INVALID_REQUEST", "cn_date must be a valid date in YYYY-MM-DD format"
	}
	if req.RefundMethod != "" && req.RefundMethod != "deduct" && req.RefundMethod != "refund" && req.RefundMethod != "credit_balance" {
		return "INVALID_REQUEST", "refund_method must be deduct, refund, or credit_balance"
	}
	if len(req.Lines) == 0 {
		return "INVALID_REQUEST", "at least one line is required"
	}
	for index, line := range req.Lines {
		if line.ItemID <= 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: item_id is required", index)
		}
		if line.Qty <= 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: qty must be greater than 0", index)
		}
		if line.UnitPriceCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: unit_price_cents must be >= 0", index)
		}
		if line.UnitCostCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: unit_cost_cents must be >= 0", index)
		}
	}
	return "", ""
}

func cnErrorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "INVOICE_NOT_FOUND", "invoice not found"
	}
	var overflow dpOverflowError
	if errors.As(err, &overflow) {
		return http.StatusConflict, "DP_EXCEEDS_ORDER", overflow.Error()
	}
	return http.StatusInternalServerError, "CN_CREATE_FAILED", err.Error()
}

func (service *Service) fetchCN(ctx context.Context, tx pgx.Tx, tenant, id int64) (*creditNoteResponse, error) {
	result := &creditNoteResponse{}
	var cnDate pgtype.Date
	var reason pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT cn.id, cn.number, cn.invoice_id, cn.customer_id, c.name AS customer_name,
		       cn.cn_date, cn.refund_method, cn.reason, cn.status,
		       cn.total_cents, cn.ar_deducted_cents, cn.cogs_reversed_cents
		FROM credit_notes cn
		JOIN customers c ON c.tenant_id = cn.tenant_id AND c.id = cn.customer_id
		WHERE cn.tenant_id = $1 AND cn.id = $2
	`, tenant, id).Scan(&result.ID, &result.Number, &result.InvoiceID, &result.CustomerID, &result.CustomerName,
		&cnDate, &result.RefundMethod, &reason, &result.Status,
		&result.TotalCents, &result.ARDeductedCents, &result.COGSReversedCents)
	if err != nil {
		return nil, err
	}
	result.CNDate = dateString(cnDate)
	result.Reason = textValue(reason)

	rows, err := tx.Query(ctx, `
		SELECT l.id, l.item_id, i.code AS item_code, i.name AS item_name,
		       l.invoice_line_id, l.line_no, l.qty, l.unit_price_cents, l.unit_cost_cents,
		       l.line_total_cents, l.cogs_reversed_cents, l.description
		FROM credit_note_lines l
		LEFT JOIN items i ON i.tenant_id = l.tenant_id AND i.id = l.item_id
		WHERE l.tenant_id = $1 AND l.credit_note_id = $2
		ORDER BY l.line_no
	`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result.Lines = []cnLineResponse{}
	for rows.Next() {
		var line cnLineResponse
		var qty pgtype.Numeric
		var desc, itemCode, itemName pgtype.Text
		var invLineID pgtype.Int8
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &invLineID,
			&line.LineNo, &qty, &line.UnitPriceCents, &line.UnitCostCents,
			&line.LineTotalCents, &line.COGSReversedCents, &desc); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(desc)
		if invLineID.Valid {
			line.InvoiceLineID = invLineID.Int64
		}
		result.Lines = append(result.Lines, line)
	}
	return result, rows.Err()
}

func (service *Service) findCNByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (creditNoteResponse, error) {
	var result creditNoteResponse
	var cnDate pgtype.Date
	var reason pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, number, invoice_id, customer_id, cn_date, refund_method,
		       reason, status, total_cents, ar_deducted_cents, cogs_reversed_cents
		FROM credit_notes
		WHERE tenant_id = $1 AND revenue_journal_entry_id = $2
	`, tenant, journalID).Scan(&result.ID, &result.Number, &result.InvoiceID, &result.CustomerID,
		&cnDate, &result.RefundMethod, &reason, &result.Status,
		&result.TotalCents, &result.ARDeductedCents, &result.COGSReversedCents)
	if err != nil {
		return creditNoteResponse{}, err
	}
	result.CNDate = dateString(cnDate)
	result.Reason = textValue(reason)
	return result, nil
}

func nextCNNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'CN', 'CN', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}
