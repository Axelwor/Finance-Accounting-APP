package assets

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/db"
)

// US-060..063: Fixed Assets (Aset Tetap).
//
// Asset registration posts Dr 1401 Fixed Assets / Cr Cash (or AP).
// Depreciation, revaluation, disposal, and impairment each post their own
// journal entry following the same hash-chain + outbox + idempotency pattern
// as purchase (see internal/purchase/grn.go).

// Shared account codes (seeded by migration 000019 / auth.seedDefaultCOA).
const (
	fixedAssetAccountCode  = "1401" // Fixed Assets
	accumDepAccountCode    = "1402" // Accumulated Depreciation
	depExpenseAccountCode  = "5206" // Depreciation Expense
	revaluationSurplusCode = "3401" // Revaluation Surplus (OCI)
	gainOnDisposalCode     = "4903" // Gain on Asset Disposal
	lossOnDisposalCode     = "5903" // Loss on Asset Disposal
	impairmentAccountCode  = "5207" // Impairment Loss
	impairmentLossCode     = "5207" // Impairment Loss (alias for clarity in impairment handler)
	cashAccountCode        = "1101" // Cash (default acquisition/disposal counter)
)

// Asset lifecycle statuses (stored in fixed_assets.status).
const (
	statusActive   = "ACTIVE"
	statusDisposed = "DISPOSED"
	statusImpaired = "IMPAIRED"
)

// Asset transaction types (stored in asset_transactions.tx_type).
const (
	txTypeAcquisition  = "ACQUISITION"
	txTypeDepreciation = "DEPRECIATION"
	txTypeRevaluation  = "REVALUATION"
	txTypeDisposal     = "DISPOSAL"
	txTypeImpairment   = "IMPAIRMENT"
)

// Intent types for fixed-asset journal entries.
const (
	intentAssetAcquisition  = "ASSET_ACQUISITION"
	intentAssetDepreciation = "ASSET_DEPRECIATION"
	intentAssetRevaluation  = "ASSET_REVALUATION"
	intentAssetDisposal     = "ASSET_DISPOSAL"
	intentAssetImpairment   = "ASSET_IMPAIRMENT"
)

// Depreciation methods (stored in fixed_assets.depreciation_method).
const (
	methodStraightLine      = "straight_line"
	methodDecliningBalance  = "declining_balance"
	methodUnitsOfProduction = "units_of_production"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Post("/fixed-assets", service.RegisterAsset)
	router.Get("/fixed-assets", service.ListAssets)
	router.Get("/fixed-assets/{id}", service.GetAsset)

	router.Get("/assets/register", service.AssetRegister)

	router.Post("/fixed-assets/{id}/depreciate", service.DepreciateAsset)
	router.Post("/fixed-assets/{id}/revalue", service.RevalueAsset)
	router.Post("/fixed-assets/{id}/dispose", service.DisposeAsset)
	router.Post("/fixed-assets/{id}/impair", service.ImpairAsset)

	// F-13: Asset maintenance tracking.
	router.Post("/asset-maintenance", service.CreateMaintenance)
	router.Get("/asset-maintenance", service.ListMaintenance)
	router.Get("/asset-maintenance/upcoming", service.UpcomingMaintenance)
}

// ---------------------------------------------------------------------------
// Asset Registration (US-060)
// ---------------------------------------------------------------------------

type RegisterAssetRequest struct {
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	AcquisitionDate      string `json:"acquisition_date"`
	AcquisitionCostCents int64  `json:"acquisition_cost_cents"`
	SalvageValueCents    int64  `json:"salvage_value_cents"`
	UsefulLifeMonths     int    `json:"useful_life_months"`
	DepreciationMethod   string `json:"depreciation_method"`
	Rate                 string `json:"rate"`
	UnitsTotal           int64  `json:"units_total"`
	PaymentAccountCode   string `json:"payment_account_code"`
	Notes                string `json:"notes"`
}

type assetScheduleResponse struct {
	ID                int64  `json:"id"`
	PeriodYear        int    `json:"period_year"`
	PeriodMonth       int    `json:"period_month"`
	DepreciationCents int64  `json:"depreciation_cents"`
	JournalEntryID    int64  `json:"journal_entry_id,omitempty"`
	Posted            bool   `json:"posted"`
	PostedAt          string `json:"posted_at,omitempty"`
}

type assetTransactionResponse struct {
	ID             int64  `json:"id"`
	TxType         string `json:"tx_type"`
	TxDate         string `json:"tx_date"`
	AmountCents    int64  `json:"amount_cents"`
	JournalEntryID int64  `json:"journal_entry_id,omitempty"`
	Description    string `json:"description,omitempty"`
}

type assetResponse struct {
	ID                   int64                      `json:"id"`
	Code                 string                     `json:"code"`
	Name                 string                     `json:"name"`
	AssetAccountID       int64                      `json:"asset_account_id"`
	AccumDepAccountID    int64                      `json:"accum_dep_account_id"`
	DepExpenseAccountID  int64                      `json:"dep_expense_account_id"`
	ImpairmentAccountID  int64                      `json:"impairment_account_id,omitempty"`
	AcquisitionDate      string                     `json:"acquisition_date"`
	AcquisitionCostCents int64                      `json:"acquisition_cost_cents"`
	SalvageValueCents    int64                      `json:"salvage_value_cents"`
	UsefulLifeMonths     int                        `json:"useful_life_months"`
	DepreciationMethod   string                     `json:"depreciation_method"`
	Rate                 string                     `json:"rate"`
	UnitsTotal           int64                      `json:"units_total,omitempty"`
	UnitsUsed            int64                      `json:"units_used"`
	Status               string                     `json:"status"`
	BookValueCents       int64                      `json:"book_value_cents"`
	AccumDepCents        int64                      `json:"accum_dep_cents"`
	JournalEntryID       int64                      `json:"journal_entry_id,omitempty"`
	Schedule             []assetScheduleResponse    `json:"schedule,omitempty"`
	Transactions         []assetTransactionResponse `json:"transactions,omitempty"`
}

func (service *Service) RegisterAsset(writer http.ResponseWriter, request *http.Request) {
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
	var req RegisterAssetRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateRegisterRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	uid := userID(request)

	var result assetResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}

		// Idempotent replay: if a journal already exists for this key, find the
		// asset that references it and return it.
		existing, err := db.New(tx).GetJournalByIdempotencyKey(request.Context(), db.GetJournalByIdempotencyKeyParams{
			TenantID:       tenant,
			IdempotencyKey: uuidValue(idem),
		})
		if err == nil {
			asset, err := fetchAssetByJournal(request.Context(), tx, tenant, existing.ID)
			if err != nil {
				return err
			}
			result = *asset
			return nil
		} else if !isNoRows(err) {
			return err
		}

		// Resolve accounts.
		assetAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, fixedAssetAccountCode)
		if err != nil {
			return err
		}
		accumDepAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, accumDepAccountCode)
		if err != nil {
			return err
		}
		depExpenseAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, depExpenseAccountCode)
		if err != nil {
			return err
		}

		paymentCode := cashAccountCode
		if req.PaymentAccountCode != "" {
			paymentCode = req.PaymentAccountCode
		}
		paymentAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, paymentCode)
		if err != nil {
			return err
		}

		// Build journal: Dr 1401 Fixed Assets / Cr Cash (or AP).
		journalLines := []accounting.Line{
			{AccountID: assetAcctID, DebitCents: req.AcquisitionCostCents, SourceLineRef: "asset"},
			{AccountID: paymentAcctID, CreditCents: req.AcquisitionCostCents, SourceLineRef: "payment"},
		}
		if err := accounting.BalanceCheck(journalLines); err != nil {
			return err
		}

		sourceRef := fmt.Sprintf("FA-ACQ-%s", req.Code)
		journal := accounting.Journal{
			TenantID:    tenant,
			SourceRef:   sourceRef,
			IntentType:  accounting.IntentType("ASSET_ACQUISITION"),
			EntryDate:   req.AcquisitionDate,
			Description: fmt.Sprintf("Acquisition: %s (%s)", req.Name, req.Code),
			Lines:       journalLines,
		}
		head, err := lockOrSeedHead(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		journal.PreviousHash = head.LastHash
		journal.Hash = hashJournal(journal)

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
		if err := insertOutbox(request.Context(), tx, tenant, "asset.acquired", mustJSON(map[string]any{
			"journal_id": entryID, "code": req.Code,
		})); err != nil {
			return err
		}

		// Allocate asset number.
		assetNumber, err := nextDocNumber(request.Context(), tx, tenant, "FA", "FA")
		if err != nil {
			return err
		}
		acqDate, _ := parseDate(req.AcquisitionDate)

		rateValue := pgtype.Numeric{}
		if req.Rate != "" {
			_ = rateValue.Scan(req.Rate)
		}
		var unitsTotal pgtype.Int8
		if req.UnitsTotal > 0 {
			unitsTotal = pgtype.Int8{Int64: req.UnitsTotal, Valid: true}
		}

		var assetID int64
		err = tx.QueryRow(request.Context(), `
			INSERT INTO fixed_assets (
				tenant_id, code, name, asset_account_id, accum_dep_account_id, dep_expense_account_id,
				acquisition_date, acquisition_cost_cents, salvage_value_cents, useful_life_months,
				depreciation_method, rate, units_total, book_value_cents, accum_dep_cents, status, journal_entry_id, created_by
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 0, 'ACTIVE', $15, $16)
			RETURNING id
		`, tenant, assetNumber, req.Name, assetAcctID, accumDepAcctID, depExpenseAcctID,
			acqDate, req.AcquisitionCostCents, req.SalvageValueCents, req.UsefulLifeMonths,
			req.DepreciationMethod, rateValue, unitsTotal, req.AcquisitionCostCents,
			entryID, int8Value(uid)).Scan(&assetID)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("asset code %s already exists", req.Code)
			}
			return err
		}

		// Record acquisition transaction.
		if _, err := tx.Exec(request.Context(), `
			INSERT INTO asset_transactions (tenant_id, asset_id, tx_type, tx_date, amount_cents, journal_entry_id, description)
			VALUES ($1, $2, 'ACQUISITION', $3, $4, $5, $6)
		`, tenant, assetID, acqDate, req.AcquisitionCostCents, entryID, textValueOptional(req.Notes)); err != nil {
			return err
		}

		result = assetResponse{
			ID:                   assetID,
			Code:                 assetNumber,
			Name:                 req.Name,
			AssetAccountID:       assetAcctID,
			AccumDepAccountID:    accumDepAcctID,
			DepExpenseAccountID:  depExpenseAcctID,
			AcquisitionDate:      req.AcquisitionDate,
			AcquisitionCostCents: req.AcquisitionCostCents,
			SalvageValueCents:    req.SalvageValueCents,
			UsefulLifeMonths:     req.UsefulLifeMonths,
			DepreciationMethod:   req.DepreciationMethod,
			Rate:                 req.Rate,
			UnitsTotal:           req.UnitsTotal,
			UnitsUsed:            0,
			Status:               "ACTIVE",
			BookValueCents:       req.AcquisitionCostCents,
			AccumDepCents:        0,
			JournalEntryID:       entryID,
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, "REGISTER_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func validateRegisterRequest(req RegisterAssetRequest) (string, string) {
	if req.Code == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if req.Name == "" {
		return "INVALID_REQUEST", "name is required"
	}
	if !validDate(req.AcquisitionDate) {
		return "INVALID_REQUEST", "acquisition_date is required (YYYY-MM-DD)"
	}
	if req.AcquisitionCostCents <= 0 {
		return "INVALID_REQUEST", "acquisition_cost_cents must be > 0"
	}
	if req.SalvageValueCents < 0 {
		return "INVALID_REQUEST", "salvage_value_cents must be >= 0"
	}
	if req.UsefulLifeMonths <= 0 {
		return "INVALID_REQUEST", "useful_life_months must be > 0"
	}
	switch req.DepreciationMethod {
	case "straight_line", "declining_balance", "units_of_production":
	default:
		return "INVALID_REQUEST", "depreciation_method must be straight_line, declining_balance, or units_of_production"
	}
	if req.DepreciationMethod == "declining_balance" && req.Rate == "" {
		return "INVALID_REQUEST", "rate is required for declining_balance method"
	}
	if req.DepreciationMethod == "units_of_production" && req.UnitsTotal <= 0 {
		return "INVALID_REQUEST", "units_total must be > 0 for units_of_production method"
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// List & Get (US-060)
// ---------------------------------------------------------------------------

func (service *Service) ListAssets(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	items := make([]assetResponse, 0)
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, code, name, acquisition_date, acquisition_cost_cents, salvage_value_cents,
			       useful_life_months, depreciation_method, status, book_value_cents, accum_dep_cents,
			       journal_entry_id
			FROM fixed_assets
			WHERE tenant_id = $1
			ORDER BY code
		`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a assetResponse
			var acqDate pgtype.Date
			var journalID pgtype.Int8
			if err := rows.Scan(&a.ID, &a.Code, &a.Name, &acqDate, &a.AcquisitionCostCents,
				&a.SalvageValueCents, &a.UsefulLifeMonths, &a.DepreciationMethod, &a.Status,
				&a.BookValueCents, &a.AccumDepCents, &journalID); err != nil {
				return err
			}
			a.AcquisitionDate = dateString(acqDate)
			a.JournalEntryID = int8ValueRaw(journalID)
			items = append(items, a)
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

func (service *Service) GetAsset(writer http.ResponseWriter, request *http.Request) {
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

	var asset *assetResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		var err error
		asset, err = fetchAssetByID(request.Context(), tx, tenant, id)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	// Load schedule.
	schedule := make([]assetScheduleResponse, 0)
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		schedRows, err := tx.Query(request.Context(), `
			SELECT id, period_year, period_month, depreciation_cents, journal_entry_id, posted, posted_at
			FROM asset_depreciation_schedule
			WHERE tenant_id = $1 AND asset_id = $2
			ORDER BY period_year, period_month
		`, tenant, id)
		if err != nil {
			return err
		}
		defer schedRows.Close()
		for schedRows.Next() {
			var s assetScheduleResponse
			var journalID pgtype.Int8
			var postedAt pgtype.Timestamptz
			if err := schedRows.Scan(&s.ID, &s.PeriodYear, &s.PeriodMonth, &s.DepreciationCents,
				&journalID, &s.Posted, &postedAt); err != nil {
				return err
			}
			s.JournalEntryID = int8ValueRaw(journalID)
			if postedAt.Valid {
				s.PostedAt = postedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			}
			schedule = append(schedule, s)
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LOAD_FAILED", err.Error())
		return
	}
	asset.Schedule = schedule

	// Load transactions.
	transactions := make([]assetTransactionResponse, 0)
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		txRows, err := tx.Query(request.Context(), `
			SELECT id, tx_type, tx_date, amount_cents, journal_entry_id, description
			FROM asset_transactions
			WHERE tenant_id = $1 AND asset_id = $2
			ORDER BY tx_date, id
		`, tenant, id)
		if err != nil {
			return err
		}
		defer txRows.Close()
		for txRows.Next() {
			var t assetTransactionResponse
			var txDate pgtype.Date
			var journalID pgtype.Int8
			var desc pgtype.Text
			if err := txRows.Scan(&t.ID, &t.TxType, &txDate, &t.AmountCents, &journalID, &desc); err != nil {
				return err
			}
			t.TxDate = dateString(txDate)
			t.JournalEntryID = int8ValueRaw(journalID)
			t.Description = textValue(desc)
			transactions = append(transactions, t)
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "LOAD_FAILED", err.Error())
		return
	}
	asset.Transactions = transactions

	writeJSON(writer, http.StatusOK, asset)
}

func fetchAssetByID(ctx context.Context, pool db.DBTX, tenant, id int64) (*assetResponse, error) {
	var a assetResponse
	var acqDate pgtype.Date
	var assetAcct, accumDepAcct, depExpAcct int64
	var impairmentAcct pgtype.Int8
	var rate pgtype.Numeric
	var unitsTotal pgtype.Int8
	var unitsUsed int64
	var journalID pgtype.Int8

	err := pool.QueryRow(ctx, `
		SELECT id, code, name, asset_account_id, accum_dep_account_id, dep_expense_account_id,
		       impairment_account_id, acquisition_date, acquisition_cost_cents, salvage_value_cents,
		       useful_life_months, depreciation_method, rate, units_total, units_used,
		       status, book_value_cents, accum_dep_cents, journal_entry_id
		FROM fixed_assets
		WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(&a.ID, &a.Code, &a.Name, &assetAcct, &accumDepAcct, &depExpAcct,
		&impairmentAcct, &acqDate, &a.AcquisitionCostCents, &a.SalvageValueCents,
		&a.UsefulLifeMonths, &a.DepreciationMethod, &rate, &unitsTotal, &unitsUsed,
		&a.Status, &a.BookValueCents, &a.AccumDepCents, &journalID)
	if err != nil {
		return nil, err
	}
	a.AssetAccountID = assetAcct
	a.AccumDepAccountID = accumDepAcct
	a.DepExpenseAccountID = depExpAcct
	a.ImpairmentAccountID = int8ValueRaw(impairmentAcct)
	a.AcquisitionDate = dateString(acqDate)
	a.Rate = numericToString(rate)
	if unitsTotal.Valid {
		a.UnitsTotal = unitsTotal.Int64
	}
	a.UnitsUsed = unitsUsed
	a.JournalEntryID = int8ValueRaw(journalID)
	return &a, nil
}

func fetchAssetByJournal(ctx context.Context, tx pgx.Tx, tenant, journalID int64) (*assetResponse, error) {
	var a assetResponse
	var acqDate pgtype.Date
	var assetAcct, accumDepAcct, depExpAcct int64
	var impairmentAcct pgtype.Int8
	var rate pgtype.Numeric
	var unitsTotal pgtype.Int8
	var unitsUsed int64
	var journalIDOut pgtype.Int8

	err := tx.QueryRow(ctx, `
		SELECT id, code, name, asset_account_id, accum_dep_account_id, dep_expense_account_id,
		       impairment_account_id, acquisition_date, acquisition_cost_cents, salvage_value_cents,
		       useful_life_months, depreciation_method, rate, units_total, units_used,
		       status, book_value_cents, accum_dep_cents, journal_entry_id
		FROM fixed_assets
		WHERE tenant_id = $1 AND journal_entry_id = $2
	`, tenant, journalID).Scan(&a.ID, &a.Code, &a.Name, &assetAcct, &accumDepAcct, &depExpAcct,
		&impairmentAcct, &acqDate, &a.AcquisitionCostCents, &a.SalvageValueCents,
		&a.UsefulLifeMonths, &a.DepreciationMethod, &rate, &unitsTotal, &unitsUsed,
		&a.Status, &a.BookValueCents, &a.AccumDepCents, &journalIDOut)
	if err != nil {
		return nil, err
	}
	a.AssetAccountID = assetAcct
	a.AccumDepAccountID = accumDepAcct
	a.DepExpenseAccountID = depExpAcct
	a.ImpairmentAccountID = int8ValueRaw(impairmentAcct)
	a.AcquisitionDate = dateString(acqDate)
	a.Rate = numericToString(rate)
	if unitsTotal.Valid {
		a.UnitsTotal = unitsTotal.Int64
	}
	a.UnitsUsed = unitsUsed
	a.JournalEntryID = int8ValueRaw(journalIDOut)
	return &a, nil
}
