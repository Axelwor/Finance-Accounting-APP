package coa

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// Export returns the tenant's chart of accounts as CSV (i-004).
// GET /accounts/export → CSV download with header row.
func (service *Service) Export(writer http.ResponseWriter, request *http.Request) {
	tenantID, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var rows []accountRow
	err = withTenantRead(request.Context(), service.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rs, err := tx.Query(ctx, `
			SELECT `+accountColumns+`
			FROM accounts
			WHERE tenant_id = $1
			ORDER BY code
		`, tenantID)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var row accountRow
			if err := rs.Scan(&row.ID, &row.Code, &row.Name, &row.ReportGroup, &row.AccountType,
				&row.ParentID, &row.IsGroup, &row.IsActive, &row.ValidFrom, &row.ValidTo); err != nil {
				return err
			}
			rows = append(rows, row)
		}
		return rs.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ACCOUNTS_EXPORT_FAILED", err.Error())
		return
	}

	writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	writer.Header().Set("Content-Disposition", `attachment; filename="chart_of_accounts.csv"`)
	w := csv.NewWriter(writer)
	defer w.Flush()

	// Header row
	_ = w.Write([]string{"id", "code", "name", "report_group", "account_type", "parent_id", "is_group", "is_active", "valid_from", "valid_to"})
	for _, row := range rows {
		parentID := ""
		if row.ParentID.Valid {
			parentID = strconv.FormatInt(row.ParentID.Int64, 10)
		}
		isGroup := "false"
		if row.IsGroup {
			isGroup = "true"
		}
		isActive := "false"
		if row.IsActive {
			isActive = "true"
		}
		validFrom := ""
		if row.ValidFrom.Valid {
			validFrom = row.ValidFrom.Time.Format("2006-01-02")
		}
		validTo := ""
		if row.ValidTo.Valid {
			validTo = row.ValidTo.Time.Format("2006-01-02")
		}
		_ = w.Write([]string{
			strconv.FormatInt(row.ID, 10),
			row.Code,
			row.Name,
			row.ReportGroup,
			row.AccountType,
			parentID,
			isGroup,
			isActive,
			validFrom,
			validTo,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		// Headers already sent; log only.
		fmt.Printf("coa export csv write error: %v\n", err)
	}
}
