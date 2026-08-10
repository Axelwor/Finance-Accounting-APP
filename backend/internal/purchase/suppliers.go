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
	Code             string `json:"code"`
	Name             string `json:"name"`
	NPWP             string `json:"npwp"`
	ContactPerson    string `json:"contact_person"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	Address          string `json:"address"`
	City             string `json:"city"`
	Province         string `json:"province"`
	PostalCode       string `json:"postal_code"`
	PaymentTermID    int64  `json:"payment_term_id"`
	CreditLimitCents int64  `json:"credit_limit_cents"`
}

type supplierResponse struct {
	ID               int64  `json:"id"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	NPWP             string `json:"npwp"`
	ContactPerson    string `json:"contact_person"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	Address          string `json:"address"`
	City             string `json:"city"`
	Province         string `json:"province"`
	PostalCode       string `json:"postal_code"`
	PaymentTermID    int64  `json:"payment_term_id"`
	CreditLimitCents int64  `json:"credit_limit_cents"`
	IsActive         bool   `json:"is_active"`
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

		err := tx.QueryRow(request.Context(), `
			INSERT INTO suppliers (tenant_id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active
		`, tenant, strings.TrimSpace(req.Code), strings.TrimSpace(req.Name),
			npwp, contactPerson, phone, email, address, city, province, postalCode,
			optionalInt8(req.PaymentTermID), optionalInt8(req.CreditLimitCents),
		).Scan(&result.ID, &result.Code, &result.Name, &result.NPWP, &result.ContactPerson,
			&result.Phone, &result.Email, &result.Address, &result.City, &result.Province,
			&result.PostalCode, &result.PaymentTermID, &result.CreditLimitCents, &result.IsActive)
		return err
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
			SELECT id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active
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
			var paymentTermID, creditLimit pgtype.Int8
			if err := rows.Scan(&s.ID, &s.Code, &s.Name, &npwp, &contactPerson, &phone, &email,
				&address, &city, &province, &postalCode, &paymentTermID, &creditLimit, &s.IsActive); err != nil {
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
		var paymentTermID, creditLimit pgtype.Int8
		err := tx.QueryRow(request.Context(), `
			SELECT id, code, name, npwp, contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit_cents, is_active
			FROM suppliers WHERE tenant_id = $1 AND id = $2
		`, tenant, id).Scan(&result.ID, &result.Code, &result.Name, &npwp, &contactPerson, &phone, &email,
			&address, &city, &province, &postalCode, &paymentTermID, &creditLimit, &result.IsActive)
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
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, "SUPPLIER_NOT_FOUND", err.Error())
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
