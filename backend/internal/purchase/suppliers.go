package purchase

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"finance-accounting-app/backend/internal/db"
)

type CreateSupplierRequest struct {
	Code                string `json:"code"`
	Name                string `json:"name"`
	NPWP                string `json:"npwp"`
	ContactPerson       string `json:"contact_person"`
	Phone               string `json:"phone"`
	Email               string `json:"email"`
	Address             string `json:"address"`
	City                string `json:"city"`
	Province            string `json:"province"`
	PostalCode          string `json:"postal_code"`
	PaymentTermID       int64  `json:"payment_term_id"`
	CreditLimitCents    int64  `json:"credit_limit_cents"`
	SupplierType        string `json:"supplier_type"`
	IsPKP               bool   `json:"is_pkp"`
	CurrencyCode        string `json:"currency_code"`
	BankName            string `json:"bank_name"`
	BankAccountNumber   string `json:"bank_account_number"`
	BankAccountName     string `json:"bank_account_name"`
	Website             string `json:"website"`
	Fax                 string `json:"fax"`
	ContactPerson2      string `json:"contact_person_2"`
	Phone2              string `json:"phone_2"`
	OpeningBalanceCents int64  `json:"opening_balance_cents"`
	OpeningBalanceDate  string `json:"opening_balance_date"`
}

type supplierResponse struct {
	ID                  int64  `json:"id"`
	Code                string `json:"code"`
	Name                string `json:"name"`
	NPWP                string `json:"npwp"`
	ContactPerson       string `json:"contact_person"`
	Phone               string `json:"phone"`
	Email               string `json:"email"`
	Address             string `json:"address"`
	City                string `json:"city"`
	Province            string `json:"province"`
	PostalCode          string `json:"postal_code"`
	PaymentTermID       int64  `json:"payment_term_id"`
	CreditLimitCents    int64  `json:"credit_limit_cents"`
	IsActive            bool   `json:"is_active"`
	SupplierType        string `json:"supplier_type"`
	IsPKP               bool   `json:"is_pkp"`
	CurrencyCode        string `json:"currency_code"`
	BankName            string `json:"bank_name"`
	BankAccountNumber   string `json:"bank_account_number"`
	BankAccountName     string `json:"bank_account_name"`
	Website             string `json:"website"`
	Fax                 string `json:"fax"`
	ContactPerson2      string `json:"contact_person_2"`
	Phone2              string `json:"phone_2"`
	OpeningBalanceCents int64  `json:"opening_balance_cents"`
	OpeningBalanceDate  string `json:"opening_balance_date"`
}

// supplierColumns lists the supplier columns shared by INSERT ... RETURNING
// and SELECT, in the order scanned by supplierScanDest.
const supplierColumns = `id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active,
	supplier_type, is_pkp, currency_code, bank_name, bank_account_number, bank_account_name, website, fax, contact_person_2, phone_2, opening_balance_cents, opening_balance_date`

// supplierRow mirrors the suppliers table with every nullable column typed as
// pgtype.Text/pgtype.Int8/pgtype.Date so NULL values scan cleanly (QA-04:
// scanning NULL into *string/*int64 made a minimal supplier impossible to
// create).
type supplierRow struct {
	ID                  int64
	Code                string
	Name                string
	NPWP                pgtype.Text
	ContactPerson       pgtype.Text
	Phone               pgtype.Text
	Email               pgtype.Text
	Address             pgtype.Text
	City                pgtype.Text
	Province            pgtype.Text
	PostalCode          pgtype.Text
	PaymentTermID       pgtype.Int8
	CreditLimitCents    pgtype.Int8
	IsActive            bool
	SupplierType        pgtype.Text
	IsPKP               bool
	CurrencyCode        pgtype.Text
	BankName            pgtype.Text
	BankAccountNumber   pgtype.Text
	BankAccountName     pgtype.Text
	Website             pgtype.Text
	Fax                 pgtype.Text
	ContactPerson2      pgtype.Text
	Phone2              pgtype.Text
	OpeningBalanceCents pgtype.Int8
	OpeningBalanceDate  pgtype.Date
}

// supplierScanDest returns the scan destinations matching supplierColumns.
func supplierScanDest(row *supplierRow) []any {
	return []any{&row.ID, &row.Code, &row.Name, &row.NPWP, &row.ContactPerson, &row.Phone, &row.Email,
		&row.Address, &row.City, &row.Province, &row.PostalCode, &row.PaymentTermID, &row.CreditLimitCents, &row.IsActive,
		&row.SupplierType, &row.IsPKP, &row.CurrencyCode, &row.BankName, &row.BankAccountNumber, &row.BankAccountName,
		&row.Website, &row.Fax, &row.ContactPerson2, &row.Phone2, &row.OpeningBalanceCents, &row.OpeningBalanceDate}
}

// response converts a scanned row into the API response; NULL optional
// columns map to zero values ("", 0) instead of scan errors.
func (row supplierRow) response() supplierResponse {
	result := supplierResponse{
		ID:       row.ID,
		Code:     row.Code,
		Name:     row.Name,
		IsActive: row.IsActive,
		IsPKP:    row.IsPKP,
	}
	result.NPWP = textValue(row.NPWP)
	result.ContactPerson = textValue(row.ContactPerson)
	result.Phone = textValue(row.Phone)
	result.Email = textValue(row.Email)
	result.Address = textValue(row.Address)
	result.City = textValue(row.City)
	result.Province = textValue(row.Province)
	result.PostalCode = textValue(row.PostalCode)
	if row.PaymentTermID.Valid {
		result.PaymentTermID = row.PaymentTermID.Int64
	}
	if row.CreditLimitCents.Valid {
		result.CreditLimitCents = row.CreditLimitCents.Int64
	}
	result.SupplierType = textValue(row.SupplierType)
	result.CurrencyCode = textValue(row.CurrencyCode)
	result.BankName = textValue(row.BankName)
	result.BankAccountNumber = textValue(row.BankAccountNumber)
	result.BankAccountName = textValue(row.BankAccountName)
	result.Website = textValue(row.Website)
	result.Fax = textValue(row.Fax)
	result.ContactPerson2 = textValue(row.ContactPerson2)
	result.Phone2 = textValue(row.Phone2)
	if row.OpeningBalanceCents.Valid {
		result.OpeningBalanceCents = row.OpeningBalanceCents.Int64
	}
	result.OpeningBalanceDate = dateString(row.OpeningBalanceDate)
	return result
}

// normalizeSupplierType uppercases the supplier_type input so lowercase
// values such as "goods" are accepted; the DB CHECK only allows
// GOODS/SERVICE/MIXED.
func normalizeSupplierType(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func (service *Service) CreateSupplier(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateSupplierRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateSupplierRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}
	supplierOpeningBalanceDate, err := optionalDate(req.OpeningBalanceDate)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "opening_balance_date must be a valid YYYY-MM-DD date")
		return
	}

	var result supplierResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var row supplierRow
		err := tx.QueryRow(request.Context(), `
			INSERT INTO suppliers (tenant_id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents,
				supplier_type, is_pkp, currency_code, bank_name, bank_account_number, bank_account_name, website, fax, contact_person_2, phone_2, opening_balance_cents, opening_balance_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
			RETURNING `+supplierColumns+`
		`, tenant, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
			textValueOptional(req.NPWP), textValueOptional(req.ContactPerson), textValueOptional(req.Phone), textValueOptional(req.Email),
			textValueOptional(req.Address), textValueOptional(req.City), textValueOptional(req.Province), textValueOptional(req.PostalCode),
			optionalInt8(req.PaymentTermID), optionalInt8(req.CreditLimitCents),
			textValueOptional(normalizeSupplierType(req.SupplierType)), req.IsPKP, textValueOptional(req.CurrencyCode),
			textValueOptional(req.BankName), textValueOptional(req.BankAccountNumber), textValueOptional(req.BankAccountName),
			textValueOptional(req.Website), textValueOptional(req.Fax), textValueOptional(req.ContactPerson2), textValueOptional(req.Phone2),
			req.OpeningBalanceCents, supplierOpeningBalanceDate,
		).Scan(supplierScanDest(&row)...)
		if err != nil {
			return err
		}
		result = row.response()
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "SUPPLIER_EXISTS", "supplier code already exists")
			return
		}
		if isCheckViolation(err) {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid supplier field value: supplier_type must be GOODS, SERVICE, or MIXED")
			return
		}
		writeError(writer, http.StatusBadRequest, "SUPPLIER_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) ListSuppliers(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var results []supplierResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		rows, err := tx.Query(request.Context(), `
			SELECT `+supplierColumns+`
			FROM suppliers
			ORDER BY name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []supplierResponse{}
		for rows.Next() {
			var row supplierRow
			if err := rows.Scan(supplierScanDest(&row)...); err != nil {
				return err
			}
			results = append(results, row.response())
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "SUPPLIER_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

func (service *Service) GetSupplier(writer http.ResponseWriter, request *http.Request) {
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
	var result supplierResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var row supplierRow
		err := tx.QueryRow(request.Context(), `
			SELECT `+supplierColumns+`
			FROM suppliers WHERE tenant_id = $1 AND id = $2
		`, tenant, id).Scan(supplierScanDest(&row)...)
		if err != nil {
			return err
		}
		result = row.response()
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "SUPPLIER_NOT_FOUND", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) UpdateSupplier(writer http.ResponseWriter, request *http.Request) {
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
	var req CreateSupplierRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, msg := validateSupplierRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, msg)
		return
	}

	var result supplierResponse
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		var row supplierRow

		// The opening balance is immutable after create; all other master
		// fields are replaced with the request values.
		err := tx.QueryRow(request.Context(), `
			UPDATE suppliers SET
				code = $3, name = $4, npwp = $5, contact_person = $6, phone = $7, email = $8,
				address = $9, city = $10, province = $11, postal_code = $12,
				payment_term_id = $13, credit_limit_cents = $14,
				supplier_type = $15, is_pkp = $16, currency_code = $17,
				bank_name = $18, bank_account_number = $19, bank_account_name = $20,
				website = $21, fax = $22, contact_person_2 = $23, phone_2 = $24,
				updated_at = now()
			WHERE tenant_id = $1 AND id = $2
			RETURNING `+supplierColumns+`
		`, tenant, id, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
			textValueOptional(req.NPWP), textValueOptional(req.ContactPerson), textValueOptional(req.Phone), textValueOptional(req.Email),
			textValueOptional(req.Address), textValueOptional(req.City), textValueOptional(req.Province), textValueOptional(req.PostalCode),
			optionalInt8(req.PaymentTermID), optionalInt8(req.CreditLimitCents),
			textValueOptional(normalizeSupplierType(req.SupplierType)), req.IsPKP, textValueOptional(req.CurrencyCode),
			textValueOptional(req.BankName), textValueOptional(req.BankAccountNumber), textValueOptional(req.BankAccountName),
			textValueOptional(req.Website), textValueOptional(req.Fax), textValueOptional(req.ContactPerson2), textValueOptional(req.Phone2),
		).Scan(supplierScanDest(&row)...)
		if err != nil {
			return err
		}
		result = row.response()
		return nil
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier does not exist for this tenant")
			return
		}
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "SUPPLIER_EXISTS", "supplier code already exists")
			return
		}
		if isCheckViolation(err) {
			writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid supplier field value: supplier_type must be GOODS, SERVICE, or MIXED")
			return
		}
		writeError(writer, http.StatusBadRequest, "SUPPLIER_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) DeactivateSupplier(writer http.ResponseWriter, request *http.Request) {
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
	err = db.WithTransaction(request.Context(), service.pool, func(tx pgx.Tx) error {
		if err := withTenant(request.Context(), tx, tenant); err != nil {
			return err
		}
		_, err := tx.Exec(request.Context(), `
			UPDATE suppliers SET is_active = false, updated_at = $1 WHERE tenant_id = $2 AND id = $3
		`, time.Now(), tenant, id)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "SUPPLIER_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "is_active": false})
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

func validateSupplierRequest(req CreateSupplierRequest) (string, string) {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return "INVALID_REQUEST", "code and name are required"
	}
	switch normalizeSupplierType(req.SupplierType) {
	case "", "GOODS", "SERVICE", "MIXED":
	default:
		return "INVALID_REQUEST", "supplier_type must be one of GOODS, SERVICE, MIXED"
	}
	return "", ""
}
