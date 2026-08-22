package budget

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/db"
)

// US-093: Dimensions (cabang / proyek / departemen / cost center). Dimensions
// tag journal lines (many-to-many) and scope budgets. CRUD only; no journal
// posting.

type dimensionRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	DimensionType string `json:"dimension_type"`
}

type dimensionResponse struct {
	ID            int64  `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	DimensionType string `json:"dimension_type"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     string `json:"created_at"`
}

// CreateDimension — POST /dimensions
func (service *Service) CreateDimension(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req dimensionRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	dtype := strings.ToLower(strings.TrimSpace(req.DimensionType))
	if code == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "code is required")
		return
	}
	if name == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}
	if !validDimensionType(dtype) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST",
			"dimension_type must be one of branch, project, department, cost_center")
		return
	}

	ctx := request.Context()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_CREATE_FAILED", err.Error())
		return
	}
	defer tx.Rollback(ctx)
	if err := withTenant(ctx, tx, tenant); err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_CREATE_FAILED", err.Error())
		return
	}

	var d dimensionResponse
	err = tx.QueryRow(ctx, `
		INSERT INTO dimensions (tenant_id, code, name, dimension_type, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, code, name, dimension_type, is_active, created_at
	`, tenant, code, name, dtype).Scan(&d.ID, &d.Code, &d.Name, &d.DimensionType, &d.IsActive, &d.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "DIMENSION_EXISTS", "dimension code already exists")
			return
		}
		writeError(writer, http.StatusInternalServerError, "DIMENSION_CREATE_FAILED", err.Error())
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, d)
}

// ListDimensions — GET /dimensions?dimension_type=&is_active=
func (service *Service) ListDimensions(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	q := request.URL.Query()
	dtype := strings.ToLower(strings.TrimSpace(q.Get("dimension_type")))
	if dtype != "" && !validDimensionType(dtype) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST",
			"dimension_type must be one of branch, project, department, cost_center")
		return
	}
	activeParam := strings.TrimSpace(q.Get("is_active"))

	args := []any{tenant, nullableStr(dtype), nullableBool(activeParam)}
	items := []dimensionResponse{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, code, name, dimension_type, is_active, created_at
			FROM dimensions
			WHERE tenant_id = $1
			  AND ($2::text IS NULL OR dimension_type = $2)
			  AND ($3::boolean IS NULL OR is_active = $3)
			ORDER BY dimension_type, code
		`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d dimensionResponse
			if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.DimensionType, &d.IsActive, &d.CreatedAt); err != nil {
				return err
			}
			items = append(items, d)
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

type tagRequest struct {
	DimensionIDs []int64 `json:"dimension_ids"`
}

// TagJournalLine — POST /journal-lines/{id}/dimensions
// Replaces the set of dimensions on a journal line. The line must belong to a
// POSTED journal (tagging metadata, not a journal mutation).
func (service *Service) TagJournalLine(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	lineID, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req tagRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	ctx := request.Context()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_TAG_FAILED", err.Error())
		return
	}
	defer tx.Rollback(ctx)
	if err := withTenant(ctx, tx, tenant); err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_TAG_FAILED", err.Error())
		return
	}

	// Verify the journal line exists for this tenant.
	var exists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM journal_lines WHERE tenant_id = $1 AND id = $2)
	`, tenant, lineID).Scan(&exists)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_TAG_FAILED", err.Error())
		return
	}
	if !exists {
		writeError(writer, http.StatusNotFound, "JOURNAL_LINE_NOT_FOUND", "journal line not found")
		return
	}

	// Replace tags: remove existing, insert the new set.
	if _, err := tx.Exec(ctx, `
		DELETE FROM journal_line_dimensions WHERE tenant_id = $1 AND journal_line_id = $2
	`, tenant, lineID); err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_TAG_FAILED", err.Error())
		return
	}
	for _, dimID := range req.DimensionIDs {
		if dimID <= 0 {
			continue
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO journal_line_dimensions (tenant_id, journal_line_id, dimension_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, journal_line_id, dimension_id) DO NOTHING
		`, tenant, lineID, dimID)
		if err != nil {
			if isForeignKeyViolation(err) {
				writeError(writer, http.StatusBadRequest, "DIMENSION_NOT_FOUND",
					"one or more dimension_ids do not exist for this tenant")
				return
			}
			writeError(writer, http.StatusInternalServerError, "DIMENSION_TAG_FAILED", err.Error())
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(writer, http.StatusInternalServerError, "DIMENSION_TAG_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"journal_line_id": lineID,
		"dimension_ids":   req.DimensionIDs,
		"tagged":          true,
	})
}

// nullableStr returns a *string for SQL ($N::text IS NULL OR col = $N): empty
// becomes nil (NULL).
func nullableStr(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}

// nullableBool maps "true"/"false" to a bool pointer; anything else -> nil.
func nullableBool(raw string) any {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return nil
}

// Ensure the pgx import is used by the package (scan helpers).
var _ = pgx.ErrNoRows
var _ = errors.New
