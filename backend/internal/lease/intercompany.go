package lease

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/db"
)

// Sentinel errors for the inter-company transaction endpoints.
var (
	errICTXEntryNotFound     = errors.New("journal entry not found for tenant")
	errICTXAlreadyEliminated = errors.New("inter-company transaction already eliminated")
)

// ---------------------------------------------------------------------------
// A-29: Inter-company transaction population endpoints.
//
// The inter_company_transactions table existed since migration 000024 and
// computeEliminations (consolidation.go) reads it, but nothing could ever
// write to it — the elimination feature could not be exercised end-to-end.
//
// This file provides:
//   - POST   /inter-company-transactions          register a transaction
//   - GET    /inter-company-transactions          list (filter by type/date)
//   - DELETE /inter-company-transactions/{id}     remove a wrongly-registered tx
//
// Registration validates that both tenants belong to one consolidation group
// (the counterparty must be a parent or child of the requesting tenant) so
// unrelated tenants cannot be linked. When a journal_entry_id is supplied it
// must exist and belong to the registering tenant; the RLS tenant policy on
// the table (migration 000024) enforces row isolation on top of this.
// ---------------------------------------------------------------------------

// validICTxTypes mirrors the CHECK constraint on inter_company_transactions.
var validICTxTypes = map[string]bool{
	"SALE":           true,
	"PURCHASE":       true,
	"LOAN":           true,
	"INTEREST":       true,
	"DIVIDEND":       true,
	"MANAGEMENT_FEE": true,
}

// CreateInterCompanyTxRequest is the body for POST /inter-company-transactions.
type CreateInterCompanyTxRequest struct {
	CounterpartyTenantID int64  `json:"counterparty_tenant_id"`
	TxType               string `json:"tx_type"`
	JournalEntryID       int64  `json:"journal_entry_id"`
	AmountCents          int64  `json:"amount_cents"`
	TxDate               string `json:"tx_date"`
	Description          string `json:"description"`
}

// interCompanyTxResponse is one row of the list/create response.
type interCompanyTxResponse struct {
	ID                   int64  `json:"id"`
	TenantID             int64  `json:"tenant_id"`
	CounterpartyTenantID int64  `json:"counterparty_tenant_id"`
	CounterpartyName     string `json:"counterparty_name,omitempty"`
	TxType               string `json:"tx_type"`
	JournalEntryID       *int64 `json:"journal_entry_id"`
	AmountCents          int64  `json:"amount_cents"`
	TxDate               string `json:"tx_date"`
	Description          string `json:"description,omitempty"`
	Eliminated           bool   `json:"eliminated"`
	CreatedAt            string `json:"created_at"`
}

// validateCreateInterCompanyTx checks the request body. Returns "" when valid.
func validateCreateInterCompanyTx(req CreateInterCompanyTxRequest) (string, string) {
	if req.CounterpartyTenantID <= 0 {
		return "INVALID_REQUEST", "counterparty_tenant_id is required"
	}
	if !validICTxTypes[req.TxType] {
		return "INVALID_REQUEST", "tx_type must be one of SALE, PURCHASE, LOAN, INTEREST, DIVIDEND, MANAGEMENT_FEE"
	}
	if req.AmountCents <= 0 {
		return "INVALID_REQUEST", "amount_cents must be greater than 0"
	}
	if !validDate(req.TxDate) {
		return "INVALID_REQUEST", "tx_date must be a valid YYYY-MM-DD date"
	}
	if req.JournalEntryID < 0 {
		return "INVALID_REQUEST", "journal_entry_id must be a positive id or omitted"
	}
	return "", ""
}

// CreateInterCompanyTx registers an inter-company transaction for elimination.
func (service *Service) CreateInterCompanyTx(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateInterCompanyTxRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateCreateInterCompanyTx(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	if req.CounterpartyTenantID == tenant {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "counterparty must be a different tenant")
		return
	}

	// The counterparty must be in the same consolidation group: either a
	// child of this tenant or its parent. This prevents linking arbitrary
	// tenants and keeps eliminations meaningful.
	related, err := service.isInConsolidationGroup(request.Context(), tenant, req.CounterpartyTenantID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ICTX_CREATE_FAILED", err.Error())
		return
	}
	if !related {
		writeError(writer, http.StatusBadRequest, "NOT_IN_GROUP",
			"counterparty_tenant_id is not in this tenant's consolidation group (register the entity hierarchy first)")
		return
	}

	var entryID *int64
	if req.JournalEntryID > 0 {
		entryID = &req.JournalEntryID
	}

	var result interCompanyTxResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		// When a journal entry is supplied it must belong to this tenant.
		if entryID != nil {
			var exists bool
			if err := tx.QueryRow(request.Context(), `
				SELECT EXISTS(SELECT 1 FROM journal_entries WHERE tenant_id = $1 AND id = $2)
			`, tenant, *entryID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return errICTXEntryNotFound
			}
		}
		if err := tx.QueryRow(request.Context(), `
			INSERT INTO inter_company_transactions
				(tenant_id, counterparty_tenant_id, tx_type, journal_entry_id, amount_cents, tx_date, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, tenant_id, counterparty_tenant_id, tx_type, journal_entry_id, amount_cents, tx_date, description, eliminated, created_at
		`, tenant, req.CounterpartyTenantID, req.TxType, entryID, req.AmountCents, req.TxDate, req.Description).
			Scan(&result.ID, &result.TenantID, &result.CounterpartyTenantID, &result.TxType,
				&result.JournalEntryID, &result.AmountCents, &result.TxDate, &result.Description,
				&result.Eliminated, &result.CreatedAt); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if err == errICTXEntryNotFound {
			writeError(writer, http.StatusBadRequest, "ENTRY_NOT_FOUND", "journal_entry_id does not exist for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "ICTX_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// ListInterCompanyTx lists inter-company transactions for the tenant, newest
// first. Optional ?tx_type= and ?eliminated= filters narrow the result.
func (service *Service) ListInterCompanyTx(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	txType := request.URL.Query().Get("tx_type")
	if txType != "" && !validICTxTypes[txType] {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "tx_type filter is not a valid type")
		return
	}
	eliminated := request.URL.Query().Get("eliminated")

	items := []interCompanyTxResponse{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT ic.id, ic.tenant_id, ic.counterparty_tenant_id,
			       COALESCE(t2.name, ''),
			       ic.tx_type, ic.journal_entry_id, ic.amount_cents, ic.tx_date,
			       COALESCE(ic.description, ''), ic.eliminated, ic.created_at
			FROM inter_company_transactions ic
			LEFT JOIN tenants t2 ON t2.id = ic.counterparty_tenant_id
			WHERE ic.tenant_id = $1
			  AND ($2 = '' OR ic.tx_type = $2)
			  AND ($3 = '' OR ic.eliminated = ($3 = 'true'))
			ORDER BY ic.tx_date DESC, ic.id DESC
			LIMIT 500
		`, tenant, txType, eliminated)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it interCompanyTxResponse
			var txDate time.Time
			if err := rows.Scan(&it.ID, &it.TenantID, &it.CounterpartyTenantID, &it.CounterpartyName,
				&it.TxType, &it.JournalEntryID, &it.AmountCents, &txDate,
				&it.Description, &it.Eliminated, &it.CreatedAt); err != nil {
				return err
			}
			it.TxDate = txDate.Format("2006-01-02")
			items = append(items, it)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "ICTX_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

// DeleteInterCompanyTx removes a registered inter-company transaction. Only
// non-eliminated rows can be deleted — an eliminated row already affected a
// consolidated report and must be kept for the audit trail.
func (service *Service) DeleteInterCompanyTx(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}

	var alreadyEliminated bool
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		if err := tx.QueryRow(request.Context(), `
			SELECT eliminated FROM inter_company_transactions
			WHERE tenant_id = $1 AND id = $2 FOR UPDATE
		`, tenant, id).Scan(&alreadyEliminated); err != nil {
			return err
		}
		if alreadyEliminated {
			return errICTXAlreadyEliminated
		}
		_, err := tx.Exec(request.Context(), `
			DELETE FROM inter_company_transactions WHERE tenant_id = $1 AND id = $2
		`, tenant, id)
		return err
	})
	if err != nil {
		if err == errICTXAlreadyEliminated {
			writeError(writer, http.StatusConflict, "ALREADY_ELIMINATED",
				"transaction was already eliminated in a consolidated report and cannot be deleted")
			return
		}
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "NOT_FOUND", "inter-company transaction not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "ICTX_DELETE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

// isInConsolidationGroup reports whether counterparty is the parent or a
// child of tenant (either direction of the entity_hierarchy edge).
func (service *Service) isInConsolidationGroup(ctx context.Context, tenant, counterparty int64) (bool, error) {
	var related bool
	err := db.WithTenantData(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM entity_hierarchy
				WHERE (tenant_id = $1 AND parent_tenant_id = $2)
				   OR (tenant_id = $2 AND parent_tenant_id = $1)
			)
		`, tenant, counterparty).Scan(&related)
	})
	return related, err
}
