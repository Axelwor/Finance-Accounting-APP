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
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
)

// Supplier invoice statuses.
const (
	siIssued        = "ISSUED"
	siPartiallyPaid = "PARTIALLY_PAID"
	siPaid          = "PAID"
	siVoid          = "VOID"
)

// SupplierInvoiceLineRequest is one line of a create-supplier-invoice request.
type SupplierInvoiceLineRequest struct {
	ItemID         int64   `json:"item_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxRate        float64 `json:"tax_rate"`
	Description    string  `json:"description"`
}

// CreateSupplierInvoiceRequest is the POST /supplier-invoices body.
type CreateSupplierInvoiceRequest struct {
	SupplierID            int64                        `json:"supplier_id"`
	GRNID                 int64                        `json:"grn_id"`
	InvoiceDate           string                       `json:"invoice_date"`
	DueDate               string                       `json:"due_date"`
	SupplierInvoiceNumber string                       `json:"supplier_invoice_number"`
	DPAppliedCents        int64                        `json:"dp_applied_cents"`
	Notes                 string                       `json:"notes"`
	Lines                 []SupplierInvoiceLineRequest `json:"lines"`
}

type supplierInvoiceLineResponse struct {
	ID             int64   `json:"id"`
	ItemID         int64   `json:"item_id"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	LineNo         int     `json:"line_no"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxRate        float64 `json:"tax_rate"`
	LineTotalCents int64   `json:"line_total_cents"`
	Description    string  `json:"description"`
}

type supplierInvoiceResponse struct {
	ID                    int64                         `json:"id"`
	Number                string                        `json:"number"`
	SupplierID            int64                         `json:"supplier_id"`
	SupplierName          string                        `json:"supplier_name"`
	GRNID                 int64                         `json:"grn_id,omitempty"`
	InvoiceDate           string                        `json:"invoice_date"`
	DueDate               string                        `json:"due_date"`
	SupplierInvoiceNumber string                        `json:"supplier_invoice_number,omitempty"`
	DPPCents              int64                         `json:"dpp_cents"`
	VATCents              int64                         `json:"vat_cents"`
	TotalCents            int64                         `json:"total_cents"`
	DPAppliedCents        int64                         `json:"dp_applied_cents"`
	PayableCents          int64                         `json:"payable_cents"`
	Notes                 string                        `json:"notes"`
	Status                string                        `json:"status"`
	JournalEntryID        int64                         `json:"journal_entry_id,omitempty"`
	Lines                 []supplierInvoiceLineResponse `json:"lines,omitempty"`
}

// CreateSupplierInvoice handles POST /supplier-invoices.
//
// It records a supplier invoice (Tagihan) and posts one balanced journal:
//
//	Dr 2105 Uninvoiced Payables  (dpp_cents)
//	Dr 1203 Input VAT            (vat_cents, if any)
//	Cr 2101 Accounts Payable     (dpp_cents + vat_cents)
//
// DP realization (Dr 2101 / Cr 1205) is deferred until the purchase DP table
// exists — for now only the reclassification journal is posted.
func (service *Service) CreateSupplierInvoice(writer http.ResponseWriter, request *http.Request) {
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
	var req CreateSupplierInvoiceRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateSupplierInvoiceRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	uid := userID(request)
	requestHash := httperr.ComputeRequestHash(request)

	var result supplierInvoiceResponse
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
			// M-023: verify payload match by comparing request hashes.
			var storedHash string
			_ = tx.QueryRow(request.Context(), `SELECT COALESCE(request_hash, '') FROM journal_entries WHERE id = $1`, existing.ID).Scan(&storedHash)
			if err := httperr.CheckIdempotencyHash(storedHash, requestHash); err != nil {
				return httperr.ErrIdempotencyKeyReuse
			}
			inv, err := fetchSupplierInvoiceByJournal(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = inv
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Verify supplier exists.
		var supplierName string
		if err := tx.QueryRow(request.Context(), `
			SELECT name FROM suppliers WHERE tenant_id = $1 AND id = $2
		`, tenant, req.SupplierID).Scan(&supplierName); err != nil {
			return err
		}

		// Prepare lines and compute DPP / VAT / total.
		lines, dppCents, vatCents, err := prepareSupplierInvoiceLines(req.Lines)
		if err != nil {
			return err
		}
		totalCents := dppCents + vatCents

		// Resolve accounts.
		uninvoicedAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, uninvoicedPayableCode)
		if err != nil {
			return err
		}
		apAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, apAccountCode)
		if err != nil {
			return err
		}
		inputVATAcctID := int64(0)
		if vatCents > 0 {
			inputVATAcctID, err = resolveAccountByCode(request.Context(), tx, tenant, inputVATAccountCode)
			if err != nil {
				return err
			}
		}

		// Build journal: Dr 2105 (dpp) / Dr 1203 (vat) / Cr 2101 (dpp+vat).
		journalLines := []accounting.Line{
			{AccountID: uninvoicedAcctID, DebitCents: dppCents, SourceLineRef: "uninvoiced"},
			{AccountID: apAcctID, CreditCents: totalCents, SourceLineRef: "ap"},
		}
		if vatCents > 0 {
			journalLines = append(journalLines, accounting.Line{
				AccountID: inputVATAcctID, DebitCents: vatCents, SourceLineRef: "vat",
			})
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("BIL-%d", req.SupplierID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("SUPPLIER_INVOICE"),
			EntryDate:   req.InvoiceDate,
			Description: fmt.Sprintf("Supplier invoice: %s", supplierName),
			Lines:       journalLines,
		}
		// Hash-chain.
		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashJournal(journal)

		// Resolve period.
		periodID, err := resolvePeriod(request.Context(), tx, tenant, journal.EntryDate)
		if err != nil {
			return err
		}
		// Allocate journal number.
		jrnNumber, err := nextJournalNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		// Insert journal entry.
		var entryID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by, request_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id
		`, tenant, jrnNumber, journal.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem,
			journal.Hash, journal.PreviousHash, int8Value(uid), textValueOptional(requestHash)).Scan(&entryID)
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
		if err := insertOutbox(request.Context(), tx, tenant, "supplier-invoice.posted", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber,
		})); err != nil {
			return err
		}

		// A-20: Update the AP sub-ledger — increase AP by the invoice total.
		if err := upsertSupplierBalance(request.Context(), tx, tenant, req.SupplierID, totalCents, 0); err != nil {
			return err
		}

		// Allocate BIL number.
		bilNumber, err := nextDocNumber(request.Context(), tx, tenant, "BIL", "BIL")
		if err != nil {
			return err
		}
		invoiceDate, err := parseDate(req.InvoiceDate)
		if err != nil {
			return err
		}
		dueDate, err := optionalDate(req.DueDate)
		if err != nil {
			return err
		}
		var grnID pgtype.Int8
		if req.GRNID > 0 {
			grnID = pgtype.Int8{Int64: req.GRNID, Valid: true}
		}
		// A-21: DP realization — the caller may pass dp_applied_cents to
		// reduce the payable by the amount of supplier prepayment (account
		// 1205) to realize against this invoice. The full DP realization
		// journal (Dr 1205 Purchase Prepayment / Cr 2101 Accounts Payable)
		// is NOT yet posted here — only the field is accepted so the
		// payable is reduced correctly.
		// TODO(A-21): implement the DP realization journal when the
		// purchase DP table and prepayment balance lookup exist.
		dpApplied := req.DPAppliedCents
		if dpApplied < 0 {
			dpApplied = 0
		}
		if dpApplied > totalCents {
			dpApplied = totalCents
		}
		payable := totalCents - dpApplied

		// Insert supplier invoice header.
		var invID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO supplier_invoices
				(tenant_id, number, supplier_id, grn_id, invoice_date, due_date,
				 supplier_invoice_number, dpp_cents, vat_cents, total_cents,
				 dp_applied_cents, payable_cents, notes, status, journal_entry_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'ISSUED', $14, $15)
			RETURNING id
		`, tenant, bilNumber, req.SupplierID, grnID, invoiceDate, dueDate,
			textValueOptional(req.SupplierInvoiceNumber), dppCents, vatCents, totalCents,
			dpApplied, payable, textValueOptional(req.Notes), entryID, int8Value(uid)).Scan(&invID)
		if err != nil {
			return err
		}

		// Insert supplier invoice lines.
		result.Lines = make([]supplierInvoiceLineResponse, 0, len(lines))
		for i, p := range lines {
			var lineID int64
			var itemCode, itemName pgtype.Text
			err := tx.QueryRow(request.Context(), `
				INSERT INTO supplier_invoice_lines
					(tenant_id, invoice_id, item_id, line_no, qty, unit_price_cents,
					 discount_cents, tax_rate, line_total_cents, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				RETURNING id, (SELECT code FROM items WHERE tenant_id = $1 AND id = $3),
				         (SELECT name FROM items WHERE tenant_id = $1 AND id = $3)
			`, tenant, invID, optionalInt8(p.Line.ItemID), i+1,
				pgtypeFloat(p.Line.Qty), p.Line.UnitPriceCents, p.Line.DiscountCents,
				pgtypeFloat(p.Line.TaxRate), p.LineTotalCents,
				textValueOptional(p.Line.Description)).Scan(&lineID, &itemCode, &itemName)
			if err != nil {
				return err
			}
			result.Lines = append(result.Lines, supplierInvoiceLineResponse{
				ID:             lineID,
				ItemID:         p.Line.ItemID,
				ItemCode:       textValue(itemCode),
				ItemName:       textValue(itemName),
				LineNo:         i + 1,
				Qty:            p.Line.Qty,
				UnitPriceCents: p.Line.UnitPriceCents,
				DiscountCents:  p.Line.DiscountCents,
				TaxRate:        p.Line.TaxRate,
				LineTotalCents: p.LineTotalCents,
				Description:    p.Line.Description,
			})
		}

		result.ID = invID
		result.Number = bilNumber
		result.SupplierID = req.SupplierID
		result.SupplierName = supplierName
		if grnID.Valid {
			result.GRNID = grnID.Int64
		}
		result.InvoiceDate = dateString(invoiceDate)
		result.DueDate = dateString(dueDate)
		result.SupplierInvoiceNumber = strings.TrimSpace(req.SupplierInvoiceNumber)
		result.DPPCents = dppCents
		result.VATCents = vatCents
		result.TotalCents = totalCents
		result.DPAppliedCents = dpApplied
		result.PayableCents = payable
		result.Notes = strings.TrimSpace(req.Notes)
		result.Status = siIssued
		result.JournalEntryID = entryID

		if err := audit.Log(request.Context(), tx, tenant, uid, "supplier_invoice", invID, audit.ActionPost, nil, map[string]any{
			"number":           bilNumber,
			"supplier_id":      req.SupplierID,
			"total_cents":      totalCents,
			"dpp_cents":        dppCents,
			"vat_cents":        vatCents,
			"payable_cents":    payable,
			"journal_entry_id": entryID,
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, httperr.ErrIdempotencyKeyReuse) {
			writeError(writer, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", err.Error())
			return
		}
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier does not exist for this tenant")
			return
		}
		status, code := httperr.Classify(err)
		writeError(writer, status, code, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListSupplierInvoices handles GET /supplier-invoices (optional ?status=).
func (service *Service) ListSupplierInvoices(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	status := strings.TrimSpace(request.URL.Query().Get("status"))

	var results []supplierInvoiceResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT si.id, si.number, si.supplier_id, s.name, si.grn_id,
			       si.invoice_date, si.due_date, si.supplier_invoice_number,
			       si.dpp_cents, si.vat_cents, si.total_cents, si.dp_applied_cents,
			       si.payable_cents, si.notes, si.status, si.journal_entry_id
			FROM supplier_invoices si
			JOIN suppliers s ON s.tenant_id = si.tenant_id AND s.id = si.supplier_id
		`
		args := []any{}
		if status != "" {
			query += " WHERE si.status = $1"
			args = append(args, status)
		}
		query += " ORDER BY si.invoice_date DESC, si.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []supplierInvoiceResponse{}
		for rows.Next() {
			var inv supplierInvoiceResponse
			var invoiceDate, dueDate pgtype.Date
			var grnID pgtype.Int8
			var supplierInvNo, notes pgtype.Text
			var journalID pgtype.Int8
			if err := rows.Scan(&inv.ID, &inv.Number, &inv.SupplierID, &inv.SupplierName, &grnID,
				&invoiceDate, &dueDate, &supplierInvNo,
				&inv.DPPCents, &inv.VATCents, &inv.TotalCents, &inv.DPAppliedCents,
				&inv.PayableCents, &notes, &inv.Status, &journalID); err != nil {
				return err
			}
			if grnID.Valid {
				inv.GRNID = grnID.Int64
			}
			inv.InvoiceDate = dateString(invoiceDate)
			inv.DueDate = dateString(dueDate)
			inv.SupplierInvoiceNumber = textValue(supplierInvNo)
			inv.Notes = textValue(notes)
			if journalID.Valid {
				inv.JournalEntryID = journalID.Int64
			}
			results = append(results, inv)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "SUPPLIER_INVOICE_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetSupplierInvoice handles GET /supplier-invoices/{id}.
func (service *Service) GetSupplierInvoice(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	invID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *supplierInvoiceResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = fetchSupplierInvoice(request.Context(), tx, tenant, invID)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "SUPPLIER_INVOICE_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Supplier invoice helpers
// ---------------------------------------------------------------------------

// validateSupplierInvoiceRequest validates the POST /supplier-invoices body.
func validateSupplierInvoiceRequest(req CreateSupplierInvoiceRequest) (string, string) {
	if req.SupplierID <= 0 {
		return "INVALID_REQUEST", "supplier_id is required"
	}
	if !validDate(req.InvoiceDate) {
		return "INVALID_REQUEST", "invoice_date must be a valid date in YYYY-MM-DD format"
	}
	if req.DueDate != "" && !validDate(req.DueDate) {
		return "INVALID_REQUEST", "due_date must be a valid date in YYYY-MM-DD format"
	}
	if req.DPAppliedCents < 0 {
		return "INVALID_REQUEST", "dp_applied_cents must be >= 0"
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
		if line.DiscountCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: discount_cents must be >= 0", index)
		}
		if line.TaxRate < 0 || line.TaxRate > 100 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: tax_rate must be between 0 and 100", index)
		}
	}
	return "", ""
}

// preparedSupplierInvoiceLine carries a validated supplier invoice line plus
// its computed line total (DPP contribution).
type preparedSupplierInvoiceLine struct {
	Line           SupplierInvoiceLineRequest
	LineTotalCents int64
}

// prepareSupplierInvoiceLines validates each line, computes line totals, and
// returns the prepared lines, the DPP (sum of line totals), and the VAT
// (sum of line_total × tax_rate).
func prepareSupplierInvoiceLines(lines []SupplierInvoiceLineRequest) ([]preparedSupplierInvoiceLine, int64, int64, error) {
	prepared := make([]preparedSupplierInvoiceLine, 0, len(lines))
	var dpp int64
	var vat int64
	for _, line := range lines {
		if line.Qty <= 0 {
			return nil, 0, 0, fmt.Errorf("lines: qty must be greater than 0")
		}
		if line.UnitPriceCents < 0 {
			return nil, 0, 0, fmt.Errorf("lines: unit_price_cents must be >= 0")
		}
		if line.DiscountCents < 0 {
			return nil, 0, 0, fmt.Errorf("lines: discount_cents must be >= 0")
		}
		if line.TaxRate < 0 || line.TaxRate > 100 {
			return nil, 0, 0, fmt.Errorf("lines: tax_rate must be between 0 and 100")
		}
		lineTotal := supplierLineTotalCents(line.Qty, line.UnitPriceCents, line.DiscountCents)
		dpp += lineTotal
		vat += supplierVATCents(lineTotal, line.TaxRate)
		prepared = append(prepared, preparedSupplierInvoiceLine{Line: line, LineTotalCents: lineTotal})
	}
	return prepared, dpp, vat, nil
}

// supplierLineTotalCents computes line_total_cents = qty * unit_price_cents - discount_cents.
// qty is NUMERIC(18,3); converted to float64 for the multiplication, gross is
// rounded to whole cents before the (already integer) discount is applied.
func supplierLineTotalCents(qty float64, unitPriceCents, discountCents int64) int64 {
	gross := qty * float64(unitPriceCents)
	return roundCents(gross) - discountCents
}

// supplierVATCents computes the VAT (PPN masukan) for a line:
// round(line_total_cents * tax_rate / 100).
func supplierVATCents(lineTotalCents int64, taxRate float64) int64 {
	if taxRate <= 0 {
		return 0
	}
	return roundCents(float64(lineTotalCents) * taxRate / 100.0)
}

// roundCents rounds a float to the nearest whole cent using math.Round
// (round-half-away-from-zero).
func roundCents(value float64) int64 {
	return int64(math.Round(value))
}

// fetchSupplierInvoice loads a supplier invoice (with lines) by id.
func fetchSupplierInvoice(ctx context.Context, tx pgx.Tx, tenant, invID int64) (*supplierInvoiceResponse, error) {
	var inv supplierInvoiceResponse
	var invoiceDate, dueDate pgtype.Date
	var grnID pgtype.Int8
	var supplierInvNo, notes pgtype.Text
	var journalID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT si.id, si.number, si.supplier_id, s.name, si.grn_id,
		       si.invoice_date, si.due_date, si.supplier_invoice_number,
		       si.dpp_cents, si.vat_cents, si.total_cents, si.dp_applied_cents,
		       si.payable_cents, si.notes, si.status, si.journal_entry_id
		FROM supplier_invoices si
		JOIN suppliers s ON s.tenant_id = si.tenant_id AND s.id = si.supplier_id
		WHERE si.tenant_id = $1 AND si.id = $2
	`, tenant, invID).Scan(&inv.ID, &inv.Number, &inv.SupplierID, &inv.SupplierName, &grnID,
		&invoiceDate, &dueDate, &supplierInvNo,
		&inv.DPPCents, &inv.VATCents, &inv.TotalCents, &inv.DPAppliedCents,
		&inv.PayableCents, &notes, &inv.Status, &journalID)
	if err != nil {
		return nil, err
	}
	if grnID.Valid {
		inv.GRNID = grnID.Int64
	}
	inv.InvoiceDate = dateString(invoiceDate)
	inv.DueDate = dateString(dueDate)
	inv.SupplierInvoiceNumber = textValue(supplierInvNo)
	inv.Notes = textValue(notes)
	if journalID.Valid {
		inv.JournalEntryID = journalID.Int64
	}

	rows, err := tx.Query(ctx, `
		SELECT sil.id, sil.item_id, i.code, i.name, sil.line_no, sil.qty,
		       sil.unit_price_cents, sil.discount_cents, sil.tax_rate,
		       sil.line_total_cents, sil.description
		FROM supplier_invoice_lines sil
		LEFT JOIN items i ON i.tenant_id = sil.tenant_id AND i.id = sil.item_id
		WHERE sil.tenant_id = $1 AND sil.invoice_id = $2
		ORDER BY sil.line_no
	`, tenant, invID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	inv.Lines = []supplierInvoiceLineResponse{}
	for rows.Next() {
		var line supplierInvoiceLineResponse
		var itemCode, itemName, desc pgtype.Text
		var qty, taxRate pgtype.Numeric
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&qty, &line.UnitPriceCents, &line.DiscountCents, &taxRate,
			&line.LineTotalCents, &desc); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.TaxRate = numericToFloat(taxRate)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(desc)
		inv.Lines = append(inv.Lines, line)
	}
	return &inv, rows.Err()
}

// fetchSupplierInvoiceByJournal loads a supplier invoice by its journal entry id
// (used for idempotent replay).
func fetchSupplierInvoiceByJournal(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (supplierInvoiceResponse, error) {
	var inv supplierInvoiceResponse
	var invoiceDate, dueDate pgtype.Date
	var grnID pgtype.Int8
	var supplierInvNo, notes pgtype.Text
	var journalIDOut pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT si.id, si.number, si.supplier_id, s.name, si.grn_id,
		       si.invoice_date, si.due_date, si.supplier_invoice_number,
		       si.dpp_cents, si.vat_cents, si.total_cents, si.dp_applied_cents,
		       si.payable_cents, si.notes, si.status, si.journal_entry_id
		FROM supplier_invoices si
		JOIN suppliers s ON s.tenant_id = si.tenant_id AND s.id = si.supplier_id
		WHERE si.tenant_id = $1 AND si.journal_entry_id = $2
	`, tenant, journalID).Scan(&inv.ID, &inv.Number, &inv.SupplierID, &inv.SupplierName, &grnID,
		&invoiceDate, &dueDate, &supplierInvNo,
		&inv.DPPCents, &inv.VATCents, &inv.TotalCents, &inv.DPAppliedCents,
		&inv.PayableCents, &notes, &inv.Status, &journalIDOut)
	if err != nil {
		return supplierInvoiceResponse{}, err
	}
	if grnID.Valid {
		inv.GRNID = grnID.Int64
	}
	inv.InvoiceDate = dateString(invoiceDate)
	inv.DueDate = dateString(dueDate)
	inv.SupplierInvoiceNumber = textValue(supplierInvNo)
	inv.Notes = textValue(notes)
	if journalIDOut.Valid {
		inv.JournalEntryID = journalIDOut.Int64
	}
	return inv, nil
}
