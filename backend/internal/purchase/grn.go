package purchase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/costing"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
)

// GRN posts: Dr 1301 Inventory / Cr 2105 Uninvoiced Payables.
// Account codes resolved from seeded COA (constants in helpers.go).

type CreateGRNRequest struct {
	PurchaseOrderID int64            `json:"purchase_order_id"`
	GRNDate         string           `json:"grn_date"`
	Notes           string           `json:"notes"`
	Lines           []GRNLineRequest `json:"lines"`
}

type GRNLineRequest struct {
	ItemID        int64   `json:"item_id"`
	POLineID      int64   `json:"po_line_id"`
	Qty           float64 `json:"qty"`
	UnitCostCents int64   `json:"unit_cost_cents"`
	Description   string  `json:"description"`
}

type grnLineResponse struct {
	ID             int64   `json:"id"`
	ItemID         int64   `json:"item_id"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	LineNo         int     `json:"line_no"`
	Qty            float64 `json:"qty"`
	UnitCostCents  int64   `json:"unit_cost_cents"`
	LineTotalCents int64   `json:"line_total_cents"`
	Description    string  `json:"description"`
}

type grnResponse struct {
	ID              int64             `json:"id"`
	Number          string            `json:"number"`
	PurchaseOrderID int64             `json:"purchase_order_id"`
	SupplierID      int64             `json:"supplier_id"`
	SupplierName    string            `json:"supplier_name"`
	GRNDate         string            `json:"grn_date"`
	Notes           string            `json:"notes"`
	Status          string            `json:"status"`
	JournalEntryID  int64             `json:"journal_entry_id,omitempty"`
	TotalCents      int64             `json:"total_cents"`
	Lines           []grnLineResponse `json:"lines,omitempty"`
}

func (service *Service) CreateGRN(writer http.ResponseWriter, request *http.Request) {
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
	var req CreateGRNRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateGRNRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	uid := userID(request)
	requestHash := httperr.ComputeRequestHash(request)

	var result grnResponse
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
			// Find GRN by journal id.
			grn, err := fetchGRNByJournal(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = *grn
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load PO.
		var poNumber, poStatus string
		var supplierID int64
		var supplierName string
		err = tx.QueryRow(request.Context(), `
			SELECT po.number, po.status, po.supplier_id, s.name
			FROM purchase_orders po
			JOIN suppliers s ON s.tenant_id = po.tenant_id AND s.id = po.supplier_id
			WHERE po.tenant_id = $1 AND po.id = $2
		`, tenant, req.PurchaseOrderID).Scan(&poNumber, &poStatus, &supplierID, &supplierName)
		if err != nil {
			return fmt.Errorf("purchase order not found: %w", err)
		}
		if poStatus == poStatusCancelled {
			return fmt.Errorf("PO %s is CANCELLED", poNumber)
		}

		// Prepare lines: validate items are goods, resolve inventory account.
		type preparedGRNLine struct {
			line          GRNLineRequest
			lineTotal     int64
			inventoryAcct int64
			costingMethod string
		}
		prepared := make([]preparedGRNLine, 0, len(req.Lines))
		var totalGRN int64
		for _, line := range req.Lines {
			var itemType, itemCode, itemName string
			var invAcct pgtype.Int8
			var costingMethod pgtype.Text
			err := tx.QueryRow(request.Context(), `
				SELECT item_type, code, name, inventory_account_id, costing_method
				FROM items WHERE tenant_id = $1 AND id = $2
			`, tenant, line.ItemID).Scan(&itemType, &itemCode, &itemName, &invAcct, &costingMethod)
			if err != nil {
				return fmt.Errorf("item %d not found: %w", line.ItemID, err)
			}
			if itemType != "goods" {
				return fmt.Errorf("item %s (%s) is a service — services cannot be received", itemCode, itemName)
			}
			if !invAcct.Valid {
				return fmt.Errorf("item %s (%s) is missing inventory account", itemCode, itemName)
			}
			lineTotal := grnLineTotal(line.Qty, line.UnitCostCents)
			totalGRN += lineTotal
			prepared = append(prepared, preparedGRNLine{
				line:          line,
				lineTotal:     lineTotal,
				inventoryAcct: invAcct.Int64,
				costingMethod: textValue(costingMethod),
			})
		}

		// Resolve uninvoiced payable account.
		uninvoicedAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, uninvoicedPayableCode)
		if err != nil {
			return err
		}

		// Build journal: Dr Inventory / Cr Uninvoiced Payables.
		journalLines := make([]accounting.Line, 0, len(prepared)*2)
		for i, p := range prepared {
			journalLines = append(journalLines,
				accounting.Line{AccountID: p.inventoryAcct, DebitCents: p.lineTotal, SourceLineRef: fmt.Sprintf("inv-%d", i)},
				accounting.Line{AccountID: uninvoicedAcctID, CreditCents: p.lineTotal, SourceLineRef: fmt.Sprintf("payable-%d", i)},
			)
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("GRN-%d", req.PurchaseOrderID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("PURCHASE_RECEIPT"),
			EntryDate:   req.GRNDate,
			Description: fmt.Sprintf("GRN: PO %s (%s)", poNumber, supplierName),
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
			INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by, request_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id
		`, tenant, jrnNumber, journal.EntryDate, periodID, journal.Description,
			journal.SourceRef, string(journal.IntentType), idem,
			journal.Hash, journal.PreviousHash, int8Value(uid), textValueOptional(requestHash)).Scan(&entryID)
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
		if err := insertOutbox(request.Context(), tx, tenant, "grn.posted", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber,
		})); err != nil {
			return err
		}

		// Allocate GRN number.
		grnNumber, err := nextDocNumber(request.Context(), tx, tenant, "GRN", "GRN")
		if err != nil {
			return err
		}
		grnDate, _ := parseDate(req.GRNDate)

		// Insert GRN header.
		var grnID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO goods_received_notes (tenant_id, number, purchase_order_id, supplier_id, grn_date, notes, status, journal_entry_id, total_cents)
			VALUES ($1, $2, $3, $4, $5, $6, 'RECEIVED', $7, $8)
			RETURNING id
		`, tenant, grnNumber, req.PurchaseOrderID, supplierID, grnDate,
			textValueOptional(req.Notes), entryID, totalGRN).Scan(&grnID)
		if err != nil {
			return err
		}

		// Insert GRN lines.
		for i, p := range prepared {
			var lineID int64
			var itemCode, itemName pgtype.Text
			err := tx.QueryRow(request.Context(), `
				INSERT INTO grn_lines (tenant_id, grn_id, item_id, po_line_id, line_no, qty, unit_cost_cents, line_total_cents, inventory_account_id, description)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				RETURNING id, (SELECT code FROM items WHERE id = $3), (SELECT name FROM items WHERE id = $3)
			`, tenant, grnID, p.line.ItemID, optionalInt8(p.line.POLineID), i+1,
				pgtypeFloat(p.line.Qty), p.line.UnitCostCents, p.lineTotal,
				p.inventoryAcct, textValueOptional(p.line.Description)).Scan(&lineID, &itemCode, &itemName)
			if err != nil {
				return err
			}
			result.Lines = append(result.Lines, grnLineResponse{
				ID: lineID, ItemID: p.line.ItemID,
				ItemCode: textValue(itemCode), ItemName: textValue(itemName),
				LineNo: i + 1, Qty: p.line.Qty,
				UnitCostCents: p.line.UnitCostCents, LineTotalCents: p.lineTotal,
				Description: p.line.Description,
			})
			// Record inventory movement (qty positive = stock in).
			var posQty pgtype.Numeric
			_ = posQty.Scan(fmt.Sprintf("%g", p.line.Qty))
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
				VALUES ($1, $2, 'GRN', $3, $4, $5, $6)
			`, tenant, p.line.ItemID, posQty, p.line.UnitCostCents, grnNumber, grnID); err != nil {
				return err
			}
			// Post the costing layer / balance update (PSAK 14).
			if err := costing.PostGRN(request.Context(), tx, tenant, p.line.ItemID, 0,
				p.line.Qty, p.line.UnitCostCents, p.costingMethod); err != nil {
				return err
			}
		}

		// Update PO status to PARTIALLY_RECEIVED or RECEIVED.
		newStatus := poStatusAfterGRN(poStatus)
		if _, err := tx.Exec(request.Context(), `
			UPDATE purchase_orders SET status = $1, received_cents = received_cents + $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, newStatus, totalGRN, tenant, req.PurchaseOrderID); err != nil {
			return err
		}

		result.ID = grnID
		result.Number = grnNumber
		result.PurchaseOrderID = req.PurchaseOrderID
		result.SupplierID = supplierID
		result.SupplierName = supplierName
		result.GRNDate = req.GRNDate
		result.Notes = req.Notes
		result.Status = "RECEIVED"
		result.JournalEntryID = entryID
		result.TotalCents = totalGRN

		if err := audit.Log(request.Context(), tx, tenant, uid, "grn", grnID, audit.ActionPost, nil, map[string]any{
			"number":            grnNumber,
			"purchase_order_id": req.PurchaseOrderID,
			"supplier_id":       supplierID,
			"total_cents":       totalGRN,
			"journal_entry_id":  entryID,
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, httperr.ErrIdempotencyKeyReuse) {
			writeError(writer, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", err.Error())
			return
		}
		status, code := httperr.Classify(err)
		writeError(writer, status, code, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListGRNs handles GET /goods-received-notes.
func (service *Service) ListGRNs(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	status := request.URL.Query().Get("status")

	var results []grnResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		query := `
			SELECT grn.id, grn.number, grn.purchase_order_id, grn.supplier_id, s.name,
			       grn.grn_date, grn.notes, grn.status, grn.journal_entry_id, grn.total_cents
			FROM goods_received_notes grn
			JOIN suppliers s ON s.tenant_id = grn.tenant_id AND s.id = grn.supplier_id
		`
		args := []any{}
		if status != "" {
			query += " WHERE grn.status = $1"
			args = append(args, status)
		}
		query += " ORDER BY grn.grn_date DESC, grn.id DESC"
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []grnResponse{}
		for rows.Next() {
			var grn grnResponse
			var grnDate pgtype.Date
			var notes pgtype.Text
			var journalID pgtype.Int8
			if err := rows.Scan(&grn.ID, &grn.Number, &grn.PurchaseOrderID, &grn.SupplierID, &grn.SupplierName,
				&grnDate, &notes, &grn.Status, &journalID, &grn.TotalCents); err != nil {
				return err
			}
			grn.GRNDate = dateString(grnDate)
			grn.Notes = textValue(notes)
			if journalID.Valid {
				grn.JournalEntryID = journalID.Int64
			}
			results = append(results, grn)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "GRN_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetGRN handles GET /goods-received-notes/{id}.
func (service *Service) GetGRN(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	grnID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *grnResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = fetchGRN(request.Context(), tx, tenant, grnID)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "GRN_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func fetchGRN(ctx context.Context, tx pgx.Tx, tenant, grnID int64) (*grnResponse, error) {
	var grn grnResponse
	var grnDate pgtype.Date
	var notes pgtype.Text
	var journalID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT grn.id, grn.number, grn.purchase_order_id, grn.supplier_id, s.name,
		       grn.grn_date, grn.notes, grn.status, grn.journal_entry_id, grn.total_cents
		FROM goods_received_notes grn
		JOIN suppliers s ON s.tenant_id = grn.tenant_id AND s.id = grn.supplier_id
		WHERE grn.tenant_id = $1 AND grn.id = $2
	`, tenant, grnID).Scan(&grn.ID, &grn.Number, &grn.PurchaseOrderID, &grn.SupplierID, &grn.SupplierName,
		&grnDate, &notes, &grn.Status, &journalID, &grn.TotalCents)
	if err != nil {
		return nil, err
	}
	grn.GRNDate = dateString(grnDate)
	grn.Notes = textValue(notes)
	if journalID.Valid {
		grn.JournalEntryID = journalID.Int64
	}
	rows, err := tx.Query(ctx, `
		SELECT gl.id, gl.item_id, i.code, i.name, gl.line_no, gl.qty,
		       gl.unit_cost_cents, gl.line_total_cents, gl.description
		FROM grn_lines gl
		LEFT JOIN items i ON i.tenant_id = gl.tenant_id AND i.id = gl.item_id
		WHERE gl.tenant_id = $1 AND gl.grn_id = $2
		ORDER BY gl.line_no
	`, tenant, grnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grn.Lines = []grnLineResponse{}
	for rows.Next() {
		var line grnLineResponse
		var itemCode, itemName, desc pgtype.Text
		var qty pgtype.Numeric
		if err := rows.Scan(&line.ID, &line.ItemID, &itemCode, &itemName, &line.LineNo,
			&qty, &line.UnitCostCents, &line.LineTotalCents, &desc); err != nil {
			return nil, err
		}
		line.Qty = numericToFloat(qty)
		line.ItemCode = textValue(itemCode)
		line.ItemName = textValue(itemName)
		line.Description = textValue(desc)
		grn.Lines = append(grn.Lines, line)
	}
	return &grn, rows.Err()
}

func fetchGRNByJournal(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (*grnResponse, error) {
	var grn grnResponse
	var grnDate pgtype.Date
	var notes pgtype.Text
	var journalIDOut pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT id, number, purchase_order_id, supplier_id, grn_date, notes, status, journal_entry_id, total_cents
		FROM goods_received_notes
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&grn.ID, &grn.Number, &grn.PurchaseOrderID, &grn.SupplierID,
		&grnDate, &notes, &grn.Status, &journalIDOut, &grn.TotalCents)
	if err != nil {
		return nil, err
	}
	grn.GRNDate = dateString(grnDate)
	grn.Notes = textValue(notes)
	if journalIDOut.Valid {
		grn.JournalEntryID = journalIDOut.Int64
	}
	return &grn, nil
}

func grnLineTotal(qty float64, unitCostCents int64) int64 {
	return int64(qty * float64(unitCostCents))
}

// poStatusAfterGRN keeps a RECEIVED PO at RECEIVED and moves any other
// non-cancelled status to PARTIALLY_RECEIVED (cancelled POs are rejected earlier).
func poStatusAfterGRN(current string) string {
	if current == poStatusReceived {
		return poStatusReceived
	}
	return poStatusPartiallyReceived
}

func validateGRNRequest(req CreateGRNRequest) (string, string) {
	if req.PurchaseOrderID <= 0 {
		return "INVALID_REQUEST", "purchase_order_id is required"
	}
	if !validDate(req.GRNDate) {
		return "INVALID_REQUEST", "grn_date must be a valid YYYY-MM-DD date"
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

func resolveAccountByCode(ctx context.Context, tx pgx.Tx, tenantID int64, code string) (int64, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2`,
		tenantID, code).Scan(&accountID)
	if err != nil {
		return 0, fmt.Errorf("account %s not found: %w", code, err)
	}
	return accountID, nil
}

func lockOrSeedHead(ctx context.Context, tx pgx.Tx, tenantID int64) (db.LedgerChainHead, error) {
	head, err := db.New(tx).LockLedgerChainHead(ctx, tenantID)
	if err == nil {
		return head, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, NULL, 'genesis') ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID); err != nil {
		return db.LedgerChainHead{}, err
	}
	return db.New(tx).LockLedgerChainHead(ctx, tenantID)
}

func resolvePeriod(ctx context.Context, tx pgx.Tx, tenantID int64, date string) (int64, error) {
	var periodID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounting_periods
		WHERE tenant_id = $1 AND $2::date BETWEEN period_start AND period_end
		  AND status IN ('OPEN', 'REOPENED')
		ORDER BY period_start DESC LIMIT 1
	`, tenantID, date).Scan(&periodID)
	if err != nil {
		return 0, fmt.Errorf("entry date is outside an open period: %w", err)
	}
	return periodID, nil
}

func nextJournalNumber(ctx context.Context, tx pgx.Tx, tenantID int64) (string, error) {
	return nextDocNumber(ctx, tx, tenantID, "JRN", "JRN")
}

func upsertHead(ctx context.Context, tx pgx.Tx, tenantID, lastJournalID int64, lastHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_chain_heads (tenant_id, last_journal_id, last_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE
		SET last_journal_id = EXCLUDED.last_journal_id, last_hash = EXCLUDED.last_hash, updated_at = now()
	`, tenantID, lastJournalID, lastHash)
	return err
}

func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID int64, topic string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, topic, payload)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, topic, payload)
	return err
}

func hashJournal(journal accounting.Journal) string {
	return accounting.HashJournal(journal)
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func pgtypeFloat(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%g", v))
	return n
}
