// Package httperr provides a unified error response shape across all HTTP
// handlers. It implements the API_CONTRACT.md error convention:
//
//	{ "code": "...", "message": "...", "details": {...}, "request_id": "..." }
package httperr

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Response is the canonical error body returned by all API endpoints.
type Response struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

// Write sends a JSON error response with the given status code, error code,
// human-readable message, and optional details. A unique request_id is
// generated for every call so logs and client reports can be correlated.
func Write(w http.ResponseWriter, status int, code, message string, details ...map[string]any) {
	requestID := generateRequestID()
	resp := Response{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}
	if len(details) > 0 {
		resp.Details = details[0]
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// SanitizeMessage logs the raw error message via slog (with the request_id
// for traceability) and returns a generic message for 5xx errors. For 4xx
// errors the original message is returned as-is — validation messages are
// safe to show to clients and are needed for the UX.
//
// Usage in local writeErr helpers:
//
//	func writeErr(w http.ResponseWriter, status int, code, message string) {
//	    message = httperr.SanitizeMessage(status, code, message)
//	    writeJSON(w, status, errorResponse{Code: code, Message: message})
//	}
func SanitizeMessage(status int, code, message string) string {
	if status >= 500 {
		slog.Error("internal error returned to client",
			"code", code,
			"raw_message", message,
		)
		return "An internal error occurred. Please try again or contact support."
	}
	return message
}

// generateRequestID returns a short hex string (8 bytes → 16 chars).
func generateRequestID() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(raw)
}
