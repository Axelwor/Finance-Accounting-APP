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

	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// Purchase Order (PO) — commitment only, no journal posted.
// ---------------------------------------------------------------------------

const (
	poStatusConfirmed         = "CONFIRMED"
	poStatusPartiallyReceived = "PARTIALLY_RECEIVED"
	poStatusReceived          = "RECEIVED"
	poStatusCancelled         = "CANCELLED"
)

type PurchaseOrderLineRequest struct {
	ItemID         int64   `json:"item_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxRate        float64 `json:"tax_rate"`
	Description    string  `json:"description"`
}

type CreatePurchaseOrderRequest struct {
	SupplierID          int64                      `json:"supplier_id"`
	OrderDate           string                     `json:"order_date"`
	PaymentTermID       int64                      `json:"payment_term_id"`
	Notes               string                     `json:"notes"`
	Lines               []PurchaseOrderLineRequest `json:"lines"`
	SupplierQuoteNumber string                     `json:"supplier_quote_number"`
	SupplierQuoteDate   string                     `json:"supplier_quote_date"`
	BuyerID             int64                      `json:"buyer_id"`
}

type poLineResponse struct {
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
	ReceivedQty    float64 `json:"received_qty"`
	Description    string  `json:"description"`
}

type purchaseOrderResponse struct {
	ID                  int64            `json:"id"`
	Number              string           `json:"number"`
	SupplierID          int64            `json:"supplier_id"`
	SupplierName        string           `json:"supplier_name"`
	OrderDate           string           `json:"order_date"`
	PaymentTermID       int64            `json:"payment_term_id"`
	Notes               string           `json:"notes"`
	Status              string           `json:"status"`
	TotalCents          int64            `json:"total_cents"`
	ReceivedCents       int64            `json:"received_cents"`
	SupplierQuoteNumber string           `json:"supplier_quote_number,omitempty"`
	SupplierQuoteDate   string           `json:"supplier_quote_date,omitempty"`
	BuyerID             int64            `json:"buyer_id,omitempty"`
	Lines               []poLineResponse `json:"lines,omitempty"`
}

// CreatePurchaseOrder handles POST /purchase-orders.
func (service *Service) CreatePurchaseOrder(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreatePurchaseOrderRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validatePORequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	prepared, total, err := preparePOLines(req.Lines)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	orderDate, _ := parseDate(req.OrderDate)
	supplierQuoteDate, err := optionalDate(req.SupplierQuoteDate)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "supplier_quote_date must be a valid YYYY-MM-DD date")
		return
	}

	var result purchaseOrderResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		// Verify supplier belongs to tenant.
		var supplierName string
		var supplierActive bool
		err := tx.QueryRow(request.Context(), `
			SELECT name, is_active FROM suppliers WHERE tenant_id = $1 AND id = $2
		`, tenant, req.SupplierID).Scan(&supplierName, &supplierActive)
		if err != nil || !supplierActive {
			return errors.New("supplier not found or inactive")
		}
		// Allocate PO number.
		number, err := nextDocNumber(request.Context(), tx, tenant, "PO", "PO")
		if err != nil {
			return err
		}
		// Insert header.
		err = tx.QueryRow(request.Context(), `
			INSERT INTO purchase_orders (tenant_id, number, supplier_id, order_date, payment_term_id, notes, status, total_cents,
				supplier_quote_number, supplier_quote_date, buyer_id)
			VALUES ($1, $2, $3, $4, $5, $6, 'CONFIRMED', $7, $8, $9, $10)
			RETURNING id, number
		`, tenant, number, req.SupplierID, orderDate, optionalInt8(req.PaymentTermID),
			textValueOptional(req.Notes), total,
			textValueOptional(req.SupplierQuoteNumber), supplierQuoteDate, optionalInt8(req.BuyerID)).Scan(&result.ID, &result.Number)
		if err != nil {
			return err
		}
		// Insert lines.
		for i, p := range prepared {
			var lineID int64
			var itemCode, itemName pgtype.Text
			err := tx.QueryRow(request.Context(), `
				INSERT INTO purchase_orders_lines (tenant_id, order_id, item_id, line_no, qty, unit_price_cents, discount_cents, tax_rate, line_total_cents, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				RETURNING id, (SELECT code FROM items WHERE id = $3), (SELECT name FROM items WHERE id = $3)
			`, tenant, result.ID, p.Line.ItemID, i+1,
				pgtypeFloat(p.Line.Qty), p.Line.UnitPriceCents, p.Line.DiscountCents,
				pgtypeFloat(p.Line.TaxRate), p.LineTotalCents,
				textValueOptional(p.Line.Description)).Scan(&lineID, &itemCode, &itemName)
			if err != nil {
				return err
			}
			result.Lines = append(result.Lines, poLineResponse{
				ID: lineID, ItemID: p.Line.ItemID,
				ItemCode: textValue(itemCode), ItemName: textValue(itemName),
				LineNo: i + 1, Qty: p.Line.Qty,
				UnitPriceCents: p.Line.UnitPriceCents, DiscountCents: p.Line.DiscountCents,
				TaxRate: p.Line.TaxRate, LineTotalCents: p.LineTotalCents,
			})
		}
		result.SupplierID = req.SupplierID
		result.SupplierName = supplierName
		result.OrderDate = dateString(orderDate)
		result.PaymentTermID = req.PaymentTermID
		result.Notes = strings.TrimSpace(req.Notes)
		result.Status = poStatusConfirmed
		result.TotalCents = total
		result.SupplierQuoteNumber = strings.TrimSpace(req.SupplierQuoteNumber)
		result.SupplierQuoteDate = dateString(supplierQuoteDate)
		result.BuyerID = req.BuyerID
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "PO_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListPurchaseOrders handles GET /purchase-orders.
func (service *Service) ListPurchaseOrders(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	status := strings.TrimSpace(request.URL.Query().Get("status"))

	var results []purchaseOrderResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT po.id, po.number, po.supplier_id, s.name,
			       po.order_date, po.payment_term_id, po.notes, po.status,
			       po.total_cents, po.received_cents,
			       po.supplier_quote_number, po.supplier_quote_date, po.buyer_id
			FROM purchase_orders po
			JOIN suppliers s ON s.tenant_id = po.tenant_id AND s.id = po.supplier_id
		`
		args := []any{}
		if status != "" {
			query += " WHERE po.status = $1"
			args = append(args, status)
		}
		query += " ORDER BY po.order_date DESC, po.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []purchaseOrderResponse{}
		for rows.Next() {
			var po purchaseOrderResponse
			var orderDate pgtype.Date
			var paymentTerm pgtype.Int8
			var notes pgtype.Text
			var quoteNumber pgtype.Text
			var quoteDate pgtype.Date
			var buyerID pgtype.Int8
			if err := rows.Scan(&po.ID, &po.Number, &po.SupplierID, &po.SupplierName,
				&orderDate, &paymentTerm, &notes, &po.Status, &po.TotalCents, &po.ReceivedCents,
				&quoteNumber, &quoteDate, &buyerID); err != nil {
				return err
			}
			po.OrderDate = dateString(orderDate)
			if paymentTerm.Valid {
				po.PaymentTermID = paymentTerm.Int64
			}
			po.Notes = textValue(notes)
			po.SupplierQuoteNumber = textValue(quoteNumber)
			po.SupplierQuoteDate = dateString(quoteDate)
			if buyerID.Valid {
				po.BuyerID = buyerID.Int64
			}
			results = append(results, po)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PO_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetPurchaseOrder handles GET /purchase-orders/{id}.
func (service *Service) GetPurchaseOrder(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	poID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *purchaseOrderResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = fetchPO(request.Context(), tx, tenant, poID)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "PO_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func fetchPO(ctx context.Context, tx pgx.Tx, tenant, poID int64) (*purchaseOrderResponse, error) {
	var po purchaseOrderResponse
	var orderDate pgtype.Date
	var paymentTerm pgtype.Int8
	var notes pgtype.Text
	var quoteNumber pgtype.Text
	var quoteDate pgtype.Date
	var buyerID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT po.id, po.number, po.supplier_id, s.name,
		       po.order_date, po.payment_term_id, po.notes, po.status,
		       po.total_cents, po.received_cents,
		       po.supplier_quote_number, po.supplier_quote_date, po.buyer_id
		FROM purchase_orders po
		JOIN suppliers s ON s.tenant_id = po.tenant_id AND s.id = po.supplier_id
		WHERE po.tenant_id = $1 AND po.id = $2
	`, tenant, poID).Scan(&po.ID, &po.Number, &po.SupplierID, &po.SupplierName,
		&orderDate, &paymentTerm, &notes, &po.Status, &po.TotalCents, &po.ReceivedCents,
		&quoteNumber, &quoteDate, &buyerID)
	if err != nil {
		return nil, err
	}
	po.OrderDate = dateString(orderDate)
	if paymentTerm.Valid {
		po.PaymentTermID = paymentTerm.Int64
	}
	po.Notes = textValue(notes)
	po.SupplierQuoteNumber = textValue(quoteNumber)
	po.SupplierQuoteDate = dateString(quoteDate)
	if buyerID.Valid {
		po.BuyerID = buyerID.Int64
	}

	rows, err := tx.Query(ctx, `
		SELECT pol.id, pol.item_id, i.code, i.name, pol.line_no, pol.qty,
		       pol.unit_price_cents, pol.discount_cents, pol.tax_rate,
		       pol.line_total_cents, pol.received_qty, pol.description
		FROM purchase_orders_lines pol
		LEFT JOIN items i ON i.tenant_id = pol.tenant_id AND i.id = pol.item_id
		WHERE pol.tenant_id = $1 AND pol.order_id = $2
		ORDER BY pol.line_no
	`, tenant, poID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	po.Lines = []poLineResponse{}
	for rows.Next() {
		var line poLineResponse
		var itemCode, itemName pgtype.Text
		var receivedQty pgtype.Numeric
		var desc pgtype.Text
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&line.Qty, &line.UnitPriceCents, &line.DiscountCents, &line.TaxRate,
			&line.LineTotalCents, &receivedQty, &desc); err != nil {
			return nil, err
		}
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.ReceivedQty = numericToFloat(receivedQty)
		line.Description = textValue(desc)
		po.Lines = append(po.Lines, line)
	}
	return &po, rows.Err()
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

func validatePORequest(req CreatePurchaseOrderRequest) (string, string) {
	if req.SupplierID <= 0 {
		return "INVALID_REQUEST", "supplier_id is required"
	}
	if !validDate(req.OrderDate) {
		return "INVALID_REQUEST", "order_date must be a valid YYYY-MM-DD date"
	}
	if len(req.Lines) == 0 {
		return "INVALID_REQUEST", "at least one line is required"
	}
	for _, line := range req.Lines {
		if line.ItemID <= 0 {
			return "INVALID_REQUEST", "lines: item_id is required"
		}
		if line.Qty <= 0 {
			return "INVALID_REQUEST", "lines: qty must be > 0"
		}
		if line.UnitPriceCents < 0 {
			return "INVALID_REQUEST", "lines: unit_price_cents must be >= 0"
		}
		if line.DiscountCents < 0 {
			return "INVALID_REQUEST", "lines: discount_cents must be >= 0"
		}
		if line.TaxRate < 0 || line.TaxRate > 100 {
			return "INVALID_REQUEST", "lines: tax_rate must be in 0..100"
		}
	}
	return "", ""
}

type preparedPOLine struct {
	Line           PurchaseOrderLineRequest
	LineTotalCents int64
}

func preparePOLines(lines []PurchaseOrderLineRequest) ([]preparedPOLine, int64, error) {
	prepared := make([]preparedPOLine, 0, len(lines))
	var total int64
	for _, line := range lines {
		gross := line.Qty * float64(line.UnitPriceCents)
		lineTotal := int64(math.Round(gross)) - line.DiscountCents
		if lineTotal < 0 {
			return nil, 0, fmt.Errorf("lines: discount exceeds gross")
		}
		total += lineTotal
		prepared = append(prepared, preparedPOLine{Line: line, LineTotalCents: lineTotal})
	}
	return prepared, total, nil
}
