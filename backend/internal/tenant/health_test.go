package tenant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Phase D: /healthz/detail must expose a connection-pool snapshot so
// saturation becomes observable before it turns into query timeouts.
func TestHealthDetailed_ReportsPoolStats(t *testing.T) {
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
	defer pool.Close()

	service := NewHandler(pool)
	rr := httptest.NewRecorder()
	service.HealthDetailed(rr, httptest.NewRequest(http.MethodGet, "/healthz/detail", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Pool map[string]any `json:"pool"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rr.Body.String())
	}
	for _, key := range []string{"max_conns", "total_conns", "acquired_conns", "idle_conns", "acquire_count", "acquire_duration_ms"} {
		if _, ok := body.Pool[key]; !ok {
			t.Errorf("pool.%s missing from response: %s", key, rr.Body.String())
		}
	}
	if maxConns, _ := body.Pool["max_conns"].(float64); maxConns <= 0 {
		t.Errorf("pool.max_conns = %v, want positive", body.Pool["max_conns"])
	}
}
