// Package middleware provides HTTP middleware for the Finance Accounting API:
// rate limiting, panic recovery, request logging, CORS, and request timeouts.
package middleware

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// ---------------------------------------------------------------------------
// i-008: Recover — catch panics, log stack trace, return 500.
// ---------------------------------------------------------------------------

// Recover wraps the handler in a defer/recover that logs the panic and stack
// trace, then returns a 500 JSON error instead of crashing the process.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				log.Printf("PANIC: %v\n%s", rv, debug.Stack())
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

// RequestLogger logs method, path, status, and duration for every request.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		log.Printf("%s %s → %d (%s)", r.Method, r.URL.Path, ww.Status(), time.Since(start).Round(time.Millisecond))
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
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "Idempotency-Key", "X-Tenant-ID"},
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
// the given duration, the context is cancelled and a 504 Gateway Timeout is
// returned.
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()
			r = r.WithContext(ctx)

			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r)
				close(done)
			}()

			select {
			case <-done:
				// Request completed normally.
			case <-ctx.Done():
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusGatewayTimeout)
				_, _ = w.Write([]byte(`{"code":"REQUEST_TIMEOUT","message":"request timed out"}`))
			}
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
}

// NewRateLimiter creates a rate limiter that allows max requests per window
// per IP address.
func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*visitor),
		max:      max,
		window:   window,
	}
}

// Middleware returns an http.Handler that enforces the rate limit.
// If the limit is exceeded, a 429 Too Many Requests is returned.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	// Background cleanup of stale visitor entries every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
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

// clientIP extracts the client IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list.
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Fall back to RemoteAddr (host:port).
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}
