package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/customer"
	"finance-accounting-app/backend/internal/db"
	"finance-accounting-app/backend/internal/item"
	"finance-accounting-app/backend/internal/reporting"
)

func TestLineTotalCents(t *testing.T) {
	tests := []struct {
		name           string
		qty            float64
		unitPriceCents int64
		discountCents  int64
		want           int64
	}{
		{name: "integer qty", qty: 2, unitPriceCents: 1500, discountCents: 0, want: 3000},
		{name: "fractional qty rounds", qty: 1.5, unitPriceCents: 1000, discountCents: 0, want: 1500},
		{name: "discount applied", qty: 10, unitPriceCents: 100, discountCents: 250, want: 750},
		{name: "rounds half up", qty: 0.125, unitPriceCents: 100, discountCents: 0, want: 13},
		{name: "qty three decimals", qty: 1.333, unitPriceCents: 3000, discountCents: 0, want: 3999},
		{name: "discount exceeds gross clamps to negative int", qty: 1, unitPriceCents: 100, discountCents: 500, want: -400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lineTotalCents(tc.qty, tc.unitPriceCents, tc.discountCents)
			if got != tc.want {
				t.Errorf("lineTotalCents(%v, %d, %d) = %d, want %d", tc.qty, tc.unitPriceCents, tc.discountCents, got, tc.want)
			}
		})
	}
}

func validCreate() CreateQuotationRequest {
	return CreateQuotationRequest{
		CustomerID:    1,
		QuotationDate: "2026-08-08",
		Lines: []QuotationLineRequest{
			{ItemID: 1, Qty: 2, UnitPriceCents: 1000, TaxRate: 11},
		},
	}
}

func TestValidateCreateRequest(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CreateQuotationRequest)
		wantError bool
	}{
		{name: "valid", mutate: func(*CreateQuotationRequest) {}},
		{name: "missing customer", mutate: func(r *CreateQuotationRequest) { r.CustomerID = 0 }, wantError: true},
		{name: "bad quotation date", mutate: func(r *CreateQuotationRequest) { r.QuotationDate = "not-a-date" }, wantError: true},
		{name: "bad valid_until", mutate: func(r *CreateQuotationRequest) { r.ValidUntil = "08-08-2026" }, wantError: true},
		{name: "empty lines", mutate: func(r *CreateQuotationRequest) { r.Lines = nil }, wantError: true},
		{name: "negative qty", mutate: func(r *CreateQuotationRequest) { r.Lines[0].Qty = -1 }, wantError: true},
		{name: "zero qty", mutate: func(r *CreateQuotationRequest) { r.Lines[0].Qty = 0 }, wantError: true},
		{name: "negative unit price", mutate: func(r *CreateQuotationRequest) { r.Lines[0].UnitPriceCents = -1 }, wantError: true},
		{name: "negative discount", mutate: func(r *CreateQuotationRequest) { r.Lines[0].DiscountCents = -5 }, wantError: true},
		{name: "tax rate above 100", mutate: func(r *CreateQuotationRequest) { r.Lines[0].TaxRate = 101 }, wantError: true},
		{name: "tax rate negative", mutate: func(r *CreateQuotationRequest) { r.Lines[0].TaxRate = -1 }, wantError: true},
		{name: "missing item id", mutate: func(r *CreateQuotationRequest) { r.Lines[0].ItemID = 0 }, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validCreate()
			tc.mutate(&req)
			code, _ := validateCreateRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateCreateRequest errorCode=%q, wantError=%v", code, tc.wantError)
			}
		})
	}
}

func TestPrepareLinesTotal(t *testing.T) {
	prepared, total, err := prepareLines([]QuotationLineRequest{
		{ItemID: 1, Qty: 2, UnitPriceCents: 1000, DiscountCents: 0},   // 2000
		{ItemID: 2, Qty: 1.5, UnitPriceCents: 1000, DiscountCents: 0}, // 1500
	})
	if err != nil {
		t.Fatalf("prepareLines returned error: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("expected 2 prepared lines, got %d", len(prepared))
	}
	if total != 3500 {
		t.Errorf("total = %d, want 3500", total)
	}
	if prepared[0].LineTotalCents != 2000 || prepared[1].LineTotalCents != 1500 {
		t.Errorf("line totals = %d,%d want 2000,1500", prepared[0].LineTotalCents, prepared[1].LineTotalCents)
	}
}

func TestTransitions(t *testing.T) {
	if !canSend(statusDraft) {
		t.Error("DRAFT should be able to send")
	}
	if canSend(statusSent) || canSend(statusCancelled) {
		t.Error("only DRAFT can be sent")
	}
	for _, status := range []string{statusDraft, statusSent} {
		if !canCancel(status) {
			t.Errorf("%s should be cancellable", status)
		}
	}
	if canCancel(statusConverted) || canCancel(statusCancelled) {
		t.Error("CONVERTED/CANCELLED should not be cancellable")
	}
	for _, status := range []string{statusDraft, statusSent} {
		if !canExpire(status) {
			t.Errorf("%s should be expirable", status)
		}
	}
	if canExpire(statusCancelled) || canExpire(statusConverted) {
		t.Error("CANCELLED/CONVERTED should not be expirable")
	}
}

// TestRequireRevenueAccount covers the quotation-create fix: an item without a
// revenue (sale) account must surface as errItemRevenueAccountRequired naming
// the item (400 ITEM_REVENUE_ACCOUNT_REQUIRED), not a swallowed line-insert
// error that aborts the transaction into a 500.
func TestRequireRevenueAccount(t *testing.T) {
	tests := []struct {
		name     string
		itemID   int64
		code     string
		itemName string
		revenue  int64
		wantErr  bool
	}{
		{name: "revenue set passes", itemID: 7, code: "ITM-001", itemName: "Kabel", revenue: 42},
		{name: "missing revenue names the item", itemID: 7, code: "ITM-001", itemName: "Kabel", revenue: 0, wantErr: true},
		{name: "missing revenue without name still names code", itemID: 9, code: "ITM-002", itemName: "", revenue: 0, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireRevenueAccount(tc.itemID, tc.code, tc.itemName, tc.revenue)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("requireRevenueAccount(%d, %q, %q, %d) = %v, want nil", tc.itemID, tc.code, tc.itemName, tc.revenue, err)
				}
				return
			}
			if !errors.Is(err, errItemRevenueAccountRequired) {
				t.Fatalf("error = %v, want errItemRevenueAccountRequired", err)
			}
			if !strings.Contains(err.Error(), strconv.FormatInt(tc.itemID, 10)) {
				t.Errorf("message %q does not mention item id %d", err.Error(), tc.itemID)
			}
			if !strings.Contains(err.Error(), tc.code) {
				t.Errorf("message %q does not mention item code %q", err.Error(), tc.code)
			}
		})
	}
}

// TestRequireRevenueAccountSalesOrder mirrors the quotation guard for the
// createOrderInTx path: an SO line whose item has no revenue (sale) account
// must fail with errItemRevenueAccountRequired — mapped by CreateOrder to
// 400 ITEM_REVENUE_ACCOUNT_REQUIRED — instead of reaching the
// sales_orders_lines insert with revenue_account_id=0 and dying on the FK.
func TestRequireRevenueAccountSalesOrder(t *testing.T) {
	tests := []struct {
		name     string
		itemID   int64
		code     string
		itemName string
		revenue  int64
		wantErr  bool
	}{
		{name: "sale account set passes", itemID: 11, code: "BRG-01", itemName: "Semen 50kg", revenue: 900},
		{name: "missing sale account names item", itemID: 12, code: "BRG-02", itemName: "Besi Beton", revenue: 0, wantErr: true},
		{name: "missing sale account falls back to code", itemID: 13, code: "JASA-07", itemName: "   ", revenue: 0, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireRevenueAccount(tc.itemID, tc.code, tc.itemName, tc.revenue)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("requireRevenueAccount(%d, %q, %q, %d) = %v, want nil", tc.itemID, tc.code, tc.itemName, tc.revenue, err)
				}
				return
			}
			if !errors.Is(err, errItemRevenueAccountRequired) {
				t.Fatalf("error = %v, want errItemRevenueAccountRequired", err)
			}
			if !strings.Contains(err.Error(), strconv.FormatInt(tc.itemID, 10)) {
				t.Errorf("message %q does not mention item id %d", err.Error(), tc.itemID)
			}
			wantLabel := strings.TrimSpace(tc.itemName)
			if wantLabel == "" {
				wantLabel = tc.code
			}
			if !strings.Contains(err.Error(), wantLabel) {
				t.Errorf("message %q does not mention label %q", err.Error(), wantLabel)
			}
		})
	}
}

// TestInvoiceVoidGuard pins the pure status/payment gate behind
// POST /invoices/{id}/void: only posted invoices without recorded payments are
// voidable; every rejection maps to a distinct sentinel (→ explicit 4xx code
// in voidErrorFor), so a paid invoice can never be voided by accident.
func TestInvoiceVoidGuard(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		hasPayments bool
		wantErr     error
	}{
		{name: "issued without payments is voidable", status: invIssued},
		{name: "partially_paid without payments is voidable", status: invPartiallyPaid},
		{name: "issued with payments rejected", status: invIssued, hasPayments: true, wantErr: errInvoiceHasPayments},
		{name: "partially_paid with payments rejected", status: invPartiallyPaid, hasPayments: true, wantErr: errInvoiceHasPayments},
		{name: "paid rejected — use credit note", status: invPaid, wantErr: errInvoiceNotVoidable},
		{name: "already void rejected", status: invVoid, wantErr: errInvoiceAlreadyVoid},
		{name: "draft rejected", status: invDraft, wantErr: errInvoiceDraft},
		{name: "unknown status rejected", status: "APPROVED", wantErr: errInvoiceNotVoidable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := invoiceVoidGuard(tc.status, tc.hasPayments)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("invoiceVoidGuard(%q, %v) = %v, want nil", tc.status, tc.hasPayments, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("invoiceVoidGuard(%q, %v) = %v, want %v", tc.status, tc.hasPayments, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// End-to-end void verification against a live database (TEST_DATABASE_URL).
// Exercises the exact mounted path /api/v1/invoices/{id}/void through chi +
// the real auth middleware: register → customer → goods item → posted invoice
// (DPP + PPN) → void → trial balance nets back to the pre-invoice baseline;
// replay with the same idempotency key is idempotent and a second void is
// rejected with 409 INVOICE_ALREADY_VOID.
// ---------------------------------------------------------------------------

type tbSnapshot map[int64][2]int64 // account_id -> [debit, credit]

func snapshotTB(t *testing.T, result reporting.TrialBalanceResult) tbSnapshot {
	t.Helper()
	snap := tbSnapshot{}
	for _, row := range result.Rows {
		snap[row.AccountID] = [2]int64{row.DebitCents, row.CreditCents}
	}
	return snap
}

func TestVoidInvoiceEndToEnd(t *testing.T) {
	ctx := context.Background()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	authService := auth.NewService(pool, "test-secret-that-is-long-enough-32ch")
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "void-e2e-" + suffix + "@test.local"

	// Register a fresh tenant book (seeds the COA + an OPEN current-year period).
	regBody, _ := json.Marshal(auth.RegisterRequest{
		Email:      email,
		Password:   "OwnerPass!2026",
		FullName:   "Void E2E",
		TenantName: "Void E2E " + suffix,
	})
	rr := httptest.NewRecorder()
	authService.Register(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("register = %d (%s)", rr.Code, rr.Body.String())
	}
	var reg struct {
		AccessToken string `json:"access_token"`
		TenantID    int64  `json:"tenant_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}
	if reg.AccessToken == "" || reg.TenantID <= 0 {
		t.Fatalf("register response missing access_token/tenant_id: %s", rr.Body.String())
	}
	t.Cleanup(func() { cleanupTenantBook(ctx, adminURL, reg.TenantID, email) })

	// Mount like main.go's write group: auth middleware + sales routes.
	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authService.Middleware)
			r.Post("/customers", customer.NewHandler(pool).CreateCustomer)
			r.Post("/items", item.NewHandler(pool).Create)
			r.Get("/reports/trial-balance", reporting.NewHandler(pool).TrialBalance)
			NewHandler(pool).Routes(r)
		})
	})
	server := httptest.NewServer(router)
	defer server.Close()

	call := func(method, path string, body any, idemKey string) (int, map[string]any) {
		t.Helper()
		var reader io.Reader
		if body != nil {
			raw, _ := json.Marshal(body)
			reader = bytes.NewReader(raw)
		} else {
			reader = bytes.NewReader(nil)
		}
		req, _ := http.NewRequestWithContext(ctx, method, server.URL+path, reader)
		req.Header.Set("Authorization", "Bearer "+reg.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	mustID := func(status int, out map[string]any, what string) int64 {
		t.Helper()
		if status >= 300 {
			t.Fatalf("%s create = %d (%v)", what, status, out)
		}
		id, ok := out["id"].(float64)
		if !ok || id <= 0 {
			t.Fatalf("%s response missing id: %v", what, out)
		}
		return int64(id)
	}

	status, out := call(http.MethodPost, "/api/v1/customers", map[string]any{
		"code": "CUST-W6B", "name": "Void E2E Customer",
	}, "")
	customerID := mustID(status, out, "customer")

	var saleAcct, cogsAcct, invAcct int64
	if err := db.WithTenantData(ctx, pool, reg.TenantID, func(tx pgx.Tx) error {
		for code, dest := range map[string]*int64{"4101": &saleAcct, "5101": &cogsAcct, "1301": &invAcct} {
			if err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE tenant_id = $1 AND code = $2`, reg.TenantID, code).Scan(dest); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("resolve seeded accounts: %v", err)
	}
	fifo := "fifo"
	status, out = call(http.MethodPost, "/api/v1/items", map[string]any{
		"code": "ITM-W6B", "name": "Void E2E Item", "item_type": "goods",
		"costing_method": &fifo, "is_tracked_stock": true,
		"sale_account_id": &saleAcct, "cogs_account_id": &cogsAcct, "inventory_account_id": &invAcct,
	}, "")
	itemID := mustID(status, out, "item")

	fetchTB := func() reporting.TrialBalanceResult {
		t.Helper()
		st, tbOut := call(http.MethodGet, "/api/v1/reports/trial-balance", nil, "")
		if st != http.StatusOK {
			t.Fatalf("trial-balance = %d (%v)", st, tbOut)
		}
		raw, _ := json.Marshal(tbOut)
		var result reporting.TrialBalanceResult
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	baseline := fetchTB()
	baselineSnap := snapshotTB(t, baseline)

	// Posted invoice: 2 x Rp5.000.000 DPP + 11% PPN = Rp11.100.000 total.
	invoicePayload := map[string]any{
		"customer_id":  customerID,
		"invoice_date": time.Now().Format("2006-01-02"),
		"lines": []map[string]any{
			{"item_id": itemID, "qty": 2, "unit_price_cents": 500000, "tax_rate": 11},
		},
	}
	status, out = call(http.MethodPost, "/api/v1/invoices", invoicePayload, uuid.NewString())
	invoiceID := mustID(status, out, "invoice")
	if out["status"] != invIssued {
		t.Fatalf("invoice status = %v, want ISSUED", out["status"])
	}
	if got, ok := out["total_cents"].(float64); !ok || int64(got) != 1110000 {
		t.Fatalf("total_cents = %v, want 1110000", out["total_cents"])
	}

	afterInvoice := snapshotTB(t, fetchTB())
	moved := len(afterInvoice) != len(baselineSnap)
	if !moved {
		for accountID, before := range baselineSnap {
			if after := afterInvoice[accountID]; after != before {
				moved = true
				break
			}
		}
	}
	if !moved {
		t.Fatal("invoice did not move the trial balance — nothing to void")
	}

	// Void it.
	voidKey := uuid.NewString()
	status, out = call(http.MethodPost, fmt.Sprintf("/api/v1/invoices/%d/void", invoiceID),
		map[string]any{"reason": "e2e verification"}, voidKey)
	if status != http.StatusOK {
		t.Fatalf("void = %d (%v)", status, out)
	}
	if out["status"] != invVoid {
		t.Fatalf("status after void = %v, want VOID", out["status"])
	}
	if got, ok := out["receivable_cents"].(float64); !ok || got != 0 {
		t.Fatalf("receivable after void = %v, want 0", out["receivable_cents"])
	}

	// Trial balance must net back exactly to the pre-invoice baseline:
	// SALES_INVOICE and SALES_INVOICE_VOID cancel each other per account.
	// Gross totals may include the offsetting pairs (both journals stay
	// POSTED — the immutable audit trail), so the invariants are:
	// still balanced, and every account's NET equals its baseline net.
	final := fetchTB()
	if !final.Balanced {
		t.Fatal("trial balance not balanced after void")
	}
	netOf := func(snap tbSnapshot) map[int64]int64 {
		nets := map[int64]int64{}
		for accountID, dc := range snap {
			nets[accountID] = dc[0] - dc[1]
		}
		return nets
	}
	beforeNets := netOf(baselineSnap)
	afterNets := netOf(snapshotTB(t, final))
	for accountID := range beforeNets {
		if afterNets[accountID] != beforeNets[accountID] {
			t.Errorf("account %d net moved: before=%d after=%d", accountID, beforeNets[accountID], afterNets[accountID])
		}
	}
	for accountID := range afterNets {
		if _, ok := beforeNets[accountID]; !ok && afterNets[accountID] != 0 {
			t.Errorf("new account %d has non-zero net %d after void", accountID, afterNets[accountID])
		}
	}

	// Exactly one SALES_INVOICE_VOID reversal journal for this invoice.
	var reversalCount int
	if err := db.WithTenantData(ctx, pool, reg.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM journal_entries
			WHERE tenant_id = $1 AND intent_type = 'SALES_INVOICE_VOID'
			  AND source_ref = $2
		`, reg.TenantID, fmt.Sprintf("INV-VOID-%d", invoiceID)).Scan(&reversalCount)
	}); err != nil {
		t.Fatal(err)
	}
	if reversalCount != 1 {
		t.Fatalf("reversal journals = %d, want 1", reversalCount)
	}

	// Replay with the same idempotency key returns the same VOID state (200).
	replayStatus, replayOut := call(http.MethodPost, fmt.Sprintf("/api/v1/invoices/%d/void", invoiceID),
		map[string]any{"reason": "e2e verification"}, voidKey)
	if replayStatus != http.StatusOK || replayOut["status"] != invVoid {
		t.Fatalf("idempotent replay = %d (%v), want 200 VOID", replayStatus, replayOut)
	}

	// A genuinely new void attempt is rejected as already void.
	againStatus, againOut := call(http.MethodPost, fmt.Sprintf("/api/v1/invoices/%d/void", invoiceID),
		nil, uuid.NewString())
	if againStatus != http.StatusConflict {
		t.Fatalf("second void = %d (%v), want 409", againStatus, againOut)
	}
	if againOut["code"] != "INVOICE_ALREADY_VOID" {
		t.Fatalf("second void code = %v, want INVOICE_ALREADY_VOID", againOut["code"])
	}
}

// cleanupTenantBook removes the freshly created tenant book best-effort so the
// shared test database stays clean across runs.
func cleanupTenantBook(ctx context.Context, adminURL string, tenantID int64, email string) {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	_, _ = conn.Exec(ctx, `
		DELETE FROM journal_lines WHERE tenant_id = $1;
		DELETE FROM journal_entries WHERE tenant_id = $1;
		DELETE FROM ledger_chain_heads WHERE tenant_id = $1;
		DELETE FROM outbox_events WHERE tenant_id = $1;
		DELETE FROM audit_logs WHERE tenant_id = $1;
		DELETE FROM customer_balances WHERE tenant_id = $1;
		DELETE FROM invoice_payments WHERE tenant_id = $1;
		DELETE FROM invoice_lines WHERE tenant_id = $1;
		DELETE FROM invoices WHERE tenant_id = $1;
		DELETE FROM sales_quotations_lines WHERE tenant_id = $1;
		DELETE FROM sales_quotations WHERE tenant_id = $1;
		DELETE FROM sales_orders_lines WHERE tenant_id = $1;
		DELETE FROM sales_orders WHERE tenant_id = $1;
		DELETE FROM sales_down_payments WHERE tenant_id = $1;
		DELETE FROM credit_note_lines WHERE tenant_id = $1;
		DELETE FROM credit_notes WHERE tenant_id = $1;
		DELETE FROM delivery_order_lines WHERE tenant_id = $1;
		DELETE FROM delivery_orders WHERE tenant_id = $1;
		DELETE FROM inventory_movements WHERE tenant_id = $1;
		DELETE FROM item_prices WHERE tenant_id = $1;
		DELETE FROM items WHERE tenant_id = $1;
		DELETE FROM customers WHERE tenant_id = $1;
		DELETE FROM accounts WHERE tenant_id = $1;
		DELETE FROM categories WHERE tenant_id = $1;
		DELETE FROM accounting_periods WHERE tenant_id = $1;
		DELETE FROM report_frameworks WHERE tenant_id = $1;
		DELETE FROM tax_rates WHERE tenant_id = $1;
		DELETE FROM document_numbering WHERE tenant_id = $1;
		DELETE FROM user_tenants WHERE tenant_id = $1;
		DELETE FROM tenants WHERE id = $1;
		DELETE FROM user_tokens WHERE user_id IN (SELECT id FROM users WHERE email = $2);
		DELETE FROM users WHERE email = $2;
	`, tenantID, email)
}
