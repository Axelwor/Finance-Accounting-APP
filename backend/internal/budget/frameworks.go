package budget

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// US-090A: Report framework selection (EMKM / ETAP / SAK Umum). The framework
// does not change posted data — it only changes how the reporting layer
// presents the same totals. A tenant picks one framework as default.

type frameworkResponse struct {
	ID        int64  `json:"id"`
	Framework string `json:"framework"`
	IsDefault bool   `json:"is_default"`
	TenantID  int64  `json:"tenant_id"`
	CreatedAt string `json:"created_at"`
}

type frameworkRequest struct {
	Framework string `json:"framework"`
	IsDefault bool   `json:"is_default"`
}

// ListFrameworks — GET /report-frameworks
func (service *Service) ListFrameworks(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	items := []frameworkResponse{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, framework, is_default, tenant_id, created_at
			FROM report_frameworks
			WHERE tenant_id = $1
			ORDER BY is_default DESC, framework
		`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f frameworkResponse
			var createdAt pgtype.Timestamptz
			if err := rows.Scan(&f.ID, &f.Framework, &f.IsDefault, &f.TenantID, &createdAt); err != nil {
				return err
			}
			f.CreatedAt = timestampString(createdAt)
			items = append(items, f)
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "FRAMEWORK_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

// SetFramework — POST /report-frameworks
// Inserts (or upserts) a framework row for the tenant. When is_default is
// true the previous default is cleared first so only one default remains.
func (service *Service) SetFramework(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req frameworkRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	fw := strings.ToUpper(strings.TrimSpace(req.Framework))
	if !validFramework(fw) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "framework must be one of EMKM, ETAP, SAK_UMUM")
		return
	}

	ctx := request.Context()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "FRAMEWORK_SET_FAILED", err.Error())
		return
	}
	defer tx.Rollback(ctx)

	if err := withTenant(ctx, tx, tenant); err != nil {
		writeError(writer, http.StatusInternalServerError, "FRAMEWORK_SET_FAILED", err.Error())
		return
	}
	if req.IsDefault {
		// Clear any existing default so only one remains.
		if _, err := tx.Exec(ctx, `
			UPDATE report_frameworks SET is_default = false WHERE tenant_id = $1
		`, tenant); err != nil {
			writeError(writer, http.StatusInternalServerError, "FRAMEWORK_SET_FAILED", err.Error())
			return
		}
	}

	var f frameworkResponse
	var createdAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		INSERT INTO report_frameworks (tenant_id, framework, is_default)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, framework) DO UPDATE
		SET is_default = EXCLUDED.is_default
		RETURNING id, framework, is_default, tenant_id, created_at
	`, tenant, fw, req.IsDefault).Scan(&f.ID, &f.Framework, &f.IsDefault, &f.TenantID, &createdAt)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "FRAMEWORK_SET_FAILED", err.Error())
		return
	}
	f.CreatedAt = timestampString(createdAt)
	if err := tx.Commit(ctx); err != nil {
		writeError(writer, http.StatusInternalServerError, "FRAMEWORK_SET_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, f)
}

// DefaultFramework loads the tenant's default framework code. Returns "" when
// the tenant has not picked one yet (callers fall back to SAK_UMUM).
func (service *Service) DefaultFramework(ctx context.Context, tenant int64) (string, error) {
	var fw string
	err := db.WithTenantData(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT framework FROM report_frameworks
			WHERE tenant_id = $1 AND is_default = true
			LIMIT 1
		`, tenant).Scan(&fw)
	})
	if err != nil {
		if isNoRows(err) {
			return "", nil
		}
		return "", err
	}
	return fw, nil
}

// Ensure error used to satisfy the linter that errors may be compared.
var _ = errors.New
