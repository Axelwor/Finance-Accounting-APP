package assets

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

// F-13: Asset maintenance tracking. Records maintenance activities per fixed
// asset and exposes upcoming-due maintenance for planning. Maintenance cost
// is recorded but NOT auto-posted — expense it via cash-out/journal if the
// tenant wants it in the ledger.

const (
	maintenanceTypeRoutine    = "ROUTINE"
	maintenanceTypeRepair     = "REPAIR"
	maintenanceTypeInspection = "INSPECTION"
	maintenanceTypeOverhaul   = "OVERHAUL"
	maintenanceTypeOther      = "OTHER"
)

type maintenanceRequest struct {
	AssetID         int64  `json:"asset_id"`
	MaintenanceDate string `json:"maintenance_date"`
	MaintenanceType string `json:"maintenance_type"`
	Description     string `json:"description"`
	CostCents       int64  `json:"cost_cents"`
	PerformedBy     string `json:"performed_by"`
	NextDueDate     string `json:"next_due_date"`
}

type maintenanceResponse struct {
	ID              int64  `json:"id"`
	AssetID         int64  `json:"asset_id"`
	AssetCode       string `json:"asset_code"`
	AssetName       string `json:"asset_name"`
	MaintenanceDate string `json:"maintenance_date"`
	MaintenanceType string `json:"maintenance_type"`
	Description     string `json:"description"`
	CostCents       int64  `json:"cost_cents"`
	PerformedBy     string `json:"performed_by"`
	NextDueDate     string `json:"next_due_date"`
}

func validateMaintenance(req maintenanceRequest) (string, string) {
	if req.AssetID <= 0 {
		return "INVALID_REQUEST", "asset_id is required"
	}
	if !validDate(req.MaintenanceDate) {
		return "INVALID_REQUEST", "maintenance_date must be a valid YYYY-MM-DD date"
	}
	switch strings.ToUpper(strings.TrimSpace(req.MaintenanceType)) {
	case maintenanceTypeRoutine, maintenanceTypeRepair, maintenanceTypeInspection, maintenanceTypeOverhaul, maintenanceTypeOther:
	case "":
		return "INVALID_REQUEST", "maintenance_type is required"
	default:
		return "INVALID_REQUEST", "maintenance_type must be ROUTINE, REPAIR, INSPECTION, OVERHAUL, or OTHER"
	}
	if req.CostCents < 0 {
		return "INVALID_REQUEST", "cost_cents must be >= 0"
	}
	if req.NextDueDate != "" && !validDate(req.NextDueDate) {
		return "INVALID_REQUEST", "next_due_date must be a valid YYYY-MM-DD date"
	}
	return "", ""
}

// CreateMaintenance records a maintenance activity for a fixed asset.
func (service *Service) CreateMaintenance(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req maintenanceRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, msg := validateMaintenance(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	uid := userID(request)

	var id int64
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		// Verify the asset exists for this tenant.
		var exists bool
		if err := tx.QueryRow(request.Context(),
			`SELECT EXISTS(SELECT 1 FROM fixed_assets WHERE tenant_id = $1 AND id = $2)`,
			tenant, req.AssetID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}
		maintDate, err := parseDate(req.MaintenanceDate)
		if err != nil {
			return err
		}
		var nextDue pgtype.Date
		if req.NextDueDate != "" {
			nextDue, err = parseDate(req.NextDueDate)
			if err != nil {
				return err
			}
		}
		return tx.QueryRow(request.Context(), `
			INSERT INTO asset_maintenance
			    (tenant_id, asset_id, maintenance_date, maintenance_type, description,
			     cost_cents, performed_by, next_due_date, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id
		`, tenant, req.AssetID, maintDate, strings.ToUpper(strings.TrimSpace(req.MaintenanceType)),
			req.Description, req.CostCents, textValueOptional(req.PerformedBy), nextDue, int8Value(uid)).Scan(&id)
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "ASSET_NOT_FOUND", "asset not found for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "MAINTENANCE_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]int64{"id": id})
}

// ListMaintenance lists maintenance activities, optionally filtered by asset.
func (service *Service) ListMaintenance(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	query := `
		SELECT m.id, m.asset_id, a.code, a.name, m.maintenance_date, m.maintenance_type,
		       m.description, m.cost_cents, m.performed_by, m.next_due_date
		FROM asset_maintenance m
		JOIN fixed_assets a ON a.tenant_id = m.tenant_id AND a.id = m.asset_id
		WHERE m.tenant_id = $1
	`
	args := []any{tenant}
	if assetID := strings.TrimSpace(request.URL.Query().Get("asset_id")); assetID != "" {
		if id, err := strconv.ParseInt(assetID, 10, 64); err == nil && id > 0 {
			query += ` AND m.asset_id = $2`
			args = append(args, id)
		}
	}
	query += ` ORDER BY m.maintenance_date DESC, m.id DESC LIMIT 200`

	rows, err := service.pool.Query(request.Context(), query, args...)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "MAINTENANCE_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	list := []maintenanceResponse{}
	for rows.Next() {
		var row maintenanceResponse
		var maintDate pgtype.Date
		var performedBy pgtype.Text
		var nextDue pgtype.Date
		if err := rows.Scan(&row.ID, &row.AssetID, &row.AssetCode, &row.AssetName,
			&maintDate, &row.MaintenanceType, &row.Description, &row.CostCents,
			&performedBy, &nextDue); err != nil {
			writeError(writer, http.StatusInternalServerError, "MAINTENANCE_LIST_FAILED", err.Error())
			return
		}
		row.MaintenanceDate = dateString(maintDate)
		row.PerformedBy = textValue(performedBy)
		row.NextDueDate = dateString(nextDue)
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "MAINTENANCE_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, list)
}

// UpcomingMaintenance lists maintenance entries whose next_due_date is within
// the horizon (days, default 30) for proactive planning.
func (service *Service) UpcomingMaintenance(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	days := 30
	if raw := strings.TrimSpace(request.URL.Query().Get("days")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			days = parsed
		}
	}
	rows, err := service.pool.Query(request.Context(), `
		SELECT m.id, m.asset_id, a.code, a.name, m.next_due_date, m.maintenance_type, m.description
		FROM asset_maintenance m
		JOIN fixed_assets a ON a.tenant_id = m.tenant_id AND a.id = m.asset_id
		WHERE m.tenant_id = $1
		  AND m.next_due_date IS NOT NULL
		  AND m.next_due_date <= CURRENT_DATE + $2
		ORDER BY m.next_due_date
	`, tenant, days)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "MAINTENANCE_DUE_FAILED", err.Error())
		return
	}
	defer rows.Close()

	list := []map[string]any{}
	for rows.Next() {
		var id, assetID int64
		var code, name, mType, desc string
		var nextDue pgtype.Date
		if err := rows.Scan(&id, &assetID, &code, &name, &nextDue, &mType, &desc); err != nil {
			writeError(writer, http.StatusInternalServerError, "MAINTENANCE_DUE_FAILED", err.Error())
			return
		}
		list = append(list, map[string]any{
			"id": id, "asset_id": assetID, "asset_code": code, "asset_name": name,
			"next_due_date": dateString(nextDue), "maintenance_type": mType, "description": desc,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "MAINTENANCE_DUE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"horizon_days": days, "upcoming": list})
}
