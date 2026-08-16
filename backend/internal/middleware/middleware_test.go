package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRecover(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestRecoverNormal(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCORS_AllowAll(t *testing.T) {
	cfg := DefaultCORSConfig()
	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	handler.ServeHTTP(rr, req)
	// After B-07, origins are specific (no wildcard). The allowed origin
	// is echoed back.
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("expected http://localhost:5173, got %s", got)
	}
}

func TestCORS_Options(t *testing.T) {
	cfg := DefaultCORSConfig()
	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for OPTIONS")
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestRateLimiter_AllowUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rr.Code)
		}
	}
}

func TestRateLimiter_BlockOverLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rr.Code)
		}
	}
	// 4th request should be rate limited
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// IP 1
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("IP1 request %d: expected 200, got %d", i, rr.Code)
		}
	}
	// IP 2 should not be affected
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "5.6.7.8:5678"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("IP2: expected 200, got %d", rr.Code)
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Use up the limit
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	}
	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)
	// Should be allowed again
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("after window expiry: expected 200, got %d", rr.Code)
	}
}

func TestTimeout_CompletesInTime(t *testing.T) {
	handler := Timeout(100 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name   string
		xff    string
		remote string
		want   string
	}{
		{"XFF single IP (trusted proxy)", "1.2.3.4", "127.0.0.1:80", "1.2.3.4"},
		{"XFF client + trusted proxy", "1.2.3.4, 127.0.0.1", "127.0.0.1:80", "1.2.3.4"},
		{"XFF spoofed + real (trusted proxy)", "1.2.3.4, 5.6.7.8", "127.0.0.1:80", "5.6.7.8"},
		{"XFF all trusted proxies (fall back)", "127.0.0.1, ::1", "127.0.0.1:80", "127.0.0.1"},
		{"XFF ignored from untrusted proxy", "1.2.3.4", "9.9.9.9:80", "9.9.9.9"},
		{"XFF from Docker network (trusted)", "203.0.113.1", "172.17.0.1:80", "203.0.113.1"},
		{"RemoteAddr only", "", "1.2.3.4:80", "1.2.3.4"},
		{"RemoteAddr no port", "", "1.2.3.4", "1.2.3.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			req.RemoteAddr = tt.remote
			got := clientIP(req)
			if got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	var wg sync.WaitGroup
	allowed := 0
	blocked := 0
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
			mu.Lock()
			if rr.Code == http.StatusOK {
				allowed++
			} else {
				blocked++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if allowed != 10 {
		t.Errorf("expected 10 allowed, got %d", allowed)
	}
	if blocked != 10 {
		t.Errorf("expected 10 blocked, got %d", blocked)
	}
}

// TestRateLimiter_Stop verifies the B-10 fix: Stop() closes the done channel
// so the background cleanup goroutine exits. We cannot observe the goroutine
// directly, but we can assert Stop() is safe to call (no panic on double-use
// of a closed channel scenario is prevented by it being called once) and that
// the limiter still functions after Stop.
func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the limit.
	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Stop the cleanup goroutine — must not panic.
	rl.Stop()

	// The limiter keeps enforcing after Stop (middleware still mounted).
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after limit exhausted, got %d", rr.Code)
	}
}

// TestRequestID_GeneratesAndEchoes verifies the RequestID middleware injects
// a request id into the context, echoes it via X-Request-ID response header,
// and honors a client-supplied X-Request-ID for distributed tracing.
func TestRequestID_GeneratesAndEchoes(t *testing.T) {
	var seen string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if seen == "" {
		t.Fatal("request id missing from context")
	}
	if got := rr.Header().Get("X-Request-ID"); got != seen {
		t.Errorf("X-Request-ID header = %q, want %q", got, seen)
	}
}

// TestRequestID_HonorsIncomingHeader verifies a caller-supplied X-Request-ID
// is propagated to the context and echoed back (trace continuation through
// the proxy chain).
func TestRequestID_HonorsIncomingHeader(t *testing.T) {
	var seen string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if seen != "trace-abc-123" {
		t.Errorf("context request id = %q, want trace-abc-123", seen)
	}
	if got := rr.Header().Get("X-Request-ID"); got != "trace-abc-123" {
		t.Errorf("echoed header = %q, want trace-abc-123", got)
	}
}

// TestRequestIDFromContextEmpty verifies the helper returns "" for a context
// without a request id (e.g. background jobs).
func TestRequestIDFromContextEmpty(t *testing.T) {
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty request id, got %q", got)
	}
}
