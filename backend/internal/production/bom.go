package production

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// Bill of Materials (US-070)
// ---------------------------------------------------------------------------

// BOMLineRequest is one input/output line on a BOM.
type BOMLineRequest struct {
	ItemID        int64   `json:"item_id"`
	Qty           float64 `json:"qty"`
	UnitCostCents int64   `json:"unit_cost_cents"`
	CostType      string  `json:"cost_type"`
	Description   string  `json:"description"`
}

// CreateBOMRequest is the body of POST /bill-of-materials.
type CreateBOMRequest struct {
	Code               string           `json:"code"`
	Name               string           `json:"name"`
	FinishedGoodItemID int64            `json:"finished_good_item_id"`
	OutputQty          float64          `json:"output_qty"`
	Lines              []BOMLineRequest `json:"lines"`
}

// bomLineResponse is one BOM line in the API response.
type bomLineResponse struct {
	ID             int64   `json:"id"`
	ItemID         int64   `json:"item_id"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	LineNo         int     `json:"line_no"`
	Qty            float64 `json:"qty"`
	UnitCostCents  int64   `json:"unit_cost_cents"`
	LineTotalCents int64   `json:"line_total_cents"`
	CostType       string  `json:"cost_type"`
	Description    string  `json:"description"`
}

// bomResponse is the full BOM response (header + lines).
type bomResponse struct {
	ID                 int64             `json:"id"`
	Code               string            `json:"code"`
	Name               string            `json:"name"`
	FinishedGoodItemID int64             `json:"finished_good_item_id"`
	FinishedGoodCode   string            `json:"finished_good_code"`
	FinishedGoodName   string            `json:"finished_good_name"`
	OutputQty          float64           `json:"output_qty"`
	Status             string            `json:"status"`
	Lines              []bomLineResponse `json:"lines,omitempty"`
}

// CreateBOM handles POST /bill-of-materials.
func (service *Service) CreateBOM(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateBOMRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateBOMRequest(&req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}

	var result bomResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Validate finished-good item: must be a goods item.
		var fgType, fgCode, fgName string
		err := tx.QueryRow(request.Context(), `
			SELECT item_type, code, name FROM items WHERE tenant_id = $1 AND id = $2
		`, tenant, req.FinishedGoodItemID).Scan(&fgType, &fgCode, &fgName)
		if err != nil {
			return fmt.Errorf("finished good item %d not found: %w", req.FinishedGoodItemID, err)
		}
		if fgType != "goods" {
			return fmt.Errorf("finished good item %s (%s) is a service — only goods can be produced", fgCode, fgName)
		}

		// Validate line items: must be goods items.
		type preparedBOMLine struct {
			line      BOMLineRequest
			lineNo    int
			itemCode  string
			itemName  string
			lineTotal int64
		}
		prepared := make([]preparedBOMLine, 0, len(req.Lines))
		for i, line := range req.Lines {
			var itemType, itemCode, itemName string
			err := tx.QueryRow(request.Context(), `
				SELECT item_type, code, name FROM items WHERE tenant_id = $1 AND id = $2
			`, tenant, line.ItemID).Scan(&itemType, &itemCode, &itemName)
			if err != nil {
				return fmt.Errorf("line item %d not found: %w", line.ItemID, err)
			}
			if itemType != "goods" {
				return fmt.Errorf("line item %s (%s) is a service — only goods can be consumed", itemCode, itemName)
			}
			lineTotal := int64(line.Qty * float64(line.UnitCostCents))
			prepared = append(prepared, preparedBOMLine{
				line: line, lineNo: i + 1, itemCode: itemCode, itemName: itemName, lineTotal: lineTotal,
			})
		}

		// Insert BOM header.
		var bomID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO bill_of_materials (tenant_id, code, name, finished_good_item_id, output_qty, status)
			VALUES ($1, $2, $3, $4, $5, 'ACTIVE')
			RETURNING id
		`, tenant, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
			req.FinishedGoodItemID, pgtypeFloat(req.OutputQty)).Scan(&bomID)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("BOM code %s already exists", req.Code)
			}
			return err
		}

		result = bomResponse{
			ID:                 bomID,
			Code:               strings.TrimSpace(req.Code),
			Name:               strings.TrimSpace(req.Name),
			FinishedGoodItemID: req.FinishedGoodItemID,
			FinishedGoodCode:   fgCode,
			FinishedGoodName:   fgName,
			OutputQty:          req.OutputQty,
			Status:             "ACTIVE",
		}

		// Insert BOM lines.
		for _, p := range prepared {
			var lineID int64
			err := tx.QueryRow(request.Context(), `
				INSERT INTO bom_lines (tenant_id, bom_id, item_id, line_no, qty, unit_cost_cents, cost_type, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING id
			`, tenant, bomID, p.line.ItemID, p.lineNo, pgtypeFloat(p.line.Qty),
				p.line.UnitCostCents, p.line.CostType, textValueOptional(p.line.Description)).Scan(&lineID)
			if err != nil {
				return err
			}
			result.Lines = append(result.Lines, bomLineResponse{
				ID:             lineID,
				ItemID:         p.line.ItemID,
				ItemCode:       p.itemCode,
				ItemName:       p.itemName,
				LineNo:         p.lineNo,
				Qty:            p.line.Qty,
				UnitCostCents:  p.line.UnitCostCents,
				LineTotalCents: p.lineTotal,
				CostType:       p.line.CostType,
				Description:    p.line.Description,
			})
		}
		return nil
	})
	if err != nil {
		if isForeignKeyViolation(err) || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "service") {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "BOM_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListBOMs handles GET /bill-of-materials.
func (service *Service) ListBOMs(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var results []bomResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		rows, err := tx.Query(request.Context(), `
			SELECT bom.id, bom.code, bom.name, bom.finished_good_item_id,
			       i.code, i.name, bom.output_qty, bom.status
			FROM bill_of_materials bom
			LEFT JOIN items i ON i.tenant_id = bom.tenant_id AND i.id = bom.finished_good_item_id
			WHERE bom.tenant_id = $1
			ORDER BY bom.code
		`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []bomResponse{}
		for rows.Next() {
			var bom bomResponse
			var fgCode, fgName pgtype.Text
			var outputQty pgtype.Numeric
			if err := rows.Scan(&bom.ID, &bom.Code, &bom.Name, &bom.FinishedGoodItemID,
				&fgCode, &fgName, &outputQty, &bom.Status); err != nil {
				return err
			}
			bom.FinishedGoodCode = textValue(fgCode)
			bom.FinishedGoodName = textValue(fgName)
			bom.OutputQty = numericToFloat(outputQty)
			results = append(results, bom)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "BOM_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetBOM handles GET /bill-of-materials/{id}.
func (service *Service) GetBOM(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	bomID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *bomResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = fetchBOM(request.Context(), tx, tenant, bomID)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "BOM_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// fetchBOM loads a BOM header + lines.
func fetchBOM(ctx context.Context, tx pgx.Tx, tenant, bomID int64) (*bomResponse, error) {
	var bom bomResponse
	var fgCode, fgName pgtype.Text
	var outputQty pgtype.Numeric
	err := tx.QueryRow(ctx, `
		SELECT bom.id, bom.code, bom.name, bom.finished_good_item_id,
		       i.code, i.name, bom.output_qty, bom.status
		FROM bill_of_materials bom
		LEFT JOIN items i ON i.tenant_id = bom.tenant_id AND i.id = bom.finished_good_item_id
		WHERE bom.tenant_id = $1 AND bom.id = $2
	`, tenant, bomID).Scan(&bom.ID, &bom.Code, &bom.Name, &bom.FinishedGoodItemID,
		&fgCode, &fgName, &outputQty, &bom.Status)
	if err != nil {
		return nil, err
	}
	bom.FinishedGoodCode = textValue(fgCode)
	bom.FinishedGoodName = textValue(fgName)
	bom.OutputQty = numericToFloat(outputQty)

	rows, err := tx.Query(ctx, `
		SELECT bl.id, bl.item_id, i.code, i.name, bl.line_no, bl.qty,
		       bl.unit_cost_cents, bl.cost_type, bl.description
		FROM bom_lines bl
		LEFT JOIN items i ON i.tenant_id = bl.tenant_id AND i.id = bl.item_id
		WHERE bl.tenant_id = $1 AND bl.bom_id = $2
		ORDER BY bl.line_no
	`, tenant, bomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bom.Lines = []bomLineResponse{}
	for rows.Next() {
		var line bomLineResponse
		var itemCode, itemName, desc pgtype.Text
		var qty pgtype.Numeric
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&qty, &line.UnitCostCents, &line.CostType, &desc); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(desc)
		line.LineTotalCents = int64(line.Qty * float64(line.UnitCostCents))
		bom.Lines = append(bom.Lines, line)
	}
	return &bom, rows.Err()
}

// validateBOMRequest returns a non-empty code when the request is invalid.
func validateBOMRequest(req *CreateBOMRequest) (string, string) {
	if strings.TrimSpace(req.Code) == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if strings.TrimSpace(req.Name) == "" {
		return "INVALID_REQUEST", "name is required"
	}
	if req.FinishedGoodItemID <= 0 {
		return "INVALID_REQUEST", "finished_good_item_id is required"
	}
	if req.OutputQty <= 0 {
		return "INVALID_REQUEST", "output_qty must be > 0"
	}
	if len(req.Lines) == 0 {
		return "INVALID_REQUEST", "at least one BOM line is required"
	}
	for i, line := range req.Lines {
		if line.ItemID <= 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: item_id is required", i)
		}
		if line.Qty <= 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: qty must be > 0", i)
		}
		if line.UnitCostCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: unit_cost_cents must be >= 0", i)
		}
		ct := strings.TrimSpace(line.CostType)
		if ct == "" {
			line.CostType = "material"
		}
		if ct != "material" && ct != "labor" && ct != "overhead" {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: cost_type must be material, labor, or overhead", i)
		}
	}
	return "", ""
}
