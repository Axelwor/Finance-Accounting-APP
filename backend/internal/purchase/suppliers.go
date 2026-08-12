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
		var npwp, contactPerson, phone, email, address, city, province, postalCode pgtype.Text
		if req.NPWP != "" {
			npwp = textValueOptional(req.NPWP)
		}
		if req.ContactPerson != "" {
			contactPerson = textValueOptional(req.ContactPerson)
		}
		if req.Phone != "" {
			phone = textValueOptional(req.Phone)
		}
		if req.Email != "" {
			email = textValueOptional(req.Email)
		}
		if req.Address != "" {
			address = textValueOptional(req.Address)
		}
		if req.City != "" {
			city = textValueOptional(req.City)
		}
		if req.Province != "" {
			province = textValueOptional(req.Province)
		}
		if req.PostalCode != "" {
			postalCode = textValueOptional(req.PostalCode)
		}

		supplierType, currencyCode := pgtype.Text{}, pgtype.Text{}
		bankName, bankAccountNumber, bankAccountName := pgtype.Text{}, pgtype.Text{}, pgtype.Text{}
		website, fax, contactPerson2, phone2 := pgtype.Text{}, pgtype.Text{}, pgtype.Text{}, pgtype.Text{}
		openingBalanceCents := pgtype.Int8{}
		openingBalanceDate := pgtype.Date{}

		err := tx.QueryRow(request.Context(), `
			INSERT INTO suppliers (tenant_id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents,
				supplier_type, is_pkp, currency_code, bank_name, bank_account_number, bank_account_name, website, fax, contact_person_2, phone_2, opening_balance_cents, opening_balance_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
			RETURNING id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active,
				supplier_type, is_pkp, currency_code, bank_name, bank_account_number, bank_account_name, website, fax, contact_person_2, phone_2, opening_balance_cents, opening_balance_date
		`, tenant, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
			npwp, contactPerson, phone, email, address, city, province, postalCode,
			optionalInt8(req.PaymentTermID), optionalInt8(req.CreditLimitCents),
			textValueOptional(req.SupplierType), req.IsPKP, textValueOptional(req.CurrencyCode),
			textValueOptional(req.BankName), textValueOptional(req.BankAccountNumber), textValueOptional(req.BankAccountName),
			textValueOptional(req.Website), textValueOptional(req.Fax), textValueOptional(req.ContactPerson2), textValueOptional(req.Phone2),
			req.OpeningBalanceCents, supplierOpeningBalanceDate,
		).Scan(&result.ID, &result.Code, &result.Name, &result.NPWP, &result.ContactPerson,
			&result.Phone, &result.Email, &result.Address, &result.City, &result.Province,
			&result.PostalCode, &result.PaymentTermID, &result.CreditLimitCents, &result.IsActive,
			&supplierType, &result.IsPKP, &currencyCode, &bankName, &bankAccountNumber, &bankAccountName,
			&website, &fax, &contactPerson2, &phone2, &openingBalanceCents, &openingBalanceDate)
		if err != nil {
			return err
		}
		result.SupplierType = textValue(supplierType)
		result.CurrencyCode = textValue(currencyCode)
		result.BankName = textValue(bankName)
		result.BankAccountNumber = textValue(bankAccountNumber)
		result.BankAccountName = textValue(bankAccountName)
		result.Website = textValue(website)
		result.Fax = textValue(fax)
		result.ContactPerson2 = textValue(contactPerson2)
		result.Phone2 = textValue(phone2)
		if openingBalanceCents.Valid {
			result.OpeningBalanceCents = openingBalanceCents.Int64
		}
		result.OpeningBalanceDate = dateString(openingBalanceDate)
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "SUPPLIER_EXISTS", "supplier code already exists")
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
			SELECT id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active,
				supplier_type, is_pkp, currency_code, bank_name, bank_account_number, bank_account_name, website, fax, contact_person_2, phone_2, opening_balance_cents, opening_balance_date
			FROM suppliers
			ORDER BY name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []supplierResponse{}
		for rows.Next() {
			var s supplierResponse
			var npwp, contactPerson, phone, email, address, city, province, postalCode pgtype.Text
			var paymentTermID, creditLimit, openingBalanceCents pgtype.Int8
			var supplierType, currencyCode pgtype.Text
			var bankName, bankAccountNumber, bankAccountName pgtype.Text
			var website, fax, contactPerson2, phone2 pgtype.Text
			var openingBalanceDate pgtype.Date
			if err := rows.Scan(&s.ID, &s.Code, &s.Name, &npwp, &contactPerson, &phone, &email,
				&address, &city, &province, &postalCode, &paymentTermID, &creditLimit, &s.IsActive,
				&supplierType, &s.IsPKP, &currencyCode, &bankName, &bankAccountNumber, &bankAccountName,
				&website, &fax, &contactPerson2, &phone2, &openingBalanceCents, &openingBalanceDate); err != nil {
				return err
			}
			s.NPWP = textValue(npwp)
			s.ContactPerson = textValue(contactPerson)
			s.Phone = textValue(phone)
			s.Email = textValue(email)
			s.Address = textValue(address)
			s.City = textValue(city)
			s.Province = textValue(province)
			s.PostalCode = textValue(postalCode)
			if paymentTermID.Valid {
				s.PaymentTermID = paymentTermID.Int64
			}
			if creditLimit.Valid {
				s.CreditLimitCents = creditLimit.Int64
			}
			s.SupplierType = textValue(supplierType)
			s.CurrencyCode = textValue(currencyCode)
			s.BankName = textValue(bankName)
			s.BankAccountNumber = textValue(bankAccountNumber)
			s.BankAccountName = textValue(bankAccountName)
			s.Website = textValue(website)
			s.Fax = textValue(fax)
			s.ContactPerson2 = textValue(contactPerson2)
			s.Phone2 = textValue(phone2)
			if openingBalanceCents.Valid {
				s.OpeningBalanceCents = openingBalanceCents.Int64
			}
			s.OpeningBalanceDate = dateString(openingBalanceDate)
			results = append(results, s)
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
		var npwp, contactPerson, phone, email, address, city, province, postalCode pgtype.Text
		var paymentTermID, creditLimit, openingBalanceCents pgtype.Int8
		var supplierType, currencyCode pgtype.Text
		var bankName, bankAccountNumber, bankAccountName pgtype.Text
		var website, fax, contactPerson2, phone2 pgtype.Text
		var openingBalanceDate pgtype.Date
		err := tx.QueryRow(request.Context(), `
			SELECT id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active,
				supplier_type, is_pkp, currency_code, bank_name, bank_account_number, bank_account_name, website, fax, contact_person_2, phone_2, opening_balance_cents, opening_balance_date
			FROM suppliers WHERE tenant_id = $1 AND id = $2
		`, tenant, id).Scan(&result.ID, &result.Code, &result.Name, &npwp, &contactPerson, &phone, &email,
			&address, &city, &province, &postalCode, &paymentTermID, &creditLimit, &result.IsActive,
			&supplierType, &result.IsPKP, &currencyCode, &bankName, &bankAccountNumber, &bankAccountName,
			&website, &fax, &contactPerson2, &phone2, &openingBalanceCents, &openingBalanceDate)
		if err != nil {
			return err
		}
		result.NPWP = textValue(npwp)
		result.ContactPerson = textValue(contactPerson)
		result.Phone = textValue(phone)
		result.Email = textValue(email)
		result.Address = textValue(address)
		result.City = textValue(city)
		result.Province = textValue(province)
		result.PostalCode = textValue(postalCode)
		if paymentTermID.Valid {
			result.PaymentTermID = paymentTermID.Int64
		}
		if creditLimit.Valid {
			result.CreditLimitCents = creditLimit.Int64
		}
		result.SupplierType = textValue(supplierType)
		result.CurrencyCode = textValue(currencyCode)
		result.BankName = textValue(bankName)
		result.BankAccountNumber = textValue(bankAccountNumber)
		result.BankAccountName = textValue(bankAccountName)
		result.Website = textValue(website)
		result.Fax = textValue(fax)
		result.ContactPerson2 = textValue(contactPerson2)
		result.Phone2 = textValue(phone2)
		if openingBalanceCents.Valid {
			result.OpeningBalanceCents = openingBalanceCents.Int64
		}
		result.OpeningBalanceDate = dateString(openingBalanceDate)
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
		var npwp, contactPerson, phone, email, address, city, province, postalCode pgtype.Text
		if req.NPWP != "" {
			npwp = textValueOptional(req.NPWP)
		}
		if req.ContactPerson != "" {
			contactPerson = textValueOptional(req.ContactPerson)
		}
		if req.Phone != "" {
			phone = textValueOptional(req.Phone)
		}
		if req.Email != "" {
			email = textValueOptional(req.Email)
		}
		if req.Address != "" {
			address = textValueOptional(req.Address)
		}
		if req.City != "" {
			city = textValueOptional(req.City)
		}
		if req.Province != "" {
			province = textValueOptional(req.Province)
		}
		if req.PostalCode != "" {
			postalCode = textValueOptional(req.PostalCode)
		}

		var supplierType, currencyCode pgtype.Text
		var bankName, bankAccountNumber, bankAccountName pgtype.Text
		var website, fax, contactPerson2, phone2 pgtype.Text
		var openingBalanceCents pgtype.Int8
		var openingBalanceDate pgtype.Date

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
			RETURNING id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active,
				supplier_type, is_pkp, currency_code, bank_name, bank_account_number, bank_account_name, website, fax, contact_person_2, phone_2, opening_balance_cents, opening_balance_date
		`, tenant, id, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
			npwp, contactPerson, phone, email, address, city, province, postalCode,
			optionalInt8(req.PaymentTermID), optionalInt8(req.CreditLimitCents),
			textValueOptional(req.SupplierType), req.IsPKP, textValueOptional(req.CurrencyCode),
			textValueOptional(req.BankName), textValueOptional(req.BankAccountNumber), textValueOptional(req.BankAccountName),
			textValueOptional(req.Website), textValueOptional(req.Fax), textValueOptional(req.ContactPerson2), textValueOptional(req.Phone2),
		).Scan(&result.ID, &result.Code, &result.Name, &result.NPWP, &result.ContactPerson,
			&result.Phone, &result.Email, &result.Address, &result.City, &result.Province,
			&result.PostalCode, &result.PaymentTermID, &result.CreditLimitCents, &result.IsActive,
			&supplierType, &result.IsPKP, &currencyCode, &bankName, &bankAccountNumber, &bankAccountName,
			&website, &fax, &contactPerson2, &phone2, &openingBalanceCents, &openingBalanceDate)
		if err != nil {
			return err
		}
		result.SupplierType = textValue(supplierType)
		result.CurrencyCode = textValue(currencyCode)
		result.BankName = textValue(bankName)
		result.BankAccountNumber = textValue(bankAccountNumber)
		result.BankAccountName = textValue(bankAccountName)
		result.Website = textValue(website)
		result.Fax = textValue(fax)
		result.ContactPerson2 = textValue(contactPerson2)
		result.Phone2 = textValue(phone2)
		if openingBalanceCents.Valid {
			result.OpeningBalanceCents = openingBalanceCents.Int64
		}
		result.OpeningBalanceDate = dateString(openingBalanceDate)
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
	return "", ""
}
