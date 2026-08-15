package reporting

import (
	"encoding/json"
	"net/http"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/httperr"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	message = httperr.SanitizeMessage(status, code, message)
	writeJSON(writer, status, errorResponse{Code: code, Message: message})
}

// tenantFrom reads the tenant id injected by the auth middleware.
func tenantFrom(request *http.Request) int64 {
	tenantID, ok := auth.TenantIDFromContext(request.Context())
	if !ok {
		return 0
	}
	return tenantID
}
