// Package settings exposes the tenant Settings module (SET-001):
// company profile, format preferences, default account mapping, base
// currency guard, currencies, and manual exchange-rate maintenance.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/httperr"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Put("/settings/company", service.PutCompany)
	router.Put("/settings/preferences", service.PutPreferences)
	router.Put("/settings/default-accounts", service.PutDefaultAccounts)
	router.Put("/settings/currency", service.PutBaseCurrency)
	router.Post("/exchange-rates", service.CreateExchangeRate)
	router.Put("/exchange-rates/{id}", service.UpdateExchangeRate)
	router.Delete("/exchange-rates/{id}", service.DeleteExchangeRate)
}

// RegisterReadRoutes mounts the read-only endpoints every authenticated user
// needs (the frontend formatters load GET /settings right after login).
func (service *Service) RegisterReadRoutes(router chi.Router) {
	router.Get("/settings", service.GetSettings)
	router.Get("/currencies", service.ListCurrencies)
	router.Get("/exchange-rates", service.ListExchangeRates)
	router.Get("/exchange-rates/latest", service.LatestExchangeRate)
}

// ---------------------------------------------------------------------------
// Response payloads
// ---------------------------------------------------------------------------

type CompanyInfo struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	LegalName       string `json:"legal_name"`
	Address         string `json:"address"`
	City            string `json:"city"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	TaxID           string `json:"tax_id"`
	BaseCurrency    string `json:"base_currency"`
	FiscalYearStart string `json:"fiscal_year_start"`
	HasJournals     bool   `json:"has_journals"`
}

type Preferences struct {
	DateFormat          string `json:"date_format"`
	ThousandSeparator   string `json:"thousand_separator"`
	DecimalSeparator    string `json:"decimal_separator"`
	AmountDecimalPlaces int    `json:"amount_decimal_places"`
	QtyDecimalPlaces    int    `json:"qty_decimal_places"`
}

type DefaultAccounts struct {
	DefaultSalesAccountID         *int64 `json:"default_sales_account_id"`
	DefaultPurchaseAccountID      *int64 `json:"default_purchase_account_id"`
	DefaultCogsAccountID          *int64 `json:"default_cogs_account_id"`
	DefaultARAccountID            *int64 `json:"default_ar_account_id"`
	DefaultAPAccountID            *int64 `json:"default_ap_account_id"`
	DefaultCashAccountID          *int64 `json:"default_cash_account_id"`
	DefaultCapitalAccountID       *int64 `json:"default_capital_account_id"`
	RetainedEarningsAccountID     *int64 `json:"retained_earnings_account_id"`
	OpeningBalanceEquityAccountID *int64 `json:"opening_balance_equity_account_id"`
	FxGainAccountID               *int64 `json:"fx_gain_account_id"`
	FxLossAccountID               *int64 `json:"fx_loss_account_id"`
}

type SettingsResponse struct {
	Company         CompanyInfo     `json:"company"`
	Preferences     Preferences     `json:"preferences"`
	DefaultAccounts DefaultAccounts `json:"default_accounts"`
}

type CurrencyResponse struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	DecimalPlaces int    `json:"decimal_places"`
}

type ExchangeRateResponse struct {
	ID            int64   `json:"id"`
	FromCurrency  string  `json:"from_currency"`
	ToCurrency    string  `json:"to_currency"`
	Rate          float64 `json:"rate"`
	EffectiveDate string  `json:"effective_date"`
	Source        string  `json:"source"`
}

// ---------------------------------------------------------------------------
// GET /settings
// ---------------------------------------------------------------------------

func (service *Service) GetSettings(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var result SettingsResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		company, err := loadCompany(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		prefs, defaults, err := loadOrCreateSettingsRow(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		result = SettingsResponse{Company: company, Preferences: prefs, DefaultAccounts: defaults}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "SETTINGS_LOAD_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func loadCompany(ctx context.Context, tx pgx.Tx, tenant int64) (CompanyInfo, error) {
	var c CompanyInfo
	var legalName, address, city, phone, email, taxID pgtype.Text
	var fiscalStart pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT t.id, t.name, t.slug, t.legal_name, t.address, t.city, t.phone,
		       t.email, t.tax_id, t.currency_code, t.fiscal_year_start,
		       EXISTS (SELECT 1 FROM journal_entries je WHERE je.tenant_id = t.id)
		FROM tenants t WHERE t.id = $1
	`, tenant).Scan(&c.ID, &c.Name, &c.Slug, &legalName, &address, &city, &phone,
		&email, &taxID, &c.BaseCurrency, &fiscalStart, &c.HasJournals)
	if err != nil {
		return CompanyInfo{}, err
	}
	c.LegalName = trimmed(legalName)
	c.Address = trimmed(address)
	c.City = trimmed(city)
	c.Phone = trimmed(phone)
	c.Email = trimmed(email)
	c.TaxID = trimmed(taxID)
	if fiscalStart.Valid {
		c.FiscalYearStart = fiscalStart.Time.Format("2006-01-02")
	}
	return c, nil
}

// loadOrCreateSettingsRow returns the tenant preferences + default account
// mapping, seeding a default row on first access.
func loadOrCreateSettingsRow(ctx context.Context, tx pgx.Tx, tenant int64) (Preferences, DefaultAccounts, error) {
	var prefs Preferences
	var defaults DefaultAccounts
	err := tx.QueryRow(ctx, `
		SELECT date_format, thousand_separator, decimal_separator,
		       amount_decimal_places, qty_decimal_places,
		       default_sales_account_id, default_purchase_account_id, default_cogs_account_id,
		       default_ar_account_id, default_ap_account_id, default_cash_account_id,
		       default_capital_account_id, retained_earnings_account_id,
		       opening_balance_equity_account_id, fx_gain_account_id, fx_loss_account_id
		FROM tenant_settings WHERE tenant_id = $1
	`, tenant).Scan(
		&prefs.DateFormat, &prefs.ThousandSeparator, &prefs.DecimalSeparator,
		&prefs.AmountDecimalPlaces, &prefs.QtyDecimalPlaces,
		&defaults.DefaultSalesAccountID, &defaults.DefaultPurchaseAccountID, &defaults.DefaultCogsAccountID,
		&defaults.DefaultARAccountID, &defaults.DefaultAPAccountID, &defaults.DefaultCashAccountID,
		&defaults.DefaultCapitalAccountID, &defaults.RetainedEarningsAccountID,
		&defaults.OpeningBalanceEquityAccountID, &defaults.FxGainAccountID, &defaults.FxLossAccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_settings (tenant_id) VALUES ($1) ON CONFLICT (tenant_id) DO NOTHING`, tenant); err != nil {
			return prefs, defaults, err
		}
		return loadOrCreateSettingsRow(ctx, tx, tenant)
	}
	if err != nil {
		return prefs, defaults, err
	}
	return prefs, defaults, nil
}

// ---------------------------------------------------------------------------
// PUT /settings/company
// ---------------------------------------------------------------------------

type PutCompanyRequest struct {
	LegalName *string `json:"legal_name"`
	Address   *string `json:"address"`
	City      *string `json:"city"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`
	TaxID     *string `json:"tax_id"`
}

func (service *Service) PutCompany(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var req PutCompanyRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	var result CompanyInfo
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `
			UPDATE tenants SET
				legal_name = COALESCE($2, legal_name),
				address = COALESCE($3, address),
				city = COALESCE($4, city),
				phone = COALESCE($5, phone),
				email = COALESCE($6, email),
				tax_id = COALESCE($7, tax_id)
			WHERE id = $1
		`, tenant, textPtr(req.LegalName), textPtr(req.Address), textPtr(req.City),
			textPtr(req.Phone), textPtr(req.Email), textPtr(req.TaxID)); err != nil {
			return err
		}
		company, err := loadCompany(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		result = company
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "SETTINGS_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// PUT /settings/preferences
// ---------------------------------------------------------------------------

type PutPreferencesRequest struct {
	DateFormat          *string `json:"date_format"`
	ThousandSeparator   *string `json:"thousand_separator"`
	DecimalSeparator    *string `json:"decimal_separator"`
	AmountDecimalPlaces *int    `json:"amount_decimal_places"`
	QtyDecimalPlaces    *int    `json:"qty_decimal_places"`
}

func (service *Service) PutPreferences(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var req PutPreferencesRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, message := validatePreferences(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	var prefs Preferences
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `
			INSERT INTO tenant_settings (tenant_id) VALUES ($1) ON CONFLICT (tenant_id) DO NOTHING
		`, tenant); err != nil {
			return err
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE tenant_settings SET
				date_format = COALESCE($2, date_format),
				thousand_separator = COALESCE($3, thousand_separator),
				decimal_separator = COALESCE($4, decimal_separator),
				amount_decimal_places = COALESCE($5, amount_decimal_places),
				qty_decimal_places = COALESCE($6, qty_decimal_places),
				updated_at = now()
			WHERE tenant_id = $1
		`, tenant, textPtr(req.DateFormat), textPtr(req.ThousandSeparator), textPtr(req.DecimalSeparator),
			intPtr(req.AmountDecimalPlaces), intPtr(req.QtyDecimalPlaces)); err != nil {
			return err
		}
		_, defaults, err := loadOrCreateSettingsRow(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		_ = defaults
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "SETTINGS_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, prefs)
}

func validatePreferences(req PutPreferencesRequest) (string, string) {
	if req.DateFormat != nil {
		switch *req.DateFormat {
		case "DD/MM/YYYY", "MM/DD/YYYY", "YYYY-MM-DD":
		default:
			return "INVALID_REQUEST", "date_format must be DD/MM/YYYY, MM/DD/YYYY, or YYYY-MM-DD"
		}
	}
	for _, sep := range []struct {
		name  string
		value *string
	}{
		{"thousand_separator", req.ThousandSeparator},
		{"decimal_separator", req.DecimalSeparator},
	} {
		if sep.value != nil && len(*sep.value) != 1 {
			return "INVALID_REQUEST", sep.name + " must be exactly one character"
		}
	}
	if req.ThousandSeparator != nil && req.DecimalSeparator != nil && *req.ThousandSeparator == *req.DecimalSeparator {
		return "INVALID_REQUEST", "thousand_separator and decimal_separator must differ"
	}
	for _, places := range []struct {
		name  string
		value *int
	}{
		{"amount_decimal_places", req.AmountDecimalPlaces},
		{"qty_decimal_places", req.QtyDecimalPlaces},
	} {
		if places.value != nil && (*places.value < 0 || *places.value > 4) {
			return "INVALID_REQUEST", places.name + " must be between 0 and 4"
		}
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// PUT /settings/default-accounts
// ---------------------------------------------------------------------------

func (service *Service) PutDefaultAccounts(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var req DefaultAccounts
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	var result DefaultAccounts
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		if _, err := tx.Exec(request.Context(), `INSERT INTO tenant_settings (tenant_id) VALUES ($1) ON CONFLICT (tenant_id) DO NOTHING`, tenant); err != nil {
			return err
		}
		for _, id := range []*int64{
			req.DefaultSalesAccountID, req.DefaultPurchaseAccountID, req.DefaultCogsAccountID,
			req.DefaultARAccountID, req.DefaultAPAccountID, req.DefaultCashAccountID,
			req.DefaultCapitalAccountID, req.RetainedEarningsAccountID,
			req.OpeningBalanceEquityAccountID, req.FxGainAccountID, req.FxLossAccountID,
		} {
			if id == nil {
				continue
			}
			var exists bool
			if err := tx.QueryRow(request.Context(),
				`SELECT EXISTS(SELECT 1 FROM accounts WHERE tenant_id = $1 AND id = $2 AND is_active = true)`,
				tenant, *id).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return errAccountNotFound(*id)
			}
		}
		if _, err := tx.Exec(request.Context(), `
			UPDATE tenant_settings SET
				default_sales_account_id = $2,
				default_purchase_account_id = $3,
				default_cogs_account_id = $4,
				default_ar_account_id = $5,
				default_ap_account_id = $6,
				default_cash_account_id = $7,
				default_capital_account_id = $8,
				retained_earnings_account_id = $9,
				opening_balance_equity_account_id = $10,
				fx_gain_account_id = $11,
				fx_loss_account_id = $12,
				updated_at = now()
			WHERE tenant_id = $1
		`, tenant, req.DefaultSalesAccountID, req.DefaultPurchaseAccountID, req.DefaultCogsAccountID,
			req.DefaultARAccountID, req.DefaultAPAccountID, req.DefaultCashAccountID,
			req.DefaultCapitalAccountID, req.RetainedEarningsAccountID,
			req.OpeningBalanceEquityAccountID, req.FxGainAccountID, req.FxLossAccountID); err != nil {
			return err
		}
		_, result, err = loadOrCreateSettingsRow(request.Context(), tx, tenant)
		return err
	})
	if err != nil {
		if errors.Is(err, errAccountNotFoundErr) {
			writeError(writer, http.StatusBadRequest, "ACCOUNT_NOT_FOUND", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "SETTINGS_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// PUT /settings/currency (base currency guard)
// ---------------------------------------------------------------------------

type PutCurrencyRequest struct {
	CurrencyCode string `json:"currency_code"`
}

func (service *Service) PutBaseCurrency(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var req PutCurrencyRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.CurrencyCode))
	if len(code) != 3 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "currency_code must be a 3-letter ISO code")
		return
	}
	var result CompanyInfo
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(request.Context(), `SELECT EXISTS(SELECT 1 FROM currencies WHERE code = $1)`, code).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errUnknownCurrency(code)
		}
		// The base currency is locked once journals exist: changing it would
		// restate the whole ledger (plan decision #9).
		var hasJournals bool
		if err := tx.QueryRow(request.Context(),
			`SELECT EXISTS(SELECT 1 FROM journal_entries WHERE tenant_id = $1)`, tenant).Scan(&hasJournals); err != nil {
			return err
		}
		if hasJournals {
			return errBaseCurrencyLocked
		}
		if _, err := tx.Exec(request.Context(), `UPDATE tenants SET currency_code = $2 WHERE id = $1`, tenant, code); err != nil {
			return err
		}
		company, err := loadCompany(request.Context(), tx, tenant)
		if err != nil {
			return err
		}
		result = company
		return nil
	})
	if err != nil {
		if errors.Is(err, errBaseCurrencyLocked) {
			writeError(writer, http.StatusConflict, "BASE_CURRENCY_LOCKED",
				"base currency cannot be changed after journals exist")
			return
		}
		if errors.Is(err, errUnknownCurrencyErr) {
			writeError(writer, http.StatusBadRequest, "UNKNOWN_CURRENCY", err.Error())
			return
		}
		writeError(writer, http.StatusInternalServerError, "SETTINGS_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Currencies
// ---------------------------------------------------------------------------

func (service *Service) ListCurrencies(writer http.ResponseWriter, request *http.Request) {
	results := []CurrencyResponse{}
	rows, err := service.pool.Query(request.Context(),
		`SELECT code, name, symbol, decimal_places FROM currencies ORDER BY code`)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "CURRENCY_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var c CurrencyResponse
		if err := rows.Scan(&c.Code, &c.Name, &c.Symbol, &c.DecimalPlaces); err != nil {
			writeError(writer, http.StatusInternalServerError, "CURRENCY_LIST_FAILED", err.Error())
			return
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		writeError(writer, http.StatusInternalServerError, "CURRENCY_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

// ---------------------------------------------------------------------------
// Exchange rates
// ---------------------------------------------------------------------------

type ExchangeRateRequest struct {
	FromCurrency  string  `json:"from_currency"`
	ToCurrency    string  `json:"to_currency"`
	Rate          float64 `json:"rate"`
	EffectiveDate string  `json:"effective_date"`
}

func (service *Service) ListExchangeRates(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	from := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("from")))
	results := []ExchangeRateResponse{}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		query := `SELECT id, from_currency, to_currency, rate::float8, effective_date, source
			FROM exchange_rates WHERE tenant_id = $1`
		args := []any{tenant}
		if from != "" {
			query += ` AND from_currency = $2`
			args = append(args, from)
		}
		query += ` ORDER BY effective_date DESC, id DESC`
		rows, err := tx.Query(request.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r ExchangeRateResponse
			var effective pgtype.Date
			if err := rows.Scan(&r.ID, &r.FromCurrency, &r.ToCurrency, &r.Rate, &effective, &r.Source); err != nil {
				return err
			}
			r.FromCurrency = strings.TrimSpace(r.FromCurrency)
			r.ToCurrency = strings.TrimSpace(r.ToCurrency)
			if effective.Valid {
				r.EffectiveDate = effective.Time.Format("2006-01-02")
			}
			results = append(results, r)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "RATE_LIST_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, results)
}

func (service *Service) CreateExchangeRate(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	var req ExchangeRateRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, message := validateRateRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	var result ExchangeRateResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		var effective pgtype.Date
		if err := effective.Scan(req.EffectiveDate); err != nil {
			return err
		}
		err := tx.QueryRow(request.Context(), `
			INSERT INTO exchange_rates (tenant_id, from_currency, to_currency, rate, effective_date, source)
			VALUES ($1, $2, $3, $4, $5, 'manual')
			ON CONFLICT (tenant_id, from_currency, to_currency, effective_date)
			DO UPDATE SET rate = EXCLUDED.rate
			RETURNING id, from_currency, to_currency, rate::float8, effective_date, source
		`, tenant, req.FromCurrency, req.ToCurrency, req.Rate, effective).Scan(
			&result.ID, &result.FromCurrency, &result.ToCurrency, &result.Rate, &effective, &result.Source)
		if err != nil {
			return err
		}
		result.FromCurrency = strings.TrimSpace(result.FromCurrency)
		result.ToCurrency = strings.TrimSpace(result.ToCurrency)
		if effective.Valid {
			result.EffectiveDate = effective.Time.Format("2006-01-02")
		}
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "RATE_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (service *Service) UpdateExchangeRate(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	var req ExchangeRateRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if code, message := validateRateRequest(req); code != "" {
		writeError(writer, http.StatusBadRequest, code, message)
		return
	}
	var result ExchangeRateResponse
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		var effective pgtype.Date
		if err := effective.Scan(req.EffectiveDate); err != nil {
			return err
		}
		err := tx.QueryRow(request.Context(), `
			UPDATE exchange_rates SET from_currency = $3, to_currency = $4, rate = $5, effective_date = $6
			WHERE tenant_id = $1 AND id = $2
			RETURNING id, from_currency, to_currency, rate::float8, effective_date, source
		`, tenant, id, req.FromCurrency, req.ToCurrency, req.Rate, effective).Scan(
			&result.ID, &result.FromCurrency, &result.ToCurrency, &result.Rate, &effective, &result.Source)
		if err != nil {
			return err
		}
		result.FromCurrency = strings.TrimSpace(result.FromCurrency)
		result.ToCurrency = strings.TrimSpace(result.ToCurrency)
		if effective.Valid {
			result.EffectiveDate = effective.Time.Format("2006-01-02")
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "RATE_NOT_FOUND", "exchange rate not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "RATE_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (service *Service) DeleteExchangeRate(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	id, err := pathID(chi.URLParam(request, "id"))
	if err != nil || id <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "id must be a positive integer")
		return
	}
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(request.Context(), `DELETE FROM exchange_rates WHERE tenant_id = $1 AND id = $2`, tenant, id)
		return err
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "RATE_DELETE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// LatestExchangeRate returns the most recent rate for a currency pair whose
// effective date is <= today (404 when none exists).
func (service *Service) LatestExchangeRate(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "TENANT_REQUIRED", err.Error())
		return
	}
	from := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("from")))
	to := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("to")))
	if from == "" || to == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "from and to query params are required")
		return
	}
	var result ExchangeRateResponse
	var effective pgtype.Date
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(request.Context(), `
			SELECT id, from_currency, to_currency, rate::float8, effective_date, source
			FROM exchange_rates
			WHERE tenant_id = $1 AND from_currency = $2 AND to_currency = $3
			  AND effective_date <= CURRENT_DATE
			ORDER BY effective_date DESC, id DESC
			LIMIT 1
		`, tenant, from, to).Scan(&result.ID, &result.FromCurrency, &result.ToCurrency, &result.Rate, &effective, &result.Source)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "RATE_NOT_FOUND", "no exchange rate recorded for "+from+"/"+to)
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "RATE_LOOKUP_FAILED", err.Error())
		return
	}
	result.FromCurrency = strings.TrimSpace(result.FromCurrency)
	result.ToCurrency = strings.TrimSpace(result.ToCurrency)
	if effective.Valid {
		result.EffectiveDate = effective.Time.Format("2006-01-02")
	}
	writeJSON(writer, http.StatusOK, result)
}

func validateRateRequest(req ExchangeRateRequest) (string, string) {
	req.FromCurrency = strings.ToUpper(strings.TrimSpace(req.FromCurrency))
	req.ToCurrency = strings.ToUpper(strings.TrimSpace(req.ToCurrency))
	if len(req.FromCurrency) != 3 || len(req.ToCurrency) != 3 {
		return "INVALID_REQUEST", "from_currency and to_currency must be 3-letter ISO codes"
	}
	if req.FromCurrency == req.ToCurrency {
		return "INVALID_REQUEST", "from_currency and to_currency must differ"
	}
	if req.Rate <= 0 {
		return "INVALID_REQUEST", "rate must be positive"
	}
	if !validDate(req.EffectiveDate) {
		return "INVALID_REQUEST", "effective_date must be a valid date in YYYY-MM-DD format"
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var (
	errBaseCurrencyLocked = errors.New("base currency cannot be changed after journals exist")
	errUnknownCurrencyErr = errors.New("unknown currency")
	errAccountNotFoundErr = errors.New("account not found")
)

func errUnknownCurrency(code string) error {
	return fmt.Errorf("%w: %s", errUnknownCurrencyErr, code)
}

func errAccountNotFound(id int64) error {
	return fmt.Errorf("%w: %d", errAccountNotFoundErr, id)
}

func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

func decodeJSON(request *http.Request, target any) error {
	return json.NewDecoder(request.Body).Decode(target)
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	message = httperr.SanitizeMessage(status, code, message)
	writeJSON(writer, status, map[string]string{"code": code, "message": message})
}

func pathID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

func validDate(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	var d pgtype.Date
	return d.Scan(trimmed) == nil
}

func trimmed(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return strings.TrimSpace(t.String)
}

func textPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmedValue := strings.TrimSpace(*v)
	return &trimmedValue
}

func intPtr(v *int) *int32 {
	if v == nil {
		return nil
	}
	cast := int32(*v)
	return &cast
}
