package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
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

// ---------------------------------------------------------------------------
// Production Jobs (US-070..072)
// ---------------------------------------------------------------------------

// ProductionJobCostRequest is the body of POST /production-jobs/{id}/costs.
type ProductionJobCostRequest struct {
	CostType      string  `json:"cost_type"` // material | labor | overhead
	ItemID        int64   `json:"item_id"`
	Description   string  `json:"description"`
	Qty           float64 `json:"qty"`
	UnitCostCents int64   `json:"unit_cost_cents"`
}

// productionJobCostResponse is one cost line in the API response.
type productionJobCostResponse struct {
	ID             int64   `json:"id"`
	CostType       string  `json:"cost_type"`
	ItemID         int64   `json:"item_id,omitempty"`
	ItemCode       string  `json:"item_code,omitempty"`
	ItemName       string  `json:"item_name,omitempty"`
	Description    string  `json:"description"`
	Qty            float64 `json:"qty,omitempty"`
	UnitCostCents  int64   `json:"unit_cost_cents"`
	TotalCents     int64   `json:"total_cents"`
	JournalEntryID int64   `json:"journal_entry_id,omitempty"`
	PostedAt       string  `json:"posted_at,omitempty"`
}

// productionJobResponse is the full production job response (header + costs).
type productionJobResponse struct {
	ID                    int64                       `json:"id"`
	Number                string                      `json:"number"`
	BOMID                 int64                       `json:"bom_id,omitempty"`
	FinishedGoodItemID    int64                       `json:"finished_good_item_id"`
	FinishedGoodCode      string                      `json:"finished_good_code"`
	FinishedGoodName      string                      `json:"finished_good_name"`
	TargetQty             float64                     `json:"target_qty"`
	CompletedQty          float64                     `json:"completed_qty"`
	StartDate             string                      `json:"start_date"`
	CompletionDate        string                      `json:"completion_date,omitempty"`
	Status                string                      `json:"status"`
	WIPAccountID          int64                       `json:"wip_account_id"`
	FinishedGoodAccountID int64                       `json:"finished_good_account_id"`
	TotalMaterialCents    int64                       `json:"total_material_cents"`
	TotalLaborCents       int64                       `json:"total_labor_cents"`
	TotalOverheadCents    int64                       `json:"total_overhead_cents"`
	TotalCostCents        int64                       `json:"total_cost_cents"`
	VarianceCents         int64                       `json:"variance_cents"`
	JournalEntryID        int64                       `json:"journal_entry_id,omitempty"`
	Costs                 []productionJobCostResponse `json:"costs,omitempty"`
}

// CreateProductionJobRequest is the body of POST /production-jobs.
type CreateProductionJobRequest struct {
	BOMID              int64   `json:"bom_id"`
	FinishedGoodItemID int64   `json:"finished_good_item_id"`
	TargetQty          float64 `json:"target_qty"`
	StartDate          string  `json:"start_date"`
}

// CreateProductionJob handles POST /production-jobs.
func (service *Service) CreateProductionJob(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateProductionJobRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateCreateJobRequest(&req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	uid := userID(request)

	var result productionJobResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Resolve WIP and Finished Goods accounts.
		wipAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, wipAccountCode)
		if err != nil {
			return err
		}
		fgAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, finishedGoodsAccountCode)
		if err != nil {
			return err
		}

		// Validate finished-good item: must be a goods item.
		var fgType, fgCode, fgName string
		err = tx.QueryRow(request.Context(), `
			SELECT item_type, code, name FROM items WHERE tenant_id = $1 AND id = $2
		`, tenant, req.FinishedGoodItemID).Scan(&fgType, &fgCode, &fgName)
		if err != nil {
			return fmt.Errorf("finished good item %d not found: %w", req.FinishedGoodItemID, err)
		}
		if fgType != "goods" {
			return fmt.Errorf("finished good item %s (%s) is a service — only goods can be produced", fgCode, fgName)
		}

		// Optional: load BOM and verify it matches the finished good.
		var bomID pgtype.Int8
		if req.BOMID > 0 {
			var bomFGItemID int64
			err = tx.QueryRow(request.Context(), `
				SELECT id, finished_good_item_id FROM bill_of_materials
				WHERE tenant_id = $1 AND id = $2 AND status = 'ACTIVE'
			`, tenant, req.BOMID).Scan(&bomID, &bomFGItemID)
			if err != nil {
				return fmt.Errorf("BOM %d not found: %w", req.BOMID, err)
			}
			if bomFGItemID != req.FinishedGoodItemID {
				return fmt.Errorf("BOM %d produces a different finished good", req.BOMID)
			}
		}

		startDate, err := parseDate(req.StartDate)
		if err != nil {
			return err
		}

		// Allocate job number: JOB-{YYYY}-{seq}.
		jobNumber, err := nextDocNumber(request.Context(), tx, tenant, "JOB", "JOB")
		if err != nil {
			return err
		}

		var jobID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO production_jobs
			    (tenant_id, number, bom_id, finished_good_item_id, target_qty, start_date, status,
			     wip_account_id, finished_good_account_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, 'OPEN', $7, $8, $9)
			RETURNING id
		`, tenant, jobNumber, bomID, req.FinishedGoodItemID, pgtypeFloat(req.TargetQty),
			startDate, wipAcctID, fgAcctID, int8Value(uid)).Scan(&jobID)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("job number %s already exists", jobNumber)
			}
			return err
		}

		result = productionJobResponse{
			ID:                    jobID,
			Number:                jobNumber,
			BOMID:                 req.BOMID,
			FinishedGoodItemID:    req.FinishedGoodItemID,
			FinishedGoodCode:      fgCode,
			FinishedGoodName:      fgName,
			TargetQty:             req.TargetQty,
			StartDate:             req.StartDate,
			Status:                "OPEN",
			WIPAccountID:          wipAcctID,
			FinishedGoodAccountID: fgAcctID,
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
		writeError(writer, http.StatusInternalServerError, "JOB_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListProductionJobs handles GET /production-jobs.
func (service *Service) ListProductionJobs(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var results []productionJobResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		rows, err := tx.Query(request.Context(), `
			SELECT pj.id, pj.number, COALESCE(pj.bom_id,0), pj.finished_good_item_id,
			       i.code, i.name, pj.target_qty, pj.completed_qty, pj.start_date,
			       COALESCE(pj.completion_date,''), pj.status,
			       pj.wip_account_id, pj.finished_good_account_id,
			       pj.total_material_cents, pj.total_labor_cents, pj.total_overhead_cents,
			       pj.total_cost_cents, pj.variance_cents, COALESCE(pj.journal_entry_id,0)
			FROM production_jobs pj
			LEFT JOIN items i ON i.tenant_id = pj.tenant_id AND i.id = pj.finished_good_item_id
			WHERE pj.tenant_id = $1
			ORDER BY pj.start_date DESC, pj.id DESC
		`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []productionJobResponse{}
		for rows.Next() {
			var job productionJobResponse
			var fgCode, fgName pgtype.Text
			var startDate pgtype.Date
			var completionDate pgtype.Date
			var bomID int64
			if err := rows.Scan(&job.ID, &job.Number, &bomID, &job.FinishedGoodItemID,
				&fgCode, &fgName, &job.TargetQty, &job.CompletedQty, &startDate,
				&completionDate, &job.Status, &job.WIPAccountID, &job.FinishedGoodAccountID,
				&job.TotalMaterialCents, &job.TotalLaborCents, &job.TotalOverheadCents,
				&job.TotalCostCents, &job.VarianceCents, &job.JournalEntryID); err != nil {
				return err
			}
			job.BOMID = bomID
			job.FinishedGoodCode = textValue(fgCode)
			job.FinishedGoodName = textValue(fgName)
			job.StartDate = dateString(startDate)
			job.CompletionDate = dateString(completionDate)
			results = append(results, job)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "JOB_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// GetProductionJob handles GET /production-jobs/{id}.
func (service *Service) GetProductionJob(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	jobID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var result *productionJobResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var err error
		result, err = fetchProductionJob(request.Context(), tx, tenant, jobID)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "JOB_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// AddProductionJobCost handles POST /production-jobs/{id}/costs (US-071).
//
// Material: Dr 1303 WIP / Cr 1301 Inventory (consume raw material)
// Labor:    Dr 1303 WIP / Cr 1101 Cash
// Overhead: Dr 1303 WIP / Cr 1101 Cash
//
// Intent type: PRODUCTION_COST. Idempotency-Key required.
func (service *Service) AddProductionJobCost(writer http.ResponseWriter, request *http.Request) {
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
	jobID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req ProductionJobCostRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateJobCostRequest(&req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	uid := userID(request)
	costType := strings.TrimSpace(req.CostType)

	var result productionJobCostResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Idempotent replay: if this idempotency key already produced a
		// journal entry, return the matching cost line.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			// Find the cost line posted against this journal.
			return loadCostByJournal(request.Context(), tx, tenant, existing.ID, &result)
		} else if !isNoRows(err) {
			return err
		}

		// Load the job header.
		var job productionJobResponse
		var bomID pgtype.Int8
		var startDate pgtype.Date
		err = tx.QueryRow(request.Context(), `
			SELECT pj.id, pj.number, COALESCE(pj.bom_id,0), pj.finished_good_item_id,
			       pj.target_qty, pj.completed_qty, pj.start_date, pj.status,
			       pj.wip_account_id, pj.finished_good_account_id,
			       pj.total_material_cents, pj.total_labor_cents, pj.total_overhead_cents,
			       pj.total_cost_cents
			FROM production_jobs pj
			WHERE pj.tenant_id = $1 AND pj.id = $2
		`, tenant, jobID).Scan(&job.ID, &job.Number, &bomID, &job.FinishedGoodItemID,
			&job.TargetQty, &job.CompletedQty, &startDate, &job.Status,
			&job.WIPAccountID, &job.FinishedGoodAccountID,
			&job.TotalMaterialCents, &job.TotalLaborCents, &job.TotalOverheadCents,
			&job.TotalCostCents)
		if err != nil {
			return fmt.Errorf("production job %d not found: %w", jobID, err)
		}
		if job.Status == "COMPLETED" {
			return fmt.Errorf("job %s is COMPLETED — cannot add costs", job.Number)
		}
		if job.Status == "CANCELLED" {
			return fmt.Errorf("job %s is CANCELLED — cannot add costs", job.Number)
		}

		// Resolve WIP account.
		wipAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, wipAccountCode)
		if err != nil {
			return err
		}

		// Compute the cost total. For material, resolve COGS from inventory
		// (the actual cost may differ from the caller-supplied unit cost for
		// FIFO / moving-average items). For labor/overhead the caller cost
		// is authoritative.
		totalCents := int64(0)
		unitCostCents := req.UnitCostCents
		qty := req.Qty
		var counterAccountID int64
		var itemCode, itemName string

		if costType == "material" {
			// Material: consume raw material from inventory.
			if req.ItemID <= 0 {
				return fmt.Errorf("material cost requires item_id")
			}
			var itemType, invCode, invName string
			var invAcct pgtype.Int8
			var costingMethod pgtype.Text
			err = tx.QueryRow(request.Context(), `
				SELECT item_type, code, name, inventory_account_id, costing_method
				FROM items WHERE tenant_id = $1 AND id = $2
			`, tenant, req.ItemID).Scan(&itemType, &invCode, &invName, &invAcct, &costingMethod)
			if err != nil {
				return fmt.Errorf("material item %d not found: %w", req.ItemID, err)
			}
			if itemType != "goods" {
				return fmt.Errorf("item %s (%s) is a service — services cannot be consumed", invCode, invName)
			}
			if !invAcct.Valid {
				return fmt.Errorf("item %s (%s) is missing inventory account", invCode, invName)
			}
			counterAccountID = invAcct.Int64
			itemCode = invCode
			itemName = invName
			// Resolve the actual COGS for this quantity (consumes FIFO layers
			// / adjusts moving average). The resolved cost is authoritative.
			method := textValue(costingMethod)
			resolvedCOGS, err := costing.ResolveCOGS(request.Context(), tx, tenant, req.ItemID, qty, method)
			if err != nil {
				return fmt.Errorf("costing resolve for item %d: %w", req.ItemID, err)
			}
			totalCents = resolvedCOGS
			unitCostCents = 0
			if qty > 0 {
				unitCostCents = int64(float64(resolvedCOGS) / qty)
			}
			// Record inventory movement (qty negative = stock out).
			if _, err := tx.Exec(request.Context(), `
				INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
				VALUES ($1, $2, 'PRODUCTION_OUT', $3, $4, $5, $6)
			`, tenant, req.ItemID, pgtypeFloat(-qty), unitCostCents, job.Number, job.ID); err != nil {
				return err
			}
		} else {
			// Labor: Dr WIP / Cr 5201 Direct Labor Expense
			// Overhead: Dr WIP / Cr 4902 Overhead Applied
			var counterCode string
			switch costType {
			case "labor":
				counterCode = "5201"
			case "overhead":
				counterCode = "4902"
			default:
				counterCode = "5201" // fallback to labor
			}
			counterAccountID, err = resolveAccountByCode(request.Context(), tx, tenant, counterCode)
			if err != nil {
				return fmt.Errorf("account %s not found: %w", counterCode, err)
			}
			// Integer math: qtyMilli * unitCostCents / 1000 (round half up)
			qtyMilli := int64(math.Round(qty * 1000))
			if qtyMilli <= 0 {
				qtyMilli = 1000
			}
			totalCents = (qtyMilli*req.UnitCostCents + 500) / 1000
			if totalCents <= 0 {
				totalCents = req.UnitCostCents
			}
		}

		// Build journal: Dr WIP / Cr counter (inventory or cash).
		journalLines := []accounting.Line{
			{AccountID: wipAcctID, DebitCents: totalCents, SourceLineRef: "wip-1"},
			{AccountID: counterAccountID, CreditCents: totalCents, SourceLineRef: "counter-1"},
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		entryDate := time.Now().UTC().Format("2006-01-02")
		sourceRef := fmt.Sprintf("JOB-%d-COST", job.ID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("PRODUCTION_COST"),
			EntryDate:   entryDate,
			Description: fmt.Sprintf("Production cost (%s): job %s", costType, job.Number),
			Lines:       journalLines,
		}

		// Hash-chain.
		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		// Compute hash via the same formula the engine uses.
		journal.Hash = hashJobJournal(journal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, entryDate)
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
		`, tenant, jrnNumber, entryDate, periodID, journal.Description,
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
		if err := insertOutbox(request.Context(), tx, tenant, "production_cost.posted", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber, "job_id": job.ID, "cost_type": costType,
		})); err != nil {
			return err
		}

		// Determine the next line_no.
		var lineNo int
		err = tx.QueryRow(request.Context(), `
			SELECT COALESCE(MAX(line_no), 0) + 1 FROM production_job_costs
			WHERE tenant_id = $1 AND job_id = $2
		`, tenant, job.ID).Scan(&lineNo)
		if err != nil {
			return err
		}

		// Insert the cost line.
		var costID int64
		var itemID pgtype.Int8
		if req.ItemID > 0 {
			itemID = pgtype.Int8{Int64: req.ItemID, Valid: true}
		}
		err = tx.QueryRow(request.Context(), `
			INSERT INTO production_job_costs
			    (tenant_id, job_id, line_no, cost_type, item_id, description, qty, unit_cost_cents, total_cents, journal_entry_id, posted_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
			RETURNING id
		`, tenant, job.ID, lineNo, costType, itemID, textValueOptional(req.Description),
			pgtypeFloat(qty), unitCostCents, totalCents, entryID).Scan(&costID)
		if err != nil {
			return err
		}

		// Accumulate cost on the job header.
		var materialInc, laborInc, overheadInc int64
		switch costType {
		case "material":
			materialInc = totalCents
		case "labor":
			laborInc = totalCents
		case "overhead":
			overheadInc = totalCents
		}
		_, err = tx.Exec(request.Context(), `
			UPDATE production_jobs
			SET total_material_cents = total_material_cents + $3,
			    total_labor_cents = total_labor_cents + $4,
			    total_overhead_cents = total_overhead_cents + $5,
			    total_cost_cents = total_cost_cents + $6,
			    status = CASE WHEN status = 'OPEN' THEN 'IN_PROGRESS' ELSE status END,
			    updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, job.ID, materialInc, laborInc, overheadInc, totalCents)
		if err != nil {
			return err
		}

		result = productionJobCostResponse{
			ID:             costID,
			CostType:       costType,
			ItemID:         req.ItemID,
			ItemCode:       itemCode,
			ItemName:       itemName,
			Description:    req.Description,
			Qty:            qty,
			UnitCostCents:  unitCostCents,
			TotalCents:     totalCents,
			JournalEntryID: entryID,
			PostedAt:       time.Now().UTC().Format(time.RFC3339),
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "cost already posted with this idempotency key")
			return
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "service") || strings.Contains(err.Error(), "insufficient stock") {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "JOB_COST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// CompleteProductionJob handles POST /production-jobs/{id}/complete (US-072).
//
// Dr 1304 Finished Goods / Cr 1303 WIP (accumulated cost)
// If variance > 0 (over-absorbed): Dr 5908 Variance Loss / Cr 1303 WIP
// If variance < 0 (under-absorbed): Dr 1303 WIP / Cr 4908 Variance Gain
//
// Intent type: PRODUCTION_COMPLETE. Status → COMPLETED.
// Records inventory_movement (PRODUCTION_IN for finished goods).
func (service *Service) CompleteProductionJob(writer http.ResponseWriter, request *http.Request) {
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
	jobID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	uid := userID(request)

	var result productionJobResponse
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
			// Return the job as already completed.
			job, err := fetchProductionJob(request.Context(), tx, tenant, jobID)
			if err != nil {
				return err
			}
			result = *job
			_ = existing
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Load the job header.
		var job productionJobResponse
		var bomID pgtype.Int8
		var startDate pgtype.Date
		var fgCode, fgName pgtype.Text
		err = tx.QueryRow(request.Context(), `
			SELECT pj.id, pj.number, COALESCE(pj.bom_id,0), pj.finished_good_item_id,
			       i.code, i.name, pj.target_qty, pj.completed_qty, pj.start_date, pj.status,
			       pj.wip_account_id, pj.finished_good_account_id,
			       pj.total_material_cents, pj.total_labor_cents, pj.total_overhead_cents,
			       pj.total_cost_cents
			FROM production_jobs pj
			LEFT JOIN items i ON i.tenant_id = pj.tenant_id AND i.id = pj.finished_good_item_id
			WHERE pj.tenant_id = $1 AND pj.id = $2
		`, tenant, jobID).Scan(&job.ID, &job.Number, &bomID, &job.FinishedGoodItemID,
			&fgCode, &fgName, &job.TargetQty, &job.CompletedQty, &startDate, &job.Status,
			&job.WIPAccountID, &job.FinishedGoodAccountID,
			&job.TotalMaterialCents, &job.TotalLaborCents, &job.TotalOverheadCents,
			&job.TotalCostCents)
		if err != nil {
			return fmt.Errorf("production job %d not found: %w", jobID, err)
		}
		job.FinishedGoodCode = textValue(fgCode)
		job.FinishedGoodName = textValue(fgName)
		job.StartDate = dateString(startDate)
		if job.Status == "COMPLETED" {
			return fmt.Errorf("job %s is already COMPLETED", job.Number)
		}
		if job.Status == "CANCELLED" {
			return fmt.Errorf("job %s is CANCELLED — cannot complete", job.Number)
		}

		completedQty := job.TargetQty // default: the full target is completed.
		totalCost := job.TotalCostCents

		// Variance: the difference between accumulated WIP cost and the
		// standard/expected cost. Here we treat the accumulated cost as
		// the amount transferred to Finished Goods, and the variance is
		// zero unless an expected cost is supplied (kept for future use).
		// For now the full WIP balance moves to Finished Goods: variance = 0.
		variance := int64(0)

		// Build journal: Dr Finished Goods / Cr WIP (the accumulated cost).
		journalLines := []accounting.Line{
			{AccountID: job.FinishedGoodAccountID, DebitCents: totalCost, SourceLineRef: "fg-1"},
			{AccountID: job.WIPAccountID, CreditCents: totalCost, SourceLineRef: "wip-1"},
		}

		// Book variance if non-zero (M-012: accounts are 5908 / 4908).
		varianceLossAcctID := int64(0)
		varianceGainAcctID := int64(0)
		if variance > 0 {
			// Over-absorbed: loss. Dr 5908 / Cr 1303.
			varianceLossAcctID, err = resolveAccountByCode(request.Context(), tx, tenant, varianceLossAccountCode)
			if err != nil {
				return err
			}
			journalLines = append(journalLines,
				accounting.Line{AccountID: varianceLossAcctID, DebitCents: variance, SourceLineRef: "vloss-1"},
				accounting.Line{AccountID: job.WIPAccountID, CreditCents: variance, SourceLineRef: "wip-2"},
			)
		} else if variance < 0 {
			// Under-absorbed: gain. Dr 1303 / Cr 4908.
			varianceGainAcctID, err = resolveAccountByCode(request.Context(), tx, tenant, varianceGainAccountCode)
			if err != nil {
				return err
			}
			gain := -variance
			journalLines = append(journalLines,
				accounting.Line{AccountID: job.WIPAccountID, DebitCents: gain, SourceLineRef: "wip-2"},
				accounting.Line{AccountID: varianceGainAcctID, CreditCents: gain, SourceLineRef: "vgain-1"},
			)
		}

		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		entryDate := time.Now().UTC().Format("2006-01-02")
		sourceRef := fmt.Sprintf("JOB-%d-COMPLETE", job.ID)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("PRODUCTION_COMPLETE"),
			EntryDate:   entryDate,
			Description: fmt.Sprintf("Production complete: job %s (%s)", job.Number, job.FinishedGoodName),
			Lines:       journalLines,
		}

		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashJobJournal(journal)

		periodID, err := resolvePeriod(request.Context(), tx, tenant, entryDate)
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
		`, tenant, jrnNumber, entryDate, periodID, journal.Description,
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
		if err := insertOutbox(request.Context(), tx, tenant, "production_complete.posted", mustJSON(map[string]any{
			"journal_id": entryID, "number": jrnNumber, "job_id": job.ID,
		})); err != nil {
			return err
		}

		// Record inventory movement (PRODUCTION_IN for finished goods).
		unitCost := int64(0)
		if completedQty > 0 {
			unitCost = int64(float64(totalCost) / completedQty)
		}
		if _, err := tx.Exec(request.Context(), `
			INSERT INTO inventory_movements (tenant_id, item_id, movement_type, qty, unit_cost_cents, source_ref, source_id)
			VALUES ($1, $2, 'PRODUCTION_IN', $3, $4, $5, $6)
		`, tenant, job.FinishedGoodItemID, pgtypeFloat(completedQty), unitCost, job.Number, job.ID); err != nil {
			return err
		}

		// Post the finished-goods stock receipt into costing (moving average /
		// FIFO layer) so the finished-good inventory balance is correct.
		var fgCostingMethod pgtype.Text
		err = tx.QueryRow(request.Context(), `
			SELECT costing_method FROM items WHERE tenant_id = $1 AND id = $2
		`, tenant, job.FinishedGoodItemID).Scan(&fgCostingMethod)
		if err != nil {
			return fmt.Errorf("finished good item %d costing_method: %w", job.FinishedGoodItemID, err)
		}
		method := textValue(fgCostingMethod)
		if method == "" {
			method = costing.MethodMovingAverage
		}
		if err := costing.PostGRN(request.Context(), tx, tenant, job.FinishedGoodItemID, completedQty, unitCost, method); err != nil {
			return fmt.Errorf("costing PostGRN for finished good: %w", err)
		}

		// Mark the job COMPLETED.
		if _, err := tx.Exec(request.Context(), `
			UPDATE production_jobs
			SET status = 'COMPLETED', completed_qty = $3, completion_date = current_date,
			    variance_cents = $4, journal_entry_id = $5, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, job.ID, pgtypeFloat(completedQty), variance, entryID); err != nil {
			return err
		}

		// Load the final job state with costs.
		final, err := fetchProductionJob(request.Context(), tx, tenant, jobID)
		if err != nil {
			return err
		}
		result = *final
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DUPLICATE", "job already completed with this idempotency key")
			return
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "already COMPLETED") || strings.Contains(err.Error(), "CANCELLED") {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "JOB_COMPLETE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// fetchProductionJob loads a job header + cost lines.
func fetchProductionJob(ctx context.Context, tx pgx.Tx, tenant, jobID int64) (*productionJobResponse, error) {
	var job productionJobResponse
	var bomID pgtype.Int8
	var startDate, completionDate pgtype.Date
	var fgCode, fgName pgtype.Text
	var journalID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT pj.id, pj.number, COALESCE(pj.bom_id,0), pj.finished_good_item_id,
		       i.code, i.name, pj.target_qty, pj.completed_qty, pj.start_date,
		       COALESCE(pj.completion_date,''), pj.status,
		       pj.wip_account_id, pj.finished_good_account_id,
		       pj.total_material_cents, pj.total_labor_cents, pj.total_overhead_cents,
		       pj.total_cost_cents, pj.variance_cents, pj.journal_entry_id
		FROM production_jobs pj
		LEFT JOIN items i ON i.tenant_id = pj.tenant_id AND i.id = pj.finished_good_item_id
		WHERE pj.tenant_id = $1 AND pj.id = $2
	`, tenant, jobID).Scan(&job.ID, &job.Number, &bomID, &job.FinishedGoodItemID,
		&fgCode, &fgName, &job.TargetQty, &job.CompletedQty, &startDate,
		&completionDate, &job.Status, &job.WIPAccountID, &job.FinishedGoodAccountID,
		&job.TotalMaterialCents, &job.TotalLaborCents, &job.TotalOverheadCents,
		&job.TotalCostCents, &job.VarianceCents, &journalID)
	if err != nil {
		return nil, err
	}
	job.BOMID = bomID.Int64
	if !bomID.Valid {
		job.BOMID = 0
	}
	job.FinishedGoodCode = textValue(fgCode)
	job.FinishedGoodName = textValue(fgName)
	job.StartDate = dateString(startDate)
	job.CompletionDate = dateString(completionDate)
	if journalID.Valid {
		job.JournalEntryID = journalID.Int64
	}

	rows, err := tx.Query(ctx, `
		SELECT pjc.id, pjc.cost_type, COALESCE(pjc.item_id,0), i.code, i.name,
		       pjc.description, COALESCE(pjc.qty,0), pjc.unit_cost_cents, pjc.total_cents,
		       COALESCE(pjc.journal_entry_id,0), pjc.posted_at
		FROM production_job_costs pjc
		LEFT JOIN items i ON i.tenant_id = pjc.tenant_id AND i.id = pjc.item_id
		WHERE pjc.tenant_id = $1 AND pjc.job_id = $2
		ORDER BY pjc.line_no
	`, tenant, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	job.Costs = []productionJobCostResponse{}
	for rows.Next() {
		var cost productionJobCostResponse
		var itemCode, itemName pgtype.Text
		var qty pgtype.Numeric
		var postedAt pgtype.Timestamptz
		var itemID int64
		var jeID int64
		if err := rows.Scan(&cost.ID, &cost.CostType, &itemID, &itemCode, &itemName,
			&cost.Description, &qty, &cost.UnitCostCents, &cost.TotalCents,
			&jeID, &postedAt); err != nil {
			return nil, err
		}
		cost.ItemID = itemID
		cost.ItemCode = textValue(itemCode)
		cost.ItemName = textValue(itemName)
		cost.Qty = numericToFloat(qty)
		cost.JournalEntryID = jeID
		if postedAt.Valid {
			cost.PostedAt = postedAt.Time.Format(time.RFC3339)
		}
		job.Costs = append(job.Costs, cost)
	}
	return &job, rows.Err()
}

// loadCostByJournal finds the cost line posted against a journal entry (for
// idempotent replay).
func loadCostByJournal(ctx context.Context, tx pgx.Tx, tenant, journalID int64, out *productionJobCostResponse) error {
	var itemID pgtype.Int8
	var itemCode, itemName pgtype.Text
	var qty pgtype.Numeric
	var postedAt pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		SELECT pjc.id, pjc.cost_type, pjc.item_id, i.code, i.name,
		       pjc.description, pjc.qty, pjc.unit_cost_cents, pjc.total_cents,
		       pjc.journal_entry_id, pjc.posted_at
		FROM production_job_costs pjc
		LEFT JOIN items i ON i.tenant_id = pjc.tenant_id AND i.id = pjc.item_id
		WHERE pjc.tenant_id = $1 AND pjc.journal_entry_id = $2
		LIMIT 1
	`, tenant, journalID).Scan(&out.ID, &out.CostType, &itemID, &itemCode, &itemName,
		&out.Description, &qty, &out.UnitCostCents, &out.TotalCents,
		&out.JournalEntryID, &postedAt)
	if err != nil {
		return err
	}
	if itemID.Valid {
		out.ItemID = itemID.Int64
	}
	out.ItemCode = textValue(itemCode)
	out.ItemName = textValue(itemName)
	out.Qty = numericToFloat(qty)
	if postedAt.Valid {
		out.PostedAt = postedAt.Time.Format(time.RFC3339)
	}
	return nil
}

// validateCreateJobRequest returns a non-empty code when the request is invalid.
func validateCreateJobRequest(req *CreateProductionJobRequest) (string, string) {
	if req.FinishedGoodItemID <= 0 {
		return "INVALID_REQUEST", "finished_good_item_id is required"
	}
	if req.TargetQty <= 0 {
		return "INVALID_REQUEST", "target_qty must be > 0"
	}
	if strings.TrimSpace(req.StartDate) == "" {
		return "INVALID_REQUEST", "start_date is required"
	}
	return "", ""
}

// validateJobCostRequest returns a non-empty code when the request is invalid.
func validateJobCostRequest(req *ProductionJobCostRequest) (string, string) {
	ct := strings.TrimSpace(req.CostType)
	if ct == "" {
		return "INVALID_REQUEST", "cost_type is required"
	}
	if ct != "material" && ct != "labor" && ct != "overhead" {
		return "INVALID_REQUEST", "cost_type must be material, labor, or overhead"
	}
	if ct == "material" && req.ItemID <= 0 {
		return "INVALID_REQUEST", "material cost requires item_id"
	}
	if req.Qty <= 0 && req.UnitCostCents <= 0 {
		return "INVALID_REQUEST", "qty or unit_cost_cents must be > 0"
	}
	if req.UnitCostCents < 0 {
		return "INVALID_REQUEST", "unit_cost_cents must be >= 0"
	}
	return "", ""
}

// hashJobJournal computes the SHA-256 hash of an accounting.Journal using
// the same formula as accounting.hashJournal (which is private). Keeping the
// formula in sync ensures the ledger chain stays consistent across packages.
func hashJobJournal(journal accounting.Journal) string {
	lines := append([]accounting.Line(nil), journal.Lines...)
	sortByRef(lines)
	payload := fmt.Sprintf("v1|%d|%s|%s|%s|%s|%v",
		journal.TenantID, journal.SourceRef, string(journal.IntentType),
		journal.EntryDate, journal.PreviousHash, lines)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func sortByRef(lines []accounting.Line) {
	for i := 1; i < len(lines); i++ {
		for j := i; j > 0 && lines[j-1].SourceLineRef > lines[j].SourceLineRef; j-- {
			lines[j-1], lines[j] = lines[j], lines[j-1]
		}
	}
}
