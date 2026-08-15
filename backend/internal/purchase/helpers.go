package purchase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/httperr"
)

type Service struct {
	pool *pgxpool.Pool
}

// Shared account codes (seeded by migration 000011).
const (
	apAccountCode          = "2101" // Accounts Payable (formal)
	uninvoicedPayableCode  = "2105" // Uninvoiced Payables (accrued)
	inputVATAccountCode    = "1203" // Input VAT (PPN masukan)
	purchasePrepayCode     = "1205" // Purchase Prepayment (Advance to Supplier)
	inventoryAccountCode   = "1301" // Inventory
	overpaymentAccountCode = "1204" // Other Receivables (supplier overpayment)
)

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (service *Service) Routes(router chi.Router) {
	router.Post("/suppliers", service.CreateSupplier)
	router.Get("/suppliers", service.ListSuppliers)
	router.Get("/suppliers/{id}", service.GetSupplier)
	router.Put("/suppliers/{id}", service.UpdateSupplier)
	router.Post("/suppliers/{id}/deactivate", service.DeactivateSupplier)

	router.Post("/purchase-orders", service.CreatePurchaseOrder)
	router.Get("/purchase-orders", service.ListPurchaseOrders)
	router.Get("/purchase-orders/{id}", service.GetPurchaseOrder)

	router.Post("/goods-received-notes", service.CreateGRN)
	router.Get("/goods-received-notes", service.ListGRNs)
	router.Get("/goods-received-notes/{id}", service.GetGRN)

	router.Post("/purchase-returns", service.CreatePurchaseReturn)
	router.Get("/purchase-returns", service.ListPurchaseReturns)
	router.Get("/purchase-returns/{id}", service.GetPurchaseReturn)
}

// ---------------------------------------------------------------------------
// Helpers (shared across purchase handlers)
// ---------------------------------------------------------------------------

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeJSON(request *http.Request, target any) error {
	// Buffer the body so it can be restored for downstream consumers (e.g.
	// the idempotency request-hash computed after decoding). Without this the
	// body is consumed here and the hash is always SHA-256("") — see M-023.
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return json.Unmarshal(bodyBytes, target)
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	message = httperr.SanitizeMessage(status, code, message)
	writeJSON(writer, status, errorResponse{Code: code, Message: message})
}

func tenantID(request *http.Request) (int64, error) {
	tenant, ok := auth.TenantIDFromContext(request.Context())
	if !ok || tenant <= 0 {
		return 0, errors.New("tenant context is required")
	}
	return tenant, nil
}

func userID(request *http.Request) int64 {
	value, _ := auth.UserIDFromContext(request.Context())
	return value
}

func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

func withTenant(ctx context.Context, tx pgx.Tx, tenantIDValue int64) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatInt(tenantIDValue, 10))
	return err
}

func isUniqueViolation(err error) bool {
	const pgErrCodeUniqueViolation = "23505"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation
}

func isForeignKeyViolation(err error) bool {
	const pgErrCodeForeignKeyViolation = "23503"
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgErrCodeForeignKeyViolation
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func parseDate(raw string) (pgtype.Date, error) {
	if strings.TrimSpace(raw) == "" {
		return pgtype.Date{}, errors.New("date is required")
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: parsed, Valid: true}, nil
}

func optionalDate(raw string) (pgtype.Date, error) {
	if strings.TrimSpace(raw) == "" {
		return pgtype.Date{}, nil
	}
	return parseDate(raw)
}

func optionalInt8(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

func textValueOptional(raw string) pgtype.Text {
	return pgtype.Text{String: raw, Valid: strings.TrimSpace(raw) != ""}
}

func dateString(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func numericToFloat(value pgtype.Numeric) float64 {
	f, err := value.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func validDate(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	_, err := parseDate(raw)
	return err == nil
}

func nextDocNumber(ctx context.Context, tx pgx.Tx, tenantID int64, docType, prefix string) (string, error) {
	year := time.Now().Year()
	var p string
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO document_numbering (tenant_id, doc_type, prefix, fiscal_year, last_seq)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (tenant_id, doc_type, prefix, fiscal_year) DO UPDATE
		SET last_seq = document_numbering.last_seq + 1
		RETURNING prefix, last_seq
	`, tenantID, docType, prefix, year).Scan(&p, &seq)
	if err != nil {
		return "", err
	}
	return p + "-" + strconv.FormatInt(int64(year), 10) + "-" + leftPad6(seq), nil
}

func leftPad6(seq int64) string {
	s := strconv.FormatInt(seq, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

func idempotencyKey(request *http.Request) (string, error) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", errors.New("Idempotency-Key header is required")
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(key); err != nil {
		return "", errors.New("Idempotency-Key must be a UUID")
	}
	return key, nil
}

func uuidValue(raw string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(raw)
	return value
}

func int8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value != 0}
}

// upsertSupplierBalance adjusts the AP sub-ledger balance for a supplier.
// apDelta and overpaymentDelta are added to the existing balance (use
// negative values to decrease). The row is created if it does not exist.
func upsertSupplierBalance(ctx context.Context, tx pgx.Tx, tenantID, supplierID int64, apDelta, overpaymentDelta int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO supplier_balances (tenant_id, supplier_id, ap_cents, overpayment_cents, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (tenant_id, supplier_id) DO UPDATE
		SET ap_cents = supplier_balances.ap_cents + EXCLUDED.ap_cents,
		    overpayment_cents = supplier_balances.overpayment_cents + EXCLUDED.overpayment_cents,
		    updated_at = now()
	`, tenantID, supplierID, apDelta, overpaymentDelta)
	return err
}
