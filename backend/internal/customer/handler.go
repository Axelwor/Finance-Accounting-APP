package customer

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/db"
)

// Service exposes the customer and payment-term endpoints. Tenant id comes
// from the auth middleware context.
type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Routes registers the customer and payment-term endpoints on the chi router.
func (service *Service) Routes(router chi.Router) {
	router.Get("/customers", service.ListCustomers)
	router.Post("/customers", service.CreateCustomer)
	// Static routes must precede the {id} wildcard in chi.
	router.Get("/customers/ar-balances", service.ARBalances)
	router.Get("/customers/{id}", service.GetCustomer)
	router.Put("/customers/{id}", service.UpdateCustomer)
	router.Get("/customers/{id}/ar-balance", service.ARBalance)
	router.Post("/customers/{id}/deactivate", service.DeactivateCustomer)
	router.Get("/customers/{id}/statement", service.CustomerStatement)
	router.Get("/payment-terms", service.ListPaymentTerms)
	router.Post("/payment-terms", service.CreatePaymentTerm)
}

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

// CreateCustomerRequest is the JSON body for POST /customers. Nullable
// reference ids use pointers so callers can distinguish "omitted" from zero.
type CreateCustomerRequest struct {
	Code                       string  `json:"code"`
	Name                       string  `json:"name"`
	NPWP                       *string `json:"npwp"`
	ContactPerson              *string `json:"contact_person"`
	Phone                      *string `json:"phone"`
	Email                      *string `json:"email"`
	Address                    *string `json:"address"`
	City                       *string `json:"city"`
	Province                   *string `json:"province"`
	PostalCode                 *string `json:"postal_code"`
	PaymentTermID              *int64  `json:"payment_term_id"`
	CreditLimitCents           *int64  `json:"credit_limit_cents"`
	DefaultRevenueAccountID    *int64  `json:"default_revenue_account_id"`
	DefaultReceivableAccountID *int64  `json:"default_receivable_account_id"`
	BillingAddress             *string `json:"billing_address"`
	ShippingAddress            *string `json:"shipping_address"`
	CustomerGroup              *string `json:"customer_group"`
	PriceLevel                 *string `json:"price_level"`
	CurrencyCode               *string `json:"currency_code"`
	IsPKP                      *bool   `json:"is_pkp"`
	CreditHold                 *bool   `json:"credit_hold"`
	Website                    *string `json:"website"`
	Fax                        *string `json:"fax"`
	ContactPerson2             *string `json:"contact_person_2"`
	Phone2                     *string `json:"phone_2"`
	NpwpName                   *string `json:"npwp_name"`
	OpeningBalanceCents        *int64  `json:"opening_balance_cents"`
	OpeningBalanceDate         *string `json:"opening_balance_date"`
	// IsActive is only honoured by UpdateCustomer (PUT); create always starts
	// active. Nil means "leave unchanged" on update.
	IsActive *bool `json:"is_active"`
}

// CreatePaymentTermRequest is the JSON body for POST /payment-terms.
type CreatePaymentTermRequest struct {
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	DueDays          *int32  `json:"due_days"`
	DiscountDays     *int32  `json:"discount_days"`
	DiscountPercent  *string `json:"discount_percent"`
	CashFlowCategory *string `json:"cash_flow_category"`
}

// ---------------------------------------------------------------------------
// Responses
// ---------------------------------------------------------------------------

// Customer is the JSON shape for a customer row. Nullable text/id columns are
// pointers so they serialize as null when set.
type Customer struct {
	ID                         int64   `json:"id"`
	Code                       string  `json:"code"`
	Name                       string  `json:"name"`
	NPWP                       *string `json:"npwp"`
	ContactPerson              *string `json:"contact_person"`
	Phone                      *string `json:"phone"`
	Email                      *string `json:"email"`
	Address                    *string `json:"address"`
	City                       *string `json:"city"`
	Province                   *string `json:"province"`
	PostalCode                 *string `json:"postal_code"`
	PaymentTermID              *int64  `json:"payment_term_id"`
	CreditLimitCents           *int64  `json:"credit_limit_cents"`
	DefaultRevenueAccountID    *int64  `json:"default_revenue_account_id"`
	DefaultReceivableAccountID *int64  `json:"default_receivable_account_id"`
	IsActive                   bool    `json:"is_active"`

	BillingAddress      *string `json:"billing_address"`
	ShippingAddress     *string `json:"shipping_address"`
	CustomerGroup       *string `json:"customer_group"`
	PriceLevel          *string `json:"price_level"`
	CurrencyCode        *string `json:"currency_code"`
	IsPKP               bool    `json:"is_pkp"`
	CreditHold          bool    `json:"credit_hold"`
	Website             *string `json:"website"`
	Fax                 *string `json:"fax"`
	ContactPerson2      *string `json:"contact_person_2"`
	Phone2              *string `json:"phone_2"`
	NpwpName            *string `json:"npwp_name"`
	OpeningBalanceCents int64   `json:"opening_balance_cents"`
	OpeningBalanceDate  *string `json:"opening_balance_date"`
}

// PaymentTerm is the JSON shape for a payment_terms row. discount_percent is
// scanned/encoded as a string to preserve the numeric(9,6) precision.
type PaymentTerm struct {
	ID              int64   `json:"id"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	DueDays         int32   `json:"due_days"`
	DiscountDays    *int32  `json:"discount_days"`
	DiscountPercent *string `json:"discount_percent"`
	IsActive        bool    `json:"is_active"`
}

// StatementLine is one row of a customer statement: an issued invoice
// (debit increases the receivable) or a payment (credit decreases it).
type StatementLine struct {
	Date                string `json:"date"`
	Type                string `json:"type"`
	Reference           string `json:"reference"`
	Description         string `json:"description"`
	DebitCents          int64  `json:"debit_cents"`
	CreditCents         int64  `json:"credit_cents"`
	RunningBalanceCents int64  `json:"running_balance_cents"`
}

// CustomerStatementResponse is the JSON shape for GET /customers/{id}/statement.
type CustomerStatementResponse struct {
	CustomerID          int64           `json:"customer_id"`
	Code                string          `json:"code"`
	Name                string          `json:"name"`
	FromDate            string          `json:"from_date"`
	ToDate              string          `json:"to_date"`
	OpeningBalanceCents int64           `json:"opening_balance_cents"`
	Lines               []StatementLine `json:"lines"`
	InvoicedCents       int64           `json:"invoiced_cents"`
	PaidCents           int64           `json:"paid_cents"`
	ClosingBalanceCents int64           `json:"closing_balance_cents"`
}

// statementRow is the raw row shape before the running balance is computed.
type statementRow struct {
	ID          int64
	Date        string
	Type        string
	Reference   string
	Description string
	DebitCents  int64
	CreditCents int64
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// ListCustomers returns the active customers for the tenant, ordered by code.
func (service *Service) ListCustomers(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	customers := []Customer{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, code, name, npwp, contact_person, phone, email, address, city,
			       province, postal_code, payment_term_id, credit_limit_cents,
			       default_revenue_account_id, default_receivable_account_id, is_active,
			       billing_address, shipping_address, customer_group, price_level, currency_code,
			       is_pkp, credit_hold, website, fax, contact_person_2, phone_2, npwp_name,
			       opening_balance_cents, opening_balance_date
			FROM customers
			WHERE tenant_id = $1 AND is_active = true
			ORDER BY code
		`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			customer, err := scanCustomer(rows)
			if err != nil {
				return err
			}
			customers = append(customers, customer)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "CUSTOMER_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, customers)
}

// CustomerStatement returns the AR statement for one customer over a date
// range: opening balance (invoices minus payments before from_date), then
// invoice/payment lines with a running balance, then totals.
func (service *Service) CustomerStatement(writer http.ResponseWriter, request *http.Request) {
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
	fromDate := request.URL.Query().Get("from_date")
	toDate := request.URL.Query().Get("to_date")
	if code, message := validateStatementDates(fromDate, toDate); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}

	var customer Customer
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			SELECT id, code, name, npwp, contact_person, phone, email, address, city,
			       province, postal_code, payment_term_id, credit_limit_cents, is_active
			FROM customers
			WHERE tenant_id = $1 AND id = $2
		`, tenant, id).Scan(
			&customer.ID, &customer.Code, &customer.Name, &customer.NPWP, &customer.ContactPerson,
			&customer.Phone, &customer.Email, &customer.Address, &customer.City, &customer.Province,
			&customer.PostalCode, &customer.PaymentTermID, &customer.CreditLimitCents, &customer.IsActive,
		)
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}

	var opening int64
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			SELECT COALESCE((
				SELECT SUM(total_cents) FROM invoices
				WHERE tenant_id = $1 AND customer_id = $2 AND status <> 'VOID'
				  AND invoice_date < $3::date
			), 0) - COALESCE((
				SELECT SUM(amount_cents) FROM invoice_payments
				WHERE tenant_id = $1 AND customer_id = $2 AND status = 'RECEIVED'
				  AND payment_date < $3::date
			), 0)
		`, tenant, id, fromDate).Scan(&opening)
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}

	invoices := []statementRow{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, invoice_date::text, number, COALESCE(notes, ''), total_cents
			FROM invoices
			WHERE tenant_id = $1 AND customer_id = $2 AND status <> 'VOID'
			  AND invoice_date >= $3::date AND invoice_date <= $4::date
			ORDER BY invoice_date, id
		`, tenant, id, fromDate, toDate)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row statementRow
			var total int64
			if err := rows.Scan(&row.ID, &row.Date, &row.Reference, &row.Description, &total); err != nil {
				return err
			}
			row.Type = "invoice"
			row.DebitCents = total
			invoices = append(invoices, row)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}

	payments := []statementRow{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, payment_date::text, number, COALESCE(description, ''), amount_cents
			FROM invoice_payments
			WHERE tenant_id = $1 AND customer_id = $2 AND status = 'RECEIVED'
			  AND payment_date >= $3::date AND payment_date <= $4::date
			ORDER BY payment_date, id
		`, tenant, id, fromDate, toDate)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row statementRow
			var amount int64
			if err := rows.Scan(&row.ID, &row.Date, &row.Reference, &row.Description, &amount); err != nil {
				return err
			}
			row.Type = "payment"
			row.CreditCents = amount
			payments = append(payments, row)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}

	lines, invoiced, paid := buildStatementLines(invoices, payments, opening)

	writeJSON(writer, http.StatusOK, CustomerStatementResponse{
		CustomerID:          customer.ID,
		Code:                customer.Code,
		Name:                customer.Name,
		FromDate:            fromDate,
		ToDate:              toDate,
		OpeningBalanceCents: opening,
		Lines:               lines,
		InvoicedCents:       invoiced,
		PaidCents:           paid,
		ClosingBalanceCents: opening + invoiced - paid,
	})
}

// buildStatementLines merges invoice and payment rows into statement lines
// sorted by date then row id, computes the running balance from the opening
// balance, and returns the lines plus period invoiced/paid totals.
func buildStatementLines(invoices, payments []statementRow, openingBalanceCents int64) ([]StatementLine, int64, int64) {
	rows := make([]statementRow, 0, len(invoices)+len(payments))
	rows = append(rows, invoices...)
	rows = append(rows, payments...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		return rows[i].ID < rows[j].ID
	})

	lines := make([]StatementLine, 0, len(rows))
	balance := openingBalanceCents
	var invoiced, paid int64
	for _, row := range rows {
		balance += row.DebitCents - row.CreditCents
		invoiced += row.DebitCents
		paid += row.CreditCents
		lines = append(lines, StatementLine{
			Date:                row.Date,
			Type:                row.Type,
			Reference:           row.Reference,
			Description:         row.Description,
			DebitCents:          row.DebitCents,
			CreditCents:         row.CreditCents,
			RunningBalanceCents: balance,
		})
	}
	return lines, invoiced, paid
}

// validateStatementDates checks that from_date/to_date are present, valid
// YYYY-MM-DD dates, and that from_date <= to_date.
func validateStatementDates(fromDate, toDate string) (string, string) {
	from, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return "INVALID_REQUEST", "from_date must be a valid date in YYYY-MM-DD format"
	}
	to, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return "INVALID_REQUEST", "to_date must be a valid date in YYYY-MM-DD format"
	}
	if from.After(to) {
		return "INVALID_REQUEST", "from_date must be on or before to_date"
	}
	return "", ""
}

// CreateCustomer creates a customer for the tenant.
func (service *Service) CreateCustomer(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreateCustomerRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCreateCustomer(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	if code, message := service.validateReferenceIDs(request.Context(), tenant, req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	// i-012: pre-check duplicates with clear messages before hitting the DB
	// constraint (code must be unique per tenant; name is also rejected when
	// an ACTIVE customer with the same name exists, to catch double-entry).
	if code, message := service.checkCustomerDuplicates(request.Context(), tenant, req.Code, req.Name, 0); code != "" {
		writeError(writer, http.StatusConflict, code, message)
		return
	}
	var id int64
	openingBalanceDate, err := optionalDate(req.OpeningBalanceDate)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "CUSTOMER_INVALID_FIELD", "opening_balance_date must be a valid YYYY-MM-DD date")
		return
	}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			INSERT INTO customers (
				tenant_id, code, name, npwp, contact_person, phone, email, address,
				city, province, postal_code, payment_term_id, credit_limit_cents,
				default_revenue_account_id, default_receivable_account_id,
				billing_address, shipping_address, customer_group, price_level, currency_code,
				is_pkp, credit_hold, website, fax, contact_person_2, phone_2, npwp_name,
				opening_balance_cents, opening_balance_date
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
				$16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
			RETURNING id
		`, tenant, req.Code, req.Name, req.NPWP, req.ContactPerson, req.Phone,
			req.Email, req.Address, req.City, req.Province, req.PostalCode,
			req.PaymentTermID, req.CreditLimitCents, req.DefaultRevenueAccountID,
			req.DefaultReceivableAccountID,
			req.BillingAddress, req.ShippingAddress, req.CustomerGroup, req.PriceLevel, req.CurrencyCode,
			boolOrFalse(req.IsPKP), boolOrFalse(req.CreditHold), req.Website, req.Fax,
			req.ContactPerson2, req.Phone2, req.NpwpName,
			int64OrZero(req.OpeningBalanceCents), openingBalanceDate).Scan(&id)
	})
	if err != nil {
		if isCheckViolation(err) {
			writeError(writer, http.StatusBadRequest, "CUSTOMER_INVALID_FIELD", "invalid field value (check price_level/currency_code)")
			return
		}
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "CUSTOMER_CODE_EXISTS", "a customer with this code already exists for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "CUSTOMER_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]int64{"id": id})
}

// GetCustomer returns a single customer by id for the tenant.
func (service *Service) GetCustomer(writer http.ResponseWriter, request *http.Request) {
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
	var customer Customer
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		row := tx.QueryRow(request.Context(), `
			SELECT id, code, name, npwp, contact_person, phone, email, address, city,
			       province, postal_code, payment_term_id, credit_limit_cents,
			       default_revenue_account_id, default_receivable_account_id, is_active,
			       billing_address, shipping_address, customer_group, price_level, currency_code,
			       is_pkp, credit_hold, website, fax, contact_person_2, phone_2, npwp_name,
			       opening_balance_cents, opening_balance_date
			FROM customers
			WHERE tenant_id = $1 AND id = $2
		`, tenant, id)
		var err error
		customer, err = scanCustomer(row)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "CUSTOMER_GET_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, customer)
}

// UpdateCustomer updates an existing customer for the tenant (PUT
// /customers/{id}). All master fields are replaced; the opening balance is
// immutable after create. is_active is applied when provided (nil leaves the
// current flag untouched).
func (service *Service) UpdateCustomer(writer http.ResponseWriter, request *http.Request) {
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
	var req CreateCustomerRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCreateCustomer(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	if code, message := service.validateReferenceIDs(request.Context(), tenant, req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	// Duplicate check must exclude the customer being updated itself.
	if code, message := service.checkCustomerDuplicates(request.Context(), tenant, req.Code, req.Name, id); code != "" {
		writeError(writer, http.StatusConflict, code, message)
		return
	}
	var customer Customer
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		row := tx.QueryRow(request.Context(), `
		UPDATE customers SET
			code = $3, name = $4, npwp = $5, contact_person = $6, phone = $7,
			email = $8, address = $9, city = $10, province = $11, postal_code = $12,
			payment_term_id = $13, credit_limit_cents = $14,
			default_revenue_account_id = $15, default_receivable_account_id = $16,
			is_active = COALESCE($17, is_active),
			billing_address = $18, shipping_address = $19, customer_group = $20,
			price_level = $21, currency_code = $22, is_pkp = $23, credit_hold = $24,
			website = $25, fax = $26, contact_person_2 = $27, phone_2 = $28,
			npwp_name = $29, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, code, name, npwp, contact_person, phone, email, address, city,
			province, postal_code, payment_term_id, credit_limit_cents,
			default_revenue_account_id, default_receivable_account_id, is_active,
			billing_address, shipping_address, customer_group, price_level, currency_code,
			is_pkp, credit_hold, website, fax, contact_person_2, phone_2, npwp_name,
			opening_balance_cents, opening_balance_date
		`, tenant, id, req.Code, req.Name, req.NPWP, req.ContactPerson, req.Phone,
			req.Email, req.Address, req.City, req.Province, req.PostalCode,
			req.PaymentTermID, req.CreditLimitCents, req.DefaultRevenueAccountID,
			req.DefaultReceivableAccountID, req.IsActive,
			req.BillingAddress, req.ShippingAddress, req.CustomerGroup, req.PriceLevel, req.CurrencyCode,
			boolOrFalse(req.IsPKP), boolOrFalse(req.CreditHold), req.Website, req.Fax,
			req.ContactPerson2, req.Phone2, req.NpwpName)
		var err error
		customer, err = scanCustomer(row)
		return err
	})
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant")
			return
		}
		if isCheckViolation(err) {
			writeError(writer, http.StatusBadRequest, "CUSTOMER_INVALID_FIELD", "invalid field value (check price_level/currency_code)")
			return
		}
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "CUSTOMER_CODE_EXISTS", "a customer with this code already exists for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "CUSTOMER_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, customer)
}

// DeactivateCustomer marks a customer as inactive for the tenant.
func (service *Service) DeactivateCustomer(writer http.ResponseWriter, request *http.Request) {
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
	var command pgconn.CommandTag
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		var err error
		command, err = tx.Exec(request.Context(), `
			UPDATE customers
			SET is_active = false, updated_at = now()
			WHERE tenant_id = $1 AND id = $2
		`, tenant, id)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "CUSTOMER_DEACTIVATE_FAILED", err.Error())
		return
	}
	if command.RowsAffected() == 0 {
		writeError(writer, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]bool{"deactivated": true})
}

// ListPaymentTerms returns the payment terms for the tenant, ordered by code.
func (service *Service) ListPaymentTerms(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	terms := []PaymentTerm{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
			SELECT id, code, name, due_days, discount_days, discount_percent::text, is_active
			FROM payment_terms
			WHERE tenant_id = $1
			ORDER BY code
		`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var term PaymentTerm
			var discountPercent pgtype.Text
			if err := rows.Scan(&term.ID, &term.Code, &term.Name, &term.DueDays,
				&term.DiscountDays, &discountPercent, &term.IsActive); err != nil {
				return err
			}
			term.DiscountPercent = textOrNil(discountPercent)
			terms = append(terms, term)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PAYMENT_TERM_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, terms)
}

// CreatePaymentTerm creates a payment term for the tenant.
func (service *Service) CreatePaymentTerm(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var req CreatePaymentTermRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if code, message := validateCreatePaymentTerm(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	var id int64
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			INSERT INTO payment_terms (
				tenant_id, code, name, due_days, discount_days, discount_percent, cash_flow_category
			) VALUES ($1, $2, $3, COALESCE($4, 30), $5, $6, $7)
			RETURNING id
		`, tenant, req.Code, req.Name, req.DueDays, req.DiscountDays,
			req.DiscountPercent, req.CashFlowCategory).Scan(&id)
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(writer, http.StatusConflict, "PAYMENT_TERM_CODE_EXISTS", "a payment term with this code already exists for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "PAYMENT_TERM_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]int64{"id": id})
}

// ---------------------------------------------------------------------------
// Validation (pure — no database access)
// ---------------------------------------------------------------------------

func validateCreateCustomer(req CreateCustomerRequest) (string, string) {
	if strings.TrimSpace(req.Code) == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if strings.TrimSpace(req.Name) == "" {
		return "INVALID_REQUEST", "name is required"
	}
	return "", ""
}

// checkCustomerDuplicates (i-012) rejects duplicate code or duplicate name
// among ACTIVE customers of the tenant, giving callers a clear 409 before the
// database unique constraint is reached. excludeID skips one customer id so
// updates do not conflict with the record being edited (pass 0 on create).
func (service *Service) checkCustomerDuplicates(ctx context.Context, tenantID int64, code, name string, excludeID int64) (string, string) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	var existingCode bool
	if err := db.WithTenantData(ctx, service.pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM customers WHERE tenant_id = $1 AND code = $2 AND id <> $3)`,
			tenantID, code, excludeID).Scan(&existingCode)
	}); err == nil && existingCode {
		return "CUSTOMER_CODE_EXISTS", "a customer with this code already exists for this tenant"
	}
	if name != "" {
		var existingName bool
		if err := db.WithTenantData(ctx, service.pool, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM customers WHERE tenant_id = $1 AND is_active = true AND LOWER(name) = LOWER($2) AND id <> $3)`,
				tenantID, name, excludeID).Scan(&existingName)
		}); err == nil && existingName {
			return "CUSTOMER_NAME_EXISTS", "an active customer with this name already exists for this tenant"
		}
	}
	return "", ""
}

func validateCreatePaymentTerm(req CreatePaymentTermRequest) (string, string) {
	if strings.TrimSpace(req.Code) == "" {
		return "INVALID_REQUEST", "code is required"
	}
	if strings.TrimSpace(req.Name) == "" {
		return "INVALID_REQUEST", "name is required"
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// scannable is the minimal rows/row interface scanCustomer needs.
type scannable interface {
	Scan(dest ...any) error
}

// scanCustomer maps a customers row into the response shape.
func scanCustomer(row scannable) (Customer, error) {
	var (
		customer                                Customer
		npwp, contact, phone, email             pgtype.Text
		address, city, province, postal         pgtype.Text
		paymentTermID, creditLimit, rev, receiv pgtype.Int8
		billingAddr, shipAddr, group, price     pgtype.Text
		currency                                pgtype.Text
		website, fax, contact2, phone2, npwpN   pgtype.Text
		openingBalance                          pgtype.Int8
		openingBalanceDate                      pgtype.Date
	)
	if err := row.Scan(&customer.ID, &customer.Code, &customer.Name,
		&npwp, &contact, &phone, &email, &address, &city, &province, &postal,
		&paymentTermID, &creditLimit, &rev, &receiv, &customer.IsActive,
		&billingAddr, &shipAddr, &group, &price, &currency,
		&customer.IsPKP, &customer.CreditHold, &website, &fax, &contact2, &phone2, &npwpN,
		&openingBalance, &openingBalanceDate); err != nil {
		return Customer{}, err
	}
	customer.NPWP = textOrNil(npwp)
	customer.ContactPerson = textOrNil(contact)
	customer.Phone = textOrNil(phone)
	customer.Email = textOrNil(email)
	customer.Address = textOrNil(address)
	customer.City = textOrNil(city)
	customer.Province = textOrNil(province)
	customer.PostalCode = textOrNil(postal)
	customer.PaymentTermID = int8OrNil(paymentTermID)
	customer.CreditLimitCents = int8OrNil(creditLimit)
	customer.DefaultRevenueAccountID = int8OrNil(rev)
	customer.DefaultReceivableAccountID = int8OrNil(receiv)
	customer.BillingAddress = textOrNil(billingAddr)
	customer.ShippingAddress = textOrNil(shipAddr)
	customer.CustomerGroup = textOrNil(group)
	customer.PriceLevel = textOrNil(price)
	customer.CurrencyCode = textOrNil(currency)
	customer.Website = textOrNil(website)
	customer.Fax = textOrNil(fax)
	customer.ContactPerson2 = textOrNil(contact2)
	customer.Phone2 = textOrNil(phone2)
	customer.NpwpName = textOrNil(npwpN)
	customer.OpeningBalanceCents = openingBalance.Int64
	if openingBalanceDate.Valid {
		formatted := openingBalanceDate.Time.Format("2006-01-02")
		customer.OpeningBalanceDate = &formatted
	}
	return customer, nil
}

// validateReferenceIDs verifies, when provided, that the payment term and
// account ids belong to the tenant. Returns a non-empty code on failure.
func (service *Service) validateReferenceIDs(ctx context.Context, tenant int64, req CreateCustomerRequest) (string, string) {
	if req.PaymentTermID != nil && *req.PaymentTermID > 0 {
		var ok bool
		if err := db.WithTenantData(ctx, service.pool, tenant, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM payment_terms WHERE tenant_id = $1 AND id = $2)`,
				tenant, *req.PaymentTermID).Scan(&ok)
		}); err != nil {
			return "CUSTOMER_INVALID_REFERENCE", "failed to validate payment_term_id"
		}
		if !ok {
			return "CUSTOMER_INVALID_REFERENCE", "payment_term_id does not exist for this tenant"
		}
	}
	for _, accountID := range []*int64{req.DefaultRevenueAccountID, req.DefaultReceivableAccountID} {
		if accountID == nil || *accountID <= 0 {
			continue
		}
		var ok bool
		if err := db.WithTenantData(ctx, service.pool, tenant, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM accounts WHERE tenant_id = $1 AND id = $2)`,
				tenant, *accountID).Scan(&ok)
		}); err != nil {
			return "CUSTOMER_INVALID_REFERENCE", "failed to validate account reference"
		}
		if !ok {
			return "CUSTOMER_INVALID_REFERENCE", "account reference does not exist for this tenant"
		}
	}
	return "", ""
}
