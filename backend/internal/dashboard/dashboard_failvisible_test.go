package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// F-08: widget fetchers must fail visibly. A closed pool makes every query
// fail with "pool is closed", which previously produced 200-with-zeros (the
// swallowed-scan pattern); it must now produce 503 WIDGET_DATA_UNAVAILABLE.
//
// Needs a reachable Postgres only to construct + close the pool, so it is
// guarded by TEST_DATABASE_URL like the other integration tests.
// ---------------------------------------------------------------------------

func newClosedPoolService(t *testing.T) (*Service, *httptest.ResponseRecorder, func(http.ResponseWriter) bool) {
	t.Helper()
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	poolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close() // every subsequent query errors

	return NewHandler(pool), httptest.NewRecorder(), func(w http.ResponseWriter) bool {
		return w.Header().Get("Content-Type") != ""
	}
}

func assertWidgetUnavailable(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (fail-visible)", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"WIDGET_DATA_UNAVAILABLE"`) {
		t.Errorf("body = %s, want WIDGET_DATA_UNAVAILABLE code", body)
	}
}

func TestWidgetFetchers_FailVisibleOnPoolError(t *testing.T) {
	cases := []struct {
		name  string
		fetch func(service *Service, w http.ResponseWriter, r *http.Request)
	}{
		{"cash balance", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchCashBalanceData(w, r, 1) }},
		{"pl snapshot", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchPLSnapshotData(w, r, 1) }},
		{"ar aging", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchARAgingData(w, r, 1) }},
		{"ap aging", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchAPAgingData(w, r, 1) }},
		{"low stock", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchLowStockData(w, r, 1) }},
		{"recent transactions", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchRecentTxnsData(w, r, 1) }},
		{"period status", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchPeriodStatusData(w, r, 1) }},
		{"outstanding invoices", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchOutstandingInvoicesData(w, r, 1) }},
		{"tax summary", func(s *Service, w http.ResponseWriter, r *http.Request) { s.fetchTaxSummaryData(w, r, 1) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, rr, _ := newClosedPoolService(t)
			req := httptest.NewRequest(http.MethodGet, "/dashboard/widgets/1/data", nil)
			tc.fetch(service, rr, req)
			assertWidgetUnavailable(t, rr)
		})
	}
}

// The AR/AP aging endpoints must never fall back to a partial "simple total":
// on failure they answer 503 rather than a 200 whose buckets list is empty
// but total_cents looks real.
func TestAgingFetchers_NoPartialFallback(t *testing.T) {
	service, rr, _ := newClosedPoolService(t)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/widgets/1/data", nil)
	service.fetchARAgingData(rr, req, 1)
	assertWidgetUnavailable(t, rr)
	if strings.Contains(rr.Body.String(), `"total_cents":0`) {
		t.Errorf("fallback total leaked into response: %s", rr.Body.String())
	}
}
