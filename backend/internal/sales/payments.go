package sales

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// overpaymentAccountCode is the seeded "Customer Overpayment" account (2402).
const overpaymentAccountCode = "2402"

// CreatePaymentRequest is the POST /invoices/{id}/payments body.
type CreatePaymentRequest struct {
	CashAccountID int64  `json:"cash_account_id"`
	AmountCents   int64  `json:"amount_cents"`
	PaymentDate   string `json:"payment_date"`
	Description   string `json:"description"`
}

type paymentResponse struct {
	ID               int64  `json:"id"`
	Number           string `json:"number"`
	InvoiceID        int64  `json:"invoice_id"`
	CustomerID       int64  `json:"customer_id"`
	JournalEntryID   int64  `json:"journal_entry_id,omitempty"`
	AmountCents      int64  `json:"amount_cents"`
	ARAppliedCents   int64  `json:"ar_applied_cents"`
	OverpaymentCents int64  `json:"overpayment_cents"`
	CashAccountID    int64  `json:"cash_account_id"`
	PaymentDate      string `json:"payment_date"`
	Description      string `json:"description"`
	Status           string `json:"status"`
}

// CreatePayment posts a customer payment against an invoice.
// Journal: Dr Cash/Bank / Cr 1201 AR (intent SALES_RECEIPT).
// Overpayment (amount > receivable): Dr Cash / Cr AR (receivable) + Cr 2402 (excess).
func (service *Service) CreatePayment(writer http.ResponseWriter, request *http.Request) {
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
	var req CreatePaymentRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validatePaymentRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	userID := userID(request)

	var result paymentResponse
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
			pmt, err := service.findPaymentByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = pmt
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
		`, tenant, invoiceID).Scan(&invNumber, &customerID, &invStatus, &receivable)
		if err != nil {
			return err
		}
		if invStatus == invVoid {
			return fmt.Errorf("invoice %s is VOID", invNumber)
		}

		// Compute AR applied and overpayment.
		arApplied := req.AmountCents
		overpayment := int64(0)
		if arApplied > receivable {
			overpayment = arApplied - receivable
			arApplied = receivable
		}

		// Resolve accounts.
		arAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, arAccountCode)
		if err != nil {
			return err
		}
		cashAccount, err := loadSalesAccount(request.Context(), tx, tenant, req.CashAccountID)
		if err != nil {
			return err
		}

		// Build journal lines.
		journalLines := []accounting.Line{
			{AccountID: cashAccount.ID, DebitCents: req.AmountCents, SourceLineRef: "cash"},
			{AccountID: arAccountID, CreditCents: arApplied, SourceLineRef: "ar"},
		}
		if overpayment > 0 {
			opAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, overpaymentAccountCode)
			if err != nil {
				return err
			}
			journalLines = append(journalLines, accounting.Line{
				AccountID: opAccountID, CreditCents: overpayment, SourceLineRef: "overpayment",
			})
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("PMT-%d", invoiceID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("SALES_RECEIPT"),
			EntryDate:   req.PaymentDate,
			Description: "Payment received: invoice " + invNumber,
			Lines:       journalLines,
		}
		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashDP(journal)

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
		if err := insertOutbox(request.Context(), tx, tenant, "payment.received", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber, "invoice_id": invoiceID,
		})); err != nil {
			return err
		}

		// Update invoice: reduce receivable, update status.
		newReceivable := receivable - arApplied
		newStatus := invPartiallyPaid
		if newReceivable <= 0 {
			newStatus = invPaid
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE invoices SET receivable_cents = $1, status = $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, newReceivable, newStatus, tenant, invoiceID); err != nil {
			return err
		}

		// Insert payment record.
		pmtNumber, err := nextPMTNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		pmtDate, err := parseDate(req.PaymentDate)
		if err != nil {
			return err
		}
		var pmtID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO invoice_payments
				(tenant_id, number, invoice_id, customer_id, journal_entry_id,
				 amount_cents, ar_applied_cents, overpayment_cents, cash_account_id,
				 payment_date, description, status, idempotency_key, source_ref, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'RECEIVED', $12, $13, $14)
			RETURNING id
		`, tenant, pmtNumber, invoiceID, customerID, entryID,
			req.AmountCents, arApplied, overpayment, req.CashAccountID,
			pmtDate, textValueOptional(req.Description), idem, sourceRef,
			int8Value(userID)).Scan(&pmtID)
		if err != nil {
			return err
		}
		result = paymentResponse{
			ID:               pmtID,
			Number:           pmtNumber,
			InvoiceID:        invoiceID,
			CustomerID:       customerID,
			JournalEntryID:   entryID,
			AmountCents:      req.AmountCents,
			ARAppliedCents:   arApplied,
			OverpaymentCents: overpayment,
			CashAccountID:    req.CashAccountID,
			PaymentDate:      req.PaymentDate,
			Description:      req.Description,
			Status:           "RECEIVED",
		}
		return nil
	})
	if err != nil {
		status, code, message := paymentErrorFor(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListPayments returns payments for an invoice.
func (service *Service) ListPayments(writer http.ResponseWriter, request *http.Request) {
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
	var results []paymentResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		rows, err := tx.Query(request.Context(), `
			SELECT id, number, invoice_id, customer_id, journal_entry_id,
			       amount_cents, ar_applied_cents, overpayment_cents, cash_account_id,
			       payment_date, description, status
			FROM invoice_payments
			WHERE tenant_id = $1 AND invoice_id = $2
			ORDER BY payment_date, id
		`, tenant, invoiceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []paymentResponse{}
		for rows.Next() {
			var pmt paymentResponse
			var pmtDate pgtype.Date
			var journalID pgtype.Int8
			var desc pgtype.Text
			if err := rows.Scan(&pmt.ID, &pmt.Number, &pmt.InvoiceID, &pmt.CustomerID,
				&journalID, &pmt.AmountCents, &pmt.ARAppliedCents, &pmt.OverpaymentCents,
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
// Payment helpers
// ---------------------------------------------------------------------------

func validatePaymentRequest(req CreatePaymentRequest) (string, string) {
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

func paymentErrorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "INVOICE_NOT_FOUND", "invoice not found"
	}
	var overflow dpOverflowError
	if errors.As(err, &overflow) {
		return http.StatusConflict, "DP_EXCEEDS_ORDER", overflow.Error()
	}
	return http.StatusInternalServerError, "PAYMENT_CREATE_FAILED", err.Error()
}

func (service *Service) findPaymentByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (paymentResponse, error) {
	var result paymentResponse
	var pmtDate pgtype.Date
	var journalIDOut pgtype.Int8
	var desc pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, number, invoice_id, customer_id, journal_entry_id,
		       amount_cents, ar_applied_cents, overpayment_cents, cash_account_id,
		       payment_date, description, status
		FROM invoice_payments
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&result.ID, &result.Number, &result.InvoiceID, &result.CustomerID,
		&journalIDOut, &result.AmountCents, &result.ARAppliedCents, &result.OverpaymentCents,
		&result.CashAccountID, &pmtDate, &desc, &result.Status)
	if err != nil {
		return paymentResponse{}, err
	}
	result.PaymentDate = dateString(pmtDate)
	if journalIDOut.Valid {
		result.JournalEntryID = journalIDOut.Int64
	}
	result.Description = textValue(desc)
	return result, nil
}

func nextPMTNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'PMT', 'PMT', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}
