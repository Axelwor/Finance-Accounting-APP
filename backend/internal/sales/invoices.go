package sales

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// arAccountCode is the seeded "Accounts Receivable" account (1201).
const arAccountCode = "1201"

// revenueAccountCode is the seeded "Sales Revenue" account (4101).
const revenueAccountCode = "4101"

// outputVATAccountCode is the PPN Output (VAT Payable) account (2202).
// Seeded by migration 000027. Used when invoice lines have tax_rate > 0.
const outputVATAccountCode = "2202"

// INV statuses.
const (
	invDraft         = "DRAFT"
	invIssued        = "ISSUED"
	invPartiallyPaid = "PARTIALLY_PAID"
	invPaid          = "PAID"
	invVoid          = "VOID"
)

// InvoiceLineRequest is one line of a create-invoice request.
type InvoiceLineRequest struct {
	ItemID         int64   `json:"item_id"`
	DeliveryID     int64   `json:"delivery_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxRate        float64 `json:"tax_rate"`
	Description    string  `json:"description"`
}

// CreateInvoiceRequest is the POST /invoices body.
type CreateInvoiceRequest struct {
	SalesOrderID  int64                `json:"sales_order_id"`
	CustomerID    int64                `json:"customer_id"`
	InvoiceDate   string               `json:"invoice_date"`
	DueDate       string               `json:"due_date"`
	PaymentTermID int64                `json:"payment_term_id"`
	Notes         string               `json:"notes"`
	Lines         []InvoiceLineRequest `json:"lines"`
}

type invoiceLineResponse struct {
	ID             int64   `json:"id"`
	ItemID         int64   `json:"item_id"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	DeliveryID     int64   `json:"delivery_id,omitempty"`
	LineNo         int     `json:"line_no"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxRate        float64 `json:"tax_rate"`
	LineTotalCents int64   `json:"line_total_cents"`
	Description    string  `json:"description"`
}

type invoiceResponse struct {
	ID              int64                 `json:"id"`
	Number          string                `json:"number"`
	SalesOrderID    int64                 `json:"sales_order_id,omitempty"`
	CustomerID      int64                 `json:"customer_id"`
	CustomerName    string                `json:"customer_name"`
	InvoiceDate     string                `json:"invoice_date"`
	DueDate         string                `json:"due_date,omitempty"`
	PaymentTermID   int64                 `json:"payment_term_id"`
	Notes           string                `json:"notes"`
	Status          string                `json:"status"`
	TotalCents      int64                 `json:"total_cents"`
	DPAppliedCents  int64                 `json:"dp_applied_cents"`
	ReceivableCents int64                 `json:"receivable_cents"`
	Lines           []invoiceLineResponse `json:"lines,omitempty"`
}

// CreateInvoice posts an invoice with two journals:
// 1. Revenue: Dr 1201 AR / Cr 4101 Revenue (intent SALES_INVOICE)
// 2. DP realization: Dr 2201 Customer Deposit / Cr 1201 AR (intent SALES_DP_REALIZE)
// The DP realization is only posted when the linked SO has dp_received_cents > 0.
func (service *Service) CreateInvoice(writer http.ResponseWriter, request *http.Request) {
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
	var req CreateInvoiceRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateInvoiceRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	userID := userID(request)

	var result invoiceResponse
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
			inv, err := service.findInvoiceByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = inv
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Verify customer exists.
		var customerName string
		if err := tx.QueryRow(request.Context(), `
			SELECT name FROM customers WHERE tenant_id = $1 AND id = $2
		`, tenant, req.CustomerID).Scan(&customerName); err != nil {
			return err
		}

		// Prepare lines and compute total (includes PPN).
		lines, totalCents, err := prepareInvoiceLines(req.Lines)
		if err != nil {
			return err
		}

		// Compute total DPP (revenue) and total PPN across all lines.
		var totalDPPCents, totalPPNCents int64
		for _, pl := range lines {
			totalDPPCents += pl.LineTotalCents
			totalPPNCents += pl.PPNCents
		}

		// Resolve accounts.
		arAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, arAccountCode)
		if err != nil {
			return err
		}
		revenueAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, revenueAccountCode)
		if err != nil {
			return err
		}

		// Load SO to check for DP.
		var dpReceived int64
		var soStatus string
		if req.SalesOrderID > 0 {
			if err := tx.QueryRow(request.Context(), `
				SELECT dp_received_cents, status FROM sales_orders WHERE tenant_id = $1 AND id = $2
			`, tenant, req.SalesOrderID).Scan(&dpReceived, &soStatus); err != nil {
				return err
			}
		}

		// 1. Revenue journal: Dr AR (DPP+PPN) / Cr Revenue (DPP) / Cr Output VAT (PPN).
		revenueLines := []accounting.Line{
			{AccountID: arAccountID, DebitCents: totalCents, SourceLineRef: "ar"},
			{AccountID: revenueAccountID, CreditCents: totalDPPCents, SourceLineRef: "revenue"},
		}
		// Add PPN line only if there is VAT to credit.
		if totalPPNCents > 0 {
			vatAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, outputVATAccountCode)
			if err != nil {
				return err
			}
			revenueLines = append(revenueLines, accounting.Line{
				AccountID:     vatAccountID,
				CreditCents:   totalPPNCents,
				SourceLineRef: "output-vat",
			})
		}
		if err := accounting.BalanceCheck(revenueLines); err != nil {
			return err
		}
		revenueSourceRef := fmt.Sprintf("INV-REV-%d", req.SalesOrderID)
		revenueJournal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   revenueSourceRef,
			IntentType:  accounting.IntentType("SALES_INVOICE"),
			EntryDate:   req.InvoiceDate,
			Description: "Sales invoice: " + customerName,
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
		if err := insertOutbox(request.Context(), tx, tenant, "invoice.posted", mustJSON(map[string]any{
			"journal_id": revenueEntryID, "number": jrnNumber,
		})); err != nil {
			return err
		}

		// 2. DP realization journal (if DP received on the SO).
		dpApplied := int64(0)
		var dpEntryID int64
		if dpReceived > 0 {
			dpApplied = dpReceived
			if dpApplied > totalCents {
				dpApplied = totalCents
			}
			depositAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, depositAccountCode)
			if err != nil {
				return err
			}
			dpLines := []accounting.Line{
				{AccountID: depositAccountID, DebitCents: dpApplied, SourceLineRef: "deposit"},
				{AccountID: arAccountID, CreditCents: dpApplied, SourceLineRef: "ar"},
			}
			if err := accounting.BalanceCheck(dpLines); err != nil {
				return err
			}
			dpSourceRef := fmt.Sprintf("INV-DP-%d", req.SalesOrderID)
			dpJournal := accounting.Journal{
				TenantID:    tenant,
				SourceRef:   dpSourceRef,
				IntentType:  accounting.IntentType("SALES_DP_REALIZE"),
				EntryDate:   req.InvoiceDate,
				Description: "DP realization: " + customerName,
				Lines:       dpLines,
			}
			head2, err := lockOrSeedHead(request.Context(), tx, tenant)
			if err != nil {
				return err
			}
			dpJournal.PreviousHash = head2.LastHash
			dpJournal.Hash = hashDP(dpJournal)
			dpJrnNumber, err := nextJournalNumber(request.Context(), tx, tenant)
			if err != nil {
				return err
			}
			err = tx.QueryRow(request.Context(), `
				INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, hash, prev_hash, created_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				RETURNING id
			`, tenant, dpJrnNumber, dpJournal.EntryDate, periodID, dpJournal.Description,
				dpJournal.SourceRef, string(dpJournal.IntentType),
				dpJournal.Hash, dpJournal.PreviousHash, int8Value(userID)).Scan(&dpEntryID)
			if err != nil {
				return err
			}
			for _, line := range dpJournal.Lines {
				if _, err := tx.Exec(request.Context(), `
					INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
					VALUES ($1, $2, $3, $4, $5, $6)
				`, tenant, dpEntryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
					return err
				}
			}
			if err := upsertHead(request.Context(), tx, tenant, dpEntryID, dpJournal.Hash); err != nil {
				return err
			}
			if err := insertOutbox(request.Context(), tx, tenant, "dp.realized", mustJSON(map[string]any{
				"journal_id": dpEntryID, "number": dpJrnNumber, "amount": dpApplied,
			})); err != nil {
				return err
			}
		}

		// Insert invoice header + lines.
		invNumber, err := nextINVNumber(request.Context(), tx, tenant)
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
		var soID pgtype.Int8
		if req.SalesOrderID > 0 {
			soID = pgtype.Int8{Int64: req.SalesOrderID, Valid: true}
		}
		receivable := totalCents - dpApplied
		var invID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO invoices
				(tenant_id, number, sales_order_id, customer_id, invoice_date, due_date,
				 payment_term_id, notes, status, total_cents, dp_applied_cents,
				 receivable_cents, revenue_journal_entry_id, dp_journal_entry_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'ISSUED', $9, $10, $11, $12, $13, $14)
			RETURNING id
		`, tenant, invNumber, soID, req.CustomerID, invoiceDate, dueDate,
			optionalInt8(req.PaymentTermID), textValueOptional(req.Notes), totalCents, dpApplied,
			receivable, int8Value(revenueEntryID), int8Value(dpEntryID), int8Value(userID)).Scan(&invID)
		if err != nil {
			return err
		}
		for position, p := range lines {
			var qty pgtype.Numeric
			_ = qty.Scan(p.Line.Qty)
			lineNo := position + 1
			var deliveryID pgtype.Int8
			if p.Line.DeliveryID > 0 {
				deliveryID = pgtype.Int8{Int64: p.Line.DeliveryID, Valid: true}
			}
			_, err := tx.Exec(request.Context(), `
				INSERT INTO invoice_lines
					(tenant_id, invoice_id, item_id, delivery_id, line_no, qty,
					 unit_price_cents, discount_cents, tax_rate, line_total_cents,
					 revenue_account_id, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`, tenant, invID, p.Line.ItemID, deliveryID, lineNo, qty,
				p.Line.UnitPriceCents, p.Line.DiscountCents,
				p.Line.TaxRate, p.LineTotalCents,
				revenueAccountID, textValueOptional(p.Line.Description))
			if err != nil {
				return err
			}
		}

		fetched, err := service.fetchInvoice(request.Context(), tx, tenant, invID)
		if err != nil {
			return err
		}
		result = *fetched
		return nil
	})
	if err != nil {
		status, code, message := invoiceErrorFor(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListInvoices returns the tenant's invoices.
func (service *Service) ListInvoices(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	statusFilter := strings.TrimSpace(request.URL.Query().Get("status"))
	var results []invoiceResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT i.id, i.number, i.sales_order_id, i.customer_id, c.name AS customer_name,
			       i.invoice_date, i.due_date, i.payment_term_id, i.notes, i.status,
			       i.total_cents, i.dp_applied_cents, i.receivable_cents
			FROM invoices i
			JOIN customers c ON c.tenant_id = i.tenant_id AND c.id = i.customer_id
			WHERE i.tenant_id = $1`
		args := []any{tenant}
		if statusFilter != "" {
			query += " AND i.status = $2"
			args = append(args, statusFilter)
		}
		query += " ORDER BY i.invoice_date DESC, i.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []invoiceResponse{}
		for rows.Next() {
			var item invoiceResponse
			var invoiceDate, dueDate pgtype.Date
			var notes pgtype.Text
			var soID, paymentTermID pgtype.Int8
			if err := rows.Scan(&item.ID, &item.Number, &soID, &item.CustomerID, &item.CustomerName,
				&invoiceDate, &dueDate, &paymentTermID, &notes, &item.Status,
				&item.TotalCents, &item.DPAppliedCents, &item.ReceivableCents); err != nil {
				return err
			}
			if soID.Valid {
				item.SalesOrderID = soID.Int64
			}
			item.InvoiceDate = dateString(invoiceDate)
			item.DueDate = dateString(dueDate)
			if paymentTermID.Valid {
				item.PaymentTermID = paymentTermID.Int64
			}
			item.Notes = textValue(notes)
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "INVOICE_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetInvoice returns one invoice with its lines.
func (service *Service) GetInvoice(writer http.ResponseWriter, request *http.Request) {
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
	var result *invoiceResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		result, err = service.fetchInvoice(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "INVOICE_NOT_FOUND", "invoice not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "INVOICE_FETCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// INV helpers
// ---------------------------------------------------------------------------

func validateInvoiceRequest(req CreateInvoiceRequest) (string, string) {
	if req.CustomerID <= 0 {
		return "INVALID_REQUEST", "customer_id is required"
	}
	if !validDate(req.InvoiceDate) {
		return "INVALID_REQUEST", "invoice_date must be a valid date in YYYY-MM-DD format"
	}
	if req.DueDate != "" && !validDate(req.DueDate) {
		return "INVALID_REQUEST", "due_date must be a valid date in YYYY-MM-DD format"
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

// preparedInvoiceLine carries a validated invoice line plus its computed DPP
// ( Dasar Pengenaan Pajak / taxable base), PPN (VAT), and grand total.
type preparedInvoiceLine struct {
	Line           InvoiceLineRequest
	LineTotalCents int64 // DPP (net before PPN)
	PPNCents       int64 // PPN = DPP * taxRate / 100 (integer, rounded)
}

func prepareInvoiceLines(lines []InvoiceLineRequest) ([]preparedInvoiceLine, int64, error) {
	prepared := make([]preparedInvoiceLine, 0, len(lines))
	var total int64 // grand total including PPN
	for _, line := range lines {
		if line.Qty <= 0 {
			return nil, 0, fmt.Errorf("lines: qty must be greater than 0")
		}
		if line.UnitPriceCents < 0 {
			return nil, 0, fmt.Errorf("lines: unit_price_cents must be >= 0")
		}
		if line.DiscountCents < 0 {
			return nil, 0, fmt.Errorf("lines: discount_cents must be >= 0")
		}
		lineTotal := lineTotalCents(line.Qty, line.UnitPriceCents, line.DiscountCents)
		// PPN: taxRate is a percentage (0–100). Compute using integer math:
		// ppnCents = lineTotal * taxRateMilli / 100000, where taxRateMilli = taxRate * 1000.
		// This avoids float64 entirely while supporting up to 3 decimal places of rate precision.
		taxRateMilli := int64(math.Round(line.TaxRate * 1000))
		if taxRateMilli < 0 {
			taxRateMilli = 0
		}
		ppnCents := lineTotal * taxRateMilli / 100000
		total += lineTotal + ppnCents
		prepared = append(prepared, preparedInvoiceLine{Line: line, LineTotalCents: lineTotal, PPNCents: ppnCents})
	}
	return prepared, total, nil
}

func invoiceErrorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant"
	}
	var overflow dpOverflowError
	if errors.As(err, &overflow) {
		return http.StatusConflict, "DP_EXCEEDS_ORDER", overflow.Error()
	}
	return http.StatusInternalServerError, "INVOICE_CREATE_FAILED", err.Error()
}

func (service *Service) fetchInvoice(ctx context.Context, tx pgx.Tx, tenant, id int64) (*invoiceResponse, error) {
	result := &invoiceResponse{}
	var invoiceDate, dueDate pgtype.Date
	var notes pgtype.Text
	var soID, paymentTermID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT i.id, i.number, i.sales_order_id, i.customer_id, c.name AS customer_name,
		       i.invoice_date, i.due_date, i.payment_term_id, i.notes, i.status,
		       i.total_cents, i.dp_applied_cents, i.receivable_cents
		FROM invoices i
		JOIN customers c ON c.tenant_id = i.tenant_id AND c.id = i.customer_id
		WHERE i.tenant_id = $1 AND i.id = $2
	`, tenant, id).Scan(&result.ID, &result.Number, &soID, &result.CustomerID, &result.CustomerName,
		&invoiceDate, &dueDate, &paymentTermID, &notes, &result.Status,
		&result.TotalCents, &result.DPAppliedCents, &result.ReceivableCents)
	if err != nil {
		return nil, err
	}
	if soID.Valid {
		result.SalesOrderID = soID.Int64
	}
	result.InvoiceDate = dateString(invoiceDate)
	result.DueDate = dateString(dueDate)
	if paymentTermID.Valid {
		result.PaymentTermID = paymentTermID.Int64
	}
	result.Notes = textValue(notes)

	rows, err := tx.Query(ctx, `
		SELECT l.id, l.item_id, i.code AS item_code, i.name AS item_name,
		       l.delivery_id, l.line_no, l.qty, l.unit_price_cents, l.discount_cents,
		       l.tax_rate, l.line_total_cents, l.description
		FROM invoice_lines l
		LEFT JOIN items i ON i.tenant_id = l.tenant_id AND i.id = l.item_id
		WHERE l.tenant_id = $1 AND l.invoice_id = $2
		ORDER BY l.line_no
	`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result.Lines = []invoiceLineResponse{}
	for rows.Next() {
		var line invoiceLineResponse
		var qty, taxRate pgtype.Numeric
		var desc, itemCode, itemName pgtype.Text
		var deliveryID pgtype.Int8
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &deliveryID,
			&line.LineNo, &qty, &line.UnitPriceCents, &line.DiscountCents,
			&taxRate, &line.LineTotalCents, &desc); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.TaxRate = numericToFloat(taxRate)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(desc)
		if deliveryID.Valid {
			line.DeliveryID = deliveryID.Int64
		}
		result.Lines = append(result.Lines, line)
	}
	return result, rows.Err()
}

func (service *Service) findInvoiceByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (invoiceResponse, error) {
	var result invoiceResponse
	var invoiceDate, dueDate pgtype.Date
	var notes pgtype.Text
	var soID, paymentTermID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT id, number, sales_order_id, customer_id, invoice_date, due_date,
		       payment_term_id, notes, status, total_cents, dp_applied_cents, receivable_cents
		FROM invoices
		WHERE tenant_id = $1 AND revenue_journal_entry_id = $2
	`, tenant, journalID).Scan(&result.ID, &result.Number, &soID, &result.CustomerID,
		&invoiceDate, &dueDate, &paymentTermID, &notes, &result.Status,
		&result.TotalCents, &result.DPAppliedCents, &result.ReceivableCents)
	if err != nil {
		return invoiceResponse{}, err
	}
	if soID.Valid {
		result.SalesOrderID = soID.Int64
	}
	result.InvoiceDate = dateString(invoiceDate)
	result.DueDate = dateString(dueDate)
	if paymentTermID.Valid {
		result.PaymentTermID = paymentTermID.Int64
	}
	result.Notes = textValue(notes)
	return result, nil
}

func resolveAccountByCode(ctx context.Context, tx pgx.Tx, tenantID int64, code string) (int64, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2
	`, tenantID, code).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("account %s not found: %w", code, err)
	}
	return accountID, nil
}

func nextINVNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'INV', 'INV', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}
