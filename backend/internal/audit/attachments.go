package audit

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/db"
)

// Allowed MIME types for attachments (PRD: foto struk OCR, invoice scans, PDFs).
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"application/pdf": true,
}

// maxUploadSize is 10 MB, matching the PRD validation requirement.
const maxUploadSize = 10 << 20

// attachmentRow is the JSON shape returned by the API.
type attachmentRow struct {
	ID         int64  `json:"id"`
	TenantID   int64  `json:"tenant_id"`
	OwnerType  string `json:"owner_type"`
	OwnerID    int64  `json:"owner_id"`
	FileName   string `json:"file_name"`
	FileKey    string `json:"file_key"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	OCRStatus  string `json:"ocr_status"`
	CreatedAt  string `json:"created_at"`
	UploadedBy int64  `json:"uploaded_by"`
}

// UploadAttachment handles POST /attachments (multipart form: file +
// owner_type + owner_id). Validates type/size, writes to local disk, and
// inserts the metadata row in one transaction.
func (service *Service) UploadAttachment(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	uid := userID(request)

	// Limit total request body to maxUploadSize so huge uploads fail fast.
	request.Body = http.MaxBytesReader(writer, request.Body, maxUploadSize+1024)
	if err := request.ParseMultipartForm(32 << 20); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "file is required and must be under 10MB")
		return
	}

	ownerType := strings.TrimSpace(request.FormValue("owner_type"))
	ownerIDRaw := strings.TrimSpace(request.FormValue("owner_id"))
	if !validOwnerType(ownerType) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "owner_type is not a valid entity type")
		return
	}
	ownerID, err := strconv.ParseInt(ownerIDRaw, 10, 64)
	if err != nil || ownerID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "owner_id must be a positive integer")
		return
	}

	file, header, err := request.FormFile("file")
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "file field is required")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = sniffMimeType(header.Filename)
	}
	if !allowedMimeTypes[mimeType] {
		writeError(writer, http.StatusUnsupportedMediaType, "INVALID_FILE_TYPE",
			"only image/jpeg, image/png, and application/pdf are allowed")
		return
	}
	if header.Size <= 0 || header.Size > maxUploadSize {
		writeError(writer, http.StatusBadRequest, "INVALID_FILE_SIZE",
			"file size must be between 1 byte and 10MB")
		return
	}

	fileKey := newFileKey()
	dir := filepath.Join(service.storageRoot, strconv.FormatInt(tenant, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(writer, http.StatusInternalServerError, "STORAGE_FAILED", "could not create storage directory")
		return
	}
	diskPath := filepath.Join(dir, fileKey)
	dst, err := os.Create(diskPath)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STORAGE_FAILED", "could not write file to disk")
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		_ = dst.Close()
		_ = os.Remove(diskPath)
		writeError(writer, http.StatusInternalServerError, "STORAGE_FAILED", "could not save file")
		return
	}
	_ = dst.Close()

	row := attachmentRow{
		TenantID:   tenant,
		OwnerType:  ownerType,
		OwnerID:    ownerID,
		FileName:   header.Filename,
		FileKey:    fileKey,
		MimeType:   mimeType,
		FileSize:   header.Size,
		OCRStatus:  "NONE",
		UploadedBy: uid,
	}

	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var id int64
		var createdAt string
		err := tx.QueryRow(request.Context(), `
			INSERT INTO attachments (tenant_id, owner_type, owner_id, file_name, file_key, mime_type, file_size, ocr_status, uploaded_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'NONE', $8)
			RETURNING id, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSOF')
		`, tenant, ownerType, ownerID, row.FileName, row.FileKey, row.MimeType, row.FileSize, int8OrNil(uid)).Scan(&id, &createdAt)
		if err != nil {
			return err
		}
		row.ID = id
		row.CreatedAt = createdAt
		return nil
	})
	if err != nil {
		_ = os.Remove(diskPath)
		writeError(writer, http.StatusInternalServerError, "ATTACHMENT_INSERT_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, row)
}

// ListAttachments handles GET /attachments?owner_type=X&owner_id=Y.
func (service *Service) ListAttachments(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	query := request.URL.Query()
	ownerType := query.Get("owner_type")
	ownerIDRaw := query.Get("owner_id")
	if !validOwnerType(ownerType) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "owner_type is not a valid entity type")
		return
	}
	ownerID, err := strconv.ParseInt(ownerIDRaw, 10, 64)
	if err != nil || ownerID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "owner_id must be a positive integer")
		return
	}

	rows, err := service.pool.Query(request.Context(), `
		SELECT id, tenant_id, owner_type, owner_id, file_name, file_key, mime_type, file_size, ocr_status,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'), COALESCE(uploaded_by, 0)
		FROM attachments
		WHERE tenant_id = $1 AND owner_type = $2 AND owner_id = $3
		ORDER BY created_at DESC
	`, tenant, ownerType, ownerID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ATTACHMENT_QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()

	items := make([]attachmentRow, 0)
	for rows.Next() {
		var row attachmentRow
		if err := rows.Scan(&row.ID, &row.TenantID, &row.OwnerType, &row.OwnerID,
			&row.FileName, &row.FileKey, &row.MimeType, &row.FileSize, &row.OCRStatus,
			&row.CreatedAt, &row.UploadedBy); err != nil {
			writeError(writer, http.StatusInternalServerError, "ATTACHMENT_QUERY_FAILED", err.Error())
			return
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "ATTACHMENT_QUERY_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

// DownloadAttachment handles GET /attachments/{id}/download and streams the
// binary file to the client.
func (service *Service) DownloadAttachment(writer http.ResponseWriter, request *http.Request) {
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

	var fileKey, fileName, mimeType string
	err = service.pool.QueryRow(request.Context(), `
		SELECT file_key, file_name, mime_type
		FROM attachments
		WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(&fileKey, &fileName, &mimeType)
	if err != nil {
		writeError(writer, http.StatusNotFound, "NOT_FOUND", "attachment does not exist")
		return
	}

	diskPath := filepath.Join(service.storageRoot, strconv.FormatInt(tenant, 10), fileKey)
	file, err := os.Open(diskPath)
	if err != nil {
		writeError(writer, http.StatusNotFound, "FILE_MISSING", "stored file could not be opened")
		return
	}
	defer file.Close()

	writer.Header().Set("Content-Type", mimeType)
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	http.ServeFile(writer, request, diskPath)
}

// DeleteAttachment handles DELETE /attachments/{id}. It removes the file from
// disk and deletes the row, recording an audit log entry first.
func (service *Service) DeleteAttachment(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	uid := userID(request)
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var fileKey, fileName, mimeType string
	var ownerID int64
	var ownerType string
	var fileSize int64

	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		err := tx.QueryRow(request.Context(), `
			SELECT file_key, file_name, mime_type, owner_id, owner_type, file_size
			FROM attachments
			WHERE tenant_id = $1 AND id = $2
		`, tenant, id).Scan(&fileKey, &fileName, &mimeType, &ownerID, &ownerType, &fileSize)
		if err != nil {
			return err
		}
		before := map[string]any{
			"id":         id,
			"owner_type": ownerType,
			"owner_id":   ownerID,
			"file_name":  fileName,
			"file_key":   fileKey,
			"mime_type":  mimeType,
			"file_size":  fileSize,
		}
		if err := Log(request.Context(), tx, tenant, uid, "attachment", id, ActionDelete, before, nil); err != nil {
			return err
		}
		_, err = tx.Exec(request.Context(), `DELETE FROM attachments WHERE tenant_id = $1 AND id = $2`, tenant, id)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	// Best-effort disk cleanup — the row is already gone.
	diskPath := filepath.Join(service.storageRoot, strconv.FormatInt(tenant, 10), fileKey)
	_ = os.Remove(diskPath)
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

func validOwnerType(ownerType string) bool {
	switch ownerType {
	case "journal_entry", "invoice", "payment", "grn", "delivery_order",
		"credit_note", "supplier_invoice", "supplier_payment", "purchase_return", "fixed_asset":
		return true
	}
	return false
}

// newFileKey returns a random hex string used as the on-disk file name.
func newFileKey() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(raw)
}

// int8OrNil returns nil for zero so pgx stores NULL, otherwise the int64.
func int8OrNil(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// sniffMimeType guesses a MIME type from the filename extension when the
// browser did not send a Content-Type header.
func sniffMimeType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".pdf":
		return "application/pdf"
	default:
		return ""
	}
}
