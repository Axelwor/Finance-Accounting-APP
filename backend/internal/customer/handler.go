package customer

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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
	router.Get("/customers/{id}", service.GetCustomer)
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
	ID               int64   `json:"id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	NPWP             *string `json:"npwp"`
	ContactPerson    *string `json:"contact_person"`
	Phone            *string `json:"phone"`
	Email            *string `json:"email"`
	Address          *string `json:"address"`
	City             *string `json:"city"`
	Province         *string `json:"province"`
	PostalCode       *string `json:"postal_code"`
	PaymentTermID    *int64  `json:"payment_term_id"`
	CreditLimitCents *int64  `json:"credit_limit_cents"`
	IsActive         bool    `json:"is_active"`
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
	Date               string `json:"date"`
	Type               string `json:"type"`
	Reference          string `json:"reference"`
	Description        string `json:"description"`
	DebitCents         int64  `json:"debit_cents"`
	CreditCents        int64  `json:"credit_cents"`
	RunningBalanceCents int64 `json:"running_balance_cents"`
}

// CustomerStatementResponse is the JSON shape for GET /customers/{id}/statement.
type CustomerStatementResponse struct {
	CustomerID           int64           `json:"customer_id"`
	Code                 string          `json:"code"`
	Name                 string          `json:"name"`
	FromDate             string          `json:"from_date"`
	ToDate               string          `json:"to_date"`
	OpeningBalanceCents  int64           `json:"opening_balance_cents"`
	Lines                []StatementLine `json:"lines"`
	InvoicedCents        int64           `json:"invoiced_cents"`
	PaidCents            int64           `json:"paid_cents"`
	ClosingBalanceCents  int64           `json:"closing_balance_cents"`
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
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, code, name, npwp, contact_person, phone, email, address, city,
		       province, postal_code, payment_term_id, credit_limit_cents, is_active
		FROM customers
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY code
	`, tenant)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "CUSTOMER_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()
	customers := []Customer{}
	for rows.Next() {
		customer, err := scanCustomer(rows)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "CUSTOMER_LIST_FAILED", err.Error())
			return
		}
		customers = append(customers, customer)
	}
	if err := rows.Err(); err != nil {
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
	err = service.pool.QueryRow(request.Context(), `
		SELECT id, code, name, npwp, contact_person, phone, email, address, city,
		       province, postal_code, payment_term_id, credit_limit_cents, is_active
		FROM customers
		WHERE tenant_id = $1 AND id = $2
	`, tenant, id).Scan(
		&customer.ID, &customer.Code, &customer.Name, &customer.NPWP, &customer.ContactPerson,
		&customer.Phone, &customer.Email, &customer.Address, &customer.City, &customer.Province,
		&customer.PostalCode, &customer.PaymentTermID, &customer.CreditLimitCents, &customer.IsActive,
	)
	if err != nil {
		if isNoRows(err) {
			writeError(writer, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer does not exist for this tenant")
			return
		}
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}

	var opening int64
	err = service.pool.QueryRow(request.Context(), `
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
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}

	invoices := []statementRow{}
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, invoice_date::text, number, COALESCE(notes, ''), total_cents
		FROM invoices
		WHERE tenant_id = $1 AND customer_id = $2 AND status <> 'VOID'
		  AND invoice_date >= $3::date AND invoice_date <= $4::date
		ORDER BY invoice_date, id
	`, tenant, id, fromDate, toDate)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}
	for rows.Next() {
		var row statementRow
		var total int64
		if err := rows.Scan(&row.ID, &row.Date, &row.Reference, &row.Description, &total); err != nil {
			rows.Close()
			writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
			return
		}
		row.Type = "invoice"
		row.DebitCents = total
		invoices = append(invoices, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}
	rows.Close()

	payments := []statementRow{}
	rows, err = service.pool.Query(request.Context(), `
		SELECT id, payment_date::text, number, COALESCE(description, ''), amount_cents
		FROM invoice_payments
		WHERE tenant_id = $1 AND customer_id = $2 AND status = 'RECEIVED'
		  AND payment_date >= $3::date AND payment_date <= $4::date
		ORDER BY payment_date, id
	`, tenant, id, fromDate, toDate)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}
	for rows.Next() {
		var row statementRow
		var amount int64
		if err := rows.Scan(&row.ID, &row.Date, &row.Reference, &row.Description, &amount); err != nil {
			rows.Close()
			writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
			return
		}
		row.Type = "payment"
		row.CreditCents = amount
		payments = append(payments, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeError(writer, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error())
		return
	}
	rows.Close()

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
	var id int64
	err = service.pool.QueryRow(request.Context(), `
		INSERT INTO customers (
			tenant_id, code, name, npwp, contact_person, phone, email, address,
			city, province, postal_code, payment_term_id, credit_limit_cents,
			default_revenue_account_id, default_receivable_account_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id
	`, tenant, req.Code, req.Name, req.NPWP, req.ContactPerson, req.Phone,
		req.Email, req.Address, req.City, req.Province, req.PostalCode,
		req.PaymentTermID, req.CreditLimitCents, req.DefaultRevenueAccountID,
		req.DefaultReceivableAccountID).Scan(&id)
	if err != nil {
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
	row := service.pool.QueryRow(request.Context(), `
		SELECT id, code, name, npwp, contact_person, phone, email, address, city,
		       province, postal_code, payment_term_id, credit_limit_cents, is_active
		FROM customers
		WHERE tenant_id = $1 AND id = $2
	`, tenant, id)
	customer, err := scanCustomer(row)
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
	command, err := service.pool.Exec(request.Context(), `
		UPDATE customers
		SET is_active = false, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenant, id)
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
	rows, err := service.pool.Query(request.Context(), `
		SELECT id, code, name, due_days, discount_days, discount_percent::text, is_active
		FROM payment_terms
		WHERE tenant_id = $1
		ORDER BY code
	`, tenant)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "PAYMENT_TERM_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()
	terms := []PaymentTerm{}
	for rows.Next() {
		var term PaymentTerm
		var discountPercent pgtype.Text
		if err := rows.Scan(&term.ID, &term.Code, &term.Name, &term.DueDays,
			&term.DiscountDays, &discountPercent, &term.IsActive); err != nil {
			writeError(writer, http.StatusInternalServerError, "PAYMENT_TERM_LIST_FAILED", err.Error())
			return
		}
		term.DiscountPercent = textOrNil(discountPercent)
		terms = append(terms, term)
	}
	if err := rows.Err(); err != nil {
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
	err = service.pool.QueryRow(request.Context(), `
		INSERT INTO payment_terms (
			tenant_id, code, name, due_days, discount_days, discount_percent, cash_flow_category
		) VALUES ($1, $2, $3, COALESCE($4, 30), $5, $6, $7)
		RETURNING id
	`, tenant, req.Code, req.Name, req.DueDays, req.DiscountDays,
		req.DiscountPercent, req.CashFlowCategory).Scan(&id)
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
	)
	if err := row.Scan(&customer.ID, &customer.Code, &customer.Name,
		&npwp, &contact, &phone, &email, &address, &city, &province, &postal,
		&paymentTermID, &creditLimit, &rev, &receiv, &customer.IsActive); err != nil {
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
	return customer, nil
}

// validateReferenceIDs verifies, when provided, that the payment term and
// account ids belong to the tenant. Returns a non-empty code on failure.
func (service *Service) validateReferenceIDs(ctx context.Context, tenant int64, req CreateCustomerRequest) (string, string) {
	if req.PaymentTermID != nil && *req.PaymentTermID > 0 {
		var ok bool
		if err := service.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM payment_terms WHERE tenant_id = $1 AND id = $2)`,
			tenant, *req.PaymentTermID).Scan(&ok); err != nil {
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
		if err := service.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM accounts WHERE tenant_id = $1 AND id = $2)`,
			tenant, *accountID).Scan(&ok); err != nil {
			return "CUSTOMER_INVALID_REFERENCE", "failed to validate account reference"
		}
		if !ok {
			return "CUSTOMER_INVALID_REFERENCE", "account reference does not exist for this tenant"
		}
	}
	return "", ""
}
