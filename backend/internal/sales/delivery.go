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

// DO statuses.
const (
	doShipped   = "SHIPPED"
	doReturned  = "RETURNED"
	doCancelled = "CANCELLED"
)

// DeliveryLineRequest is one line of a create-DO request.
type DeliveryLineRequest struct {
	ItemID        int64   `json:"item_id"`
	Qty           float64 `json:"qty"`
	UnitCostCents int64   `json:"unit_cost_cents"`
	Description   string  `json:"description"`
}

// CreateDeliveryRequest is the POST /delivery-orders body.
type CreateDeliveryRequest struct {
	SalesOrderID int64                 `json:"sales_order_id"`
	DeliveryDate string                `json:"delivery_date"`
	Notes        string                `json:"notes"`
	Lines        []DeliveryLineRequest `json:"lines"`
}

type deliveryLineResponse struct {
	ID                 int64   `json:"id"`
	ItemID             int64   `json:"item_id"`
	ItemCode           string  `json:"item_code"`
	ItemName           string  `json:"item_name"`
	LineNo             int     `json:"line_no"`
	Qty                float64 `json:"qty"`
	UnitCostCents      int64   `json:"unit_cost_cents"`
	COGSCents          int64   `json:"cogs_cents"`
	InventoryAccountID int64   `json:"inventory_account_id"`
	COGSAccountID      int64   `json:"cogs_account_id"`
	Description        string  `json:"description"`
}

type deliveryResponse struct {
	ID             int64                  `json:"id"`
	Number         string                 `json:"number"`
	SalesOrderID   int64                  `json:"sales_order_id"`
	CustomerID     int64                  `json:"customer_id"`
	CustomerName   string                 `json:"customer_name"`
	DeliveryDate   string                 `json:"delivery_date"`
	Notes          string                 `json:"notes"`
	Status         string                 `json:"status"`
	JournalEntryID int64                  `json:"journal_entry_id,omitempty"`
	TotalCOGSCents int64                  `json:"total_cogs_cents"`
	Lines          []deliveryLineResponse `json:"lines,omitempty"`
}

// CreateDelivery posts a delivery order. DO posts a COGS journal:
// Dr 5101 COGS / Cr 1301 Inventory per item delivered.
// Only goods items are allowed; negative stock is rejected.
func (service *Service) CreateDelivery(writer http.ResponseWriter, request *http.Request) {
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
	var req CreateDeliveryRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateDeliveryRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	userID := userID(request)

	var result deliveryResponse
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
			deliv, err := service.findDeliveryByJournalID(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = deliv
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load the SO and verify it's CONFIRMED.
		var soNumber, soStatus string
		var soCustomerID int64
		err = tx.QueryRow(request.Context(), `
			SELECT number, status, customer_id FROM sales_orders
			WHERE tenant_id = $1 AND id = $2
		`, tenant, req.SalesOrderID).Scan(&soNumber, &soStatus, &soCustomerID)
		if err != nil {
			return err
		}
		if soStatus != soConfirmed {
			return fmt.Errorf("order is %s, not CONFIRMED", soStatus)
		}

		// Validate each line: item must be goods, must have inventory+cogs accounts,
		// qty must not exceed undelivered qty on the SO line (negative stock rejected).
		type preparedDeliveryLine struct {
			line          DeliveryLineRequest
			itemID        int64
			unitCost      int64
			cogsCents     int64
			inventoryAcct int64
			cogsAcct      int64
			costingMethod string
		}
		prepared := make([]preparedDeliveryLine, 0, len(req.Lines))
		var totalCOGS int64
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
			if itemType != "goods" {
				return fmt.Errorf("item %s (%s) is a service — services cannot be delivered", itemCode, itemName)
			}
			if !invAcct.Valid || !cogsAcct.Valid {
				return fmt.Errorf("item %s (%s) is missing inventory or cogs account", itemCode, itemName)
			}
			method := textValue(costingMethod)
			// Resolve the actual COGS via the costing package. For FIFO and
			// moving-average items the caller-supplied unit_cost_cents is
			// ignored; for specific identification the caller's cost stands.
			resolvedCOGS, err := costing.ResolveCOGS(request.Context(), tx, tenant, line.ItemID, 0, line.Qty, method)
			if err != nil {
				return err
			}
			unitCost := line.UnitCostCents
			cogsCents := resolvedCOGS
			if method == costing.MethodSpecific {
				// Specific identification: caller-supplied cost is authoritative.
				unitCost = line.UnitCostCents
				cogsCents = roundQty(line.Qty) * line.UnitCostCents
			}
			totalCOGS += cogsCents
			prepared = append(prepared, preparedDeliveryLine{
				line:          line,
				itemID:        line.ItemID,
				unitCost:      unitCost,
				cogsCents:     cogsCents,
				inventoryAcct: invAcct.Int64,
				cogsAcct:      cogsAcct.Int64,
				costingMethod: method,
			})
		}

		// Build the COGS journal: one Dr COGS / Cr Inventory pair per line.
		journalLines := make([]accounting.Line, 0, len(prepared)*2)
		for i, p := range prepared {
			journalLines = append(journalLines,
				accounting.Line{AccountID: p.cogsAcct, DebitCents: p.cogsCents, SourceLineRef: fmt.Sprintf("cogs-%d", i)},
				accounting.Line{AccountID: p.inventoryAcct, CreditCents: p.cogsCents, SourceLineRef: fmt.Sprintf("inv-%d", i)},
			)
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("DO-%d", req.SalesOrderID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("SALES_DELIVERY"),
			EntryDate:   req.DeliveryDate,
			Description: "Delivery order COGS: SO " + soNumber,
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
		if err := insertOutbox(request.Context(), tx, tenant, "do.posted", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber, "source_ref": journal.SourceRef,
		})); err != nil {
			return err
		}

		// Allocate DO number and insert header + lines.
		doNumber, err := nextDONumber(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		deliveryDate, err := parseDate(req.DeliveryDate)
		if err != nil {
			return err
		}
		var doID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO delivery_orders
				(tenant_id, number, sales_order_id, customer_id, delivery_date,
				 notes, status, journal_entry_id, total_cogs_cents, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, 'SHIPPED', $7, $8, $9)
			RETURNING id
		`, tenant, doNumber, req.SalesOrderID, soCustomerID, deliveryDate,
			textValueOptional(req.Notes), entryID, totalCOGS,
			int8Value(userID)).Scan(&doID)
		if err != nil {
			return err
		}
		for position, p := range prepared {
			var qty pgtype.Numeric
			_ = qty.Scan(fmt.Sprintf("%g", p.line.Qty))
			lineNo := position + 1
			_, err := tx.Exec(request.Context(), `
				INSERT INTO delivery_orders_lines
					(tenant_id, delivery_id, item_id, line_no, qty, unit_cost_cents,
					 cogs_cents, inventory_account_id, cogs_account_id, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`, tenant, doID, p.itemID, lineNo, qty, p.unitCost, p.cogsCents,
				p.inventoryAcct, p.cogsAcct,
				textValueOptional(p.line.Description))
			if err != nil {
				return err
			}
			// Record inventory movement (qty negative = out).
			var negQty pgtype.Numeric
			_ = negQty.Scan(fmt.Sprintf("%g", -p.line.Qty))
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
				VALUES ($1, $2, 'DO', $3, $4, $5, $6)
			`, tenant, p.itemID, negQty,
				p.unitCost, doNumber, doID); err != nil {
				return err
			}
		}

		// Update SO status to reflect delivery.
		if _, err := tx.Exec(request.Context(), `
			UPDATE sales_orders SET status = 'CLOSED', updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, req.SalesOrderID); err != nil {
			return err
		}

		fetched, err := service.fetchDelivery(request.Context(), tx, tenant, doID)
		if err != nil {
			return err
		}
		result = *fetched
		return nil
	})
	if err != nil {
		status, code, message := deliveryErrorFor(err)
		writeError(writer, status, code, message)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListDeliveries returns the tenant's delivery orders.
func (service *Service) ListDeliveries(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	statusFilter := strings.TrimSpace(request.URL.Query().Get("status"))
	var results []deliveryResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT d.id, d.number, d.sales_order_id, d.customer_id, c.name AS customer_name,
			       d.delivery_date, d.notes, d.status, d.journal_entry_id, d.total_cogs_cents
			FROM delivery_orders d
			JOIN customers c ON c.tenant_id = d.tenant_id AND c.id = d.customer_id
			WHERE d.tenant_id = $1`
		args := []any{tenant}
		if statusFilter != "" {
			query += " AND d.status = $2"
			args = append(args, statusFilter)
		}
		query += " ORDER BY d.delivery_date DESC, d.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []deliveryResponse{}
		for rows.Next() {
			var item deliveryResponse
			var deliveryDate pgtype.Date
			var notes pgtype.Text
			var journalID pgtype.Int8
			if err := rows.Scan(&item.ID, &item.Number, &item.SalesOrderID, &item.CustomerID, &item.CustomerName,
				&deliveryDate, &notes, &item.Status, &journalID, &item.TotalCOGSCents); err != nil {
				return err
			}
			item.DeliveryDate = dateString(deliveryDate)
			item.Notes = textValue(notes)
			if journalID.Valid {
				item.JournalEntryID = journalID.Int64
			}
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DELIVERY_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetDelivery returns one delivery order with its lines.
func (service *Service) GetDelivery(writer http.ResponseWriter, request *http.Request) {
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
	var result *deliveryResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		result, err = service.fetchDelivery(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "DELIVERY_NOT_FOUND", "delivery order not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "DELIVERY_FETCH_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// DO helpers
// ---------------------------------------------------------------------------

func validateDeliveryRequest(req CreateDeliveryRequest) (string, string) {
	if req.SalesOrderID <= 0 {
		return "INVALID_REQUEST", "sales_order_id is required"
	}
	if !validDate(req.DeliveryDate) {
		return "INVALID_REQUEST", "delivery_date must be a valid date in YYYY-MM-DD format"
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
		if line.UnitCostCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: unit_cost_cents must be >= 0", index)
		}
	}
	return "", ""
}

func deliveryErrorFor(err error) (int, string, string) {
	if isNoRows(err) {
		return http.StatusNotFound, "ORDER_NOT_FOUND", "sales order not found"
	}
	var overflow dpOverflowError
	if errors.As(err, &overflow) {
		return http.StatusConflict, "DP_EXCEEDS_ORDER", overflow.Error()
	}
	return http.StatusInternalServerError, "DELIVERY_CREATE_FAILED", err.Error()
}

func (service *Service) fetchDelivery(ctx context.Context, tx pgx.Tx, tenant, id int64) (*deliveryResponse, error) {
	result := &deliveryResponse{}
	var deliveryDate pgtype.Date
	var notes pgtype.Text
	var journalID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT d.id, d.number, d.sales_order_id, d.customer_id, c.name AS customer_name,
		       d.delivery_date, d.notes, d.status, d.journal_entry_id, d.total_cogs_cents
		FROM delivery_orders d
		JOIN customers c ON c.tenant_id = d.tenant_id AND c.id = d.customer_id
		WHERE d.tenant_id = $1 AND d.id = $2
	`, tenant, id).Scan(&result.ID, &result.Number, &result.SalesOrderID, &result.CustomerID, &result.CustomerName,
		&deliveryDate, &notes, &result.Status, &journalID, &result.TotalCOGSCents)
	if err != nil {
		return nil, err
	}
	result.DeliveryDate = dateString(deliveryDate)
	result.Notes = textValue(notes)
	if journalID.Valid {
		result.JournalEntryID = journalID.Int64
	}

	rows, err := tx.Query(ctx, `
		SELECT l.id, l.item_id, i.code AS item_code, i.name AS item_name,
		       l.line_no, l.qty, l.unit_cost_cents, l.cogs_cents,
		       l.inventory_account_id, l.cogs_account_id, l.description
		FROM delivery_orders_lines l
		LEFT JOIN items i ON i.tenant_id = l.tenant_id AND i.id = l.item_id
		WHERE l.tenant_id = $1 AND l.delivery_id = $2
		ORDER BY l.line_no
	`, tenant, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result.Lines = []deliveryLineResponse{}
	for rows.Next() {
		var line deliveryLineResponse
		var qty pgtype.Numeric
		var desc, itemCode, itemName pgtype.Text
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&qty, &line.UnitCostCents, &line.COGSCents,
			&line.InventoryAccountID, &line.COGSAccountID, &desc); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(desc)
		result.Lines = append(result.Lines, line)
	}
	return result, rows.Err()
}

func (service *Service) findDeliveryByJournalID(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (deliveryResponse, error) {
	var result deliveryResponse
	var deliveryDate pgtype.Date
	var notes pgtype.Text
	var journalIDOut pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT id, number, sales_order_id, customer_id, delivery_date,
		       notes, status, journal_entry_id, total_cogs_cents
		FROM delivery_orders
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&result.ID, &result.Number, &result.SalesOrderID, &result.CustomerID,
		&deliveryDate, &notes, &result.Status, &journalIDOut, &result.TotalCOGSCents)
	if err != nil {
		return deliveryResponse{}, err
	}
	result.DeliveryDate = dateString(deliveryDate)
	result.Notes = textValue(notes)
	if journalIDOut.Valid {
		result.JournalEntryID = journalIDOut.Int64
	}
	return result, nil
}

func nextDONumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	year := time.Now().Year()
	var prefix string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, 'DO', 'DO', $2, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, year).Scan(&prefix, &seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, year, seq), nil
}

// roundQty converts a float64 qty to the nearest int64 (for cost calculations).
func roundQty(qty float64) int64 {
	return int64(qty + 0.5)
}
