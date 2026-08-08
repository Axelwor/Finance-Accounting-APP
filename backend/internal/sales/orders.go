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

	"finance-accounting-app/backend/internal/db"
)

// SO statuses. SO is a commitment only and never posts a journal.
const (
	soConfirmed = "CONFIRMED"
	soClosed    = "CLOSED"
	soCancelled = "CANCELLED"
)

// SalesOrderLineRequest is one line of a create-SO request.
type SalesOrderLineRequest struct {
	ItemID         int64   `json:"item_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxRate        float64 `json:"tax_rate"`
	Description    string  `json:"description"`
}

// CreateSalesOrderRequest is the POST /sales-orders body.
type CreateSalesOrderRequest struct {
	CustomerID    int64                   `json:"customer_id"`
	QuotationID   int64                   `json:"quotation_id"`
	OrderDate     string                  `json:"order_date"`
	PaymentTermID int64                   `json:"payment_term_id"`
	Notes         string                  `json:"notes"`
	Lines         []SalesOrderLineRequest `json:"lines"`
}

type orderLineResponse struct {
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

type orderResponse struct {
	ID              int64               `json:"id"`
	Number          string              `json:"number"`
	QuotationID     int64               `json:"quotation_id,omitempty"`
	CustomerID      int64               `json:"customer_id"`
	CustomerName    string              `json:"customer_name"`
	OrderDate       string              `json:"order_date"`
	PaymentTermID   int64               `json:"payment_term_id"`
	Notes           string              `json:"notes"`
	Status          string              `json:"status"`
	TotalCents      int64               `json:"total_cents"`
	DPReceivedCents int64               `json:"dp_received_cents"`
	Lines           []orderLineResponse `json:"lines,omitempty"`
	DownPayments    []dpResponse        `json:"down_payments,omitempty"`
}

// ---------------------------------------------------------------------------
// SO handlers
// ---------------------------------------------------------------------------

// CreateOrder inserts a sales order and its lines. SO posts no journal.
func (service *Service) CreateOrder(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateSalesOrderRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateOrderRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}

	runCreate := func() (*orderResponse, error) {
		var result *orderResponse
		err := db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
			if err := withTenant(request.Context(), tx, tenant); err != nil {
				return err
			}
			result, err = service.createOrderInTx(request.Context(), tx, tenant, req, userID(request))
			return err
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
			continue
		}
		switch {
		case isForeignKeyViolation(err):
			writeError(writer, http.StatusConflict, "INVALID_REQUEST", "order references a resource that does not exist for this tenant")
		case isNoRows(err):
			writeError(writer, http.StatusConflict, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant")
		default:
			writeError(writer, http.StatusInternalServerError, "ORDER_CREATE_FAILED", err.Error())
		}
		return
	}
	writeError(writer, http.StatusConflict, "ORDER_NUMBER_EXISTS", "could not allocate a unique order number")
}

func (service *Service) createOrderInTx(ctx context.Context, tx pgx.Tx, tenant int64, req CreateSalesOrderRequest, actingUser int64) (*orderResponse, error) {
	var customerName string
	if err := tx.QueryRow(ctx, `
		SELECT name FROM customers WHERE tenant_id = $1 AND id = $2
	`, tenant, req.CustomerID).Scan(&customerName); err != nil {
		return nil, err
	}

	lines, totalCents, err := prepareOrderLines(req.Lines)
	if err != nil {
		return nil, err
	}

	number, err := nextSONumber(ctx, tx, tenant)
	if err != nil {
		return nil, err
	}

	orderDate, err := parseDate(req.OrderDate)
	if err != nil {
		return nil, err
	}

	var quotationID pgtype.Int8
	if req.QuotationID > 0 {
		quotationID = pgtype.Int8{Int64: req.QuotationID, Valid: true}
	}

	var orderID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO sales_orders
			(tenant_id, number, quotation_id, customer_id, order_date,
			 payment_term_id, notes, status, total_cents, dp_received_cents, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'CONFIRMED', $8, 0, $9)
		RETURNING id
	`, tenant, number, quotationID, req.CustomerID, orderDate,
		optionalInt8(req.PaymentTermID), textValueOptional(req.Notes), totalCents,
		int8Nullable(actingUser)).Scan(&orderID)
	if err != nil {
		return nil, err
	}

	for position, prepared := range lines {
		revenueAccountID, cogsAccountID, inventoryAccountID, err := loadItemDefaults(ctx, tx, tenant, prepared.Line.ItemID)
		if err != nil {
			return nil, err
		}
		var qty pgtype.Numeric
		_ = qty.Scan(prepared.Line.Qty)
		lineNo := position + 1
		_, err = tx.Exec(ctx, `
			INSERT INTO sales_orders_lines
				(tenant_id, order_id, item_id, line_no, qty, unit_price_cents,
				 discount_cents, tax_rate, line_total_cents, revenue_account_id,
				 cogs_account_id, inventory_account_id, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, tenant, orderID, prepared.Line.ItemID, lineNo, qty,
			prepared.Line.UnitPriceCents, prepared.Line.DiscountCents,
			prepared.Line.TaxRate, prepared.LineTotalCents,
			revenueAccountID, cogsAccountID, inventoryAccountID,
			textValueOptional(prepared.Line.Description))
		if err != nil {
			return nil, err
		}
	}

	if req.QuotationID > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE sales_quotations SET status = 'CONVERTED', updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, req.QuotationID); err != nil {
			return nil, err
		}
	}

	return service.fetchOrder(ctx, tx, tenant, orderID)
}

// ListOrders returns the tenant's sales orders ordered by date desc.
func (service *Service) ListOrders(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	statusFilter := strings.TrimSpace(request.URL.Query().Get("status"))

	var results []orderResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT o.id, o.number, o.quotation_id, o.customer_id, c.name AS customer_name,
			       o.order_date, o.payment_term_id, o.notes, o.status,
			       o.total_cents, o.dp_received_cents
			FROM sales_orders o
			JOIN customers c ON c.tenant_id = o.tenant_id AND c.id = o.customer_id
			WHERE o.tenant_id = $1`
		args := []any{tenant}
		if statusFilter != "" {
			query += " AND o.status = $2"
			args = append(args, statusFilter)
		}
		query += " ORDER BY o.order_date DESC, o.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []orderResponse{}
		for rows.Next() {
			var item orderResponse
			var orderDate pgtype.Date
			var quotationID pgtype.Int8
			var notes pgtype.Text
			var paymentTermID pgtype.Int8
			if err := rows.Scan(&item.ID, &item.Number, &quotationID, &item.CustomerID, &item.CustomerName,
				&orderDate, &paymentTermID, &notes, &item.Status,
				&item.TotalCents, &item.DPReceivedCents); err != nil {
				return err
			}
			if quotationID.Valid {
				item.QuotationID = quotationID.Int64
			}
			item.OrderDate = dateString(orderDate)
			item.PaymentTermID = paymentTermID.Int64
			if paymentTermID.Valid {
				item.PaymentTermID = paymentTermID.Int64
			} else {
				item.PaymentTermID = 0
			}
			item.Notes = textValue(notes)
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ORDER_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetOrder returns one sales order with its lines and down payments.
func (service *Service) GetOrder(writer http.ResponseWriter, request *http.Request) {
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
	var result *orderResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		result, err = service.fetchOrder(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "ORDER_NOT_FOUND", "sales order not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "ORDER_FETCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// CancelOrder transitions CONFIRMED -> CANCELLED (only when no DP received).
func (service *Service) CancelOrder(writer http.ResponseWriter, request *http.Request) {
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
	var result *orderResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var status string
		var dpReceived int64
		if err := tx.QueryRow(request.Context(), `
			SELECT status, dp_received_cents FROM sales_orders WHERE tenant_id = $1 AND id = $2
		`, tenant, id).Scan(&status, &dpReceived); err != nil {
			return err
		}
		if status != soConfirmed {
			return errBadTransition{current: status}
		}
		if dpReceived > 0 {
			return errDPExists
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE sales_orders SET status = 'CANCELLED', updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, id); err != nil {
			return err
		}
		result, err = service.fetchOrder(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		var bad errBadTransition
		switch {
		case isNoRows(err):
			writeError(writer, http.StatusNotFound, "ORDER_NOT_FOUND", "sales order not found")
		case errors.As(err, &bad):
			writeError(writer, http.StatusConflict, "ORDER_BAD_TRANSITION",
				"cannot cancel order in status "+bad.current)
		case errors.Is(err, errDPExists):
			writeError(writer, http.StatusConflict, "DP_EXISTS",
				"refund all down payments before cancelling the order")
		default:
			writeError(writer, http.StatusInternalServerError, "ORDER_UPDATE_FAILED", err.Error())
		}
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// SO helpers
// ---------------------------------------------------------------------------

var errDPExists = &dpExistsError{}

type dpExistsError struct{}

func (*dpExistsError) Error() string { return "down payments exist" }

func validateOrderRequest(req CreateSalesOrderRequest) (string, string) {
	if req.CustomerID <= 0 {
		return "INVALID_REQUEST", "customer_id is required"
	}
	if !validDate(req.OrderDate) {
		return "INVALID_REQUEST", "order_date must be a valid date in YYYY-MM-DD format"
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

func prepareOrderLines(lines []SalesOrderLineRequest) ([]preparedLine, int64, error) {
	prepared := make([]preparedLine, 0, len(lines))
	var total int64
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
		total += lineTotal
		prepared = append(prepared, preparedLine{Line: QuotationLineRequest{
			ItemID:         line.ItemID,
			Qty:            line.Qty,
			UnitPriceCents: line.UnitPriceCents,
			DiscountCents:  line.DiscountCents,
			TaxRate:        line.TaxRate,
			Description:    line.Description,
		}, LineTotalCents: lineTotal})
	}
	return prepared, total, nil
}

func (service *Service) fetchOrder(ctx context.Context, tx pgx.Tx, tenant, id int64) (*orderResponse, error) {
	result := &orderResponse{}
	var orderDate pgtype.Date
	var quotationID pgtype.Int8
	var notes pgtype.Text
	var paymentTermID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT o.id, o.number, o.quotation_id, o.customer_id, c.name AS customer_name,
		       o.order_date, o.payment_term_id, o.notes, o.status,
		       o.total_cents, o.dp_received_cents
		FROM sales_orders o
		JOIN customers c ON c.tenant_id = o.tenant_id AND c.id = o.customer_id
		WHERE o.tenant_id = $1 AND o.id = $2
	`, tenant, id).Scan(&result.ID, &result.Number, &quotationID, &result.CustomerID, &result.CustomerName,
		&orderDate, &paymentTermID, &notes, &result.Status,
		&result.TotalCents, &result.DPReceivedCents)
	if err != nil {
		return nil, err
	}
	if quotationID.Valid {
		result.QuotationID = quotationID.Int64
	}
	result.OrderDate = dateString(orderDate)
	if paymentTermID.Valid {
		result.PaymentTermID = paymentTermID.Int64
	}
	result.Notes = textValue(notes)

	rows, err := tx.Query(ctx, `
		SELECT l.id, l.item_id, i.code AS item_code, i.name AS item_name,
		       l.line_no, l.qty, l.unit_price_cents, l.discount_cents,
		       l.tax_rate, l.line_total_cents, l.description
		FROM sales_orders_lines l
		LEFT JOIN items i ON i.tenant_id = l.tenant_id AND i.id = l.item_id
		WHERE l.tenant_id = $1 AND l.order_id = $2
		ORDER BY l.line_no
	`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result.Lines = []orderLineResponse{}
	for rows.Next() {
		var line orderLineResponse
		var qty, taxRate pgtype.Numeric
		var description, itemCode, itemName pgtype.Text
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	dpRows, err := tx.Query(ctx, `
		SELECT id, number, amount_cents, dp_date, status, journal_entry_id
		FROM sales_down_payments
		WHERE tenant_id = $1 AND order_id = $2
		ORDER BY dp_date, id
	`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer dpRows.Close()
	result.DownPayments = []dpResponse{}
	for dpRows.Next() {
		var dp dpResponse
		var dpDate pgtype.Date
		var journalID pgtype.Int8
		if err := dpRows.Scan(&dp.ID, &dp.Number, &dp.AmountCents, &dpDate, &dp.Status, &journalID); err != nil {
			return nil, err
		}
		dp.DPDate = dateString(dpDate)
		if journalID.Valid {
			dp.JournalEntryID = journalID.Int64
		}
		result.DownPayments = append(result.DownPayments, dp)
	}
	return result, dpRows.Err()
}

func nextSONumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'SO', 'SO', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}
