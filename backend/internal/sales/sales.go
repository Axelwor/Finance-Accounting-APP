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
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/approval"
	"finance-accounting-app/backend/internal/db"
)

// Service exposes the quotation (SQ) endpoints. Tenant id comes from the auth
// middleware context; every table access runs inside a transaction with the
// app.tenant_id setting scoped so FORCE RLS rows are visible.
type Service struct {
	pool *pgxpool.Pool
	gate *approval.Gate
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, gate: approval.NewGate(pool)}
}

// Gate exposes the F-03 approval gate so posting handlers can check and
// consume approvals inside their own transactions.
func (service *Service) Gate() *approval.Gate {
	return service.gate
}

// Routes registers the quotation and sales order endpoints on the chi router.
func (service *Service) Routes(router chi.Router) {
	router.Post("/quotations", service.Create)
	router.Get("/quotations", service.List)
	// NOTE: GET /reports/quotation-stats is registered manually in
	// cmd/api/main.go alongside other GET endpoints (sales.Routes is not
	// called for GET routes — they are registered manually).
	router.Get("/quotations/{id}", service.Get)
	router.Post("/quotations/{id}/send", service.Send)
	router.Post("/quotations/{id}/cancel", service.Cancel)
	router.Post("/quotations/{id}/mark-expired", service.MarkExpired)

	router.Post("/sales-orders", service.CreateOrder)
	router.Get("/sales-orders", service.ListOrders)
	router.Get("/sales-orders/{id}", service.GetOrder)
	router.Post("/sales-orders/{id}/cancel", service.CancelOrder)

	router.Post("/sales-orders/{id}/down-payments", service.CreateDP)
	router.Get("/sales-orders/{id}/down-payments", service.ListDPs)
	router.Post("/down-payments/{id}/refund", service.RefundDP)

	router.Post("/delivery-orders", service.CreateDelivery)
	router.Get("/delivery-orders", service.ListDeliveries)
	router.Get("/delivery-orders/{id}", service.GetDelivery)

	router.Post("/invoices", service.CreateInvoice)
	router.Get("/invoices", service.ListInvoices)
	router.Get("/invoices/{id}", service.GetInvoice)
	router.Post("/invoices/{id}/payments", service.CreatePayment)
	router.Get("/invoices/{id}/payments", service.ListPayments)

	router.Post("/credit-notes", service.CreateCreditNote)
	router.Get("/credit-notes", service.ListCreditNotes)
	router.Get("/credit-notes/{id}", service.GetCreditNote)
}

// Result types shared by list/detail/create responses.

type quotationLineResponse struct {
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

type quotationResponse struct {
	ID            int64                   `json:"id"`
	Number        string                  `json:"number"`
	CustomerID    int64                   `json:"customer_id"`
	CustomerName  string                  `json:"customer_name"`
	QuotationDate string                  `json:"quotation_date"`
	ValidUntil    string                  `json:"valid_until"`
	PaymentTermID int64                   `json:"payment_term_id"`
	Notes         string                  `json:"notes"`
	Status        string                  `json:"status"`
	TotalCents    int64                   `json:"total_cents"`
	Lines         []quotationLineResponse `json:"lines,omitempty"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// Create inserts a quotation and its lines in one transaction. SQ never posts a
// journal; it is a commitment only (see ACCOUNTING_ENGINE.md).
func (service *Service) Create(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateQuotationRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCreateRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	// Optional Idempotency-Key guards against double submit (M-023 pattern).
	idem, err := optionalIdempotencyKey(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// The number is allocated inside the same transaction, so a concurrent
	// create can still race on allocation. Retry the whole transaction a few
	// times: each attempt gets the next SQ-{year}-{seq} and a unique_violation
	// simply loops to allocate a fresh number.
	runCreate := func() (*quotationResponse, error) {
		var result *quotationResponse
		err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
			if err := withTenant(request.Context(), tx, tenant); err != nil {
				return err
			}
			quote, err := service.createInTx(request.Context(), tx, tenant, req, userID(request), idem)
			if err != nil {
				return err
			}
			result = quote
			return nil
		})
		return result, err
	}

	for attempt := 0; attempt < 5; attempt++ {
		result, err := runCreate()
		if err == nil {
			writeJSON(writer, http.StatusCreated, result)
			return
		}
		if isUniqueViolation(err) {
			continue // re-allocate the next number and retry
		}
		switch {
		case isForeignKeyViolation(err):
			writeError(writer, http.StatusConflict, "INVALID_REQUEST", "quotation references a resource that does not exist for this tenant")
		case isNoRows(err):
			writeError(writer, http.StatusConflict, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant")
		default:
			writeError(writer, http.StatusInternalServerError, "QUOTATION_CREATE_FAILED", err.Error())
		}
		return
	}
	writeError(writer, http.StatusConflict, "QUOTATION_NUMBER_EXISTS", "could not allocate a unique quotation number")
}

// createInTx performs the quotation + lines insert inside an open transaction
// that already has the tenant context set. When idem is non-empty it first
// replays an existing quotation stored under the same idempotency key.
func (service *Service) createInTx(ctx context.Context, tx pgx.Tx, tenant int64, req CreateQuotationRequest, actingUser int64, idem string) (*quotationResponse, error) {
	// Idempotent replay: same key → return the stored quotation.
	if idem != "" {
		var existingID int64
		err := tx.QueryRow(ctx, `
			SELECT id FROM sales_quotations
			WHERE tenant_id = $1 AND idempotency_key = $2
		`, tenant, idem).Scan(&existingID)
		if err == nil {
			return service.fetchQuotation(ctx, tx, tenant, existingID)
		} else if !isNoRows(err) {
			return nil, err
		}
	}

	// Customer must belong to the tenant (RLS guarantees visibility; the
	// explicit check keeps a clean 404 vs a generic FK error).
	var customerName string
	if err := tx.QueryRow(ctx, `
		SELECT name FROM customers WHERE tenant_id = $1 AND id = $2
	`, tenant, req.CustomerID).Scan(&customerName); err != nil {
		return nil, err
	}

	lines, totalCents, err := prepareLines(req.Lines)
	if err != nil {
		return nil, err
	}

	number, err := nextSQNumber(ctx, tx, tenant)
	if err != nil {
		return nil, err
	}

	quotationDate, err := parseDate(req.QuotationDate)
	if err != nil {
		return nil, err
	}
	validUntil, err := optionalDate(req.ValidUntil)
	if err != nil {
		return nil, err
	}

	var quotationID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO sales_quotations
			(tenant_id, number, customer_id, quotation_date, valid_until,
			 payment_term_id, notes, status, total_cents, created_by, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'DRAFT', $8, $9, $10)
		RETURNING id
	`, tenant, number, req.CustomerID, quotationDate, validUntil,
		optionalInt8(req.PaymentTermID), textValueOptional(req.Notes), totalCents,
		int8Nullable(actingUser), pgtypeUUIDOpt(idem)).Scan(&quotationID)
	if err != nil {
		return nil, err
	}

	for position, prepared := range lines {
		revenueAccountID, cogsAccountID, inventoryAccountID, err := loadItemDefaults(ctx, tx, tenant, prepared.Line.ItemID)
		if err != nil {
			return nil, err
		}
		var lineID int64
		var qty pgtype.Numeric
		_ = qty.Scan(fmt.Sprintf("%g", prepared.Line.Qty))
		lineNo := position + 1
		_ = tx.QueryRow(ctx, `
			INSERT INTO sales_quotations_lines
				(tenant_id, quotation_id, item_id, line_no, qty, unit_price_cents,
				 discount_cents, tax_rate, line_total_cents, revenue_account_id,
				 cogs_account_id, inventory_account_id, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id
		`, tenant, quotationID, prepared.Line.ItemID, lineNo, qty,
			prepared.Line.UnitPriceCents, prepared.Line.DiscountCents,
			prepared.Line.TaxRate, prepared.LineTotalCents,
			revenueAccountID, cogsAccountID, inventoryAccountID,
			textValueOptional(prepared.Line.Description)).Scan(&lineID)
		if err != nil {
			return nil, err
		}
	}

	// Rebuild the full detail (with item and customer names) from what was just
	// inserted so the response matches GET /quotations/{id}.
	return service.fetchQuotation(ctx, tx, tenant, quotationID)
}

// List returns the tenant's quotations ordered by date desc. It does not fetch
// lines.
func (service *Service) List(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	statusFilter := strings.TrimSpace(request.URL.Query().Get("status"))

	var results []quotationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT q.id, q.number, q.customer_id, c.name AS customer_name,
			       q.quotation_date, q.valid_until, q.payment_term_id,
			       q.notes, q.status, q.total_cents
			FROM sales_quotations q
			JOIN customers c ON c.tenant_id = q.tenant_id AND c.id = q.customer_id
			WHERE q.tenant_id = $1`
		args := []any{tenant}
		if statusFilter != "" {
			query += " AND q.status = $2"
			args = append(args, statusFilter)
		}
		query += " ORDER BY q.quotation_date DESC, q.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []quotationResponse{}
		for rows.Next() {
			var item quotationResponse
			var quotationDate, validUntil pgtype.Date
			var notes pgtype.Text
			if err := rows.Scan(&item.ID, &item.Number, &item.CustomerID, &item.CustomerName,
				&quotationDate, &validUntil, &item.PaymentTermID, &notes,
				&item.Status, &item.TotalCents); err != nil {
				return err
			}
			item.QuotationDate = dateString(quotationDate)
			item.ValidUntil = dateString(validUntil)
			item.Notes = textValue(notes)
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "QUOTATION_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// Get returns one quotation with its lines.
func (service *Service) Get(writer http.ResponseWriter, request *http.Request) {
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
	var result *quotationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		result, err = service.fetchQuotation(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "QUOTATION_NOT_FOUND", "quotation not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "QUOTATION_FETCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// Send transitions DRAFT -> SENT.
func (service *Service) Send(writer http.ResponseWriter, request *http.Request) {
	service.transition(writer, request, statusSent)
}

// Cancel transitions DRAFT or SENT -> CANCELLED.
func (service *Service) Cancel(writer http.ResponseWriter, request *http.Request) {
	service.transition(writer, request, statusCancelled)
}

// MarkExpired sets EXPIRED when the quotation is not already CANCELLED/CONVERTED.
func (service *Service) MarkExpired(writer http.ResponseWriter, request *http.Request) {
	service.transition(writer, request, statusExpired)
}

// transition applies one status change and returns the updated quotation.
func (service *Service) transition(writer http.ResponseWriter, request *http.Request, target string) {
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
	var result *quotationResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		current, err := currentStatus(request.Context(), tx, tenant, id)
		if err != nil {
			return err
		}
		allowed := false
		switch target {
		case statusSent:
			allowed = canSend(current)
		case statusCancelled:
			allowed = canCancel(current)
		case statusExpired:
			allowed = canExpire(current)
		}
		if !allowed {
			return errBadTransition{current: current}
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE sales_quotations SET status = $1, updated_at = now()
			WHERE tenant_id = $2 AND id = $3
		`, target, tenant, id); err != nil {
			return err
		}
		result, err = service.fetchQuotation(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		var bad errBadTransition
		switch {
		case isNoRows(err):
			writeError(writer, http.StatusNotFound, "QUOTATION_NOT_FOUND", "quotation not found")
		case errors.As(err, &bad):
			writeError(writer, http.StatusConflict, "QUOTATION_BAD_TRANSITION",
				fmt.Sprintf("cannot change status from %s to %s", bad.current, target))
		default:
			writeError(writer, http.StatusInternalServerError, "QUOTATION_UPDATE_FAILED", err.Error())
		}
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// DB helpers
// ---------------------------------------------------------------------------

// errBadTransition marks an illegal status transition.
type errBadTransition struct{ current string }

func (errBadTransition) Error() string { return "bad status transition" }

// currentStatus reads the quotation status.
func currentStatus(ctx context.Context, tx pgx.Tx, tenant, id int64) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `
		SELECT status FROM sales_quotations WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(&status)
	return status, err
}

// loadItemDefaults returns the item's account defaults used to stamp each line.
// The item must belong to the tenant (RLS enforces visibility).
func loadItemDefaults(ctx context.Context, tx pgx.Tx, tenant, itemID int64) (revenue, cogs, inventory int64, err error) {
	revenue, cogs, inventory = 0, 0, 0
	var rev, cog, inv pgtype.Int8
	err = tx.QueryRow(ctx, `
		SELECT sale_account_id, cogs_account_id, inventory_account_id
		FROM items WHERE tenant_id = $1 AND id = $2
	`, tenant, itemID).Scan(&rev, &cog, &inv)
	if rev.Valid {
		revenue = rev.Int64
	}
	if cog.Valid {
		cogs = cog.Int64
	}
	if inv.Valid {
		inventory = inv.Int64
	}
	return
}

// fetchQuotation loads a quotation header plus its lines with joined names.
func (service *Service) fetchQuotation(ctx context.Context, tx pgx.Tx, tenant, id int64) (*quotationResponse, error) {
	result := &quotationResponse{}
	var quotationDate, validUntil pgtype.Date
	var notes pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT q.id, q.number, q.customer_id, c.name AS customer_name,
		       q.quotation_date, q.valid_until, q.payment_term_id, q.notes,
		       q.status, q.total_cents
		FROM sales_quotations q
		JOIN customers c ON c.tenant_id = q.tenant_id AND c.id = q.customer_id
		WHERE q.tenant_id = $1 AND q.id = $2
	`, tenant, id).Scan(&result.ID, &result.Number, &result.CustomerID, &result.CustomerName,
		&quotationDate, &validUntil, &result.PaymentTermID, &notes,
		&result.Status, &result.TotalCents)
	if err != nil {
		return nil, err
	}
	result.QuotationDate = dateString(quotationDate)
	result.ValidUntil = dateString(validUntil)
	result.Notes = textValue(notes)

	rows, err := tx.Query(ctx, `
		SELECT l.id, l.item_id, i.code AS item_code, i.name AS item_name,
		       l.line_no, l.qty, l.unit_price_cents, l.discount_cents,
		       l.tax_rate, l.line_total_cents, l.description
		FROM sales_quotations_lines l
		LEFT JOIN items i ON i.tenant_id = l.tenant_id AND i.id = l.item_id
		WHERE l.tenant_id = $1 AND l.quotation_id = $2
		ORDER BY l.line_no
	`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result.Lines = []quotationLineResponse{}
	for rows.Next() {
		var line quotationLineResponse
		var qty pgtype.Numeric
		var taxRate pgtype.Numeric
		var description pgtype.Text
		var itemCode, itemName pgtype.Text
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&qty, &line.UnitPriceCents, &line.DiscountCents, &taxRate,
			&line.LineTotalCents, &description); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.TaxRate = numericToFloat(taxRate)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(description)
		result.Lines = append(result.Lines, line)
	}
	return result, rows.Err()
}

// nextSQNumber allocates the next SQ-{year}-{seq} number for the tenant inside
// the transaction (atomic, monotonic). It reuses the existing document_numbering
// counter (migration 000001) scoped to doc_type 'SQ', exactly like journal
// numbers use 'JRN'.
func nextSQNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'SQ', 'SQ', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}

func optionalDate(raw string) (pgtype.Date, error) {
	if strings.TrimSpace(raw) == "" {
		return pgtype.Date{}, nil
	}
	return parseDate(raw)
}

func optionalInt8(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func int8Nullable(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func textValueOptional(raw string) pgtype.Text {
	return pgtype.Text{String: raw, Valid: strings.TrimSpace(raw) != ""}
}

func numericToFloat(value pgtype.Numeric) float64 {
	f, err := value.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
