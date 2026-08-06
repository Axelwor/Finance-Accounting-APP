package coa

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type createReportMappingRequest struct {
	AccountID  int64  `json:"account_id"`
	ReportType string `json:"report_type"`
	ReportLine string `json:"report_line"`
	Priority   int    `json:"priority"`
}

// CreateReportMapping validates the payload and inserts a new mapping from an
// account to a report line. report_mappings has no RLS policy, so the tenant
// id is still set transaction-locally and every statement is explicitly
// tenant-scoped.
func (service *Service) CreateReportMapping(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req createReportMappingRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateReportMappingInput(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}

	var created reportMapping
	err = pgx.BeginFunc(request.Context(), service.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprintf("%d", tenantID)); err != nil {
			return err
		}
		if err := accountExists(request.Context(), tx, tenantID, req.AccountID); err != nil {
			return err
		}
		return tx.QueryRow(request.Context(), `
			INSERT INTO report_mappings (tenant_id, account_id, report_type, report_line, priority)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, account_id, report_type, report_line, priority`,
			tenantID, req.AccountID, req.ReportType, req.ReportLine, req.Priority,
		).Scan(&created.ID, &created.AccountID, &created.ReportType, &created.ReportLine, &created.Priority)
	})
	if err != nil {
		writeCreateReportMappingError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, created)
}

// reportMapping is the JSON representation of a report_mappings row.
type reportMapping struct {
	ID         int64  `json:"id"`
	AccountID  int64  `json:"account_id"`
	ReportType string `json:"report_type"`
	ReportLine string `json:"report_line"`
	Priority   int    `json:"priority"`
}

func writeCreateReportMappingError(writer http.ResponseWriter, err error) {
	var validationErr *parentValidationError
	if errors.As(err, &validationErr) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", validationErr.Error())
		return
	}
	if isUniqueViolation(err) {
		writeError(writer, http.StatusConflict, "REPORT_MAPPING_EXISTS", "an account can only map to one line per report type")
		return
	}
	if isForeignKeyViolation(err) {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "account does not exist for this tenant")
		return
	}
	writeError(writer, http.StatusInternalServerError, "REPORT_MAPPING_CREATE_FAILED", err.Error())
}
