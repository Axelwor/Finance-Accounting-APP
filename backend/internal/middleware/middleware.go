// Package middleware provides HTTP middleware for the Finance Accounting API:
// rate limiting, panic recovery, request logging, CORS, and request timeouts.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// RequestID generates a short random hex ID and injects it into the request
// context. Downstream loggers and error responses include this ID so a user
// report can be traced to a single request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// ---------------------------------------------------------------------------
// i-008: Recover — catch panics, log stack trace, return 500.
// ---------------------------------------------------------------------------

// Recover wraps the handler in a defer/recover that logs the panic and stack
// trace, then returns a 500 JSON error instead of crashing the process.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				slog.Error("panic recovered",
					"error", rv,
					"request_id", RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"an unexpected error occurred"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// i-009: RequestLogger — structured logging per request.
// ---------------------------------------------------------------------------

// RequestLogger logs method, path, status, and duration for every request
// using structured logging (slog) with the request_id for traceability.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", RequestIDFromContext(r.Context()),
			"client_ip", clientIP(r),
		)
	})
}

// ---------------------------------------------------------------------------
// i-010: CORS — allow frontend origins.
// ---------------------------------------------------------------------------

// CORSConfig holds allowed origins, methods, and headers.
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

// DefaultCORSConfig returns a config suitable for a React dev server.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{
			"https://accounting.tikuma.net",
			"http://localhost:5173",
			"http://localhost:4173",
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "Idempotency-Key"},
	}
}

// CORS applies Cross-Origin Resource Sharing headers.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range cfg.AllowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				if len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Max-Age", "3600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// i-011: Timeout — per-request context deadline.
// ---------------------------------------------------------------------------

// Timeout sets a per-request deadline. If the handler does not finish within
// the given duration, http.TimeoutHandler returns a 503 Service Unavailable
// with the given JSON body. Unlike a manual goroutine + context approach,
// http.TimeoutHandler correctly manages the goroutine lifecycle and prevents
// concurrent writes to the ResponseWriter.
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, duration, `{"code":"REQUEST_TIMEOUT","message":"Request timed out"}`)
	}
}

// ---------------------------------------------------------------------------
// F-05: LimitBody — global request body cap.
// ---------------------------------------------------------------------------

// LimitBody bounds every request body to n bytes and answers 413 when the
// client exceeds it. Routes that need their own (tighter or looser) cap can
// wrap r.Body with http.MaxBytesReader again — the inner reader wins because
// it is applied closer to the handler.
func LimitBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// M-027: RateLimit — simple per-IP rate limiter for login endpoint.
// ---------------------------------------------------------------------------

// visitor tracks request timestamps for one IP address.
type visitor struct {
	mu       sync.Mutex
	requests []time.Time
}

// RateLimiter is a simple in-memory sliding-window rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	max      int           // max requests per window
	window   time.Duration // sliding window duration
	done     chan struct{} // signals the cleanup goroutine to stop
}

// NewRateLimiter creates a rate limiter that allows max requests per window
// per IP address.
func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		max:      max,
		window:   window,
		done:     make(chan struct{}),
	}
	return rl
}

// Stop signals the background cleanup goroutine to exit. Call this when the
// rate limiter is no longer needed (e.g. during graceful shutdown).
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

// Middleware returns an http.Handler that enforces the rate limit.
// If the limit is exceeded, a 429 Too Many Requests is returned.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	// Background cleanup of stale visitor entries every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-rl.done:
				return
			case <-ticker.C:
				rl.cleanup()
			}
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		rl.mu.Lock()
		v, exists := rl.visitors[ip]
		if !exists {
			v = &visitor{}
			rl.visitors[ip] = v
		}
		rl.mu.Unlock()

		v.mu.Lock()
		now := time.Now()
		// Remove timestamps outside the sliding window.
		cutoff := now.Add(-rl.window)
		keep := v.requests[:0]
		for _, t := range v.requests {
			if t.After(cutoff) {
				keep = append(keep, t)
			}
		}
		v.requests = keep

		if len(v.requests) >= rl.max {
			v.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"RATE_LIMITED","message":"too many requests, please try again later"}`))
			return
		}

		v.requests = append(v.requests, now)
		v.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// cleanup removes visitor entries that haven't been seen in the window.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	for ip, v := range rl.visitors {
		v.mu.Lock()
		if len(v.requests) == 0 || v.requests[len(v.requests)-1].Before(cutoff) {
			delete(rl.visitors, ip)
		}
		v.mu.Unlock()
	}
}

// isTrustedProxy reports whether the direct connection comes from a trusted
// proxy: localhost or a private (RFC 1918) / Docker-internal address. Only
// trusted proxies may set X-Forwarded-For.
func isTrustedProxy(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr // no port
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	// Trust loopback (localhost) and private ranges (Docker bridge / VPN).
	return ip.IsLoopback() || ip.IsPrivate()
}

// clientIP extracts the client IP from X-Forwarded-For or RemoteAddr. The
// X-Forwarded-For header is only honored when the direct connection comes
// from a trusted proxy (localhost or a private/Docker network). When honored,
// the XFF list is walked right-to-left and the rightmost non-trusted entry
// is returned — this is the real client IP appended by the last trusted
// proxy, and prevents spoofing by clients who inject a fake IP at the start.
func clientIP(r *http.Request) string {
	if isTrustedProxy(r.RemoteAddr) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				ip := strings.TrimSpace(parts[i])
				if ip != "" && !isTrustedProxy(ip) {
					return ip
				}
			}
			// All entries are trusted proxies — fall through to RemoteAddr.
		}
	}
	// Fall back to RemoteAddr (host:port).
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
