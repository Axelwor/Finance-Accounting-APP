package forecast

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
)

// ---------------------------------------------------------------------------
// F-06: Cash Flow Forecasting
//   Projects future cash position based on:
//   - Current cash & bank balances
//   - Outstanding AR (expected collection dates from due_date)
//   - Outstanding AP (expected payment dates from due_date)
//   - Recurring transactions (next_date + amount)
//   Returns daily/weekly buckets for the forecast horizon.
// ---------------------------------------------------------------------------

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Routes(r chi.Router) {
	r.Get("/forecast/cash-flow", s.GetCashFlowForecast)
}

type forecastRequest struct {
	HorizonDays int `json:"horizon_days"` // default 30
}

type forecastBucket struct {
	Date           string `json:"date"`
	InflowCents    int64  `json:"inflow_cents"`
	OutflowCents   int64  `json:"outflow_cents"`
	NetCents       int64  `json:"net_cents"`
	RunningBalance int64  `json:"running_balance_cents"`
}

type forecastResponse struct {
	StartingBalanceCents int64            `json:"starting_balance_cents"`
	HorizonDays          int              `json:"horizon_days"`
	Buckets              []forecastBucket `json:"buckets"`
	TotalInflowCents     int64            `json:"total_inflow_cents"`
	TotalOutflowCents    int64            `json:"total_outflow_cents"`
	EndingBalanceCents   int64            `json:"ending_balance_cents"`
}

func (s *Service) GetCashFlowForecast(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantIDFromContext(r.Context())
	if !ok || tid <= 0 {
		writeJSON(w, http.StatusUnauthorized, errBody{"TENANT_REQUIRED", "tenant context is required"})
		return
	}

	horizon := 30
	if h := r.URL.Query().Get("horizon"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 && parsed <= 365 {
			horizon = parsed
		}
	}

	ctx := r.Context()

	// 1. Get starting balance (sum of all CASH/BANK account balances)
	var startingBalance int64
	err := db.WithTenantData(ctx, s.pool, tid, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jl.debit_cents - jl.credit_cents), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.entry_id AND je.tenant_id = jl.tenant_id
		JOIN accounts a ON a.id = jl.account_id AND a.tenant_id = jl.tenant_id
		WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
		  AND a.account_type IN ('CASH', 'BANK')
	`, tid).Scan(&startingBalance)
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{"FORECAST_FAILED", err.Error()})
		return
	}

	// 2. Get expected AR inflows (unpaid invoices by due_date)
	type arFlow struct {
		dueDate time.Time
		amount  int64
	}
	arFlows := []arFlow{}
	err = db.WithTenantData(ctx, s.pool, tid, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
		SELECT i.due_date, i.receivable_cents
		FROM invoices i
		WHERE i.tenant_id = $1 AND i.status = 'POSTED' AND i.receivable_cents > 0
		  AND i.due_date >= CURRENT_DATE
		ORDER BY i.due_date
	`, tid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f arFlow
			if err := rows.Scan(&f.dueDate, &f.amount); err != nil {
				return err
			}
			arFlows = append(arFlows, f)
		}
		return rows.Err()
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{"FORECAST_FAILED", err.Error()})
		return
	}

	// 3. Get expected AP outflows (unpaid supplier invoices by due_date)
	type apFlow struct {
		dueDate time.Time
		amount  int64
	}
	apFlows := []apFlow{}
	err = db.WithTenantData(ctx, s.pool, tid, func(tx pgx.Tx) error {
		rows2, err := tx.Query(ctx, `
		SELECT si.due_date, si.amount_due_cents
		FROM supplier_invoices si
		WHERE si.tenant_id = $1 AND si.status = 'POSTED' AND si.amount_due_cents > 0
		  AND si.due_date >= CURRENT_DATE
		ORDER BY si.due_date
	`, tid)
		if err != nil {
			return err
		}
		defer rows2.Close()
		for rows2.Next() {
			var f apFlow
			if err := rows2.Scan(&f.dueDate, &f.amount); err != nil {
				return err
			}
			apFlows = append(apFlows, f)
		}
		return rows2.Err()
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{"FORECAST_FAILED", err.Error()})
		return
	}

	// 4. Get recurring transaction flows
	type recFlow struct {
		nextDate time.Time
		amount   int64
		isInflow bool
	}
	recFlows := []recFlow{}
	err = db.WithTenantData(ctx, s.pool, tid, func(tx pgx.Tx) error {
		rows3, err := tx.Query(ctx, `
		SELECT next_date, amount_cents,
		       CASE WHEN intent_type IN ('CASH_IN', 'LEASE_PAYMENT') THEN true ELSE false END
		FROM recurring_transactions
		WHERE tenant_id = $1 AND is_active = true AND next_date >= CURRENT_DATE
	`, tid)
		if err != nil {
			return err
		}
		defer rows3.Close()
		for rows3.Next() {
			var f recFlow
			if err := rows3.Scan(&f.nextDate, &f.amount, &f.isInflow); err != nil {
				return err
			}
			recFlows = append(recFlows, f)
		}
		return rows3.Err()
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{"FORECAST_FAILED", err.Error()})
		return
	}

	// 5. Build daily buckets
	buckets := make([]forecastBucket, horizon)
	today := time.Now()
	for i := 0; i < horizon; i++ {
		date := bucketDate(today, i)
		buckets[i].Date = date.Format("2006-01-02")

		// AR inflows
		for _, f := range arFlows {
			if f.dueDate.Equal(date) {
				buckets[i].InflowCents += f.amount
			}
		}

		// AP outflows
		for _, f := range apFlows {
			if f.dueDate.Equal(date) {
				buckets[i].OutflowCents += f.amount
			}
		}

		// Recurring flows
		for _, f := range recFlows {
			if f.nextDate.Equal(date) {
				if f.isInflow {
					buckets[i].InflowCents += f.amount
				} else {
					buckets[i].OutflowCents += f.amount
				}
			}
		}

		buckets[i].NetCents = netCents(buckets[i])

		// Running balance
		if i == 0 {
			buckets[i].RunningBalance = startingBalance + buckets[i].NetCents
		} else {
			buckets[i].RunningBalance = buckets[i-1].RunningBalance + buckets[i].NetCents
		}
	}

	// 6. Compute totals
	var totalIn, totalOut int64
	for _, b := range buckets {
		totalIn += b.InflowCents
		totalOut += b.OutflowCents
	}

	resp := forecastResponse{
		StartingBalanceCents: startingBalance,
		HorizonDays:          horizon,
		Buckets:              buckets,
		TotalInflowCents:     totalIn,
		TotalOutflowCents:    totalOut,
		EndingBalanceCents:   endingBalance(startingBalance, totalIn, totalOut),
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Pure helpers (bucket math & date calculations) — no DB access.
// ---------------------------------------------------------------------------

// bucketDate returns the calendar date for forecast bucket i, defined as
// base + i days. It is pure and timezone-stable; the caller chooses the base.
func bucketDate(base time.Time, i int) time.Time {
	return base.AddDate(0, 0, i)
}

// netCents returns inflow - outflow for a single forecast bucket.
func netCents(b forecastBucket) int64 {
	return b.InflowCents - b.OutflowCents
}

// endingBalance returns the projected ending balance given a starting
// balance and aggregated inflows/outflows:
//
//	ending = starting + totalInflow - totalOutflow
//
// It may be negative, indicating a projected overdraft.
func endingBalance(starting, totalInflow, totalOutflow int64) int64 {
	return starting + totalInflow - totalOutflow
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
