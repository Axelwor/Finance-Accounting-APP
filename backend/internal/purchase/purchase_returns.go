package purchase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/costing"
	"finance-accounting-app/backend/internal/db"
)

// PR (purchase return) statuses.
const (
	prApplied = "APPLIED"
	prVoid    = "VOID"
)

// Supplier invoice statuses (mirror of supplier_invoices migration 000012).
const (
	invIssued        = "ISSUED"
	invPartiallyPaid = "PARTIALLY_PAID"
	invPaid          = "PAID"
	invVoid          = "VOID"
)

// PurchaseReturnLineRequest is one line of a create-PR request.
type PurchaseReturnLineRequest struct {
	ItemID         int64   `json:"item_id"`
	InvoiceLineID  int64   `json:"invoice_line_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	Description    string  `json:"description"`
}

// CreatePurchaseReturnRequest is the POST /purchase-returns body.
type CreatePurchaseReturnRequest struct {
	InvoiceID    int64                       `json:"invoice_id"`
	SupplierID   int64                       `json:"supplier_id"`
	ReturnDate   string                      `json:"return_date"`
	RefundMethod string                      `json:"refund_method"`
	Reason       string                      `json:"reason"`
	Lines        []PurchaseReturnLineRequest `json:"lines"`
}

type prLineResponse struct {
	ID             int64   `json:"id"`
	ItemID         int64   `json:"item_id"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	InvoiceLineID  int64   `json:"invoice_line_id,omitempty"`
	LineNo         int     `json:"line_no"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	LineTotalCents int64   `json:"line_total_cents"`
	Description    string  `json:"description"`
}

type purchaseReturnResponse struct {
	ID               int64            `json:"id"`
	Number           string           `json:"number"`
	SupplierID       int64            `json:"supplier_id"`
	SupplierName     string           `json:"supplier_name"`
	InvoiceID        int64            `json:"invoice_id"`
	ReturnDate       string           `json:"return_date"`
	RefundMethod     string           `json:"refund_method"`
	Reason           string           `json:"reason"`
	Status           string           `json:"status"`
	TotalCents       int64            `json:"total_cents"`
	VATReversedCents int64            `json:"vat_reversed_cents"`
	APDeductedCents  int64            `json:"ap_deducted_cents"`
	Lines            []prLineResponse `json:"lines,omitempty"`
}

// CreatePurchaseReturn posts a purchase return (Retur Pembelian). It posts one
// balanced journal:
//
//	Dr 2101 Accounts Payable (total + vat_reversed) — AP goes back up
//	Cr 1301 Inventory (total) — reduce inventory
//	Cr 1203 Input VAT (vat_reversed) — reverse input VAT (if any)
//
// The supplier invoice's payable_cents is increased (AP owed rises).
// Inventory movements are recorded (qty negative = stock out).
func (service *Service) CreatePurchaseReturn(writer http.ResponseWriter, request *http.Request) {
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
	var req CreatePurchaseReturnRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateReturnRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	userID := userID(request)

	var result purchaseReturnResponse
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
			pr, err := service.findPRByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = pr
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load the supplier invoice.
		var invNumber string
		var invSupplierID int64
		var invStatus string
		var payable int64
		var dppCents int64
		var vatCents int64
		err = tx.QueryRow(request.Context(), `
			SELECT number, supplier_id, status, payable_cents, dpp_cents, vat_cents
			FROM supplier_invoices WHERE tenant_id = $1 AND id = $2
			FOR UPDATE
		`, tenant, req.InvoiceID).Scan(&invNumber, &invSupplierID, &invStatus, &payable, &dppCents, &vatCents)
		if err != nil {
			return err
		}
		if invStatus == invVoid {
			return fmt.Errorf("supplier invoice %s is VOID", invNumber)
		}
		if req.SupplierID != invSupplierID {
			return fmt.Errorf("supplier_id %d does not match invoice %s supplier %d", req.SupplierID, invNumber, invSupplierID)
		}
		var supplierName string
		if err := tx.QueryRow(request.Context(), `
			SELECT name FROM suppliers WHERE tenant_id = $1 AND id = $2
		`, tenant, req.SupplierID).Scan(&supplierName); err != nil {
			return err
		}

		// Compute the invoice's VAT rate so each return line can reverse its
		// proportional VAT. If the invoice has no VAT, vat_reversed is 0.
		var vatRate float64
		if dppCents > 0 && vatCents > 0 {
			vatRate = float64(vatCents) / float64(dppCents)
		}

		// Prepare lines: compute totals and VAT reversal per line.
		type preparedPRLine struct {
			line          PurchaseReturnLineRequest
			lineTotal     int64
			vatReversed   int64
			costingMethod string
		}
		prepared := make([]preparedPRLine, 0, len(req.Lines))
		var totalReturn int64
		var totalVATReversed int64
		for _, line := range req.Lines {
			lineTotal := returnLineTotal(line.Qty, line.UnitPriceCents)
			vatReversed := vatReversedForReturn(lineTotal, vatRate)
			totalReturn += lineTotal
			totalVATReversed += vatReversed
			// Read the item's costing method so the costing reversal can be
			// applied correctly (PSAK 14).
			var costingMethod pgtype.Text
			_ = tx.QueryRow(request.Context(), `
				SELECT costing_method FROM items WHERE tenant_id = $1 AND id = $2
			`, tenant, line.ItemID).Scan(&costingMethod)
			prepared = append(prepared, preparedPRLine{
				line:          line,
				lineTotal:     lineTotal,
				vatReversed:   vatReversed,
				costingMethod: textValue(costingMethod),
			})
		}

		// Return total (incl. VAT) cannot exceed invoice total (dpp + vat).
		invoiceTotal := dppCents + vatCents
		if totalReturn+totalVATReversed > invoiceTotal {
			return &returnExceedsError{
				returnCents:  totalReturn + totalVATReversed,
				invoiceCents: invoiceTotal,
			}
		}

		// Pre-check stock availability so the user gets a clear error
		// before the journal is posted. ResolveCOGS would reject negative
		// stock later in the transaction, but only after the journal and
		// sub-ledger have already been written (the tx rolls back, but the
		// error is opaque).
		for _, p := range prepared {
			qoh, _, err := costing.GetStockBalance(request.Context(), tx, tenant, p.line.ItemID, 0)
			if err != nil {
				return err
			}
			if qoh < p.line.Qty {
				return &insufficientStockError{
					itemID: p.line.ItemID,
					qoh:    qoh,
					need:   p.line.Qty,
				}
			}
		}

		// Resolve accounts: 2101 AP, 1301 Inventory, 1203 Input VAT.
		apAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, apAccountCode)
		if err != nil {
			return err
		}
		inventoryAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, inventoryAccountCode)
		if err != nil {
			return err
		}
		inputVATAccountID := int64(0)
		if totalVATReversed > 0 {
			inputVATAccountID, err = resolveAccountByCode(request.Context(), tx, tenant, inputVATAccountCode)
			if err != nil {
				return err
			}
		}

		// Build the single balanced journal:
		//   Dr 2101 AP (total + vat_reversed)
		//   Cr 1301 Inventory (total)
		//   Cr 1203 Input VAT (vat_reversed, if any)
		journalLines := []accounting.Line{
			{AccountID: apAccountID, DebitCents: totalReturn + totalVATReversed, SourceLineRef: "ap"},
			{AccountID: inventoryAccountID, CreditCents: totalReturn, SourceLineRef: "inventory"},
		}
		if totalVATReversed > 0 {
			journalLines = append(journalLines, accounting.Line{
				AccountID: inputVATAccountID, CreditCents: totalVATReversed, SourceLineRef: "input_vat",
			})
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("PRET-%d", req.InvoiceID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("PURCHASE_RETURN"),
			EntryDate:   req.ReturnDate,
			Description: fmt.Sprintf("Purchase return: supplier invoice %s", invNumber),
			Lines:       journalLines,
		}
		// Hash-chain.
		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashJournal(journal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, journal.EntryDate)
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
		`, tenant, jrnNumber, journal.EntryDate, periodID, journal.Description,
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
		if err := insertOutbox(request.Context(), tx, tenant, "pret.applied", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber,
		})); err != nil {
			return err
		}

		// A-20: Update the AP sub-ledger — AP goes back up by the return
		// total plus reversed VAT (the supplier owes us more).
		if err := upsertSupplierBalance(request.Context(), tx, tenant, req.SupplierID,
			totalReturn+totalVATReversed, 0); err != nil {
			return err
		}

		// Record inventory movements (qty negative = stock out).
		for _, p := range prepared {
			var negQty pgtype.Numeric
			_ = negQty.Scan(fmt.Sprintf("%g", -p.line.Qty))
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
				VALUES ($1, $2, 'PURCHASE_RETURN', $3, $4, $5, $6)
			`, tenant, p.line.ItemID, negQty, p.line.UnitPriceCents,
				fmt.Sprintf("PRET-%d", req.InvoiceID), 0); err != nil {
				return err
			}
			// Reduce the cost layers / average cost to reflect inventory
			// leaving the warehouse (stock out to supplier, PSAK 14).
			// ResolveCOGS decreases qty_on_hand — correct for purchase
			// returns where stock goes back to the supplier. The GL side
			// (Cr 1301 Inventory) is already handled by the journal above.
			if _, err := costing.ResolveCOGS(request.Context(), tx, tenant, p.line.ItemID, 0,
				p.line.Qty, p.costingMethod); err != nil {
				return err
			}
		}

		// Update supplier invoice: increase payable_cents (AP owed rises).
		newPayable := payable + totalReturn + totalVATReversed
		newStatus := invStatus
		if newStatus == invPaid {
			newStatus = invPartiallyPaid
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE supplier_invoices SET payable_cents = $1, status = $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, newPayable, newStatus, tenant, req.InvoiceID); err != nil {
			return err
		}

		// Allocate PRET number and insert the return header + lines.
		prNumber, err := nextDocNumber(request.Context(), tx, tenant, "PRET", "PRET")
		if err != nil {
			return err
		}
		returnDate, err := parseDate(req.ReturnDate)
		if err != nil {
			return err
		}
		refundMethod := req.RefundMethod
		if refundMethod == "" {
			refundMethod = "deduct"
		}
		var prID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO purchase_returns
				(tenant_id, number, supplier_id, invoice_id, return_date, refund_method,
				 reason, status, total_cents, vat_reversed_cents, ap_deducted_cents,
				 journal_entry_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'APPLIED', $8, $9, $10, $11, $12)
			RETURNING id
		`, tenant, prNumber, req.SupplierID, req.InvoiceID, returnDate, refundMethod,
			textValueOptional(req.Reason), totalReturn, totalVATReversed, totalReturn+totalVATReversed,
			int8Value(entryID), int8Value(userID)).Scan(&prID)
		if err != nil {
			return err
		}
		for position, p := range prepared {
			var qty pgtype.Numeric
			_ = qty.Scan(fmt.Sprintf("%g", p.line.Qty))
			lineNo := position + 1
			var invLineID pgtype.Int8
			if p.line.InvoiceLineID > 0 {
				invLineID = pgtype.Int8{Int64: p.line.InvoiceLineID, Valid: true}
			}
			var itemID pgtype.Int8
			if p.line.ItemID > 0 {
				itemID = pgtype.Int8{Int64: p.line.ItemID, Valid: true}
			}
			_, err := tx.Exec(request.Context(), `
				INSERT INTO purchase_return_lines
					(tenant_id, return_id, item_id, invoice_line_id, line_no, qty,
					 unit_price_cents, line_total_cents, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, tenant, prID, itemID, invLineID, lineNo, qty,
				p.line.UnitPriceCents, p.lineTotal, textValueOptional(p.line.Description))
			if err != nil {
				return err
			}
		}

		fetched, err := service.fetchPR(request.Context(), tx, tenant, prID)
		if err != nil {
			return err
		}
		result = *fetched
		return nil
	})
	if err != nil {
		status, code, message := prErrorFor(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListPurchaseReturns returns the tenant's purchase returns.
func (service *Service) ListPurchaseReturns(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	statusFilter := strings.TrimSpace(request.URL.Query().Get("status"))
	var results []purchaseReturnResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT pr.id, pr.number, pr.supplier_id, s.name AS supplier_name,
			       pr.invoice_id, pr.return_date, pr.refund_method, pr.reason, pr.status,
			       pr.total_cents, pr.vat_reversed_cents, pr.ap_deducted_cents
			FROM purchase_returns pr
			JOIN suppliers s ON s.tenant_id = pr.tenant_id AND s.id = pr.supplier_id
			WHERE pr.tenant_id = $1`
		args := []any{tenant}
		if statusFilter != "" {
			query += " AND pr.status = $2"
			args = append(args, statusFilter)
		}
		query += " ORDER BY pr.return_date DESC, pr.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []purchaseReturnResponse{}
		for rows.Next() {
			var item purchaseReturnResponse
			var returnDate pgtype.Date
			var reason pgtype.Text
			if err := rows.Scan(&item.ID, &item.Number, &item.SupplierID, &item.SupplierName,
				&item.InvoiceID, &returnDate, &item.RefundMethod, &reason, &item.Status,
				&item.TotalCents, &item.VATReversedCents, &item.APDeductedCents); err != nil {
				return err
			}
			item.ReturnDate = dateString(returnDate)
			item.Reason = textValue(reason)
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PR_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetPurchaseReturn returns one purchase return with its lines.
func (service *Service) GetPurchaseReturn(writer http.ResponseWriter, request *http.Request) {
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
	var result *purchaseReturnResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		result, err = service.fetchPR(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "PR_NOT_FOUND", "purchase return not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "PR_FETCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Purchase return helpers
// ---------------------------------------------------------------------------

// returnExceedsError signals the return total exceeds the invoice total.
type returnExceedsError struct {
	returnCents  int64
	invoiceCents int64
}

func (e *returnExceedsError) Error() string {
	return fmt.Sprintf("return total %d cents exceeds invoice total %d cents", e.returnCents, e.invoiceCents)
}

// insufficientStockError signals that the on-hand quantity is below the
// requested return quantity for an item.
type insufficientStockError struct {
	itemID int64
	qoh    float64
	need   float64
}

func (e *insufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock for item %d: on_hand=%.3f, return_qty=%.3f", e.itemID, e.qoh, e.need)
}

// validateReturnRequest validates the create body. Returns "" code on success.
func validateReturnRequest(req CreatePurchaseReturnRequest) (string, string) {
	if req.InvoiceID <= 0 {
		return "INVALID_REQUEST", "invoice_id is required"
	}
	if req.SupplierID <= 0 {
		return "INVALID_REQUEST", "supplier_id is required"
	}
	if !validDate(req.ReturnDate) {
		return "INVALID_REQUEST", "return_date must be a valid date in YYYY-MM-DD format"
	}
	if req.RefundMethod != "" && req.RefundMethod != "deduct" && req.RefundMethod != "refund" && req.RefundMethod != "credit_balance" {
		return "INVALID_REQUEST", "refund_method must be deduct, refund, or credit_balance"
	}
	if len(req.Lines) == 0 {
		return "INVALID_REQUEST", "at least one line is required"
	}
	for index, line := range req.Lines {
		if line.Qty <= 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: qty must be greater than 0", index)
		}
		if line.UnitPriceCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: unit_price_cents must be >= 0", index)
		}
	}
	return "", ""
}

func prErrorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "INVOICE_NOT_FOUND", "supplier invoice not found"
	}
	var overflow *returnExceedsError
	if errors.As(err, &overflow) {
		return http.StatusConflict, "RETUR_EXCEEDS_INVOICE", overflow.Error()
	}
	var stockErr *insufficientStockError
	if errors.As(err, &stockErr) {
		return http.StatusConflict, "INSUFFICIENT_STOCK", stockErr.Error()
	}
	return http.StatusInternalServerError, "PR_CREATE_FAILED", err.Error()
}

func (service *Service) fetchPR(ctx context.Context, tx pgx.Tx, tenant, id int64) (*purchaseReturnResponse, error) {
	result := &purchaseReturnResponse{}
	var returnDate pgtype.Date
	var reason pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT pr.id, pr.number, pr.supplier_id, s.name AS supplier_name,
		       pr.invoice_id, pr.return_date, pr.refund_method, pr.reason, pr.status,
		       pr.total_cents, pr.vat_reversed_cents, pr.ap_deducted_cents
		FROM purchase_returns pr
		JOIN suppliers s ON s.tenant_id = pr.tenant_id AND s.id = pr.supplier_id
		WHERE pr.tenant_id = $1 AND pr.id = $2
	`, tenant, id).Scan(&result.ID, &result.Number, &result.SupplierID, &result.SupplierName,
		&result.InvoiceID, &returnDate, &result.RefundMethod, &reason, &result.Status,
		&result.TotalCents, &result.VATReversedCents, &result.APDeductedCents)
	if err != nil {
		return nil, err
	}
	result.ReturnDate = dateString(returnDate)
	result.Reason = textValue(reason)

	rows, err := tx.Query(ctx, `
		SELECT l.id, l.item_id, i.code AS item_code, i.name AS item_name,
		       l.invoice_line_id, l.line_no, l.qty, l.unit_price_cents,
		       l.line_total_cents, l.description
		FROM purchase_return_lines l
		LEFT JOIN items i ON i.tenant_id = l.tenant_id AND i.id = l.item_id
		WHERE l.tenant_id = $1 AND l.return_id = $2
		ORDER BY l.line_no
	`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var line prLineResponse
		var itemID pgtype.Int8
		var invLineID pgtype.Int8
		var qty pgtype.Numeric
		var itemCode, itemName, description pgtype.Text
		if err := rows.Scan(&line.ID, &itemID, &itemCode, &itemName,
			&invLineID, &line.LineNo, &qty, &line.UnitPriceCents,
			&line.LineTotalCents, &description); err != nil {
			return nil, err
		}
		if itemID.Valid {
			line.ItemID = itemID.Int64
		}
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		if invLineID.Valid {
			line.InvoiceLineID = invLineID.Int64
		}
		line.Qty = numericToFloat(qty)
		line.Description = textValue(description)
		result.Lines = append(result.Lines, line)
	}
	return result, rows.Err()
}

func (service *Service) findPRByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (purchaseReturnResponse, error) {
	result := purchaseReturnResponse{}
	var returnDate pgtype.Date
	var reason pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, number, supplier_id, invoice_id, return_date, refund_method,
		       reason, status, total_cents, vat_reversed_cents, ap_deducted_cents
		FROM purchase_returns
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&result.ID, &result.Number, &result.SupplierID, &result.InvoiceID,
		&returnDate, &result.RefundMethod, &reason, &result.Status,
		&result.TotalCents, &result.VATReversedCents, &result.APDeductedCents)
	if err != nil {
		return purchaseReturnResponse{}, err
	}
	result.ReturnDate = dateString(returnDate)
	result.Reason = textValue(reason)
	return result, nil
}

// returnLineTotal computes line_total_cents = qty * unit_price_cents, rounded.
func returnLineTotal(qty float64, unitPriceCents int64) int64 {
	return int64(math.Round(qty * float64(unitPriceCents)))
}

// vatReversedForReturn computes the proportional input VAT reversed for a
// return of the given DPP amount, using the invoice's VAT rate (vat/dpp).
// Returns 0 when the rate is zero or negative.
func vatReversedForReturn(returnDPPCents int64, vatRate float64) int64 {
	if vatRate <= 0 {
		return 0
	}
	return int64(math.Round(float64(returnDPPCents) * vatRate))
}
