package purchase

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/db"
)

// Supplier invoice statuses (ISSUED / PARTIALLY_PAID / PAID / VOID) are defined
// as local string literals here to avoid colliding with any constants the
// supplier-invoice handler (US-033) may export from its own file.
const (
	supplierPaymentDocType   = "PAY"
	supplierPaymentDocPrefix = "PAY"
)

// CreateSupplierPaymentRequest is the POST /supplier-invoices/{id}/payments body.
type CreateSupplierPaymentRequest struct {
	CashAccountID int64  `json:"cash_account_id"`
	AmountCents   int64  `json:"amount_cents"`
	PaymentDate   string `json:"payment_date"`
	Description   string `json:"description"`
}

type supplierPaymentResponse struct {
	ID               int64  `json:"id"`
	Number           string `json:"number"`
	InvoiceID        int64  `json:"invoice_id"`
	SupplierID       int64  `json:"supplier_id"`
	JournalEntryID   int64  `json:"journal_entry_id,omitempty"`
	AmountCents      int64  `json:"amount_cents"`
	APAppliedCents   int64  `json:"ap_applied_cents"`
	OverpaymentCents int64  `json:"overpayment_cents"`
	CashAccountID    int64  `json:"cash_account_id"`
	PaymentDate      string `json:"payment_date"`
	Description      string `json:"description"`
	Status           string `json:"status"`
}

// CreateSupplierPayment posts a supplier payment against an invoice.
// Journal: Dr 2101 AP (ap_applied) / [Dr 1204 overpayment] / Cr Cash-Bank (amount).
// Intent: SUPPLIER_PAYMENT. Source ref: PAY-{invoiceID}.
func (service *Service) CreateSupplierPayment(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	invoiceID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idem, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateSupplierPaymentRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateSupplierPaymentRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	uid := userID(request)

	var result supplierPaymentResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		// Idempotent replay: if we already posted a journal for this key, return
		// the matching supplier payment.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			pmt, err := service.findSupplierPaymentByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = pmt
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load the supplier invoice.
		var invNumber string
		var supplierID int64
		var invStatus string
		var payable int64
		err = tx.QueryRow(request.Context(), `
			SELECT number, supplier_id, status, payable_cents
			FROM supplier_invoices WHERE tenant_id = $1 AND id = $2
		`, tenant, invoiceID).Scan(&invNumber, &supplierID, &invStatus, &payable)
		if err != nil {
			return err
		}
		if invStatus == "VOID" {
			return fmt.Errorf("invoice %s is VOID", invNumber)
		}

		// Compute AP applied and overpayment.
		//   ap_applied = min(amount, payable)
		//   overpayment = amount - ap_applied  (only when amount > payable)
		apApplied := req.AmountCents
		overpayment := int64(0)
		if apApplied > payable {
			overpayment = apApplied - payable
			apApplied = payable
		}

		// Resolve accounts.
		apAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, apAccountCode)
		if err != nil {
			return err
		}
		cashAccount, err := loadPurchaseAccount(request.Context(), tx, tenant, req.CashAccountID)
		if err != nil {
			return err
		}

		// Build journal lines.
		//   Dr 2101 AP (ap_applied) — reduce what we owe.
		//   Dr 1204 Other Receivables (overpayment) — if overpaid.
		//   Cr Cash/Bank (amount) — total cash out.
		journalLines := []accounting.Line{
			{AccountID: apAccountID, DebitCents: apApplied, SourceLineRef: "ap"},
			{AccountID: cashAccount.ID, CreditCents: req.AmountCents, SourceLineRef: "cash"},
		}
		if overpayment > 0 {
			opAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, overpaymentAccountCode)
			if err != nil {
				return err
			}
			journalLines = append(journalLines, accounting.Line{
				AccountID: opAccountID, DebitCents: overpayment, SourceLineRef: "overpayment",
			})
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("PAY-%d", invoiceID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("SUPPLIER_PAYMENT"),
			EntryDate:   req.PaymentDate,
			Description: "Supplier payment: invoice " + invNumber,
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
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, tenant, jrnNumber, journal.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem,
			journal.Hash, journal.PreviousHash, int8Value(uid)).Scan(&entryID)
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
		if err := insertOutbox(request.Context(), tx, tenant, "supplier.payment", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber, "invoice_id": invoiceID,
		})); err != nil {
			return err
		}

		// Update supplier invoice: reduce payable, update status.
		newPayable := payable - apApplied
		newStatus := "PARTIALLY_PAID"
		if newPayable <= 0 {
			newStatus = "PAID"
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE supplier_invoices SET payable_cents = $1, status = $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, newPayable, newStatus, tenant, invoiceID); err != nil {
			return err
		}

		// Insert payment record.
		pmtNumber, err := nextPayNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		pmtDate, err := parseDate(req.PaymentDate)
		if err != nil {
			return err
		}
		var pmtID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO supplier_payments
				(tenant_id, number, supplier_id, invoice_id, journal_entry_id,
				 amount_cents, ap_applied_cents, overpayment_cents, cash_account_id,
				 payment_date, description, status, idempotency_key, source_ref, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'PAID', $12, $13, $14)
			RETURNING id
		`, tenant, pmtNumber, supplierID, invoiceID, entryID,
			req.AmountCents, apApplied, overpayment, req.CashAccountID,
			pmtDate, textValueOptional(req.Description), idem, sourceRef,
			int8Value(uid)).Scan(&pmtID)
		if err != nil {
			return err
		}
		result = supplierPaymentResponse{
			ID:               pmtID,
			Number:           pmtNumber,
			InvoiceID:        invoiceID,
			SupplierID:       supplierID,
			JournalEntryID:   entryID,
			AmountCents:      req.AmountCents,
			APAppliedCents:   apApplied,
			OverpaymentCents: overpayment,
			CashAccountID:    req.CashAccountID,
			PaymentDate:      req.PaymentDate,
			Description:      req.Description,
			Status:           "PAID",
		}

		if err := audit.Log(request.Context(), tx, tenant, uid, "supplier_payment", pmtID, audit.ActionPost, nil, map[string]any{
			"number":            pmtNumber,
			"amount_cents":      req.AmountCents,
			"ap_applied_cents":  apApplied,
			"overpayment_cents": overpayment,
			"invoice_id":        invoiceID,
			"journal_entry_id":  entryID,
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		status, code, message := supplierPaymentErrorFor(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListSupplierPayments returns payments for a supplier invoice.
func (service *Service) ListSupplierPayments(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	invoiceID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var results []supplierPaymentResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		rows, err := tx.Query(request.Context(), `
			SELECT id, number, invoice_id, supplier_id, journal_entry_id,
			       amount_cents, ap_applied_cents, overpayment_cents, cash_account_id,
			       payment_date, description, status
			FROM supplier_payments
			WHERE tenant_id = $1 AND invoice_id = $2
			ORDER BY payment_date, id
		`, tenant, invoiceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []supplierPaymentResponse{}
		for rows.Next() {
			var pmt supplierPaymentResponse
			var pmtDate pgtype.Date
			var journalID pgtype.Int8
			var desc pgtype.Text
			if err := rows.Scan(&pmt.ID, &pmt.Number, &pmt.InvoiceID, &pmt.SupplierID,
				&journalID, &pmt.AmountCents, &pmt.APAppliedCents, &pmt.OverpaymentCents,
				&pmt.CashAccountID, &pmtDate, &desc, &pmt.Status); err != nil {
				return err
			}
			pmt.PaymentDate = dateString(pmtDate)
			if journalID.Valid {
				pmt.JournalEntryID = journalID.Int64
			}
			pmt.Description = textValue(desc)
			results = append(results, pmt)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PAYMENT_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// ---------------------------------------------------------------------------
// Supplier payment helpers
// ---------------------------------------------------------------------------

func validateSupplierPaymentRequest(req CreateSupplierPaymentRequest) (string, string) {
	if req.CashAccountID <= 0 {
		return "INVALID_REQUEST", "cash_account_id is required"
	}
	if req.AmountCents <= 0 {
		return "INVALID_REQUEST", "amount_cents must be positive"
	}
	if !validDate(req.PaymentDate) {
		return "INVALID_REQUEST", "payment_date must be a valid date in YYYY-MM-DD format"
	}
	return "", ""
}

func supplierPaymentErrorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "SUPPLIER_INVOICE_NOT_FOUND", "supplier invoice not found"
	}
	if isForeignKeyViolation(err) {
		return http.StatusBadRequest, "INVALID_REQUEST", "cash account or invoice not found"
	}
	if isUniqueViolation(err) {
		return http.StatusConflict, "DUPLICATE_PAYMENT", "payment already exists"
	}
	return http.StatusInternalServerError, "SUPPLIER_PAYMENT_FAILED", err.Error()
}

// loadPurchaseAccount loads a cash/bank account row by id, scoped to the tenant.
// Mirrors sales.loadSalesAccount.
func loadPurchaseAccount(ctx context.Context, tx pgx.Tx, tenantID, accountID int64) (db.Account, error) {
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

func (service *Service) findSupplierPaymentByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (supplierPaymentResponse, error) {
	var result supplierPaymentResponse
	var pmtDate pgtype.Date
	var journalIDOut pgtype.Int8
	var desc pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, number, invoice_id, supplier_id, journal_entry_id,
		       amount_cents, ap_applied_cents, overpayment_cents, cash_account_id,
		       payment_date, description, status
		FROM supplier_payments
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&result.ID, &result.Number, &result.InvoiceID, &result.SupplierID,
		&journalIDOut, &result.AmountCents, &result.APAppliedCents, &result.OverpaymentCents,
		&result.CashAccountID, &pmtDate, &desc, &result.Status)
	if err != nil {
		return supplierPaymentResponse{}, err
	}
	result.PaymentDate = dateString(pmtDate)
	if journalIDOut.Valid {
		result.JournalEntryID = journalIDOut.Int64
	}
	result.Description = textValue(desc)
	return result, nil
}

func nextPayNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	return nextDocNumber(ctx, tx, tenantID, supplierPaymentDocType, supplierPaymentDocPrefix)
}
