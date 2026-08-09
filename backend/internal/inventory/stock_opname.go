package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/costing"
	"finance-accounting-app/backend/internal/db"
)

// Stock Opname (physical count adjustment). When approved, posts a journal:
//   surplus  (diff > 0): Dr 1301 Inventory / Cr 4907 Inventory Adjustment Gain
//   shortage (diff < 0): Dr 5907 Inventory Adjustment Loss / Cr 1301 Inventory
// Records inventory_movements: OPNAME_IN (surplus) / OPNAME_OUT (shortage).

type StockOpnameLineRequest struct {
	ItemID        int64   `json:"item_id"`
	CountedQty    float64 `json:"counted_qty"`
	UnitCostCents int64   `json:"unit_cost_cents"`
	Reason        string  `json:"reason"`
}

type CreateStockOpnameRequest struct {
	OpnameDate string                   `json:"opname_date"`
	Notes      string                   `json:"notes"`
	Lines      []StockOpnameLineRequest `json:"lines"`
}

type stockOpnameLineResponse struct {
	ID                 int64   `json:"id"`
	ItemID             int64   `json:"item_id"`
	ItemCode           string  `json:"item_code"`
	ItemName           string  `json:"item_name"`
	LineNo             int     `json:"line_no"`
	SystemQty          float64 `json:"system_qty"`
	CountedQty         float64 `json:"counted_qty"`
	DiffQty            float64 `json:"diff_qty"`
	UnitCostCents      int64   `json:"unit_cost_cents"`
	AdjustmentCents    int64   `json:"adjustment_cents"`
	InventoryAccountID int64   `json:"inventory_account_id"`
	Reason             string  `json:"reason"`
	costingMethod      string  `json:"-"`
}

type stockOpnameResponse struct {
	ID                   int64                     `json:"id"`
	Number               string                    `json:"number"`
	OpnameDate           string                    `json:"opname_date"`
	Notes                string                    `json:"notes"`
	Status               string                    `json:"status"`
	JournalEntryID       int64                     `json:"journal_entry_id,omitempty"`
	TotalAdjustmentCents int64                     `json:"total_adjustment_cents"`
	Lines                []stockOpnameLineResponse `json:"lines,omitempty"`
}

const opnameStatusDraft = "DRAFT"
const opnameStatusCounted = "COUNTED"
const opnameStatusApproved = "APPROVED"
const opnameStatusVoid = "VOID"

func (service *Service) CreateStockOpname(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var req CreateStockOpnameRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateStockOpname(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}

	uid := userID(request)
	var result *stockOpnameResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Allocate opname number.
		opnNumber, err := nextDocNumber(request.Context(), tx, tenant, "OPN", "OPN")
		if err != nil {
			return err
		}
		opnDate, _ := parseDate(req.OpnameDate)

		// Insert header.
		var opnID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO stock_opnames (tenant_id, number, opname_date, notes, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, tenant, opnNumber, opnDate, textValueOptional(req.Notes), opnameStatusCounted, int8Value(uid)).Scan(&opnID)
		if err != nil {
			return err
		}

		// Prepare lines: compute system_qty, diff, adjustment per line.
		type preparedLine struct {
			req       StockOpnameLineRequest
			lineNo    int
			systemQty float64
			diffQty   float64
			adjCents  int64
			invAcctID int64
			itemCode  string
			itemName  string
		}
		prepared := make([]preparedLine, 0, len(req.Lines))
		var totalAdj int64
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
				return fmt.Errorf("item %s (%s) is a service — services cannot be opnamed", itemCode, itemName)
			}
			if !invAcct.Valid {
				return fmt.Errorf("item %s (%s) is missing inventory account", itemCode, itemName)
			}
			sys, err := systemQty(request.Context(), tx, tenant, line.ItemID)
			if err != nil {
				return fmt.Errorf("system qty for item %d: %w", line.ItemID, err)
			}
			diff := line.CountedQty - sys
			adjCents := int64(diff * float64(line.UnitCostCents))
			totalAdj += adjCents
			prepared = append(prepared, preparedLine{
				req:       line,
				lineNo:    i + 1,
				systemQty: sys,
				diffQty:   diff,
				adjCents:  adjCents,
				invAcctID: invAcct.Int64,
				itemCode:  itemCode,
				itemName:  itemName,
			})
		}

		// Insert lines.
		for _, p := range prepared {
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO stock_opname_lines
				    (tenant_id, opname_id, item_id, line_no, system_qty, counted_qty,
				     diff_qty, unit_cost_cents, adjustment_cents, inventory_account_id, reason)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`, tenant, opnID, p.req.ItemID, p.lineNo,
				pgtypeFloat(p.systemQty), pgtypeFloat(p.req.CountedQty), pgtypeFloat(p.diffQty),
				p.req.UnitCostCents, p.adjCents, p.invAcctID, textValueOptional(p.req.Reason)); err != nil {
				return err
			}
		}

		// Update header total.
		if _, err := tx.Exec(request.Context(), `
			UPDATE stock_opnames SET total_adjustment_cents = $1, updated_at = now()
			WHERE tenant_id = $2 AND id = $3
		`, totalAdj, tenant, opnID); err != nil {
			return err
		}

		result = &stockOpnameResponse{
			ID:                   opnID,
			Number:               opnNumber,
			OpnameDate:           req.OpnameDate,
			Notes:                req.Notes,
			Status:               opnameStatusCounted,
			TotalAdjustmentCents: totalAdj,
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "stock opname number already exists")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "STOCK_OPNAME_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) ListStockOpnames(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, number, opname_date, COALESCE(notes,''), status,
		       COALESCE(journal_entry_id,0), total_adjustment_cents
		FROM stock_opnames
		WHERE tenant_id = $1
		ORDER BY opname_date DESC, id DESC
	`, tenant)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()
	items := []stockOpnameResponse{}
	for rows.Next() {
		var opn stockOpnameResponse
		var notes pgtype.Text
		var journalID pgtype.Int8
		var opnDate pgtype.Date
		if err := rows.Scan(&opn.ID, &opn.Number, &opnDate, &notes, &opn.Status,
			&journalID, &opn.TotalAdjustmentCents); err != nil {
			writeError(writer, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		opn.OpnameDate = dateString(opnDate)
		opn.Notes = textValue(notes)
		if journalID.Valid {
			opn.JournalEntryID = journalID.Int64
		}
		items = append(items, opn)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

func (service *Service) GetStockOpname(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	opnID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *stockOpnameResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = fetchStockOpname(request.Context(), tx, tenant, opnID)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "STOCK_OPNAME_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) ApproveStockOpname(writer http.ResponseWriter, request *http.Request) {
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
	opnID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	uid := userID(request)

	var result *stockOpnameResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Load header.
		var opn stockOpnameResponse
		var opnDate pgtype.Date
		var notes pgtype.Text
		var journalID pgtype.Int8
		err := tx.QueryRow(request.Context(), `
			SELECT id, number, opname_date, COALESCE(notes,''), status,
			       COALESCE(journal_entry_id,0), total_adjustment_cents
			FROM stock_opnames
			WHERE tenant_id = $1 AND id = $2
		`, tenant, opnID).Scan(&opn.ID, &opn.Number, &opnDate, &notes, &opn.Status,
			&journalID, &opn.TotalAdjustmentCents)
		if err != nil {
			return err
		}
		opn.OpnameDate = dateString(opnDate)
		opn.Notes = textValue(notes)
		if journalID.Valid {
			opn.JournalEntryID = journalID.Int64
		}
		if opn.Status == opnameStatusApproved {
			// Idempotent: already approved.
			result = &opn
			return nil
		}
		if opn.Status == opnameStatusVoid {
			return fmt.Errorf("cannot approve a VOID stock opname")
		}

		// Load lines.
		rows, err := tx.Query(request.Context(), `
			SELECT sol.id, sol.item_id, i.code, i.name, sol.line_no, sol.system_qty,
			       sol.counted_qty, sol.diff_qty, sol.unit_cost_cents, sol.adjustment_cents,
			       sol.inventory_account_id, COALESCE(sol.reason,''), COALESCE(i.costing_method,'')
			FROM stock_opname_lines sol
			LEFT JOIN items i ON i.tenant_id = sol.tenant_id AND i.id = sol.item_id
			WHERE sol.tenant_id = $1 AND sol.opname_id = $2
			ORDER BY sol.line_no
		`, tenant, opnID)
		if err != nil {
			return err
		}
		defer rows.Close()
		opn.Lines = []stockOpnameLineResponse{}
		for rows.Next() {
			var line stockOpnameLineResponse
			var itemCode, itemName, reason pgtype.Text
			var systemQty, countedQty, diffQty pgtype.Numeric
			var costingMethod pgtype.Text
			if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
				&systemQty, &countedQty, &diffQty, &line.UnitCostCents, &line.AdjustmentCents,
				&line.InventoryAccountID, &reason, &costingMethod); err != nil {
				return err
			}
			line.SystemQty = numericToFloat(systemQty)
			line.CountedQty = numericToFloat(countedQty)
			line.DiffQty = numericToFloat(diffQty)
			line.ItemCode = textValue(itemCode)
			line.ItemName = textValue(itemName)
			line.Reason = textValue(reason)
			line.costingMethod = textValue(costingMethod)
			opn.Lines = append(opn.Lines, line)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		// Resolve adjustment accounts.
		adjGainAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, adjustmentGainAccountCode)
		if err != nil {
			return err
		}
		adjLossAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, adjustmentLossAccountCode)
		if err != nil {
			return err
		}

		// Build journal: per-line surplus or shortage.
		journalLines := make([]accounting.Line, 0, len(opn.Lines)*2)
		for i, line := range opn.Lines {
			if line.DiffQty == 0 {
				continue
			}
			ref := fmt.Sprintf("OPN-%d-L%d", opn.ID, i+1)
			if line.DiffQty > 0 {
				// Surplus: Dr Inventory / Cr Adjustment Gain.
				journalLines = append(journalLines,
					accounting.Line{AccountID: line.InventoryAccountID, DebitCents: line.AdjustmentCents, CreditCents: 0, SourceLineRef: ref + "-DR"},
					accounting.Line{AccountID: adjGainAcctID, DebitCents: 0, CreditCents: line.AdjustmentCents, SourceLineRef: ref + "-CR"},
				)
			} else {
				// Shortage: Dr Adjustment Loss / Cr Inventory.
				adjCents := -line.AdjustmentCents
				journalLines = append(journalLines,
					accounting.Line{AccountID: adjLossAcctID, DebitCents: adjCents, CreditCents: 0, SourceLineRef: ref + "-DR"},
					accounting.Line{AccountID: line.InventoryAccountID, DebitCents: 0, CreditCents: adjCents, SourceLineRef: ref + "-CR"},
				)
			}
		}

		var entryID int64
		if len(journalLines) > 0 {
			sourceRef := fmt.Sprintf("OPN-%d", opn.ID)
			journal := accounting.Journal{
				TenantID:    tenant,
				SourceRef:   sourceRef,
				IntentType:  accounting.IntentType("STOCK_OPNAME"),
				EntryDate:   opn.OpnameDate,
				Description: fmt.Sprintf("Stock Opname %s", opn.Number),
				Lines:       journalLines,
			}
			head, err := lockOrSeedHead(request.Context(), tx, tenant)
			if err != nil {
				return err
			}
			journal.PreviousHash = head.LastHash
			journal.Hash = hashJournalForOpname(journal)

			periodID, err := resolvePeriod(request.Context(), tx, tenant, journal.EntryDate)
			if err != nil {
				return err
			}
			jrnNumber, err := nextJournalNumber(request.Context(), tx, tenant)
			if err != nil {
				return err
			}
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
			if err := insertOutbox(request.Context(), tx, tenant, "stock_opname.approved", mustJSON(map[string]any{
				"journal_id": entryID, "number": jrnNumber, "opname_id": opn.ID,
			})); err != nil {
				return err
			}
		}

		// Record inventory movements (OPNAME_IN for surplus, OPNAME_OUT for shortage).
		for _, line := range opn.Lines {
			if line.DiffQty == 0 {
				continue
			}
			movementType := "OPNAME_IN"
			qty := line.DiffQty
			if line.DiffQty < 0 {
				movementType = "OPNAME_OUT"
			}
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, tenant, line.ItemID, movementType, pgtypeFloat(qty), line.UnitCostCents, opn.Number, opn.ID); err != nil {
				return err
			}
		}

		// Mark approved.
		if _, err := tx.Exec(request.Context(), `
			UPDATE stock_opnames
			SET status = $1, journal_entry_id = COALESCE($2, journal_entry_id), updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, opnameStatusApproved, optionalInt8(entryID), tenant, opn.ID); err != nil {
			return err
		}
		opn.Status = opnameStatusApproved
		if entryID != 0 {
			opn.JournalEntryID = entryID
		}
		result = &opn
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "stock opname already approved with this idempotency key")
			return
		}
		writeError(writer, http.StatusInternalServerError, "STOCK_OPNAME_APPROVE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// fetchStockOpname loads an opname header + lines.
func fetchStockOpname(ctx context.Context, tx pgx.Tx, tenant, opnID int64) (*stockOpnameResponse, error) {
	var opn stockOpnameResponse
	var opnDate pgtype.Date
	var notes pgtype.Text
	var journalID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT id, number, opname_date, COALESCE(notes,''), status,
		       COALESCE(journal_entry_id,0), total_adjustment_cents
		FROM stock_opnames
		WHERE tenant_id = $1 AND id = $2
	`, tenant, opnID).Scan(&opn.ID, &opn.Number, &opnDate, &notes, &opn.Status,
		&journalID, &opn.TotalAdjustmentCents)
	if err != nil {
		return nil, err
	}
	opn.OpnameDate = dateString(opnDate)
	opn.Notes = textValue(notes)
	if journalID.Valid {
		opn.JournalEntryID = journalID.Int64
	}
	rows, err := tx.Query(ctx, `
		SELECT sol.id, sol.item_id, i.code, i.name, sol.line_no, sol.system_qty,
		       sol.counted_qty, sol.diff_qty, sol.unit_cost_cents, sol.adjustment_cents,
		       sol.inventory_account_id, COALESCE(sol.reason,'')
		FROM stock_opname_lines sol
		LEFT JOIN items i ON i.tenant_id = sol.tenant_id AND i.id = sol.item_id
		WHERE sol.tenant_id = $1 AND sol.opname_id = $2
		ORDER BY sol.line_no
	`, tenant, opnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	opn.Lines = []stockOpnameLineResponse{}
	for rows.Next() {
		var line stockOpnameLineResponse
		var itemCode, itemName, reason pgtype.Text
		var systemQty, countedQty, diffQty pgtype.Numeric
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&systemQty, &countedQty, &diffQty, &line.UnitCostCents, &line.AdjustmentCents,
			&line.InventoryAccountID, &reason); err != nil {
			return nil, err
		}
		line.SystemQty = numericToFloat(systemQty)
		line.CountedQty = numericToFloat(countedQty)
		line.DiffQty = numericToFloat(diffQty)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Reason = textValue(reason)
		opn.Lines = append(opn.Lines, line)
	}
	return &opn, rows.Err()
}

func validateStockOpname(req CreateStockOpnameRequest) (string, string) {
	if !validDate(req.OpnameDate) {
		return "INVALID_REQUEST", "opname_date must be a valid YYYY-MM-DD date"
	}
	if len(req.Lines) == 0 {
		return "INVALID_REQUEST", "at least one line is required"
	}
	for _, line := range req.Lines {
		if line.ItemID <= 0 {
			return "INVALID_REQUEST", "lines: item_id is required"
		}
		if line.CountedQty < 0 {
			return "INVALID_REQUEST", "lines: counted_qty must be >= 0"
		}
		if line.UnitCostCents < 0 {
			return "INVALID_REQUEST", "lines: unit_cost_cents must be >= 0"
		}
	}
	return "", ""
}

func hashJournalForOpname(journal accounting.Journal) string {
	lines := append([]accounting.Line(nil), journal.Lines...)
	sort.Slice(lines, func(l, r int) bool { return lines[l].SourceLineRef < lines[r].SourceLineRef })
	payload := fmt.Sprintf("v1|%d|%s|%s|%s|%s|%v",
		journal.TenantID, journal.SourceRef, journal.IntentType,
		journal.EntryDate, journal.PreviousHash, lines)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// pgtypeFloat converts a float64 into a pgtype.Numeric for NUMERIC columns.
func pgtypeFloat(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strings.TrimSpace(fmt.Sprintf("%g", v)))
	return n
}
