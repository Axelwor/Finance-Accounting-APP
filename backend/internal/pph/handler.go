package pph

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
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
	RatePPh21NonNPWP   = 0.20  // 20% for non-NPWP
	RatePPh22Import    = 0.025 // 2.5%
	RatePPh23Service   = 0.02  // 2%
	RatePPh23Rent      = 0.10  // 10%
	RatePPh23Royalty   = 0.15  // 15%
	RatePPh26NonRes    = 0.20  // 20%
	RatePPhFinalUMKM   = 0.005 // 0.5%
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
	ID             int64  `json:"id"`
	PphType        string `json:"pph_type"`
	CalculationDate string `json:"calculation_date"`
	DppCents       int64  `json:"dpp_cents"`
	RatePercent    float64 `json:"rate_percent"`
	PphCents       int64  `json:"pph_cents"`
	EntityName     string `json:"entity_name"`
	EntityNPWP     string `json:"entity_npwp"`
	Description    string `json:"description"`
	Status         string `json:"status"`
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
	id := pathID(chi.URLParam(r, "id"))
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tid, 10)); err != nil {
			return err
		}
		_, execErr := tx.Exec(r.Context(), `
			UPDATE pph_calculations SET status = 'POSTED' WHERE tenant_id = $1 AND id = $2 AND status = 'DRAFT'
		`, tid, id)
		return execErr
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "POST_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      id,
		"status":  "POSTED",
		"message": "PPh posted — post the journal: Dr 5203 Income Tax / Cr 210x PPh Payable",
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
