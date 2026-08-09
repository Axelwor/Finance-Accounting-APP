package assets

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// Depreciation (US-061)
// ---------------------------------------------------------------------------

type DepreciateAssetRequest struct {
	PeriodYear  int    `json:"period_year"`
	PeriodMonth int    `json:"period_month"`
	EntryDate   string `json:"entry_date"`
	Description string `json:"description"`
}

type DepreciationResultResponse struct {
	AssetID           int64  `json:"asset_id"`
	PeriodYear        int    `json:"period_year"`
	PeriodMonth       int    `json:"period_month"`
	DepreciationCents int64  `json:"depreciation_cents"`
	JournalEntryID    int64  `json:"journal_entry_id,omitempty"`
	ScheduleID        int64  `json:"schedule_id,omitempty"`
	BookValueCents    int64  `json:"book_value_cents"`
	AccumDepCents     int64  `json:"accum_dep_cents"`
	AlreadyPosted     bool   `json:"already_posted,omitempty"`
	Status            string `json:"status"`
}

func (service *Service) DepreciateAsset(writer http.ResponseWriter, request *http.Request) {
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
	assetID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "asset id is required")
		return
	}
	var req DepreciateAssetRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.PeriodYear < 2000 || req.PeriodYear > 2100 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "period_year is out of range")
		return
	}
	if req.PeriodMonth < 1 || req.PeriodMonth > 12 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "period_month must be 1-12")
		return
	}
	if !validDate(req.EntryDate) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "entry_date is required (YYYY-MM-DD)")
		return
	}
	uid := userID(request)

	var result DepreciationResultResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		// Idempotent replay: if this idempotency key already produced a journal,
		// return the existing schedule row that references it.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			row, ferr := fetchScheduleByJournal(request.Context(), tx, tenant, existing.ID)
			if ferr == nil {
				result = DepreciationResultResponse{
					AssetID: row.AssetID, PeriodYear: row.PeriodYear, PeriodMonth: row.PeriodMonth,
					DepreciationCents: row.DepreciationCents, JournalEntryID: existing.ID,
					ScheduleID: row.ID, AlreadyPosted: true,
				}
				return nil
			}
		} else if !isNoRows(err) {
			return err
		}

		// Load asset.
		asset, err := loadAssetForUpdate(request.Context(), tx, tenant, assetID)
		if err != nil {
			return err
		}
		if asset.Status != statusActive {
			return fmt.Errorf("asset %s is %s; only ACTIVE assets can be depreciated", asset.Code, asset.Status)
		}
		// Idempotent per period: unique (tenant_id, asset_id, period_year, period_month).
		var existingSched scheduleRow
		serr := fetchScheduleByPeriod(request.Context(), tx, tenant, assetID, req.PeriodYear, req.PeriodMonth, &existingSched)
		if serr == nil {
			// Already posted for this period — return it.
			result = DepreciationResultResponse{
				AssetID: assetID, PeriodYear: req.PeriodYear, PeriodMonth: req.PeriodMonth,
				DepreciationCents: existingSched.DepreciationCents,
				JournalEntryID:    existingSched.JournalEntryID,
				ScheduleID:        existingSched.ID, AlreadyPosted: true,
				BookValueCents: asset.BookValueCents, AccumDepCents: asset.AccumDepCents,
				Status: asset.Status,
			}
			return nil
		} else if !isNoRows(serr) {
			return serr
		}

		// Compute depreciation.
		depCents := computeDepreciation(asset)
		if depCents < 0 {
			depCents = 0
		}
		// Do not depreciate below salvage value.
		newBookValue := asset.BookValueCents - depCents
		if newBookValue < asset.SalvageValueCents {
			depCents = asset.BookValueCents - asset.SalvageValueCents
			if depCents < 0 {
				depCents = 0
			}
			newBookValue = asset.BookValueCents - depCents
		}
		if depCents == 0 {
			return fmt.Errorf("asset %s is fully depreciated (book value equals salvage value)", asset.Code)
		}

		// Build journal: Dr 5206 Depreciation Expense / Cr 1402 Accumulated Depreciation.
		journalLines := []accounting.Line{
			{AccountID: asset.DepExpenseAccountID, DebitCents: depCents, SourceLineRef: "dep-expense"},
			{AccountID: asset.AccumDepAccountID, CreditCents: depCents, SourceLineRef: "accum-dep"},
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}
		sourceRef := fmt.Sprintf("FA-DEP-%d-%04d%02d", assetID, req.PeriodYear, req.PeriodMonth)
		desc := req.Description
		if desc == "" {
			desc = fmt.Sprintf("Depreciation: %s (%s) %04d-%02d", asset.Code, asset.Name, req.PeriodYear, req.PeriodMonth)
		}
		journal := accounting.Journal{
			TenantID: tenant, SourceRef: sourceRef,
			IntentType: accounting.IntentType(intentAssetDepreciation),
			EntryDate:  req.EntryDate, Description: desc, Lines: journalLines,
		}
		entryID, err := postJournal(request.Context(), tx, tenant, journal, idem, uid)
		if err != nil {
			return err
		}

		newAccumDep := asset.AccumDepCents + depCents
		// Insert schedule row.
		var schedID int64
		if err := tx.QueryRow(request.Context(), `
			INSERT INTO asset_depreciation_schedule (tenant_id, asset_id, period_year, period_month, depreciation_cents, journal_entry_id, posted, posted_at)
			VALUES ($1, $2, $3, $4, $5, $6, true, now())
			RETURNING id
		`, tenant, assetID, req.PeriodYear, req.PeriodMonth, depCents, entryID).Scan(&schedID); err != nil {
			return err
		}
		// Update asset book value + accum dep.
		if _, err := tx.Exec(request.Context(), `
			UPDATE fixed_assets SET accum_dep_cents = $1, book_value_cents = $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, newAccumDep, newBookValue, tenant, assetID); err != nil {
			return err
		}
		// Record asset transaction.
		if err := insertAssetTransaction(request.Context(), tx, tenant, assetID, txTypeDepreciation, req.EntryDate, depCents, entryID, desc); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "asset.depreciation.posted", mustJSON(map[string]any{
			"journal_id": entryID, "asset_id": assetID, "period_year": req.PeriodYear, "period_month": req.PeriodMonth,
		})); err != nil {
			return err
		}
		result = DepreciationResultResponse{
			AssetID: assetID, PeriodYear: req.PeriodYear, PeriodMonth: req.PeriodMonth,
			DepreciationCents: depCents, JournalEntryID: entryID, ScheduleID: schedID,
			BookValueCents: newBookValue, AccumDepCents: newAccumDep, Status: asset.Status,
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			// Race: another request posted this period first.
			writeError(writer, http.StatusConflict, "ALREADY_POSTED", "depreciation for this period has already been posted")
			return
		}
		writeError(writer, http.StatusBadRequest, "DEPRECIATION_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Revaluation (US-062)
// ---------------------------------------------------------------------------

type RevalueAssetRequest struct {
	NewValueCents int64  `json:"new_value_cents"`
	EntryDate     string `json:"entry_date"`
	Description   string `json:"description"`
}

type RevaluationResultResponse struct {
	AssetID         int64  `json:"asset_id"`
	Code            string `json:"code"`
	AdjustmentCents int64  `json:"adjustment_cents"`
	Direction       string `json:"direction"`
	JournalEntryID  int64  `json:"journal_entry_id,omitempty"`
	BookValueCents  int64  `json:"book_value_cents"`
	Status          string `json:"status"`
}

func (service *Service) RevalueAsset(writer http.ResponseWriter, request *http.Request) {
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
	assetID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "asset id is required")
		return
	}
	var req RevalueAssetRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.NewValueCents <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "new_value_cents must be > 0")
		return
	}
	if !validDate(req.EntryDate) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "entry_date is required (YYYY-MM-DD)")
		return
	}
	uid := userID(request)

	var result RevaluationResultResponse
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
			txRow, ferr := fetchAssetTransactionByJournal(request.Context(), tx, tenant, existing.ID)
			if ferr == nil {
				result = RevaluationResultResponse{
					AssetID: assetID, AdjustmentCents: txRow.AmountCents, JournalEntryID: existing.ID,
				}
				return nil
			}
		} else if !isNoRows(err) {
			return err
		}

		asset, err := loadAssetForUpdate(request.Context(), tx, tenant, assetID)
		if err != nil {
			return err
		}
		if asset.Status == statusDisposed {
			return fmt.Errorf("asset %s is DISPOSED", asset.Code)
		}
		adjustment := req.NewValueCents - asset.BookValueCents
		if adjustment == 0 {
			return fmt.Errorf("new value equals current book value; no adjustment")
		}
		// Resolve OCI account 3401.
		ociAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, revaluationSurplusCode)
		if err != nil {
			return err
		}
		desc := req.Description
		if desc == "" {
			desc = fmt.Sprintf("Revaluation: %s (%s)", asset.Code, asset.Name)
		}
		var journalLines []accounting.Line
		var direction string
		if adjustment > 0 {
			// Up: Dr 1401 / Cr 3401 (OCI).
			direction = "UP"
			journalLines = []accounting.Line{
				{AccountID: asset.AssetAccountID, DebitCents: adjustment, SourceLineRef: "asset-up"},
				{AccountID: ociAccountID, CreditCents: adjustment, SourceLineRef: "oci-up"},
			}
		} else {
			// Down: Dr 3401 / Cr 1401.
			direction = "DOWN"
			absAdj := -adjustment
			journalLines = []accounting.Line{
				{AccountID: ociAccountID, DebitCents: absAdj, SourceLineRef: "oci-down"},
				{AccountID: asset.AssetAccountID, CreditCents: absAdj, SourceLineRef: "asset-down"},
			}
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}
		sourceRef := fmt.Sprintf("FA-REV-%d-%s", assetID, time.Now().Format("20060102"))
		journal := accounting.Journal{
			TenantID: tenant, SourceRef: sourceRef,
			IntentType: accounting.IntentType(intentAssetRevaluation),
			EntryDate:  req.EntryDate, Description: desc, Lines: journalLines,
		}
		entryID, err := postJournal(request.Context(), tx, tenant, journal, idem, uid)
		if err != nil {
			return err
		}
		newBookValue := req.NewValueCents
		// Revaluation adjusts the asset's recorded book value. We adjust
		// acquisition_cost_cents so that cost - accum_dep = new book value,
		// keeping accum dep unchanged (PSAK 16 revaluation model).
		newCost := newBookValue + asset.AccumDepCents
		if _, err := tx.Exec(request.Context(), `
			UPDATE fixed_assets SET acquisition_cost_cents = $1, book_value_cents = $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, newCost, newBookValue, tenant, assetID); err != nil {
			return err
		}
		absAmount := adjustment
		if absAmount < 0 {
			absAmount = -absAmount
		}
		if err := insertAssetTransaction(request.Context(), tx, tenant, assetID, txTypeRevaluation, req.EntryDate, absAmount, entryID, desc); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "asset.revaluation.posted", mustJSON(map[string]any{
			"journal_id": entryID, "asset_id": assetID, "direction": direction, "adjustment_cents": absAmount,
		})); err != nil {
			return err
		}
		result = RevaluationResultResponse{
			AssetID: assetID, Code: asset.Code, AdjustmentCents: absAmount,
			Direction: direction, JournalEntryID: entryID, BookValueCents: newBookValue, Status: asset.Status,
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "REVALUATION_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Disposal (US-062)
// ---------------------------------------------------------------------------

type DisposeAssetRequest struct {
	DisposalDate    string `json:"disposal_date"`
	ProceedsCents   int64  `json:"proceeds_cents"`
	CashAccountCode string `json:"cash_account_code"`
	Description     string `json:"description"`
}

type DisposalResultResponse struct {
	AssetID           int64  `json:"asset_id"`
	Code              string `json:"code"`
	ProceedsCents     int64  `json:"proceeds_cents"`
	BookValueCents    int64  `json:"book_value_cents"`
	AccumDepCents     int64  `json:"accum_dep_cents"`
	CostCents         int64  `json:"cost_cents"`
	GainLossCents     int64  `json:"gain_loss_cents"`
	GainLossDirection string `json:"gain_loss_direction"`
	JournalEntryID    int64  `json:"journal_entry_id,omitempty"`
	Status            string `json:"status"`
}

func (service *Service) DisposeAsset(writer http.ResponseWriter, request *http.Request) {
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
	assetID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "asset id is required")
		return
	}
	var req DisposeAssetRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if !validDate(req.DisposalDate) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "disposal_date is required (YYYY-MM-DD)")
		return
	}
	if req.ProceedsCents < 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "proceeds_cents must be >= 0")
		return
	}
	cashCode := req.CashAccountCode
	if cashCode == "" {
		cashCode = cashAccountCode // default 1101
	}
	uid := userID(request)

	var result DisposalResultResponse
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
			txRow, ferr := fetchAssetTransactionByJournal(request.Context(), tx, tenant, existing.ID)
			if ferr == nil {
				result = DisposalResultResponse{
					AssetID: assetID, ProceedsCents: txRow.AmountCents, JournalEntryID: existing.ID, Status: statusDisposed,
				}
				return nil
			}
		} else if !isNoRows(err) {
			return err
		}

		asset, err := loadAssetForUpdate(request.Context(), tx, tenant, assetID)
		if err != nil {
			return err
		}
		if asset.Status == statusDisposed {
			return fmt.Errorf("asset %s is already DISPOSED", asset.Code)
		}
		// Resolve accounts.
		cashAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, cashCode)
		if err != nil {
			return err
		}
		gainAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, gainOnDisposalCode)
		if err != nil {
			return err
		}
		lossAccountID, err := resolveAccountByCode(request.Context(), tx, tenant, lossOnDisposalCode)
		if err != nil {
			return err
		}
		bookValue := asset.BookValueCents
		// Gain/loss = proceeds - book value.
		gainLoss := req.ProceedsCents - bookValue
		desc := req.Description
		if desc == "" {
			desc = fmt.Sprintf("Disposal: %s (%s)", asset.Code, asset.Name)
		}
		// Journal:
		//   Dr Cash (proceeds)
		//   Dr 1402 Accum Dep (accum dep)
		//   Dr 5903 Loss on Disposal (if loss)
		//   Cr 1401 Fixed Asset (cost)
		//   Cr 4903 Gain on Disposal (if gain)
		journalLines := []accounting.Line{
			{AccountID: cashAccountID, DebitCents: req.ProceedsCents, SourceLineRef: "cash-proceeds"},
			{AccountID: asset.AccumDepAccountID, DebitCents: asset.AccumDepCents, SourceLineRef: "accum-dep-remove"},
			{AccountID: asset.AssetAccountID, CreditCents: asset.AcquisitionCostCents, SourceLineRef: "asset-cost-remove"},
		}
		direction := "NONE"
		if gainLoss > 0 {
			direction = "GAIN"
			journalLines = append(journalLines, accounting.Line{
				AccountID: gainAccountID, CreditCents: gainLoss, SourceLineRef: "gain-disposal",
			})
		} else if gainLoss < 0 {
			direction = "LOSS"
			absLoss := -gainLoss
			journalLines = append(journalLines, accounting.Line{
				AccountID: lossAccountID, DebitCents: absLoss, SourceLineRef: "loss-disposal",
			})
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}
		sourceRef := fmt.Sprintf("FA-DISP-%d-%s", assetID, time.Now().Format("20060102"))
		journal := accounting.Journal{
			TenantID: tenant, SourceRef: sourceRef,
			IntentType: accounting.IntentType(intentAssetDisposal),
			EntryDate:  req.DisposalDate, Description: desc, Lines: journalLines,
		}
		entryID, err := postJournal(request.Context(), tx, tenant, journal, idem, uid)
		if err != nil {
			return err
		}
		// Mark asset disposed: book value and accum dep go to 0.
		if _, err := tx.Exec(request.Context(), `
			UPDATE fixed_assets SET status = $1, book_value_cents = 0, accum_dep_cents = 0, journal_entry_id = $2, updated_at = now()
			WHERE tenant_id = $3 AND id = $4
		`, statusDisposed, entryID, tenant, assetID); err != nil {
			return err
		}
		amount := req.ProceedsCents
		if amount == 0 {
			amount = bookValue
		}
		if err := insertAssetTransaction(request.Context(), tx, tenant, assetID, txTypeDisposal, req.DisposalDate, amount, entryID, desc); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "asset.disposal.posted", mustJSON(map[string]any{
			"journal_id": entryID, "asset_id": assetID, "proceeds_cents": req.ProceedsCents,
			"gain_loss_cents": gainLoss, "direction": direction,
		})); err != nil {
			return err
		}
		result = DisposalResultResponse{
			AssetID: assetID, Code: asset.Code, ProceedsCents: req.ProceedsCents,
			BookValueCents: bookValue, AccumDepCents: asset.AccumDepCents,
			CostCents: asset.AcquisitionCostCents, GainLossCents: gainLoss,
			GainLossDirection: direction, JournalEntryID: entryID, Status: statusDisposed,
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "DISPOSAL_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Impairment (US-063)
// ---------------------------------------------------------------------------

type ImpairAssetRequest struct {
	ImpairedValueCents int64  `json:"impaired_value_cents"`
	EntryDate          string `json:"entry_date"`
	Description        string `json:"description"`
}

type ImpairmentResultResponse struct {
	AssetID             int64  `json:"asset_id"`
	Code                string `json:"code"`
	ImpairmentLossCents int64  `json:"impairment_loss_cents"`
	NewBookValueCents   int64  `json:"new_book_value_cents"`
	JournalEntryID      int64  `json:"journal_entry_id,omitempty"`
	Status              string `json:"status"`
}

func (service *Service) ImpairAsset(writer http.ResponseWriter, request *http.Request) {
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
	assetID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "asset id is required")
		return
	}
	var req ImpairAssetRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.ImpairedValueCents < 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "impaired_value_cents must be >= 0")
		return
	}
	if !validDate(req.EntryDate) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "entry_date is required (YYYY-MM-DD)")
		return
	}
	uid := userID(request)

	var result ImpairmentResultResponse
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
			txRow, ferr := fetchAssetTransactionByJournal(request.Context(), tx, tenant, existing.ID)
			if ferr == nil {
				result = ImpairmentResultResponse{
					AssetID: assetID, ImpairmentLossCents: txRow.AmountCents, JournalEntryID: existing.ID, Status: statusImpaired,
				}
				return nil
			}
		} else if !isNoRows(err) {
			return err
		}

		asset, err := loadAssetForUpdate(request.Context(), tx, tenant, assetID)
		if err != nil {
			return err
		}
		if asset.Status == statusDisposed {
			return fmt.Errorf("asset %s is DISPOSED", asset.Code)
		}
		if req.ImpairedValueCents >= asset.BookValueCents {
			return fmt.Errorf("impaired value %d must be below current book value %d", req.ImpairedValueCents, asset.BookValueCents)
		}
		loss := asset.BookValueCents - req.ImpairedValueCents
		if loss <= 0 {
			return fmt.Errorf("impairment loss must be > 0")
		}
		// Resolve impairment account 5207 (if asset has one set, prefer it; else 5207).
		impairAccountID := asset.ImpairmentAccountID
		if impairAccountID == 0 {
			impairAccountID, err = resolveAccountByCode(request.Context(), tx, tenant, impairmentLossCode)
			if err != nil {
				return err
			}
		}
		desc := req.Description
		if desc == "" {
			desc = fmt.Sprintf("Impairment: %s (%s)", asset.Code, asset.Name)
		}
		// Journal: Dr 5207 Impairment Loss / Cr 1401 Fixed Asset.
		journalLines := []accounting.Line{
			{AccountID: impairAccountID, DebitCents: loss, SourceLineRef: "impairment-loss"},
			{AccountID: asset.AssetAccountID, CreditCents: loss, SourceLineRef: "asset-impair"},
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}
		sourceRef := fmt.Sprintf("FA-IMPAIR-%d-%s", assetID, time.Now().Format("20060102"))
		journal := accounting.Journal{
			TenantID: tenant, SourceRef: sourceRef,
			IntentType: accounting.IntentType(intentAssetImpairment),
			EntryDate:  req.EntryDate, Description: desc, Lines: journalLines,
		}
		entryID, err := postJournal(request.Context(), tx, tenant, journal, idem, uid)
		if err != nil {
			return err
		}
		// Reduce asset cost so book value = impaired value; accum dep unchanged.
		newCost := asset.AcquisitionCostCents - loss
		if _, err := tx.Exec(request.Context(), `
			UPDATE fixed_assets SET status = $1, acquisition_cost_cents = $2, book_value_cents = $3, journal_entry_id = $4, updated_at = now()
			WHERE tenant_id = $5 AND id = $6
		`, statusImpaired, newCost, req.ImpairedValueCents, entryID, tenant, assetID); err != nil {
			return err
		}
		if err := insertAssetTransaction(request.Context(), tx, tenant, assetID, txTypeImpairment, req.EntryDate, loss, entryID, desc); err != nil {
			return err
		}
		if err := insertOutbox(request.Context(), tx, tenant, "asset.impairment.posted", mustJSON(map[string]any{
			"journal_id": entryID, "asset_id": assetID, "loss_cents": loss,
		})); err != nil {
			return err
		}
		result = ImpairmentResultResponse{
			AssetID: assetID, Code: asset.Code, ImpairmentLossCents: loss,
			NewBookValueCents: req.ImpairedValueCents, JournalEntryID: entryID, Status: statusImpaired,
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "IMPAIRMENT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Shared journal-posting helper (hash-chain, idempotency, outbox)
// ---------------------------------------------------------------------------

func postJournal(ctx context.Context, tx pgx.Tx, tenantID int64, journal accounting.Journal, idem string, uid int64) (int64, error) {
	head, err := lockOrSeedHead(ctx, tx, tenantID)
	if err != nil {
		return 0, err
	}
	journal.PreviousHash = head.LastHash
	journal.Hash = hashJournal(journal)

	periodID, err := resolvePeriod(ctx, tx, tenantID, journal.EntryDate)
	if err != nil {
		return 0, err
	}
	jrnNumber, err := nextJournalNumber(ctx, tx, tenantID)
	if err != nil {
		return 0, err
	}
	var entryID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO journal_entries (tenant_id, number, entry_date, period_id, description, source_ref, intent_type, idempotency_key, hash, prev_hash, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, tenantID, jrnNumber, journal.EntryDate, periodID, journal.Description,
		journal.SourceRef, string(journal.IntentType), idem,
		journal.Hash, journal.PreviousHash, int8Value(uid)).Scan(&entryID)
	if err != nil {
		return 0, err
	}
	for _, line := range journal.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO journal_lines (tenant_id, entry_id, account_id, debit_cents, credit_cents, source_line_ref)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, tenantID, entryID, line.AccountID, line.DebitCents, line.CreditCents, line.SourceLineRef); err != nil {
			return 0, err
		}
	}
	if err := upsertHead(ctx, tx, tenantID, entryID, journal.Hash); err != nil {
		return 0, err
	}
	return entryID, nil
}

// ---------------------------------------------------------------------------
// Asset / schedule / transaction fetch helpers
// ---------------------------------------------------------------------------

type assetRow struct {
	ID                   int64
	TenantID             int64
	Code                 string
	Name                 string
	AssetAccountID       int64
	AccumDepAccountID    int64
	DepExpenseAccountID  int64
	ImpairmentAccountID  int64
	AcquisitionDate      string
	AcquisitionCostCents int64
	SalvageValueCents    int64
	UsefulLifeMonths     int
	DepreciationMethod   string
	Rate                 float64
	UnitsTotal           int64
	UnitsUsed            int64
	Status               string
	BookValueCents       int64
	AccumDepCents        int64
	JournalEntryID       int64
}

func loadAssetForUpdate(ctx context.Context, tx pgx.Tx, tenantID, assetID int64) (assetRow, error) {
	var a assetRow
	var impairmentAcct pgtype.Int8
	var journalID pgtype.Int8
	var rate pgtype.Numeric
	var unitsTotal pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, code, name, asset_account_id, accum_dep_account_id, dep_expense_account_id,
		       impairment_account_id, acquisition_date::text, acquisition_cost_cents, salvage_value_cents,
		       useful_life_months, depreciation_method, rate, units_total, units_used, status,
		       book_value_cents, accum_dep_cents, journal_entry_id
		FROM fixed_assets
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE
	`, tenantID, assetID).Scan(
		&a.ID, &a.TenantID, &a.Code, &a.Name, &a.AssetAccountID, &a.AccumDepAccountID, &a.DepExpenseAccountID,
		&impairmentAcct, &a.AcquisitionDate, &a.AcquisitionCostCents, &a.SalvageValueCents,
		&a.UsefulLifeMonths, &a.DepreciationMethod, &rate, &unitsTotal, &a.UnitsUsed, &a.Status,
		&a.BookValueCents, &a.AccumDepCents, &journalID,
	)
	if err != nil {
		return assetRow{}, fmt.Errorf("asset not found: %w", err)
	}
	if impairmentAcct.Valid {
		a.ImpairmentAccountID = impairmentAcct.Int64
	}
	if journalID.Valid {
		a.JournalEntryID = journalID.Int64
	}
	a.Rate = numericToFloat(rate)
	if unitsTotal.Valid {
		a.UnitsTotal = unitsTotal.Int64
	}
	return a, nil
}

type scheduleRow struct {
	ID                int64
	AssetID           int64
	PeriodYear        int
	PeriodMonth       int
	DepreciationCents int64
	JournalEntryID    int64
	Posted            bool
}

func fetchScheduleByPeriod(ctx context.Context, tx pgx.Tx, tenantID, assetID int64, year, month int, out *scheduleRow) error {
	var journalID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT id, asset_id, period_year, period_month, depreciation_cents, journal_entry_id, posted
		FROM asset_depreciation_schedule
		WHERE tenant_id = $1 AND asset_id = $2 AND period_year = $3 AND period_month = $4
	`, tenantID, assetID, year, month).Scan(&out.ID, &out.AssetID, &out.PeriodYear, &out.PeriodMonth, &out.DepreciationCents, &journalID, &out.Posted)
	if err != nil {
		return err
	}
	if journalID.Valid {
		out.JournalEntryID = journalID.Int64
	}
	return nil
}

func fetchScheduleByJournal(ctx context.Context, tx pgx.Tx, tenantID, journalID int64) (scheduleRow, error) {
	var row scheduleRow
	var jID pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT id, asset_id, period_year, period_month, depreciation_cents, journal_entry_id, posted
		FROM asset_depreciation_schedule
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenantID, journalID).Scan(&row.ID, &row.AssetID, &row.PeriodYear, &row.PeriodMonth, &row.DepreciationCents, &jID, &row.Posted)
	if err != nil {
		return scheduleRow{}, err
	}
	if jID.Valid {
		row.JournalEntryID = jID.Int64
	}
	return row, nil
}

type assetTransactionRow struct {
	ID             int64
	AssetID        int64
	TxType         string
	TxDate         string
	AmountCents    int64
	JournalEntryID int64
	Description    string
}

func fetchAssetTransactionByJournal(ctx context.Context, tx pgx.Tx, tenantID, journalID int64) (assetTransactionRow, error) {
	var row assetTransactionRow
	var journalID2 pgtype.Int8
	var desc pgtype.Text
	err := tx.QueryRow(ctx, `
		SELECT id, asset_id, tx_type, tx_date::text, amount_cents, journal_entry_id, description
		FROM asset_transactions
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenantID, journalID).Scan(&row.ID, &row.AssetID, &row.TxType, &row.TxDate, &row.AmountCents, &journalID2, &desc)
	if err != nil {
		return assetTransactionRow{}, err
	}
	if journalID2.Valid {
		row.JournalEntryID = journalID2.Int64
	}
	row.Description = textValue(desc)
	return row, nil
}

func insertAssetTransaction(ctx context.Context, tx pgx.Tx, tenantID, assetID int64, txType, txDate string, amount, journalID int64, description string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO asset_transactions (tenant_id, asset_id, tx_type, tx_date, amount_cents, journal_entry_id, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, tenantID, assetID, txType, txDate, amount, int8Value(journalID), textValueOptional(description))
	return err
}

// computeDepreciation returns the depreciation amount for one period.
func computeDepreciation(a assetRow) int64 {
	switch a.DepreciationMethod {
	case methodStraightLine:
		// (cost - salvage) / useful_life_months
		depreciableBase := a.AcquisitionCostCents - a.SalvageValueCents
		if depreciableBase <= 0 || a.UsefulLifeMonths <= 0 {
			return 0
		}
		return depreciableBase / int64(a.UsefulLifeMonths)
	case methodDecliningBalance:
		// book_value * rate
		return int64(float64(a.BookValueCents) * a.Rate)
	case methodUnitsOfProduction:
		// (cost - salvage) / units_total * units_used this period.
		// For simplicity, units_used is treated as the period consumption.
		if a.UnitsTotal <= 0 {
			return 0
		}
		depreciableBase := a.AcquisitionCostCents - a.SalvageValueCents
		return (depreciableBase * a.UnitsUsed) / a.UnitsTotal
	default:
		return 0
	}
}
