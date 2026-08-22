package middleware

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	// F-14: allowed responses must declare Origin as a varying header so
	// shared caches never serve one origin's CORS response to another.
	if got := rr.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}

// F-14: disallowed origins get no CORS headers and no Vary: Origin (the
// response is origin-independent because it carries no allow-* headers).
func TestCORS_DisallowedOriginNoVary(t *testing.T) {
	cfg := DefaultCORSConfig()
	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	handler.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no ACAO header, got %s", got)
	}
	if got := rr.Header().Get("Vary"); got != "" {
		t.Errorf("Vary = %q, want empty for disallowed origin", got)
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

// TestLimitBody_OverCap verifies F-05: a body larger than the cap is cut off
// by http.MaxBytesReader. The handler sees a read error, and the response
// writer has been marked so the server answers 413 when the handler writes
// without an explicit status.
func TestLimitBody_OverCap(t *testing.T) {
	handler := LimitBody(16)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 64)
		if _, err := r.Body.Read(buf); err == nil {
			t.Error("expected read error for oversized body")
		}
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("POST", "/", strings.NewReader("this body is definitely longer than sixteen bytes")))
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rr.Code)
	}
}

// TestLimitBody_UnderCap verifies bodies within the cap pass through intact.
func TestLimitBody_UnderCap(t *testing.T) {
	handler := LimitBody(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		if string(body) != `{"ok":true}` {
			t.Errorf("body = %q, want original payload", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("POST", "/", strings.NewReader(`{"ok":true}`)))
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestLimitBody_ExactlyAtCap verifies the boundary: n bytes are allowed.
func TestLimitBody_ExactlyAtCap(t *testing.T) {
	handler := LimitBody(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 8 {
			t.Fatalf("expected full 8-byte body without error, got %d bytes, err=%v", len(body), err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("POST", "/", strings.NewReader("12345678")))
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Phase D: request-log sampling.
// ---------------------------------------------------------------------------

func TestShouldLogRequest(t *testing.T) {
	original := sampleRandom
	t.Cleanup(func() { sampleRandom = original })
	sampleRandom = func() float64 { return 0.99 } // never sampled

	cases := []struct {
		name     string
		status   int
		duration time.Duration
		want     bool
	}{
		{name: "fast success not logged", status: 200, duration: 5 * time.Millisecond, want: false},
		{name: "4xx always logged", status: 400, duration: time.Millisecond, want: true},
		{name: "5xx always logged", status: 500, duration: time.Millisecond, want: true},
		{name: "slow success always logged", status: 200, duration: slowRequestThreshold + time.Millisecond, want: true},
		{name: "exactly at threshold not logged", status: 200, duration: slowRequestThreshold, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldLogRequest(tc.status, tc.duration); got != tc.want {
				t.Errorf("shouldLogRequest(%d, %s) = %v, want %v", tc.status, tc.duration, got, tc.want)
			}
		})
	}

	t.Run("lucky sample logs fast success", func(t *testing.T) {
		sampleRandom = func() float64 { return 0.001 }
		if !shouldLogRequest(200, time.Millisecond) {
			t.Error("expected sample to log")
		}
	})
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func TestRequestLogger_Sampling(t *testing.T) {
	original := sampleRandom
	t.Cleanup(func() { sampleRandom = original })
	sampleRandom = func() float64 { return 0.99 } // deterministic: no luck

	t.Run("fast 2xx suppressed", func(t *testing.T) {
		buf := captureSlog(t)
		handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/ok", nil))
		if strings.Contains(buf.String(), "request") {
			t.Errorf("expected suppression, got %q", buf.String())
		}
	})

	t.Run("5xx logged", func(t *testing.T) {
		buf := captureSlog(t)
		handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/boom", nil))
		if !strings.Contains(buf.String(), `status=500`) || !strings.Contains(buf.String(), `path=/boom`) {
			t.Errorf("expected error line, got %q", buf.String())
		}
	})

	t.Run("slow 2xx logged", func(t *testing.T) {
		buf := captureSlog(t)
		handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(slowRequestThreshold + 20*time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/slow", nil))
		if !strings.Contains(buf.String(), `path=/slow`) {
			t.Errorf("expected slow-request line, got %q", buf.String())
		}
	})
}
