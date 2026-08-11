# Implementation Plan — Full Deep Audit & Roadmap (v3)

**Audit Date:** 2026-08-11  
**Method:** 4 rounds of parallel deep audits. 10 audit dimensions. Every backend file read. Every frontend screen traced. Every migration inspected. Accounting math verified. Accessibility checked. Not docs-based.

---

## Executive Summary

| Layer | Completeness | Critical Issues |
|---|---|---|
| **Backend accounting logic** | ~80% | DP double-realization, cash flow 2× inflation, credit note COGS wrong for fractional qty, ECL ages from wrong date, shipping never posted |
| **Backend infrastructure** | ~70% | 11 packages skip RLS, 4 modules never post journals, httperr package is dead code, no graceful shutdown, no backup, prod missing NextReport |
| **Frontend forms** | ~40% | 4 forms allow duplicates (no saved-state), 3 forms don't load parent lines, PPN hardcoded 0, balance check is dead code, wrong COGS rounding |
| **Frontend architecture** | ~45% | No error boundary, tab switch destroys unsaved data, can't open 2 entries of same type, no FK combobox, no pagination, no toasts |
| **Database** | ~85% | 4 account collisions, 15+ missing indexes, 6 tables RLS not FORCED, migration 000003 missing, 000027 no down |
| **Security** | ~65% | 2FA bypass (CRITICAL), wildcard CORS, no 2FA rate limit, X-Forwarded-For spoofing, file upload MIME spoofing |
| **Accessibility** | ~20% | No semantic tables, no focus management, color contrast failures, tab strip not keyboard accessible |
| **Tests** | ~15% | Zero frontend tests, zero backend integration tests, critical posting paths untested |

**Total findings: 100+** across 10 audit dimensions.

---

## Part A: CRITICAL — Accounting Logic Bugs (Fix First)

### A-01: DP double-realization — same DP applied N times

**File:** `backend/internal/sales/invoices.go:200-203, 282-285`  
**Severity:** CRITICAL — 2201 goes negative, AR over-reduced

`dp_received_cents` is read from the SO but **never decremented** after invoice realization. Multiple invoices for the same SO each read the full DP amount and realize it again. Each realization posts `Dr 2201 / Cr 1201`, but 2201 was only funded once → 2201 goes negative (debit balance), AR is over-credited.

**Fix:** Add `dp_applied_cents` tracking on SO; decrement `dp_received_cents` or track `dp_consumed_cents` on realization.

### A-02: Cash flow multi-leg double-counting — total inflated M×

**File:** `backend/internal/reporting/data.go:418-419, 359/372/382/391/402/412`  
**Severity:** CRITICAL — cash flow report inflated for every multi-leg entry

The query joins each cash line to **every offsetting line** in the same entry, then sums the **cash line's** amount (not the offsetting line's). A lease payment `Dr 2301 Principal / Dr 5906 Interest / Cr Cash X` counts the full cash amount X twice: once as financing (via 2301) and once as operating (via 5906). Total cash flow is 2× overstated.

**Fix:** Sum `ol.debit_cents`/`ol.credit_cents` (the offsetting leg) instead of `jl.*`.

### A-03: Credit note COGS wrong for fractional quantities

**File:** `backend/internal/sales/credit_notes.go:194` + `delivery.go:545-547`  
**Severity:** HIGH — COGS reversal wrong for any fractional-qty return

`roundQty(line.Qty)` rounds to nearest integer **before** multiplying by unit cost. qty=2.5 → `roundQty(2.5)=3` → COGS = 3×cost instead of 2.5×cost. Meanwhile inventory is restored for the real qty=2.5 via `ReverseCOGS(... line.Qty ...)`. GL COGS and inventory movement are inconsistent.

**Fix:** Use `int64(math.Round(qty * float64(unitCostCents)))` — round the product, not the factor.

### A-04: ECL ages from invoice_date, not due_date

**File:** `backend/internal/tax/ecl.go:235, 262`  
**Severity:** HIGH — over-provisions for in-term invoices

Aging uses `invoice_date` instead of `due_date`. A customer on 60-day terms invoiced 40 days ago is classified as "31-60 days past due" when they're actually still within terms. This systematically **over-provisions** ECL.

**Fix:** Change to `due_date` and `ageDays = max(0, asOf - dueDate)`.

### A-05: PPN truncates instead of rounding

**File:** `backend/internal/sales/invoices.go:595`  
**Severity:** MEDIUM — per-line under-collection up to 1 cent

`ppnCents = lineTotal * taxRateMilli / 100000` uses int64 division (truncation). Codebase is inconsistent: `lineTotalCents` rounds half-up, `percentageRound` rounds half-up, but PPN truncates. Example: lineTotal=333, taxRate=11 → 36 (should be 37).

**Fix:** Use `(lineTotal * taxRateMilli + 50000) / 100000` for half-up rounding.

### A-06: Stored tax_total_cents is client-supplied, not computed

**File:** `backend/internal/sales/invoices.go:377`  
**Severity:** HIGH — invoice document can diverge from posted VAT

Journal credits `totalPPNCents` (computed), but invoice row stores `req.TaxTotalCents` (client value). If client sends a different value, the invoice document and GL diverge silently. Same for `sub_total_cents`, `discount_total_cents`.

**Fix:** Store computed values, not client-supplied.

### A-07: Shipping/other charges stored but never posted

**File:** `backend/internal/sales/invoices.go:369, 374-378, 596`  
**Severity:** HIGH — customer never billed for shipping

`shipping_fee_cents`, `other_charges_cents`, `rounding_cents` are written to the invoice row but **never added to totalCents** and never posted to any journal. The customer is never billed for shipping; the invoice total doesn't match the posted AR.

**Fix:** Add shipping/other to totalCents; post to a shipping-revenue account.

### A-08: Depreciation truncation — extends depreciation past useful life

**File:** `backend/internal/assets/depreciation.go:895, 898, 906`  
**Severity:** MEDIUM — asset depreciated for N+1 months instead of N

Straight-line: `depreciableBase / usefulLifeMonths` truncates. Cost=10000, life=3 → monthly=3333 (not 3333.33). After 3 months: accum=9999, book=1. Salvage=0, so the clamp doesn't fire until month 4. Asset is depreciated for 4 months instead of 3. Same for declining balance (`int64(float64 * rate)` truncates) and units-of-production.

**Fix:** Use `int64(math.Round(...))`; add "final month absorbs residual" logic.

### A-09: Lease in-advance vs in-arrears inconsistency

**File:** `backend/internal/lease/contracts.go:479, 493`  
**Severity:** MEDIUM — PV understated, interest front-loaded

Payment dates start at `startDate` (commencement = in-advance), but interest is computed as `remaining * rate` on full PV (in-arrears convention). PV formula is ordinary annuity (no `*(1+r)` factor). For PSAK 73 lessee leases (first payment at commencement), PV is understated and period-1 interest is overstated.

**Fix:** Pick one convention and apply consistently to PV, interest, and dates.

### A-10: ECL write-off doesn't update invoice or AR subledger

**File:** `backend/internal/tax/ecl.go:362-430`  
**Severity:** HIGH — written-off invoices re-aged and re-provisioned

Write-off posts `Dr 1202 / Cr 1201` but doesn't: reduce `invoices.receivable_cents`, set invoice status, or update `customer_balances.ar_cents`. The invoice remains "ISSUED" → next ECL run re-ages and re-provisions it → double-counted loss. Also doesn't check allowance sufficiency (1202 can go debit).

**Fix:** Update invoice receivable + status + customer_balances. Check allowance balance before write-off.

### A-11: Credit note COGS trusts client unit_cost, not original delivery cost

**File:** `backend/internal/sales/credit_notes.go:42, 194, 350`  
**Severity:** MEDIUM — COGS reversal can be wrong if client sends wrong cost

CN request carries `UnitCostCents`; code uses it directly without looking up the original delivery's unit cost from `inventory_movements` or consumed FIFO layers. For FIFO, returned qty is layered at potentially-incorrect cost, distorting future issues.

**Fix:** Look up original delivery cost from `inventory_movements` by `delivery_order_id` + `item_id`.

### A-12: Credit note reversal uses warehouse_id=0

**File:** `backend/internal/sales/credit_notes.go:350`  
**Severity:** MEDIUM — stock restored to wrong warehouse

`ReverseCOGS` is called with `warehouseID=0`. If original delivery was from another warehouse, stock lands in warehouse 0, corrupting per-warehouse balances.

**Fix:** Pass the original delivery's warehouse_id.

### A-13: Excess DP stranded in 2201

**File:** `backend/internal/sales/invoices.go:282-285`  
**Severity:** LOW — customer overpayment silently retained as liability

When `dpReceived > totalCents`, `dpApplied` is clamped to `totalCents`, but the residual stays in 2201 forever. No refund or customer-credit logic.

---

## Part B: CRITICAL — Security & Backend Infrastructure

### B-01: 2FA bypass via Setup2FA (SECURITY CRITICAL)

**File:** `backend/internal/auth/auth.go:439-441`  
**Severity:** CRITICAL

`Setup2FA` sets `totp_enabled = false` unconditionally. Attacker with stolen JWT → call setup → login with just password.

### B-02: AR aging queries nonexistent 'POSTED' status

**File:** `backend/internal/aging/handler.go:99,130` + `migrations/000036:28`  
**Severity:** CRITICAL — AR aging always returns 0 rows

Queries `status = 'POSTED'` but invoices only allow `ISSUED/PARTIALLY_PAID/PAID/VOID`.

### B-03: Production uses wrong table name

**File:** `backend/internal/production/helpers.go:212`  
**Severity:** CRITICAL — runtime crash

`doc_numbering` instead of `document_numbering`, `year` instead of `fiscal_year`.

### B-04: 4 account codes missing from seed.go

**File:** `backend/internal/auth/seed.go`  
**Severity:** CRITICAL — new tenants can't use lease depreciation or production

1702, 4902, 4908, 5908 — seeded only in migrations, not in seed.go for new tenants.

### B-05: 11 packages skip RLS entirely

**Severity:** CRITICAL — cross-tenant leak risk

`aging`, `cheque`, `costcenter`, `email`, `forecast`, `pettycash`, `recurring`, `pph`, `warehouse`, `dashboard`, `reports` — no `set_config('app.tenant_id', ...)`.

### B-06: 4 modules never post journals

**Severity:** CRITICAL — "posted" status misleading, ledger impact lost

`pettycash`, `recurring`, `pph`, `cheque` — set status to "posted" but never post journals.

### B-07: 6 tables RLS ENABLE but not FORCE

**File:** `migrations/000033:232-248`  
**Severity:** HIGH — RLS bypassable, no WITH CHECK

`cheques`, `cost_centers`, `cost_center_allocations`, `budget_variance_reports`, `email_templates`, `email_queue`.

### B-08: Wildcard CORS on financial API

**File:** `backend/internal/middleware/middleware.go:65`  
**Severity:** HIGH

`AllowedOrigins: []string{"*"}` with `Authorization` in allowed headers.

### B-09: No rate limiting on 2FA verification

**File:** `backend/cmd/api/main.go:110-111`  
**Severity:** HIGH — TOTP brute-force feasible

`/auth/2fa/verify` and `/auth/2fa/disable` have no rate limiting. Also: `X-Forwarded-For` trusted without proxy verification.

### B-10: Timeout middleware goroutine race on ResponseWriter

**File:** `backend/internal/middleware/middleware.go:117-119`  
**Severity:** CRITICAL — data race + goroutine leak

When timeout fires, the spawned goroutine is still running `next.ServeHTTP(w, r)` and will also write to `w`. Concurrent writes to `http.ResponseWriter` = data race + potential panic. The handler goroutine continues running after the response is sent.

**Fix:** Use `http.TimeoutHandler` wrapper instead of goroutine + select.

### B-11: Rate limiter cleanup goroutine never stops

**File:** `backend/internal/middleware/middleware.go:166-172`  
**Severity:** MEDIUM — goroutine leak

Cleanup goroutine runs forever with no `done` channel or `Close()`. If `Middleware()` is called multiple times, each call spawns a new goroutine.

### B-12: WithTransaction no defer rollback (panic-unsafe)

**File:** `backend/internal/db/transaction.go:10-22`  
**Severity:** MEDIUM — transaction leak on panic

If `fn(tx)` panics, `tx.Rollback(ctx)` is skipped. The transaction stays open until context cancel. Should use `defer tx.Rollback(ctx)` (no-op after Commit).

### B-13: errors.Is not used for ErrNoRows

**Files:** `approval/gate.go:43`, `costing/costing.go:363`, `reports/templates.go:585`, `auth/seed.go:81`  
**Severity:** MEDIUM — wrapped errors treated as real errors

`err == pgx.ErrNoRows` fails if error is wrapped with `fmt.Errorf("...: %w", err)`. Should use `errors.Is(err, pgx.ErrNoRows)`.

### B-14: httperr package is dead code — 7 duplicate errorResponse structs

**File:** `backend/internal/httperr/httperr.go` (unused) vs 7 local `writeError`/`errorResponse`  
**Severity:** HIGH — no `request_id` correlation, internal errors leaked to clients

Canonical `{code, message, details, request_id}` shape defined but never used. 7 packages define their own `{code, message}` (dropping details + request_id). Handlers return `err.Error()` as `message` — exposes SQL text, constraint names, account internals.

### B-15: No graceful shutdown

**File:** `backend/cmd/api/main.go:313`  
**Severity:** MEDIUM — connections leak on every deploy

`log.Fatal(http.ListenAndServe(...))` — no `signal.Notify`, no `http.Server.Shutdown`, `defer pool.Close()` is skipped. `deploy.sh` does `docker compose up -d --build` + `sleep 5` — no drain.

### B-16: No backup strategy

**Severity:** HIGH — no point-in-time recovery possible

No `pg_dump` cron, no `pgbackrest`/`wal-g`, no backup volume. `deploy.sh` runs migrations without pre-backup. Only safety net is hash-chain (tamper-evidence, not backup).

### B-17: Prod missing NextReport service

**File:** `docker-compose.prod.yml`  
**Severity:** MEDIUM — report rendering 502s in production

Prod has only postgres/api/web/caddy. `POST /reports/templates/{id}/render` falls back to `localhost:3100` and 502s. `HealthDetailed` always reports `not_configured`.

### B-18: No security headers on Caddy

**File:** `Caddyfile`  
**Severity:** MEDIUM — no HSTS, X-Frame-Options, X-Content-Type-Options, CSP

HTTP `:80` block serves plain HTTP with no redirect to HTTPS (Cloudflare Flexible mode — insecure).

### B-19: No DB pool tuning

**File:** `backend/cmd/api/main.go:54`  
**Severity:** MEDIUM — uses pgxpool defaults

No `MaxConns`, `MaxConnLifetime`, `MaxConnIdleTime` configuration. No `pool.Ping` on startup.

### B-20: File upload trusts client Content-Type

**File:** `backend/internal/audit/attachments.go:83`  
**Severity:** MEDIUM — MIME spoofing → stored XSS

No content sniffing (magic bytes). Attacker can upload HTML disguised as `image/png`.

---

## Part C: Frontend Form Bugs

### C-01: CreditNoteForm stale qty — totals don't compute on first entry

**File:** `web/src/screens/entry/CreditNoteForm.tsx:71-72`  
**Severity:** CRITICAL — return total and COGS show Rp 0 on first qty entry

`l.qty > 0` checks OLD qty (before update), not new `qty`. Since seedLine starts at qty=0, first entry always sees old=0 → false → totals=0.

**Fix:** Check `qty > 0` (the parameter), not `l.qty > 0`.

### C-02: CashEntryForm balance check is dead code

**File:** `web/src/screens/entry/CashEntryForm.tsx:94-102, 114`  
**Severity:** HIGH — validation can never fail

`cashAmountCents` and `counterTotalCents` are literally the same computation from the same source array. The balance check `counterTotalCents !== cashAmountCents` can never be true.

### C-03: CashEntryForm Save&New — stale error + duplicate save

**File:** `web/src/screens/entry/CashEntryForm.tsx:174-184`  
**Severity:** HIGH — data loss + duplicates

After `await handleSubmit(...)`, `error` is the stale closure value (always null if no prior error) → form resets even on failure. Also: after successful save, Save&New button is still enabled → re-posts same data.

### C-04: DeliveryOrderForm wrong COGS rounding

**File:** `web/src/screens/entry/DeliveryOrderForm.tsx:98, 104`  
**Severity:** HIGH — wrong COGS for fractional quantities

`Math.round(qty) * l.unitCostCents` rounds qty to integer before multiplying. qty=1.5 → Math.round(1.5)=2 → COGS = 2×cost instead of 1.5×cost.

**Fix:** `Math.round(qty * l.unitCostCents)` — round the product.

### C-05: GRNForm doesn't load PO lines

**File:** `web/src/screens/entry/GRNForm.tsx:102`  
**Severity:** HIGH — user can't receive against a PO

No `useEffect` watching `poId` to load PO lines. User must manually add items and type quantities with no reference to what was ordered. No qty-ordered comparison, no over-delivery validation.

### C-06: DeliveryOrderForm doesn't load SO lines

**File:** `web/src/screens/entry/DeliveryOrderForm.tsx:169`  
**Severity:** HIGH — user can't fulfill an SO

Same pattern — SO selection only stores the ID. No qty-delivered-vs-ordered validation.

### C-07: CreditNoteForm doesn't load invoice lines

**File:** `web/src/screens/entry/CreditNoteForm.tsx:162-166`  
**Severity:** HIGH — user must manually type item IDs

No `useEffect` watching `invoiceId` to fetch invoice lines. Item field is free-text `<input>` (not a `<select>` like other forms). No return-qty-vs-invoiced-qty validation.

### C-08: SalesOrderForm has no Confirm button → DP unreachable

**File:** `web/src/screens/entry/SalesOrderForm.tsx:450`  
**Severity:** HIGH — can't receive down payments

DP panel only shows when `orderStatus === "CONFIRMED"`, but there's no Confirm button. After saving, if backend returns "DRAFT" or "OPEN", DP input form is hidden. User is stuck.

### C-09: QuotationForm Send button always disabled

**File:** `web/src/screens/entry/QuotationForm.tsx:265-268`  
**Severity:** MEDIUM — can't send quotation from form

Send button has `disabled` with no `onClick`. No Cancel or Mark-Expired button either.

### C-10: No saved-state tracking in 4 forms → duplicates

**Files:** `PurchaseOrderForm.tsx`, `QuotationForm.tsx`, `GRNForm.tsx`, `CreditNoteForm.tsx`  
**Severity:** HIGH — clicking Save again creates duplicate

After successful save, these forms don't track the saved ID. Save button stays visible, fields stay editable. User can save again → duplicate entry. Compare with InvoiceForm (`setInvId`) and SalesOrderForm (`setOrderId`) which properly transition to existing mode.

### C-11: PPN/tax hardcoded to 0 across 4 forms

**Files:** `InvoiceForm.tsx:161`, `SalesOrderForm.tsx`, `PurchaseOrderForm.tsx:30`, `QuotationForm.tsx`  
**Severity:** HIGH — PPN compliance gap

No tax/PPN input field anywhere in the UI. `tax_rate: 0` is always sent. For an Indonesian accounting app where PPN 11% is legally required, this is a compliance gap.

### C-12: CustomerForm permanent loading

**File:** `web/src/screens/entry/CustomerForm.tsx:31, 81-83`  
**Severity:** CRITICAL — form unreachable

`setLoading(true)` never set to false. 16 ERP fields wired but all behind a permanent spinner.

### C-13: No Error Boundary

**File:** `web/src/App.tsx`  
**Severity:** CRITICAL — any component throw crashes entire app

### C-14: Tab switch destroys unsaved form data

**File:** `web/src/workbench/WorkArea.tsx:155`  
**Severity:** HIGH — silent data loss

`key={activeChild?.id}` remounts form on tab switch. All useState lost. Tab shows "unsaved" dot but form is empty when returning.

### C-15: Can't open two entries of the same type

**File:** `web/src/workbench/state.tsx:278-284`  
**Severity:** HIGH — dedup bug

Dedup checks `draft` flag but not `entryId`. Two different invoices = same tab.

### C-16: 5 entryKindToListKind mappings missing

**File:** `web/src/workbench/state.tsx:324-383`  
**Severity:** HIGH — drafts open in wrong module

BOM, Production Job, Lease, Customer entries fall through to "cash-other-receipt".

### C-17: 6 entry forms accept entryId but never load existing data

**Files:** QuotationForm, CashEntryForm, CreditNoteForm, GRNForm, BudgetForm, PurchaseReturnForm  
**Severity:** MEDIUM — clicking existing entry shows blank form

### C-18: 12+ API methods defined but never called

**File:** `web/src/api.ts`  
**Severity:** MEDIUM — dead capability

reverseCash, exportReport, revalueAsset, calculateDeferredTax, tagJournalLine, sendQuotation, cancelQuotation, cancelSalesOrder, listEntityHierarchy, createEntityHierarchy, createSupplierPayment (orphaned caller).

### C-19: No getCustomer(id) API method

**File:** `web/src/api.ts`  
**Severity:** MEDIUM — customer editing impossible

### C-20: CustomerList & PurchaseSupplierList rows not clickable

**Files:** `CustomerList.tsx:58-67`, `PurchaseSupplierList.tsx`  
**Severity:** MEDIUM — can't edit from list

### C-21: FixedAssetForm money ×100 bug

**File:** `web/src/screens/entry/FixedAssetForm.tsx:118`  
**Severity:** HIGH — all asset costs off by 100×

### C-22: No pagination on any list endpoint

**Severity:** HIGH — scalability cliff

Every list returns full result set. No `limit`/`offset` support. `CashEntryList` passes `limit: 200`.

### C-23: Dead status filter pills on 6 screens

**Severity:** HIGH — fake interactive UI

InvoiceList, SalesOrderList, DeliveryOrderList, SupplierInvoiceList, Sales.tsx, CashEntryList.

### C-24: Plain `<select>` for all FK lookups

**Severity:** HIGH — critical accounting UX gap

No type-to-search, no code lookup, no lazy loading. COA has 50-200 entries, customers thousands.

### C-25: No toast/notification system

**Severity:** MEDIUM — no feedback after actions

### C-26: Three currency formatting conventions

**Severity:** MEDIUM — `IDR 1,234,567` vs `1,234,567 Rp` vs `+ 1,234,567`

### C-27: Dashboard hardcoded KPIs + fake sparklines

**File:** `DashboardScreen.tsx` + `api.ts:623-624`  
**Severity:** MEDIUM — `dueBills: 2`, `lowStock: 4` are fake constants; sparklines are `Math.sin()`

### C-28: Reports no date range + no export

**File:** `Reports.tsx`  
**Severity:** HIGH — API supports dates but UI passes `undefined`; `exportReport()` exists but zero UI

---

## Part D: Database Schema Issues

### D-01: 4 account code collisions

| Code | seed.go | Migration | Conflict |
|---|---|---|---|
| 5904 | Deferred Tax Expense | Loss on FX (000029) | Down migration deletes wrong account |
| 5209 | Bad Debt Expense | RoU Depreciation (000026) | Lease + tax post to same account |
| 5203 | Transportation | Income Tax (000031) | Latent |
| 1304 | Finished Goods | Cheques in Transit (000033) | Latent |

### D-02: 15+ missing indexes

invoices(customer_id, status, sales_order_id), sales_orders(customer_id, status), cheques(status, direction), recurring(next_date, is_active), inventory_movements(movement_type, warehouse_id), journal_lines(tenant_id, entry_id), supplier_invoices(supplier_id), grn_lines(po_line_id), delivery_orders_lines(item_id), purchase_orders(supplier_id), approval_requests(entity_id, entity_type).

### D-03: Missing CHECK constraints

20+ monetary columns lack `CHECK (*_cents >= 0)`. `recurring_transactions.intent_type` is free TEXT. `tax_rates` missing UNIQUE on (tenant_id, tax_type, effective_from). `payment_terms.discount_percent` no CHECK. Missing FKs on `cheques.journal_entry_id`, `cheques.payment_id`.

### D-04: Migration issues

000003 missing. 000027 no down file. 000029.down dangerous (deletes 5904). 000039.down incomplete. 000031:22 invalid SQL. 000001.down leaves orphan functions.

### D-05: Report templates invisible to tenants

`tenant_id = 0` under RLS filter → invisible to every real tenant.

### D-06: user_tokens no RLS

Migration 000035 adds tenant_id but no RLS policy.

---

## Part E: Accessibility Issues

### E-01: No semantic HTML tables

All 5 list screens use `<div>` grids. Screen readers can't announce as tables. No `<table>/<thead>/<th scope="col">`.

### E-02: No focus management

No focus trap in modals/dialogs. No focus restoration on tab close. No focus move on tab activation. No roving tabindex on tab strip.

### E-03: Color contrast failures

| Token | Ratio | WCAG AA (4.5:1) | Usage |
|---|---|---|---|
| `--ink-muted` #8191a0 | 3.3:1 | FAIL | 40+ occurrences (secondary text, labels) |
| `--positive` #27966f | 3.75:1 | FAIL | Status badges |
| `--warning` #d18b2c | 3.3:1 | FAIL | Warning badges |
| `--ink-faint` #aebbc7 | 2.0:1 | FAIL | Placeholder text |
| `--accent` #2f80ed | 3.9:1 | FAIL | Links, transfer badges |
| 9px badge font | — | FAIL | Too small for any exemption |

### E-04: Tab strip not keyboard accessible

`TabStrip.tsx:34-38` — `<div role="tab">` with no `tabIndex`, no `onKeyDown`. Container uses `<nav>` not `role="tablist"`. No arrow key navigation.

### E-05: Dynamic content not announced to screen readers

Saved status, net totals, outstanding receivable, loading→error transitions — no `aria-live` regions.

### E-06: Grid inputs unlabeled

Detail grid inputs (item, qty, price, discount) in all entry forms — no labels, no `aria-label`, no `aria-labelledby`. `FieldShell` component exists with proper `htmlFor`/`id`/`aria-invalid`/`aria-describedby` but is **never used** by any form.

### E-07: FixedAssetList rows missing onKeyDown

Focusable (`tabIndex={0}`) but not activatable. CustomerList rows not interactive at all.

---

## Part F: Test Coverage Gaps

### F-01: Zero frontend tests

No vitest/jest config, no test files, no test dependencies.

### F-02: Zero backend integration tests

All tests are pure-function unit tests. No test connects to a real database. All SQL, hash-chain, outbox, RLS interactions untested.

### F-03: Critical untested paths

PPN 3-line journal, GRN Dr/Cr, period close SQL, hash chain sequence, lease PV rounding.

### F-04: Scan errors silently swallowed

`forecast/handler.go:110,135,161` and `dashboard/handler.go:522,572,610,670` — `rows.Scan` errors → `continue` (skip row). `rows.Err()` never checked after loop → partial results without indication.

---

## Part G: Dead Code & Cleanup

| # | Item | Action |
|---|---|---|
| G-01 | `TransactionsScreen.tsx` — orphaned | DELETE |
| G-02 | `Assets.tsx` (AssetRegisterList) — 100% mock | DELETE |
| G-03 | `SupplierPaymentPanel.tsx` — orphaned | Wire or DELETE |
| G-04 | `asset-register` types — unreachable | DELETE |
| G-05 | `fmtIDR` duplicate in api.ts | DELETE |
| G-06 | `purchaseHandler` shadow in main.go:164 | Fix |
| G-07 | 20 stale branches (all merged) | DELETE |
| G-08 | `mockData: true` on `in-items` | REMOVE |
| G-09 | JournalEntryList vs JournalRegister 90% duplicate | MERGE |
| G-10 | `nginx.conf` — orphaned (docker uses Caddy) | DELETE |
| G-11 | `httperr` package — dead code | ADOPT or DELETE |

---

## Implementation Roadmap (Final)

### Phase 0: Critical Accounting Fixes (Day 1-4) — ~4 days

| Task | Effort |
|---|---|
| A-01: Fix DP double-realization (track dp_consumed on SO) | 4h |
| A-02: Fix cash flow multi-leg double-count (sum ol not jl) | 2h |
| A-03: Fix credit note COGS roundQty → round product | 1h |
| A-04: Fix ECL aging → use due_date not invoice_date | 2h |
| A-05: Fix PPN truncation → half-up rounding | 1h |
| A-06: Store computed tax_total not client-supplied | 2h |
| A-07: Add shipping/other charges to total + post | 3h |
| A-10: Fix ECL write-off → update invoice + subledger + allowance check | 3h |
| A-11: Look up original delivery cost for CN COGS | 2h |
| A-12: Fix CN warehouse_id=0 | 30 min |
| B-02: Fix aging 'POSTED' → 'ISSUED' + migration 000045 | 1h |
| B-03: Fix production helpers.go table name | 30 min |
| B-04: Add 4 missing accounts to seed.go + migration 000045 | 2h |

### Phase 1: Security & RLS (Day 2-5, parallel) — ~3 days

| Task | Effort |
|---|---|
| B-01: Fix 2FA bypass (Setup2FA must not disable) | 2h |
| B-05: Wrap 11 packages in withTenantRead/Write | 6h |
| B-06: Implement journal posting for 4 modules | 3 days |
| B-07: Add FORCE RLS + WITH CHECK to 6 tables | 1h |
| B-08: Fix CORS to specific origins | 30 min |
| B-09: Add rate limiting to 2FA + fix X-Forwarded-For | 2h |
| B-10: Fix timeout middleware (use http.TimeoutHandler) | 2h |
| B-12: Add defer tx.Rollback to WithTransaction | 30 min |
| B-13: Replace == with errors.Is for ErrNoRows (4 files) | 1h |
| B-20: Add content sniffing to file upload | 2h |
| D-01: Fix 4 account collisions | 3h |
| D-05: Fix report templates tenant_id=0 → per-tenant | 2h |
| D-06: Add RLS to user_tokens | 30 min |

### Phase 2: Frontend Form Fixes (Day 3-8) — ~5 days

| Task | Effort |
|---|---|
| C-12: Fix CustomerForm loading | 30 min |
| C-13: Add Error Boundary (app + per-tab) | 2h |
| C-14: Fix tab switch data loss (CSS hide vs unmount) | 3h |
| C-15: Fix dedup bug (compare entryId) | 1h |
| C-16: Fix 5 missing entryKindToListKind mappings | 1h |
| C-01: Fix CreditNoteForm stale qty | 30 min |
| C-02: Fix CashEntryForm balance check (remove dead code, add real check) | 2h |
| C-03: Fix CashEntryForm Save&New stale error + duplicate | 2h |
| C-04: Fix DeliveryOrderForm COGS rounding | 30 min |
| C-05: Add PO line loading to GRNForm | 3h |
| C-06: Add SO line loading to DeliveryOrderForm | 3h |
| C-07: Add invoice line loading to CreditNoteForm + item select | 3h |
| C-08: Add Confirm button to SalesOrderForm | 2h |
| C-09: Wire Send/Cancel in QuotationForm | 2h |
| C-10: Add saved-state tracking to 4 forms | 4h |
| C-11: Add PPN/tax input to 4 forms | 4h |
| C-21: Fix FixedAssetForm ×100 | 30 min |
| Build real ItemForm + createItem API | 1.5 days |
| Add getCustomer(id) API + fix CustomerForm 13 fields | 1 day |
| Build Combobox/SearchableSelect component | 2 days |
| Add toast/notification system | 1 day |
| Fix dead status filter pills (6 screens) | 2h |
| Fix dashboard hardcoded KPIs | 2h |
| Add report date ranges + export buttons | 4h |
| Wire 12+ orphaned API methods | 4h |
| Fix CustomerForm/PurchaseSupplierForm tabId | 1h |

### Phase 3: Backend Infrastructure (Day 5-9, parallel) — ~3 days

| Task | Effort |
|---|---|
| B-14: Adopt httperr.Write everywhere (replace 7 duplicates) | 1 day |
| B-15: Add graceful shutdown (signal.Notify + Shutdown) | 2h |
| B-19: Add DB pool tuning (MaxConns, lifetime, Ping) | 2h |
| B-16: Add backup strategy (pg_dump cron sidecar) | 4h |
| B-17: Add NextReport to prod compose | 1h |
| B-18: Add security headers to Caddy + HTTPS redirect | 1h |
| B-08: Add structured logging (log/slog + request_id) | 4h |
| Stop returning err.Error() to clients | 4h |
| Idempotency: extend request_hash to all posters | 1 day |
| Add audit.Log to all posting paths | 0.5 day |
| Fix error handling (400 → isNoRows/ValidationError/500) | 0.5 day |
| Fix reconciliation autoMatch N+1 | 0.5 day |
| Fix forecast/dashboard rows.Err() + scan error swallowing | 1h |

### Phase 4: Database Hardening (Day 8-11) — ~2 days

| Task | Effort |
|---|---|
| Add 15+ missing indexes (migration 000046) | 1 day |
| Add CHECK constraints + missing FKs (migration 000047) | 0.5 day |
| Fix 000029.down, 000039.down, write 000027.down | 1h |
| Fix 000031:22 invalid SQL | 30 min |

### Phase 5: Cross-Cutting UI (Day 10-14) — ~4 days

| Task | Effort |
|---|---|
| Add pagination to all lists (server + client) | 1 day |
| Add HTTP timeout/retry to http() | 0.5 day |
| Unify currency formatting | 0.5 day |
| Replace LoadingState with ListSkeleton | 0.5 day |
| Fix responsive design | 1 day |
| Harden InvoiceForm validation | 0.5 day |

### Phase 6: Accessibility (Day 12-16) — ~3 days

| Task | Effort |
|---|---|
| Convert list screens to semantic `<table>` | 1 day |
| Add ARIA labels to grid inputs (use FieldShell) | 1 day |
| Fix color contrast (--ink-muted, --positive, --warning) | 0.5 day |
| Make tab strip keyboard accessible (roving tabindex) | 0.5 day |
| Add aria-live for dynamic totals/status | 0.5 day |
| Add focus management (tab open/close, modal trap) | 0.5 day |

### Phase 7: Test Coverage (Day 15-20) — ~4 days

| Task | Effort |
|---|---|
| Set up vitest + write first frontend component tests | 1 day |
| Set up backend integration test harness (testcontainers) | 1 day |
| Write integration tests: PPN, GRN, period close, hash chain, DP realization | 2 days |

### Phase 8: Module Wiring (Day 8-16, parallel) — ~8 days

| Task | Effort |
|---|---|
| Wire 8 unwired backend modules (Petty Cash, Recurring, Cheque, Cost Center, Approval, Aging, Warehouse, Asset Maintenance) | 8 days |
| Wire Email Templates & Queue | 1 day |
| Wire Overhead Variance | 0.5 day |
| Wire Lease modify/terminate/depreciate UI | 1 day |

### Phase 9: Dead Code & Polish (Day 18-21) — ~1 day

| Task | Effort |
|---|---|
| Delete TransactionsScreen, Assets.tsx, nginx.conf, asset-register types | 30 min |
| Delete 20 stale branches | 10 min |
| Delete fmtIDR duplicate | 5 min |
| Merge/differentiate JournalEntryList vs JournalRegister | 0.5 day |
| Fix purchaseHandler shadow | 5 min |
| Remove mockData flag from in-items | 5 min |

---

## Verification Gate

After each phase:
```bash
cd backend && go vet ./... && go test ./... && go build ./...
cd web && npx tsc --noEmit && npx vite build
make fmt && make lint && make test && make web-build
```

## Total Effort Estimate

| Phase | Focus | Days |
|---|---|---|
| Phase 0 | Critical accounting fixes | 4.0 |
| Phase 1 | Security & RLS | 3.0 |
| Phase 2 | Frontend form fixes | 5.0 |
| Phase 3 | Backend infrastructure | 3.0 |
| Phase 4 | Database hardening | 2.0 |
| Phase 5 | Cross-cutting UI | 4.0 |
| Phase 6 | Accessibility | 3.0 |
| Phase 7 | Test coverage | 4.0 |
| Phase 8 | Module wiring (parallel) | 8.0 |
| Phase 9 | Dead code & polish | 1.0 |
| | **Total (with parallelism)** | **~35 days** |

## Priority Recommendation

1. **Phase 0 + Phase 1 (security)** immediately — accounting bugs corrupt the books, security bugs are exploitable
2. **Phase 2 (forms)** in parallel — Combobox + toast should start first (blocks all form work)
3. **Phase 3 (infra)** can run alongside Phase 2
4. **Phase 8 (module wiring)** use Agent Manager worktrees for parallel execution
5. **Phase 6 (a11y) + Phase 7 (tests)** after core fixes are stable
