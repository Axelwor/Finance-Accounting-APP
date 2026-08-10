package sales

import (
	"net/http"

	"github.com/jackc/pgx/v5"
)

// ConversionStats returns quotation funnel statistics (i-005).
// GET /quotations/conversion-stats
//
// Counts quotations per status and computes the conversion rate
// (CONVERTED / total finished quotations).
func (service *Service) ConversionStats(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	counts := []statusCount{}
	var total, converted, sent, draft, expired, cancelled int64

	err = service.pool.QueryRow(request.Context(), `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = $2),
			COUNT(*) FILTER (WHERE status = $3),
			COUNT(*) FILTER (WHERE status = $4),
			COUNT(*) FILTER (WHERE status = $5),
			COUNT(*) FILTER (WHERE status = $6)
		FROM sales_quotations
		WHERE tenant_id = $1
	`, tenant, statusConverted, statusSent, statusDraft, statusExpired, statusCancelled).
		Scan(&total, &converted, &sent, &draft, &expired, &cancelled)
	if err != nil && !isNoRows(err) {
		writeError(writer, http.StatusInternalServerError, "CONVERSION_STATS_FAILED", err.Error())
		return
	}

	counts = append(counts,
		statusCount{Status: statusConverted, Count: converted},
		statusCount{Status: statusSent, Count: sent},
		statusCount{Status: statusDraft, Count: draft},
		statusCount{Status: statusExpired, Count: expired},
		statusCount{Status: statusCancelled, Count: cancelled},
	)

	// Conversion rate = converted / (converted + expired + cancelled).
	// Quotations still DRAFT/SENT are in-flight and excluded from the rate.
	finished := converted + expired + cancelled
	conversionRatePct := 0.0
	if finished > 0 {
		conversionRatePct = float64(converted) * 100.0 / float64(finished)
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"total":               total,
		"by_status":           counts,
		"finished":            finished,
		"converted":           converted,
		"conversion_rate_pct": round2(conversionRatePct),
	})
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

// ensure pgx stays imported for future extensions.
var _ = pgx.ErrNoRows
