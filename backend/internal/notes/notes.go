package notes

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"finance-accounting-app/backend/internal/db"
)

// Financial notes (Catatan atas Laporan Keuangan) — basic disclosure text
// attached to a reporting period. CRUD only; no journal posting.

type noteRequest struct {
	PeriodYear   int    `json:"period_year"`
	NoteNumber   string `json:"note_number"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	DisplayOrder int    `json:"display_order"`
}

type noteResponse struct {
	ID           int64  `json:"id"`
	PeriodYear   int    `json:"period_year"`
	NoteNumber   string `json:"note_number"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	DisplayOrder int    `json:"display_order"`
	CreatedBy    int64  `json:"created_by"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// CreateNote — POST /financial-notes
func (service *Service) CreateNote(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req noteRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := validateNote(req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var note noteResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
		INSERT INTO financial_notes (tenant_id, period_year, note_number, title, content, display_order, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, period_year, note_number, title, content, display_order,
		          COALESCE(created_by, 0), created_at, updated_at
	`, tenant, req.PeriodYear, strings.TrimSpace(req.NoteNumber), strings.TrimSpace(req.Title),
			req.Content, req.DisplayOrder, userID(request)).
			Scan(&note.ID, &note.PeriodYear, &note.NoteNumber, &note.Title, &note.Content,
				&note.DisplayOrder, &note.CreatedBy, &note.CreatedAt, &note.UpdatedAt)
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "NOTE_EXISTS", "note_number already exists for this period")
			return
		}
		writeError(writer, http.StatusInternalServerError, "NOTE_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, note)
}

// ListNotes — GET /financial-notes?period_year=
func (service *Service) ListNotes(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	notes := []noteResponse{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
		SELECT id, period_year, note_number, title, content, display_order,
		       COALESCE(created_by, 0), created_at, updated_at
		FROM financial_notes
		WHERE tenant_id = $1
		  AND ($2::int IS NULL OR period_year = $2)
		ORDER BY period_year DESC, display_order ASC, note_number ASC
	`, tenant, optionalInt(request.URL.Query().Get("period_year")))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var note noteResponse
			if err := rows.Scan(&note.ID, &note.PeriodYear, &note.NoteNumber, &note.Title, &note.Content,
				&note.DisplayOrder, &note.CreatedBy, &note.CreatedAt, &note.UpdatedAt); err != nil {
				return err
			}
			notes = append(notes, note)
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "NOTE_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, notes)
}

// GetNote — GET /financial-notes/{id}
func (service *Service) GetNote(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var note noteResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
		SELECT id, period_year, note_number, title, content, display_order,
		       COALESCE(created_by, 0), created_at, updated_at
		FROM financial_notes
		WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(&note.ID, &note.PeriodYear, &note.NoteNumber, &note.Title, &note.Content,
			&note.DisplayOrder, &note.CreatedBy, &note.CreatedAt, &note.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(writer, http.StatusNotFound, "NOTE_NOT_FOUND", "note not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "NOTE_GET_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, note)
}

// UpdateNote — PUT /financial-notes/{id}
func (service *Service) UpdateNote(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req noteRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := validateNote(req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var note noteResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
		UPDATE financial_notes
		SET period_year = $3, note_number = $4, title = $5, content = $6,
		    display_order = $7, updated_at = $8
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, period_year, note_number, title, content, display_order,
		          COALESCE(created_by, 0), created_at, updated_at
	`, tenant, id, req.PeriodYear, strings.TrimSpace(req.NoteNumber), strings.TrimSpace(req.Title),
			req.Content, req.DisplayOrder, time.Now()).
			Scan(&note.ID, &note.PeriodYear, &note.NoteNumber, &note.Title, &note.Content,
				&note.DisplayOrder, &note.CreatedBy, &note.CreatedAt, &note.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(writer, http.StatusNotFound, "NOTE_NOT_FOUND", "note not found")
			return
		}
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "NOTE_EXISTS", "note_number already exists for this period")
			return
		}
		writeError(writer, http.StatusInternalServerError, "NOTE_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, note)
}

// DeleteNote — DELETE /financial-notes/{id}
func (service *Service) DeleteNote(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var tag pgconn.CommandTag
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		tag, err = tx.Exec(request.Context(), `
		DELETE FROM financial_notes WHERE tenant_id = $1 AND id = $2
	`, tenant, id)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "NOTE_DELETE_FAILED", err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(writer, http.StatusNotFound, "NOTE_NOT_FOUND", "note not found")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func validateNote(req noteRequest) error {
	if req.PeriodYear <= 0 {
		return errors.New("period_year is required")
	}
	if strings.TrimSpace(req.NoteNumber) == "" {
		return errors.New("note_number is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return errors.New("title is required")
	}
	if req.Content == "" {
		return errors.New("content is required")
	}
	return nil
}

// optionalInt parses a query string into an int pointer; empty -> NULL.
func optionalInt(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return value
}
