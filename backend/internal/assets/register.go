package assets

import (
	"github.com/jackc/pgx/v5"

	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// F-13: Asset Register & Maintenance report.
//
// Read-only report listing all fixed assets for the tenant with computed
// net book value per asset plus a totals summary. Accumulated depreciation
// is read from fixed_assets.accum_dep_cents, which is maintained by every
// depreciation/impairment/disposal posting in this package.

type registerAssetRow struct {
	ID                   int64  `json:"id"`
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	AcquisitionDate      string `json:"acquisition_date"`
	AcquisitionCostCents int64  `json:"acquisition_cost_cents"`
	SalvageValueCents    int64  `json:"salvage_value_cents"`
	DepreciationMethod   string `json:"depreciation_method"`
	UsefulLifeMonths     int    `json:"useful_life_months"`
	AccumulatedDepCents  int64  `json:"accumulated_depreciation_cents"`
	NetBookValueCents    int64  `json:"net_book_value_cents"`
	Status               string `json:"status"`
}

type registerTotals struct {
	TotalCostCents        int64 `json:"total_cost_cents"`
	TotalAccumulatedCents int64 `json:"total_accumulated_cents"`
	TotalNBVCents         int64 `json:"total_nbv_cents"`
	AssetCount            int   `json:"asset_count"`
}

type registerResponse struct {
	Assets []registerAssetRow `json:"assets"`
	Totals registerTotals     `json:"totals"`
}

// computeNBV returns net book value = cost - accumulated depreciation,
// never negative. Accumulated depreciation above cost is clamped so the
// register never reports a negative book value.
func computeNBV(costCents, accumulatedDepCents int64) int64 {
	nbv := costCents - accumulatedDepCents
	if nbv < 0 {
		return 0
	}
	return nbv
}

func (service *Service) AssetRegister(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	query := `
		SELECT id, code, name, acquisition_date, acquisition_cost_cents, salvage_value_cents,
		       depreciation_method, useful_life_months, accum_dep_cents, status
		FROM fixed_assets
		WHERE tenant_id = $1
	`
	args := []any{tenant}
	if status := strings.TrimSpace(request.URL.Query().Get("status")); status != "" {
		query += ` AND status = $2`
		args = append(args, strings.ToUpper(status))
	}
	query += ` ORDER BY code`

	response := registerResponse{Assets: make([]registerAssetRow, 0)}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row registerAssetRow
			var acqDate pgtype.Date
			if err := rows.Scan(&row.ID, &row.Code, &row.Name, &acqDate,
				&row.AcquisitionCostCents, &row.SalvageValueCents,
				&row.DepreciationMethod, &row.UsefulLifeMonths,
				&row.AccumulatedDepCents, &row.Status); err != nil {
				return err
			}
			row.AcquisitionDate = dateString(acqDate)
			row.NetBookValueCents = computeNBV(row.AcquisitionCostCents, row.AccumulatedDepCents)

			response.Totals.TotalCostCents += row.AcquisitionCostCents
			response.Totals.TotalAccumulatedCents += row.AccumulatedDepCents
			response.Totals.TotalNBVCents += row.NetBookValueCents
			response.Totals.AssetCount++
			response.Assets = append(response.Assets, row)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", err.Error())
		return
	}

	writeJSON(writer, http.StatusOK, response)
}
