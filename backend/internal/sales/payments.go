package sales

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
	"finance-accounting-app/backend/internal/settings"
)

// overpaymentAccountCode is the seeded "Customer Overpayment" account (2402).
const overpaymentAccountCode = "2402"

// fxGainAccountCode / fxLossAccountCode are the seeded FX accounts (4904 /
// 5905); settings.ResolveAccount overrides them via tenant_settings (SET-001).
const (
	fxGainAccountCode = "4904"
	fxLossAccountCode = "5905"
)

// errPaymentExceedsReceivable marks a payment whose amount is larger than the
// invoice's outstanding receivable (QA-06: rejected with a clean 409 instead
// of being accepted as tracked overpayment).
var errPaymentExceedsReceivable = errors.New("payment exceeds outstanding receivable")

// applyPayment computes how much of the payment applies to AR. A payment may
// never exceed the outstanding receivable: partial payments leave the rest
// open, exact amounts settle it, and anything above returns
// errPaymentExceedsReceivable so the handler answers 409 PAYMENT_EXCEEDS_RECEIVABLE.
func applyPayment(amountCents, receivable int64) (int64, error) {
	if amountCents > receivable {
		return 0, fmt.Errorf("%w: amount %d exceeds receivable %d", errPaymentExceedsReceivable, amountCents, receivable)
	}
	return amountCents, nil
}

// CreatePaymentRequest is the POST /invoices/{id}/payments body.
type CreatePaymentRequest struct {
	CashAccountID int64  `json:"cash_account_id"`
	AmountCents   int64  `json:"amount_cents"`
	PaymentDate   string `json:"payment_date"`
	Description   string `json:"description"`
	// SET-001 multi-currency: settlement rate for FC invoices. The amount is
	// still in base-currency cents (converted by the client); the rate is
	// stored so the FX gain/loss can be computed against the invoice's rate.
	ExchangeRate float64 `json:"exchange_rate"`
}

type paymentResponse struct {
	ID               int64   `json:"id"`
	Number           string  `json:"number"`
	InvoiceID        int64   `json:"invoice_id"`
	CustomerID       int64   `json:"customer_id"`
	JournalEntryID   int64   `json:"journal_entry_id,omitempty"`
	AmountCents      int64   `json:"amount_cents"`
	ARAppliedCents   int64   `json:"ar_applied_cents"`
	OverpaymentCents int64   `json:"overpayment_cents"`
	CashAccountID    int64   `json:"cash_account_id"`
	PaymentDate      string  `json:"payment_date"`
	Description      string  `json:"description"`
	Status           string  `json:"status"`
	CurrencyCode     string  `json:"currency_code"`
	ExchangeRate     float64 `json:"exchange_rate"`
	FxGainCents      int64   `json:"fx_gain_cents"`
	FxLossCents      int64   `json:"fx_loss_cents"`
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
	requestHash := httperr.ComputeRequestHash(request)

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
			// M-023: verify payload match by comparing request hashes.
			var storedHash string
			_ = tx.QueryRow(request.Context(), `SELECT COALESCE(request_hash, '') FROM journal_entries WHERE id = $1`, existing.ID).Scan(&storedHash)
			if err := httperr.CheckIdempotencyHash(storedHash, requestHash); err != nil {
				return httperr.ErrIdempotencyKeyReuse
			}
			pmt, err := service.findPaymentByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = pmt
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load the invoice. FOR UPDATE serializes concurrent payments against
		// the same invoice's receivable_cents (A-18).
		var invNumber string
		var customerID int64
		var invStatus string
		var receivable int64
		var invCurrency string
		var invRate float64
		err = tx.QueryRow(request.Context(), `
			SELECT number, customer_id, status, receivable_cents,
			       btrim(currency_code)::text, exchange_rate::float8
			FROM invoices WHERE tenant_id = $1 AND id = $2
			FOR UPDATE
		`, tenant, invoiceID).Scan(&invNumber, &customerID, &invStatus, &receivable, &invCurrency, &invRate)
		if err != nil {
			return err
		}
		if invStatus == invVoid {
			return fmt.Errorf("invoice %s is VOID", invNumber)
		}

		// Compute AR applied. Overpayment (amount > receivable) is rejected as
		// a business validation so the client gets a clean 409 instead of an
		// accepted payment that would need a tracked refund balance.
		arApplied, err := applyPayment(req.AmountCents, receivable)
		if err != nil {
			return err
		}

		// SET-001 FX gain/loss: for a foreign-currency invoice settled at a
		// different rate than the posting rate, the AR slice is released at
		// the invoice rate while cash is booked at the payment rate; the
		// difference goes to the FX gain/loss accounts.
		paymentRate := req.ExchangeRate
		if paymentRate <= 0 {
			paymentRate = invRate
		}
		var fxDiff int64
		if invCurrency != "" && invCurrency != "IDR" && invRate > 0 && paymentRate != invRate {
			fxDiff = int64(math.Round(float64(arApplied) / invRate * (paymentRate - invRate)))
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

		// Build journal lines. amount_cents is the booked (invoice-rate)
		// base-currency value of the receivable being settled — it can never
		// exceed receivable_cents. The cash actually received for that slice
		// is its document-currency value converted at the payment rate:
		// arApplied/invRate*payRate == arApplied + fxDiff. AR is released at
		// its booked value and the FX difference clears through the mapped
		// gain/loss account, so debit always equals credit.
		//
		// Sign convention for a RECEIVABLE settled in a foreign currency:
		// payment rate ABOVE the invoice rate means the same document amount
		// converts to MORE base currency than the AR booked value → gain
		// (cash in > receivable released). Rate below → loss.
		cashDebit := arApplied + fxDiff
		arCredit := arApplied
		journalLines := []accounting.Line{
			{AccountID: cashAccount.ID, DebitCents: cashDebit, SourceLineRef: "cash"},
			{AccountID: arAccountID, CreditCents: arCredit, SourceLineRef: "ar"},
		}
		var fxDelta int64
		if fxDiff > 0 {
			// Payment rate > invoice rate: cash received exceeds the booked
			// AR value of the settled slice → FX gain (Cr gain).
			fxDelta = fxDiff
			fxGainAccountID, err := settings.ResolveAccount(request.Context(), tx, tenant, settings.SettingFxGain, fxGainAccountCode)
			if err != nil {
				return err
			}
			journalLines = append(journalLines, accounting.Line{
				AccountID: fxGainAccountID, CreditCents: fxDiff, SourceLineRef: "fx-gain",
			})
		} else if fxDiff < 0 {
			// Payment rate < invoice rate: cash received is below the booked
			// AR value of the settled slice → FX loss (Dr loss).
			fxDelta = -fxDiff
			fxLossAccountID, err := settings.ResolveAccount(request.Context(), tx, tenant, settings.SettingFxLoss, fxLossAccountCode)
			if err != nil {
				return err
			}
			journalLines = append(journalLines, accounting.Line{
				AccountID: fxLossAccountID, DebitCents: -fxDiff, SourceLineRef: "fx-loss",
			})
		}
		_ = fxDelta
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		// QA-06: the journal source_ref must be unique per payment. The old
		// static "PMT-{invoiceID}" collided with journal_entries_intent_unique
		// on every second payment for the same invoice; allocate the PMT number
		// up front and use it (unique per tenant/year via document_numbering).
		pmtNumber, err := nextPMTNumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		sourceRef := pmtNumber
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
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by, request_hash, currency_code, exchange_rate)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
				RETURNING id
			`, tenant, jrnNumber, journal.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem,
			journal.Hash, journal.PreviousHash, int8Value(userID), textValueOptional(requestHash),
			invCurrency, paymentRate).Scan(&entryID)
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

		// M-007: update the AR sub-ledger (customer_balances).
		if arApplied > 0 {
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO customer_balances (tenant_id, customer_id, ar_cents, updated_at)
				VALUES ($1, $2, $3, now())
				ON CONFLICT (tenant_id, customer_id)
				DO UPDATE SET ar_cents = GREATEST(customer_balances.ar_cents - EXCLUDED.ar_cents, 0), updated_at = now()
			`, tenant, customerID, arApplied); err != nil {
				return err
			}
		}

		// Insert payment record.
		pmtDate, err := parseDate(req.PaymentDate)
		if err != nil {
			return err
		}
		var pmtID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO invoice_payments
				(tenant_id, number, invoice_id, customer_id, journal_entry_id,
				 amount_cents, ar_applied_cents, overpayment_cents, cash_account_id,
				 payment_date, description, status, idempotency_key, source_ref, created_by,
				 currency_code, exchange_rate)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'RECEIVED', $12, $13, $14, $15, $16)
			RETURNING id
		`, tenant, pmtNumber, invoiceID, customerID, entryID,
			req.AmountCents, arApplied, 0, req.CashAccountID,
			pmtDate, textValueOptional(req.Description), idem, sourceRef,
			int8Value(userID), invCurrency, paymentRate).Scan(&pmtID)
		if err != nil {
			return err
		}
		result = paymentResponse{
			ID:             pmtID,
			Number:         pmtNumber,
			InvoiceID:      invoiceID,
			CustomerID:     customerID,
			JournalEntryID: entryID,
			AmountCents:    req.AmountCents,
			ARAppliedCents: arApplied,
			CashAccountID:  req.CashAccountID,
			PaymentDate:    req.PaymentDate,
			Description:    req.Description,
			Status:         "RECEIVED",
			CurrencyCode:   invCurrency,
			ExchangeRate:   paymentRate,
			FxGainCents:    max64(fxDiff, 0),
			FxLossCents:    max64(-fxDiff, 0),
		}

		if err := audit.Log(request.Context(), tx, tenant, userID, "invoice_payment", pmtID, audit.ActionPost, nil, map[string]any{
			"number":            pmtNumber,
			"amount_cents":      req.AmountCents,
			"ar_applied_cents":  arApplied,
			"overpayment_cents": 0,
			"journal_entry_id":  entryID,
		}); err != nil {
			return err
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
			       payment_date, description, status,
			       btrim(currency_code)::text, exchange_rate::float8
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
				&pmt.CashAccountID, &pmtDate, &desc, &pmt.Status,
				&pmt.CurrencyCode, &pmt.ExchangeRate); err != nil {
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
	if req.ExchangeRate < 0 {
		return "INVALID_REQUEST", "exchange_rate must not be negative"
	}
	return "", ""
}

// max64 returns the larger of two int64 values.
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func paymentErrorFor(err error) (int, string, string) {
	if errors.Is(err, httperr.ErrIdempotencyKeyReuse) {
		return http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", err.Error()
	}
	if errors.Is(err, errPaymentExceedsReceivable) {
		return http.StatusConflict, "PAYMENT_EXCEEDS_RECEIVABLE", err.Error()
	}
	if isNoRows(err) {
		return http.StatusNotFound, "INVOICE_NOT_FOUND", "invoice not found"
	}
	var overflow dpOverflowError
	if errors.As(err, &overflow) {
		return http.StatusConflict, "DP_EXCEEDS_ORDER", overflow.Error()
	}
	status, code := httperr.Classify(err)
	return status, code, err.Error()
}

func (service *Service) findPaymentByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (paymentResponse, error) {
	var result paymentResponse
	var pmtDate pgtype.Date
	var journalIDOut pgtype.Int8
	var desc pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, number, invoice_id, customer_id, journal_entry_id,
		       amount_cents, ar_applied_cents, overpayment_cents, cash_account_id,
		       payment_date, description, status,
		       btrim(currency_code)::text, exchange_rate::float8
		FROM invoice_payments
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&result.ID, &result.Number, &result.InvoiceID, &result.CustomerID,
		&journalIDOut, &result.AmountCents, &result.ARAppliedCents, &result.OverpaymentCents,
		&result.CashAccountID, &pmtDate, &desc, &result.Status,
		&result.CurrencyCode, &result.ExchangeRate)
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
