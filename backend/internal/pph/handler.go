package pph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// F-12: PPh (Pajak Penghasilan) — Withholding Tax Management
//   Supports PPh 21 (employee income tax), PPh 22 (import/procurement),
//   PPh 23 (service/rent/royalty), PPh 26 (non-resident), PPh Final UMKM.
//   Each PPh type has its own rate and payable account.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// PPh rates (per Indonesian tax law as of 2026)
const (
	RatePPh21NonNPWP    = 0.20   // 20% for non-NPWP
	RatePPh22Import     = 0.025  // 2.5%
	RatePPh23Service    = 0.02   // 2%
	RatePPh23Rent       = 0.10   // 10%
	RatePPh23Royalty    = 0.15   // 15%
	RatePPh26NonRes     = 0.20   // 20%
	RatePPhFinalUMKM    = 0.005  // 0.5%
	RatePPhFinalUMKM075 = 0.0075 // 0.75%
)

// PPh payable account codes
const (
	AccountPPh21     = "2107"
	AccountPPh22     = "2108"
	AccountPPh23     = "2109"
	AccountPPh26     = "2110"
	AccountPPhUMKM   = "2111"
	AccountIncomeTax = "5203"
)

type CreatePPhRequest struct {
	PphType         string  `json:"pph_type"` // PPH21, PPH22, PPH23, PPH26, PPH_FINAL_UMKM
	CalculationDate string  `json:"calculation_date"`
	DppCents        int64   `json:"dpp_cents"` // Dasar Pengenaan Pajak (taxable base)
	RatePercent     float64 `json:"rate_percent"`
	EntityName      string  `json:"entity_name"`
	EntityNPWP      string  `json:"entity_npwp"`
	Description     string  `json:"description"`
}

type PPhResponse struct {
	ID              int64   `json:"id"`
	PphType         string  `json:"pph_type"`
	CalculationDate string  `json:"calculation_date"`
	DppCents        int64   `json:"dpp_cents"`
	RatePercent     float64 `json:"rate_percent"`
	PphCents        int64   `json:"pph_cents"`
	EntityName      string  `json:"entity_name"`
	EntityNPWP      string  `json:"entity_npwp"`
	Description     string  `json:"description"`
	Status          string  `json:"status"`
}

func (s *Service) Routes(r chi.Router) {
	r.Post("/pph", s.Create)
	r.Get("/pph", s.List)
	r.Get("/pph/{id}", s.Get)
	r.Post("/pph/{id}/post", s.Post)
	r.Get("/pph/rates", s.GetRates)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())
	var req CreatePPhRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validatePPh(req); code != "" {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}

	// Calculate PPh
	pphCents := calculatePPh(req.DppCents, req.RatePercent)

	// Generate number
	year := time.Now().Year()
	if req.CalculationDate != "" {
		if parsed, err := time.Parse("2006-01-02", req.CalculationDate); err == nil {
			year = parsed.Year()
		}
	}

	var resp PPhResponse
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tid, 10)); err != nil {
			return err
		}
		var seq int64
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
			VALUES ($1, 'BUPOT', 'BUPOT', $2, 1)
			ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
			SET last_seq = document_numbering.last_seq + 1
			RETURNING last_seq
		`, tid, year).Scan(&seq); err != nil {
			return err
		}

		if err := tx.QueryRow(r.Context(), `
			INSERT INTO pph_calculations
		    (tenant_id, pph_type, calculation_date, dpp_cents, rate_percent, pph_cents,
		     entity_name, entity_npwp, description, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'DRAFT', $10)
			RETURNING id, pph_type, calculation_date, dpp_cents, rate_percent, pph_cents,
			          entity_name, entity_npwp, description, status
		`, tid, strings.ToUpper(req.PphType), req.CalculationDate, req.DppCents, req.RatePercent,
			pphCents, req.EntityName, req.EntityNPWP, req.Description, uid).Scan(
			&resp.ID, &resp.PphType, &resp.CalculationDate, &resp.DppCents, &resp.RatePercent,
			&resp.PphCents, &resp.EntityName, &resp.EntityNPWP, &resp.Description, &resp.Status); err != nil {
			return err
		}
		resp.Status = "POSTED"
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusConflict, "CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	pphType := r.URL.Query().Get("pph_type")
	args := []any{tid}
	query := `
		SELECT id, pph_type, calculation_date, dpp_cents, rate_percent, pph_cents,
		       entity_name, entity_npwp, description, status
		FROM pph_calculations WHERE tenant_id = $1`
	if pphType != "" {
		query += ` AND pph_type = $2`
		args = append(args, strings.ToUpper(pphType))
	}
	query += ` ORDER BY calculation_date DESC LIMIT 100`

	var results []PPhResponse
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tid, 10)); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var resp PPhResponse
			if err := rows.Scan(&resp.ID, &resp.PphType, &resp.CalculationDate, &resp.DppCents,
				&resp.RatePercent, &resp.PphCents, &resp.EntityName, &resp.EntityNPWP,
				&resp.Description, &resp.Status); err != nil {
				return err
			}
			results = append(results, resp)
		}
		return rows.Err()
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if results == nil {
		results = []PPhResponse{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	var resp PPhResponse
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tid, 10)); err != nil {
			return err
		}
		return tx.QueryRow(r.Context(), `
			SELECT id, pph_type, calculation_date, dpp_cents, rate_percent, pph_cents,
			       entity_name, entity_npwp, description, status
			FROM pph_calculations WHERE tenant_id = $1 AND id = $2
		`, tid, id).Scan(&resp.ID, &resp.PphType, &resp.CalculationDate, &resp.DppCents,
			&resp.RatePercent, &resp.PphCents, &resp.EntityName, &resp.EntityNPWP,
			&resp.Description, &resp.Status)
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "PPh calculation not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) Post(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeErr(w, http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required")
		return
	}
	idem, err := idempotencyKey(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id := pathID(chi.URLParam(r, "id"))
	if id <= 0 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	uid, _ := auth.UserIDFromContext(r.Context())

	var resp PPhResponse
	var journalID int64
	err = db.WithTransaction(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := withTenant(r.Context(), tx, tid); err != nil {
			return err
		}

		// Load the PPh calculation (type, dpp, rate, pph_cents, status,
		// description, and any existing journal_entry_id for idempotent
		// replay).
		var pphType, calcDate, entityName, entityNPWP, description, status string
		var dppCents, pphCents int64
		var ratePercent float64
		var existingJournalID pgtype.Int8
		err := tx.QueryRow(r.Context(), `
			SELECT pph_type, calculation_date::text, dpp_cents, rate_percent, pph_cents,
			       COALESCE(entity_name,''), COALESCE(entity_npwp,''), COALESCE(description,''),
			       status, journal_entry_id
			FROM pph_calculations WHERE tenant_id = $1 AND id = $2
		`, tid, id).Scan(&pphType, &calcDate, &dppCents, &ratePercent, &pphCents,
			&entityName, &entityNPWP, &description, &status, &existingJournalID)
		if err != nil {
			return fmt.Errorf("PPh calculation not found: %w", err)
		}

		// Idempotent: already posted — return the stored state.
		if status == "POSTED" {
			resp = PPhResponse{
				ID: id, PphType: pphType, CalculationDate: calcDate, DppCents: dppCents,
				RatePercent: ratePercent, PphCents: pphCents, EntityName: entityName,
				EntityNPWP: entityNPWP, Description: description, Status: "POSTED",
			}
			if existingJournalID.Valid {
				journalID = existingJournalID.Int64
			}
			return nil
		}
		if status == "FILED" {
			return errors.New("cannot post a FILED PPh calculation")
		}
		if pphCents <= 0 {
			return errors.New("PPh calculation has zero pph_cents — nothing to post")
		}

		// Resolve the payable account by pph_type and the income tax expense
		// account (5203).
		payableCode := pphAccountForType(pphType)
		if payableCode == "" {
			return fmt.Errorf("no payable account mapped for pph_type %s", pphType)
		}
		payableAcctID, err := resolveAccountByCode(r.Context(), tx, tid, payableCode)
		if err != nil {
			return err
		}
		expenseAcctID, err := resolveAccountByCode(r.Context(), tx, tid, AccountIncomeTax)
		if err != nil {
			return err
		}

		// Build the journal: Dr 5203 Income Tax Expense / Cr 210x PPh Payable.
		journal := accounting.Journal{
			TenantID:    tid,
			SourceRef:   fmt.Sprintf("PPH-%d", id),
			IntentType:  accounting.IntentType("PPH_POST"),
			EntryDate:   calcDate,
			Description: fmt.Sprintf("PPh %s withholding%s", pphType, optionalNote(description)),
			Lines: []accounting.Line{
				{AccountID: expenseAcctID, DebitCents: pphCents, SourceLineRef: "pph-expense"},
				{AccountID: payableAcctID, CreditCents: pphCents, SourceLineRef: "pph-payable"},
			},
		}
		posted, err := postPPhJournal(r.Context(), tx, tid, idem, journal, uid)
		if err != nil {
			return err
		}
		journalID = posted.ID

		// Mark the PPh calculation as POSTED with the journal_entry_id.
		if _, err := tx.Exec(r.Context(), `
			UPDATE pph_calculations
			SET status = 'POSTED', journal_entry_id = $1, updated_at = now()
			WHERE tenant_id = $2 AND id = $3 AND status = 'DRAFT'
		`, journalID, tid, id); err != nil {
			return err
		}

		resp = PPhResponse{
			ID: id, PphType: pphType, CalculationDate: calcDate, DppCents: dppCents,
			RatePercent: ratePercent, PphCents: pphCents, EntityName: entityName,
			EntityNPWP: entityNPWP, Description: description, Status: "POSTED",
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "POST_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":               id,
		"status":           "POSTED",
		"journal_entry_id": journalID,
		"pph":              resp,
	})
}

func (s *Service) GetRates(w http.ResponseWriter, r *http.Request) {
	rates := []map[string]any{
		{"pph_type": "PPH21", "description": "Employee Income Tax", "rate_nnpwp": 0.05, "rate_non_nnpwp": RatePPh21NonNPWP},
		{"pph_type": "PPH22", "description": "Import/Procurement", "rate_default": RatePPh22Import},
		{"pph_type": "PPH23", "description": "Service (2%)", "rate_service": RatePPh23Service, "rate_rent": RatePPh23Rent, "rate_royalty": RatePPh23Royalty},
		{"pph_type": "PPH26", "description": "Non-Resident", "rate_default": RatePPh26NonRes},
		{"pph_type": "PPH_FINAL_UMKM", "description": "UMKM Final Tax", "rate_05": RatePPhFinalUMKM, "rate_075": RatePPhFinalUMKM075},
	}
	writeJSON(w, http.StatusOK, rates)
}

// =====================================================================
// PPH CALCULATION
// =====================================================================

// calculatePPh computes the withholding tax amount.
// Uses integer math: dpp * rateMilli / 100000, where rateMilli = rate * 1000.
func calculatePPh(dppCents int64, ratePercent float64) int64 {
	if dppCents <= 0 || ratePercent <= 0 {
		return 0
	}
	rateMilli := int64(ratePercent * 1000)
	return dppCents * rateMilli / 100000
}

// pphAccountForType returns the payable account code for a PPh type.
func pphAccountForType(pphType string) string {
	switch strings.ToUpper(pphType) {
	case "PPH21":
		return AccountPPh21
	case "PPH22":
		return AccountPPh22
	case "PPH23":
		return AccountPPh23
	case "PPH26":
		return AccountPPh26
	case "PPH_FINAL_UMKM":
		return AccountPPhUMKM
	default:
		return ""
	}
}

// =====================================================================
// VALIDATION & HELPERS
// =====================================================================

func validatePPh(req CreatePPhRequest) (string, string) {
	switch strings.ToUpper(req.PphType) {
	case "PPH21", "PPH22", "PPH23", "PPH26", "PPH_FINAL_UMKM":
	default:
		return "INVALID_REQUEST", fmt.Sprintf("pph_type must be one of: PPH21, PPH22, PPH23, PPH26, PPH_FINAL_UMKM (got %s)", req.PphType)
	}
	if req.DppCents <= 0 {
		return "INVALID_REQUEST", "dpp_cents must be > 0"
	}
	if req.RatePercent <= 0 || req.RatePercent > 100 {
		return "INVALID_REQUEST", "rate_percent must be between 0 and 100"
	}
	if req.CalculationDate == "" {
		return "INVALID_REQUEST", "calculation_date is required (YYYY-MM-DD)"
	}
	if _, err := time.Parse("2006-01-02", req.CalculationDate); err != nil {
		return "INVALID_REQUEST", "calculation_date must be YYYY-MM-DD"
	}
	return "", ""
}

func pathID(raw string) int64 {
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

// =====================================================================
// JOURNAL POSTING HELPERS (mirror tax/helpers.go and cash/journal.go)
// =====================================================================

// postedEntry carries the inserted journal entry id and its human number.
type postedEntry struct {
	ID     int64
	Number string
}

// postPPhJournal inserts a journal entry + its lines inside an existing
// transaction. It performs the idempotency check, head lock, journal insert,
// chain-head advance, and outbox write — mirroring the pattern used by
// cash/journal.go and tax/helpers.go.
func postPPhJournal(ctx context.Context, tx pgx.Tx, tenant int64, idem string, journal accounting.Journal, uid int64) (postedEntry, error) {
	// Idempotent replay: an identical retry returns the stored journal.
	existing, err := db.New(tx).GetJournalByIdempotencyKey(ctx, db.GetJournalByIdempotencyKeyParams{
		TenantID:       tenant,
		IdempotencyKey: uuidValue(idem),
	})
	if err == nil {
		return postedEntry{ID: existing.ID, Number: existing.Number}, nil
	} else if !isNoRows(err) {
		return postedEntry{}, err
	}

	// Lock the chain head so concurrent postings serialize on one row.
	head, err := lockOrSeedHead(ctx, tx, tenant)
	if err != nil {
		return postedEntry{}, err
	}
	journal.TenantID = tenant
	journal.PreviousHash = head.LastHash
	journal.Hash = accounting.HashJournal(journal)

	periodID, err := resolvePeriod(ctx, tx, tenant, journal.EntryDate)
	if err != nil {
		return postedEntry{}, err
	}
	number, err := nextJournalNumber(ctx, tx, tenant)
	if err != nil {
		return postedEntry{}, err
	}

	entry, err := db.New(tx).InsertJournalEntry(ctx, db.InsertJournalEntryParams{
		TenantID:       tenant,
		Number:         number,
		EntryDate:      parseDatePG(journal.EntryDate),
		PeriodID:       periodID,
		Description:    textValuePG(journal.Description),
		SourceRef:      textValuePG(journal.SourceRef),
		IntentType:     textValuePG(string(journal.IntentType)),
		IdempotencyKey: uuidValue(idem),
		Hash:           journal.Hash,
		PrevHash:       journal.PreviousHash,
		CreatedBy:      int8Value(uid),
	})
	if err != nil {
		return postedEntry{}, err
	}
	for _, line := range journal.Lines {
		if err := db.New(tx).InsertJournalLine(ctx, db.InsertJournalLineParams{
			TenantID:      tenant,
			EntryID:       entry.ID,
			AccountID:     line.AccountID,
			DebitCents:    line.DebitCents,
			CreditCents:   line.CreditCents,
			Description:   textValuePG(line.SourceLineRef),
			SourceLineRef: textValuePG(line.SourceLineRef),
			DimensionIds:  []byte("[]"),
		}); err != nil {
			return postedEntry{}, err
		}
	}
	if err := upsertHead(ctx, tx, tenant, entry.ID, journal.Hash); err != nil {
		return postedEntry{}, err
	}
	if err := insertOutbox(ctx, tx, tenant, "pph.posted", mustJSON(map[string]any{
		"journal_id": entry.ID, "number": number, "intent": string(journal.IntentType),
	})); err != nil {
		return postedEntry{}, err
	}
	return postedEntry{ID: entry.ID, Number: number}, nil
}

// idempotencyKey validates the required Idempotency-Key header (must be a UUID).
func idempotencyKey(request *http.Request) (string, error) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", errors.New("Idempotency-Key header is required")
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(key); err != nil {
		return "", errors.New("Idempotency-Key must be a UUID")
	}
	return key, nil
}

func withTenant(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantID, 10))
	return err
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func uuidValue(raw string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(raw)
	return value
}

func int8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func textValuePG(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func parseDatePG(raw string) pgtype.Date {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

func optionalNote(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return " — " + note
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

func nextDocNumber(ctx context.Context, tx pgx.Tx, tenantID int64, docType, prefix string) (string, error) {
	year := time.Now().Year()
	var p string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, docType, prefix, year).Scan(&p, &seq)
	if err != nil {
		return "", err
	}
	return p + "-" + strconv.FormatInt(int64(year), 10) + "-" + leftPad6(seq), nil
}

func leftPad6(seq int64) string {
	s := strconv.FormatInt(seq, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
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

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}
