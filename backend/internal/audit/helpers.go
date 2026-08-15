package audit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/httperr"
)

// Service exposes the attachment (US-100) and audit trail (US-101) endpoints.
type Service struct {
	pool        *pgxpool.Pool
	storageRoot string
}

// NewHandler creates the audit/attachments service. storageRoot is the local
// disk directory where uploaded files are written (e.g. /data/attachments).
func NewHandler(pool *pgxpool.Pool, storageRoot string) *Service {
	return &Service{pool: pool, storageRoot: storageRoot}
}

// Routes registers the attachment and audit-log endpoints on the chi router.
func (service *Service) Routes(router chi.Router) {
	router.Post("/attachments", service.UploadAttachment)
	router.Get("/attachments", service.ListAttachments)
	router.Get("/attachments/{id}/download", service.DownloadAttachment)
	router.Delete("/attachments/{id}", service.DeleteAttachment)
	router.Get("/audit-logs", service.ListAuditLogs)
}

// ---------------------------------------------------------------------------
// Helpers (shared across audit handlers) — mirrors purchase/helpers.go.
// ---------------------------------------------------------------------------

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

func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

func userID(request *http.Request) int64 {
	value, _ := auth.UserIDFromContext(request.Context())
	return value
}

func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

func withTenant(ctx context.Context, tx pgx.Tx, tenantIDValue int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantIDValue, 10))
	return err
}
