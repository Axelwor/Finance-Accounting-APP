package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// Action is the set of mutating actions recorded in the audit trail. These
// match the CHECK constraint on audit_logs.action (migration 000023).
type Action string

const (
	ActionCreate Action = "CREATE"
	ActionUpdate Action = "UPDATE"
	ActionDelete Action = "DELETE"
	ActionPost   Action = "POST"
	ActionVoid   Action = "VOID"
	ActionClose  Action = "CLOSE"
	ActionUnlock Action = "UNLOCK"
)

// Log inserts one audit_logs row inside the caller's transaction. It is the
// single entry point used by every handler that mutates state. before and
// after are optional JSONB snapshots (nil leaves the column NULL). The
// caller is responsible for having set app.tenant_id on the transaction.
//
// Example:
//
// audit.Log(ctx, tx, tenantID, userID, "journal_entry", entryID,
//
//	audit.ActionPost, nil, snapshot)
func Log(ctx context.Context, tx pgx.Tx, tenantID, userID int64, entityType string, entityID int64, action Action, before, after any) error {
	var beforeBytes, afterBytes []byte
	var err error
	if before != nil {
		beforeBytes, err = json.Marshal(before)
		if err != nil {
			return fmt.Errorf("audit: marshal before_data: %w", err)
		}
	}
	if after != nil {
		afterBytes, err = json.Marshal(after)
		if err != nil {
			return fmt.Errorf("audit: marshal after_data: %w", err)
		}
	}
	var entityIDValue any
	if entityID > 0 {
		entityIDValue = entityID
	}
	var userIDValue any
	if userID > 0 {
		userIDValue = userID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (tenant_id, user_id, entity_type, entity_id, action, before_data, after_data)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb)
	`, tenantID, userIDValue, entityType, entityIDValue, string(action), beforeBytes, afterBytes)
	return err
}

// auditLogRow is the database row shape for listing audit logs.
type auditLogRow struct {
	ID         int64           `json:"id"`
	TenantID   int64           `json:"tenant_id"`
	UserID     int64           `json:"user_id"`
	UserName   string          `json:"user_name"`
	EntityType string          `json:"entity_type"`
	EntityID   int64           `json:"entity_id"`
	Action     string          `json:"action"`
	BeforeData json.RawMessage `json:"before_data"`
	AfterData  json.RawMessage `json:"after_data"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ListAuditLogs handles GET /audit-logs with optional filters:
//
//	entity_type, entity_id, user_id, from_date, to_date
//
// Results are ordered newest-first and capped at 200 rows.
func (service *Service) ListAuditLogs(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	query := request.URL.Query()
	args := []any{tenant}
	clauses := []string{"al.tenant_id = $1"}

	if entityType := query.Get("entity_type"); entityType != "" {
		args = append(args, entityType)
		clauses = append(clauses, fmt.Sprintf("al.entity_type = $%d", len(args)))
	}
	if raw := query.Get("entity_id"); raw != "" {
		eid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || eid <= 0 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "entity_id must be a positive integer")
			return
		}
		args = append(args, eid)
		clauses = append(clauses, fmt.Sprintf("al.entity_id = $%d", len(args)))
	}
	if raw := query.Get("user_id"); raw != "" {
		uid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || uid <= 0 {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "user_id must be a positive integer")
			return
		}
		args = append(args, uid)
		clauses = append(clauses, fmt.Sprintf("al.user_id = $%d", len(args)))
	}
	if raw := query.Get("from_date"); raw != "" {
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "from_date must be YYYY-MM-DD")
			return
		}
		args = append(args, raw)
		clauses = append(clauses, fmt.Sprintf("al.created_at >= $%d::date", len(args)))
	}
	if raw := query.Get("to_date"); raw != "" {
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "to_date must be YYYY-MM-DD")
			return
		}
		args = append(args, raw+" 23:59:59")
		clauses = append(clauses, fmt.Sprintf("al.created_at <= $%d::timestamp", len(args)))
	}

	sql := fmt.Sprintf(`
		SELECT al.id, al.tenant_id, COALESCE(al.user_id, 0), COALESCE(u.full_name, ''),
		       al.entity_type, COALESCE(al.entity_id, 0), al.action, al.before_data, al.after_data, al.created_at
		FROM audit_logs al
		LEFT JOIN users u ON u.id = al.user_id
		WHERE %s
		ORDER BY al.created_at DESC
		LIMIT 200
	`, joinAnd(clauses))

	rows, err := service.pool.Query(request.Context(), sql, args...)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "AUDIT_QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()

	logs := make([]auditLogRow, 0)
	for rows.Next() {
		var row auditLogRow
		var before, after []byte
		if err := rows.Scan(&row.ID, &row.TenantID, &row.UserID, &row.UserName,
			&row.EntityType, &row.EntityID, &row.Action, &before, &after, &row.CreatedAt); err != nil {
			writeError(writer, http.StatusInternalServerError, "AUDIT_QUERY_FAILED", err.Error())
			return
		}
		row.BeforeData = jsonNull(before)
		row.AfterData = jsonNull(after)
		logs = append(logs, row)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "AUDIT_QUERY_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, logs)
}

func joinAnd(clauses []string) string {
	result := ""
	for i, c := range clauses {
		if i > 0 {
			result += " AND "
		}
		result += c
	}
	return result
}

// jsonNull returns nil for an empty/nil byte slice so the JSON field renders
// as null instead of an empty string; non-empty JSON is passed through.
func jsonNull(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
