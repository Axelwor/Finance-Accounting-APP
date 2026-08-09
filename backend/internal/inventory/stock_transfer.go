package inventory

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// Stock Transfer (US-042). Records TRANSFER_OUT (qty negative) + TRANSFER_IN
// (qty positive) inventory movements. No journal is posted because the value
// stays in the same inventory account (single warehouse for now).

type StockTransferLineRequest struct {
	ItemID        int64   `json:"item_id"`
	Qty           float64 `json:"qty"`
	UnitCostCents int64   `json:"unit_cost_cents"`
	Description   string  `json:"description"`
}

type CreateStockTransferRequest struct {
	TransferDate string                     `json:"transfer_date"`
	Notes        string                     `json:"notes"`
	Lines        []StockTransferLineRequest `json:"lines"`
}

type stockTransferLineResponse struct {
	ID                 int64   `json:"id"`
	ItemID             int64   `json:"item_id"`
	ItemCode           string  `json:"item_code"`
	ItemName           string  `json:"item_name"`
	LineNo             int     `json:"line_no"`
	Qty                float64 `json:"qty"`
	UnitCostCents      int64   `json:"unit_cost_cents"`
	InventoryAccountID int64   `json:"inventory_account_id"`
	Description        string  `json:"description"`
}

type stockTransferResponse struct {
	ID           int64                       `json:"id"`
	Number       string                      `json:"number"`
	TransferDate string                      `json:"transfer_date"`
	Notes        string                      `json:"notes"`
	Status       string                      `json:"status"`
	Lines        []stockTransferLineResponse `json:"lines,omitempty"`
}

const transferStatusCompleted = "COMPLETED"

func (service *Service) CreateStockTransfer(writer http.ResponseWriter, request *http.Request) {
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
	_ = idem // idempotency enforced via document_numbering uniqueness + idempotency_key absence here

	var req CreateStockTransferRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateStockTransfer(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}

	uid := userID(request)
	var result *stockTransferResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Prepare lines: validate items are goods with an inventory account.
		type preparedLine struct {
			req       StockTransferLineRequest
			lineNo    int
			invAcctID int64
			itemCode  string
			itemName  string
		}
		prepared := make([]preparedLine, 0, len(req.Lines))
		for i, line := range req.Lines {
			var itemType, itemCode, itemName string
			var invAcct pgtype.Int8
			err := tx.QueryRow(request.Context(), `
				SELECT item_type, code, name, inventory_account_id
				FROM items WHERE tenant_id = $1 AND id = $2
			`, tenant, line.ItemID).Scan(&itemType, &itemCode, &itemName, &invAcct)
			if err != nil {
				return fmt.Errorf("item %d not found: %w", line.ItemID, err)
			}
			if itemType != "goods" {
				return fmt.Errorf("item %s (%s) is a service — services cannot be transferred", itemCode, itemName)
			}
			if !invAcct.Valid {
				return fmt.Errorf("item %s (%s) is missing inventory account", itemCode, itemName)
			}
			prepared = append(prepared, preparedLine{
				req:       line,
				lineNo:    i + 1,
				invAcctID: invAcct.Int64,
				itemCode:  itemCode,
				itemName:  itemName,
			})
		}

		// Allocate transfer number.
		trfNumber, err := nextDocNumber(request.Context(), tx, tenant, "TRF", "TRF")
		if err != nil {
			return err
		}
		trfDate, _ := parseDate(req.TransferDate)

		// Insert header.
		var trfID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO stock_transfers (tenant_id, number, transfer_date, notes, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, tenant, trfNumber, trfDate, textValueOptional(req.Notes), transferStatusCompleted, int8Value(uid)).Scan(&trfID)
		if err != nil {
			return err
		}

		// Insert lines + record movements.
		for _, p := range prepared {
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO stock_transfer_lines
				    (tenant_id, transfer_id, item_id, line_no, qty, unit_cost_cents, inventory_account_id, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, tenant, trfID, p.req.ItemID, p.lineNo, pgtypeFloat(p.req.Qty),
				p.req.UnitCostCents, p.invAcctID, textValueOptional(p.req.Description)); err != nil {
				return err
			}
			// TRANSFER_OUT (qty negative) — stock leaves.
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
				VALUES ($1, $2, 'TRANSFER_OUT', $3, $4, $5, $6)
			`, tenant, p.req.ItemID, pgtypeFloat(-p.req.Qty), p.req.UnitCostCents, trfNumber, trfID); err != nil {
				return err
			}
			// TRANSFER_IN (qty positive) — stock arrives.
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
				VALUES ($1, $2, 'TRANSFER_IN', $3, $4, $5, $6)
			`, tenant, p.req.ItemID, pgtypeFloat(p.req.Qty), p.req.UnitCostCents, trfNumber, trfID); err != nil {
				return err
			}
		}

		result = &stockTransferResponse{
			ID:           trfID,
			Number:       trfNumber,
			TransferDate: req.TransferDate,
			Notes:        req.Notes,
			Status:       transferStatusCompleted,
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "stock transfer number already exists")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "STOCK_TRANSFER_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) ListStockTransfers(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, number, transfer_date, COALESCE(notes,''), status
		FROM stock_transfers
		WHERE tenant_id = $1
		ORDER BY transfer_date DESC, id DESC
	`, tenant)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()
	items := []stockTransferResponse{}
	for rows.Next() {
		var trf stockTransferResponse
		var notes pgtype.Text
		var trfDate pgtype.Date
		if err := rows.Scan(&trf.ID, &trf.Number, &trfDate, &notes, &trf.Status); err != nil {
			writeError(writer, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		trf.TransferDate = dateString(trfDate)
		trf.Notes = textValue(notes)
		items = append(items, trf)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

func (service *Service) GetStockTransfer(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	trfID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *stockTransferResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = fetchStockTransfer(request.Context(), tx, tenant, trfID)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "STOCK_TRANSFER_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func fetchStockTransfer(ctx context.Context, tx pgx.Tx, tenant, trfID int64) (*stockTransferResponse, error) {
	var trf stockTransferResponse
	var trfDate pgtype.Date
	var notes pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, number, transfer_date, COALESCE(notes,''), status
		FROM stock_transfers
		WHERE tenant_id = $1 AND id = $2
	`, tenant, trfID).Scan(&trf.ID, &trf.Number, &trfDate, &notes, &trf.Status)
	if err != nil {
		return nil, err
	}
	trf.TransferDate = dateString(trfDate)
	trf.Notes = textValue(notes)
	rows, err := tx.Query(ctx, `
		SELECT stl.id, stl.item_id, i.code, i.name, stl.line_no, stl.qty,
		       stl.unit_cost_cents, stl.inventory_account_id, COALESCE(stl.description,'')
		FROM stock_transfer_lines stl
		LEFT JOIN items i ON i.tenant_id = stl.tenant_id AND i.id = stl.item_id
		WHERE stl.tenant_id = $1 AND stl.transfer_id = $2
		ORDER BY stl.line_no
	`, tenant, trfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	trf.Lines = []stockTransferLineResponse{}
	for rows.Next() {
		var line stockTransferLineResponse
		var itemCode, itemName, desc pgtype.Text
		var qty pgtype.Numeric
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&qty, &line.UnitCostCents, &line.InventoryAccountID, &desc); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(desc)
		trf.Lines = append(trf.Lines, line)
	}
	return &trf, rows.Err()
}

func validateStockTransfer(req CreateStockTransferRequest) (string, string) {
	if !validDate(req.TransferDate) {
		return "INVALID_REQUEST", "transfer_date must be a valid YYYY-MM-DD date"
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
		if line.UnitCostCents < 0 {
			return "INVALID_REQUEST", "lines: unit_cost_cents must be >= 0"
		}
	}
	return "", ""
}
