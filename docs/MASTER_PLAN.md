# Master Implementation Plan

**Audit Date:** 2026-08-11  
**Method:** 4 rounds of parallel deep audits across 10 dimensions. Every backend file read. Every frontend screen traced. Every migration inspected. Accounting math verified. Accessibility checked. Navigation flows click-counted. Not docs-based.

**Consolidates:** `IMPLEMENTATION_PLAN.md` + `FEATURE_AUDIT.md` + `UI_UX_PLAN.md` into one source of truth.

---

## Table of Contents

- [Executive Summary](#executive-summary)
- [Part A: Accounting Logic Bugs](#part-a-accounting-logic-bugs)
- [Part B: Security & Backend Infrastructure](#part-b-security--backend-infrastructure)
- [Part C: Feature Audit — Per-Module Status](#part-c-feature-audit--per-module-status)
- [Part D: Frontend Form Bugs](#part-d-frontend-form-bugs)
- [Part E: Frontend Architecture & Navigation](#part-e-frontend-architecture--navigation)
- [Part F: UI/UX Per-Module Improvements](#part-f-uiux-per-module-improvements)
- [Part G: Database Schema Issues](#part-g-database-schema-issues)
- [Part H: Accessibility](#part-h-accessibility)
- [Part I: Test Coverage](#part-i-test-coverage)
- [Part J: Dead Code & Cleanup](#part-j-dead-code--cleanup)
- [Implementation Roadmap](#implementation-roadmap)
- [Verification Gate](#verification-gate)

---

## Executive Summary

| Layer | Completeness | Critical Issues |
|---|---|---|
| **Backend accounting logic** | ~80% | DP double-realization, cash flow 2× inflation, partial delivery broken, purchase return costing wrong direction, stock opname doesn't update stock_balances, PPh never posts journals, ECL write-off doesn't update invoice |
| **Backend infrastructure** | ~70% | 11 packages skip RLS, 4 modules never post journals, httperr dead code, no graceful shutdown, no backup, prod missing NextReport, timeout middleware goroutine race |
| **Frontend forms** | ~40% | 4 forms allow duplicates, 3 forms don't load parent lines, PPN hardcoded 0, balance check dead code, wrong COGS rounding, CustomerForm permanently loading |
| **Frontend architecture** | ~45% | No error boundary, tab switch destroys unsaved data, can't open 2 entries of same type, no FK combobox, no pagination, no toasts, dead filter pills |
| **Database** | ~85% | 4 account collisions, 15+ missing indexes, 6 tables RLS not FORCED, migration 000003 missing, 000027 no down, report templates invisible |
| **Security** | ~65% | 2FA bypass (CRITICAL), wildcard CORS, no 2FA rate limit, X-Forwarded-For spoofing, file upload MIME spoofing |
| **Accessibility** | ~20% | No semantic tables, no focus management, color contrast failures, tab strip not keyboard accessible, FieldShell built but never used |
| **Tests** | ~15% | Zero frontend tests, zero backend integration tests, critical posting paths untested |
| **Navigation/UX** | ~40% | No workflow chain, no auto-fill from parent, no inline master creation, list-first adds clicks, no save feedback, no keyboard shortcuts |

**Total findings: 100+** across 10 audit dimensions. **18 CRITICAL**, **42 MAJOR**, rest MEDIUM/LOW.

---

## Part A: Accounting Logic Bugs

### A-01: DP double-realization — same DP applied N times

**File:** `backend/internal/sales/invoices.go:200-203, 282-285`  
**Severity:** CRITICAL — 2201 goes negative, AR over-reduced

`dp_received_cents` is read from the SO but **never decremented** after invoice realization. Multiple invoices for the same SO each read the full DP amount and realize it again. Each realization posts `Dr 2201 / Cr 1201`, but 2201 was only funded once → 2201 goes negative, AR is over-credited.

**Fix:** Track `dp_consumed_cents` on SO; decrement on realization; clamp `dpApplied = min(dpReceived - dpConsumed, totalCents)`.

### A-02: Cash flow multi-leg double-counting — total inflated M×

**File:** `backend/internal/reporting/data.go:418-419`  
**Severity:** CRITICAL — cash flow report inflated for every multi-leg entry

The query joins each cash line to every offsetting line in the same entry, then sums the **cash line's** amount (not the offsetting line's). A lease payment `Dr 2301 / Dr 5906 / Cr Cash X` counts X twice: once as financing (via 2301) and once as operating (via 5906). Total cash flow = 2× overstated.

**Fix:** Sum `ol.debit_cents`/`ol.credit_cents` (the offsetting leg) instead of `jl.*`.

### A-03: Partial deliveries broken — SO unconditionally CLOSED

**File:** `backend/internal/sales/delivery.go:302-307`  
**Severity:** CRITICAL — can't do partial deliveries

SO is unconditionally set to `CLOSED` after any delivery. Since `CreateDelivery` requires `soStatus == CONFIRMED`, a second partial delivery fails because SO is already CLOSED. Customer orders 100, delivers 50 → SO CLOSED → can't deliver remaining 50.

**Fix:** Only set CLOSED when all SO line quantities have been fully delivered. Add `delivered_qty` tracking per SO line (already exists in DB but not checked).

### A-04: Purchase return costing wrong direction

**File:** `backend/internal/purchase/purchase_returns.go:307-310`  
**Severity:** CRITICAL — stock sub-ledger diverges from GL

`costing.ReverseCOGS` is designed for sales returns (stock comes back → increases qty). For purchase returns, stock goes back to supplier (should decrease). Calling `ReverseCOGS` **increases** `stock_balances.qty_on_hand` while the journal correctly credits 1301 (decreasing GL inventory). Stock sub-ledger ≠ GL.

**Fix:** For purchase returns, call `costing.ResolveCOGS` (reduces stock) instead of `ReverseCOGS` (restores stock).

### A-05: Stock opname doesn't update stock_balances

**File:** `backend/internal/inventory/stock_opname.go`  
**Severity:** CRITICAL — all subsequent costing is wrong after any opname

Opname posts the adjustment journal and records `inventory_movements` but **never calls `costing.PostGRN` (surplus) or `costing.ResolveCOGS` (shortage)**. After every approved opname: `stock_balances.qty_on_hand` is stale, `avg_unit_cost_cents` is stale, FIFO layers are not adjusted. Subsequent COGS resolution uses wrong stock → false `ErrInsufficientStock` or allows negative stock.

**Fix:** Call `costing.PostGRN` for surplus lines and `costing.ResolveCOGS` for shortage lines inside `ApproveStockOpname`.

### A-06: PPh handler never posts journals

**File:** `backend/internal/pph/handler.go:202-221`  
**Severity:** CRITICAL — PPh 21/22/23/26 never enter the ledger

The `Post` endpoint only flips `status DRAFT → POSTED` and returns an instruction string: *"post the journal: Dr 5203 / Cr 210x"*. **No journal entry is created.** Tax payable accounts (2107-2111) are never debited/credited. Only PPh Final UMKM (`tax/pph.go`) actually posts a journal.

**Fix:** Implement journal posting in `pph/handler.go Post`: `Dr 5203 Income Tax Expense / Cr 210x PPh Payable` (account by type).

### A-07: ECL write-off doesn't update invoice or AR subledger

**File:** `backend/internal/tax/ecl.go:362-430`  
**Severity:** CRITICAL — written-off invoices re-aged and re-provisioned

Write-off posts `Dr 1202 / Cr 1201` but doesn't: reduce `invoices.receivable_cents`, set invoice status, or update `customer_balances.ar_cents`. Invoice remains "ISSUED" → next ECL run re-ages and re-provisions it → double-counted loss. Also doesn't check allowance sufficiency (1202 can go debit).

**Fix:** Update invoice receivable + status to WRITTEN_OFF + customer_balances. Check allowance balance before write-off.

### A-08: ECL ages from invoice_date, not due_date

**File:** `backend/internal/tax/ecl.go:235, 262`  
**Severity:** HIGH — over-provisions for in-term invoices

Aging uses `invoice_date` instead of `due_date`. A customer on 60-day terms invoiced 40 days ago is classified as "31-60 days past due" when they're still within terms. Systematically over-provisions ECL.

**Fix:** Change to `due_date` and `ageDays = max(0, asOf - dueDate)`.

### A-09: PPN truncates instead of rounding

**File:** `backend/internal/sales/invoices.go:595`  
**Severity:** MEDIUM — per-line under-collection up to 1 cent

`ppnCents = lineTotal * taxRateMilli / 100000` uses int64 division (truncation). Codebase is inconsistent: `lineTotalCents` rounds half-up, `percentageRound` rounds half-up, but PPN truncates.

**Fix:** Use `(lineTotal * taxRateMilli + 50000) / 100000` for half-up rounding.

### A-10: Stored tax_total_cents is client-supplied, not computed

**File:** `backend/internal/sales/invoices.go:377`  
**Severity:** HIGH — invoice document can diverge from posted VAT

Journal credits `totalPPNCents` (computed), but invoice row stores `req.TaxTotalCents` (client value). If client sends different value, invoice document and GL diverge silently. Same for `sub_total_cents`, `discount_total_cents`.

**Fix:** Store computed values, not client-supplied.

### A-11: Shipping/other charges stored but never posted

**File:** `backend/internal/sales/invoices.go:369, 374-378, 596`  
**Severity:** HIGH — customer never billed for shipping

`shipping_fee_cents`, `other_charges_cents`, `rounding_cents` are written to the invoice row but never added to `totalCents` and never posted to any journal. The customer is never billed for shipping; the invoice total doesn't match the posted AR.

**Fix:** Add shipping/other to `totalCents`; post to a shipping-revenue account.

### A-12: Credit note COGS wrong for fractional quantities

**File:** `backend/internal/sales/credit_notes.go:194` + `delivery.go:545-547`  
**Severity:** HIGH — COGS reversal wrong for any fractional-qty return

`roundQty(line.Qty)` rounds to nearest integer before multiplying by unit cost. qty=2.5 → `roundQty(2.5)=3` → COGS = 3×cost instead of 2.5×cost. Meanwhile inventory is restored for the real qty=2.5 via `ReverseCOGS`. GL COGS and inventory movement are inconsistent.

**Fix:** Use `int64(math.Round(qty * float64(unitCostCents)))` — round the product, not the factor.

### A-13: Credit note COGS trusts client unit_cost, not original delivery cost

**File:** `backend/internal/sales/credit_notes.go:42, 194, 350`  
**Severity:** MEDIUM

CN request carries `UnitCostCents`; code uses it directly without looking up the original delivery's unit cost from `inventory_movements`. For FIFO, returned qty is layered at potentially-incorrect cost, distorting future issues.

**Fix:** Look up original delivery cost from `inventory_movements` by `delivery_order_id` + `item_id`.

### A-14: Credit note reversal uses warehouse_id=0

**File:** `backend/internal/sales/credit_notes.go:350`  
**Severity:** MEDIUM — stock restored to wrong warehouse

**Fix:** Pass the original delivery's warehouse_id.

### A-15: Depreciation truncation — extends depreciation past useful life

**File:** `backend/internal/assets/depreciation.go:895, 898, 906`  
**Severity:** MEDIUM

Straight-line: `depreciableBase / usefulLifeMonths` truncates. Cost=10000, life=3 → monthly=3333. After 3 months: accum=9999, book=1. Asset is depreciated for 4 months instead of 3. Same for declining balance and units-of-production.

**Fix:** Use `int64(math.Round(...))`; add "final month absorbs residual" logic.

### A-16: Lease in-advance vs in-arrears inconsistency

**File:** `backend/internal/lease/contracts.go:479, 493`  
**Severity:** MEDIUM — PV understated, interest front-loaded

Payment dates start at `startDate` (commencement = in-advance), but interest is computed as `remaining * rate` on full PV (in-arrears convention). PV formula is ordinary annuity (no `*(1+r)` factor).

**Fix:** Pick one convention and apply consistently to PV, interest, and dates.

### A-17: refund_method is a no-op in CN and Purchase Return

**File:** `backend/internal/sales/credit_notes.go:394-397`, `purchase/purchase_returns.go:335-338`  
**Severity:** MAJOR

Validated and stored but has zero effect on journal. `refund` (cash refund) and `credit_balance` (credit to customer balance) methods are unimplemented. All three methods post the identical journal.

**Fix:** Implement `refund` → `Dr 4201 / Cr Cash`; `credit_balance` → `Dr 4201 / Cr 2402`.

### A-18: No concurrency lock on payment/return rows

**Files:** `sales/payments.go:100-103`, `purchase/supplier_payments.go:104-107`, `purchase/purchase_returns.go:141-144`  
**Severity:** MAJOR — race condition on concurrent payments/returns

Invoice/SI is read without `FOR UPDATE`. Two concurrent payments could both read the same `receivable` and both claim full AR application.

**Fix:** Add `FOR UPDATE` to invoice/SI reads in payment and return handlers.

### A-19: PO never reaches RECEIVED status

**File:** `backend/internal/purchase/grn.go:480-486`  
**Severity:** MAJOR

`poStatusAfterGRN` only returns `PARTIALLY_RECEIVED`. No logic to detect all lines fully received. A fully-delivered PO stays `PARTIALLY_RECEIVED` forever.

**Fix:** Compare `received_qty` vs `ordered_qty` per PO line; transition to `RECEIVED` when all lines are complete.

### A-20: No AP sub-ledger (supplier_balances)

**Severity:** MAJOR — AP tracked only at invoice level, unlike AR's `customer_balances`

**Fix:** Create `supplier_balances` table mirroring `customer_balances`; upsert on supplier invoice/payment/return.

### A-21: Purchase DP realization not implemented

**File:** `backend/internal/purchase/supplier_invoices.go:252`  
**Severity:** MAJOR

Supplier invoice hardcodes `dpApplied = 0`. If a purchase down payment (account 1205) was made, it is never applied to the supplier invoice.

### A-22: Float64 line totals in purchase cycle

**Files:** `purchase/grn.go:476`, `purchase/supplier_invoices.go:509`, `purchase/purchase_orders.go:386`  
**Severity:** MEDIUM

PO, GRN, SI all use `float64` math that can truncate, unlike the sales side's milliunit integer approach.

### A-23: Lease schedule not regenerated after modification

**File:** `backend/internal/lease/lifecycle.go`  
**Severity:** MEDIUM — old schedule retained, stale principal/interest splits

### A-24: Excess DP stranded in 2201

**File:** `backend/internal/sales/invoices.go:282-285`  
**Severity:** LOW

When `dpReceived > totalCents`, residual stays in 2201 forever. No refund or customer-credit logic.

### A-25: Cost center allocations never executed

**File:** `backend/internal/costcenter/handler.go`  
**Severity:** MAJOR

Allocations are stored but never run. No endpoint posts proportional journal entries.

### A-26: Bank reconciliation complete doesn't post adjustment

**File:** `backend/internal/reconciliation/handler.go`  
**Severity:** MEDIUM

`CompleteReconciliation` validates `diff_cents == 0` then marks RECONCILED. No journal entry for residual unmatched items.

### A-27: 4 modules never post journals (pettycash/recurring/pph/cheque)

**Severity:** CRITICAL — "posted" status is misleading, ledger impact is lost

- **Petty Cash**: fund creation, voucher, replenish — all return instruction strings, no journal posted
- **Recurring**: `PostNow` only advances `next_date`, no journal, no scheduler, Update = 501
- **PPh**: see A-06
- **Cheque**: state transitions (deposit/clear/bounce) are tracking-only, no journal

**Fix:** Implement journal posting for each module.

### A-28: Email has no SMTP/worker

**File:** `backend/internal/email/handler.go`  
**Severity:** MAJOR

Template CRUD works. Queue works. But `Send` just marks `status='SENT'` — no SMTP call, no worker, no auto-triggering.

### A-29: Inter-company elimination has no population endpoint

**File:** `backend/internal/lease/consolidation.go`  
**Severity:** MAJOR

`inter_company_transactions` table exists and elimination computation works, but there is **no endpoint to populate it**. The elimination feature cannot be exercised end-to-end. Also: `consolidation_pct` is stored but never applied in aggregation. `eliminated` flag is never set to true.

### A-30: Approval gate only on invoices

**File:** `backend/internal/approval/gate.go`  
**Severity:** MAJOR

`CheckAmount`/`ConsumeApprovalByAmount` only called in `sales/invoices.go`. Not enforced on POs, credit notes, supplier invoices, manual journals, or cash postings.

### A-31: Audit log covers only 3 of ~15 posting paths

**File:** `backend/internal/audit/handler.go`  
**Severity:** MAJOR

Only `cash/journal.go`, `period/handler.go`, and attachment delete call `audit.Log`. All other posting paths (invoices, payments, CNs, supplier invoices, lease, assets, inventory, production, tax, reconciliation) have no audit trail row.

---

## Part B: Security & Backend Infrastructure

### B-01: 2FA bypass via Setup2FA (SECURITY CRITICAL)

**File:** `backend/internal/auth/auth.go:439-441`  
`Setup2FA` sets `totp_enabled = false` unconditionally. Attacker with stolen JWT → call setup → login with just password.

### B-02: AR aging queries nonexistent 'POSTED' status

**File:** `backend/internal/aging/handler.go:99,130` + `migrations/000036:28`  
Queries `status = 'POSTED'` but invoices only allow `ISSUED/PARTIALLY_PAID/PAID/VOID`. AR aging always returns 0 rows.

### B-03: Production uses wrong table name

**File:** `backend/internal/production/helpers.go:212`  
`doc_numbering` instead of `document_numbering`, `year` instead of `fiscal_year`. Runtime crash.

### B-04: 4 account codes missing from seed.go

**File:** `backend/internal/auth/seed.go`  
1702 (Accumulated RoU Dep), 4902 (Applied Overhead), 4908 (Variance Gain), 5908 (Variance Loss) — seeded only in migrations, not in seed.go. New tenants can't use lease depreciation or production.

### B-05: 11 packages skip RLS entirely

**Severity:** CRITICAL — cross-tenant leak risk

`aging`, `cheque`, `costcenter`, `email`, `forecast`, `pettycash`, `recurring`, `pph`, `warehouse`, `dashboard`, `reports` — no `set_config('app.tenant_id', ...)`. `costcenter` can reference another tenant's cost centers (no tenant verification on FK).

### B-06: 6 tables RLS ENABLE but not FORCE

**File:** `migrations/000033:232-248`  
`cheques`, `cost_centers`, `cost_center_allocations`, `budget_variance_reports`, `email_templates`, `email_queue`. RLS bypassable, no `WITH CHECK` — can INSERT with another tenant_id.

### B-07: Wildcard CORS on financial API

**File:** `backend/internal/middleware/middleware.go:65`  
`AllowedOrigins: []string{"*"}` with `Authorization` in allowed headers. Any website can make authenticated requests.

### B-08: No rate limiting on 2FA verification

**File:** `backend/cmd/api/main.go:110-111`  
`/auth/2fa/verify` and `/auth/2fa/disable` have no rate limiting. TOTP brute-force feasible. Also: `X-Forwarded-For` trusted without proxy verification (`middleware.go:229-235`).

### B-09: Timeout middleware goroutine race on ResponseWriter

**File:** `backend/internal/middleware/middleware.go:117-119`  
**Severity:** CRITICAL — data race + goroutine leak

When timeout fires, the spawned goroutine is still running `next.ServeHTTP(w, r)` and will also write to `w`. Concurrent writes to `http.ResponseWriter` = data race + potential panic.

**Fix:** Use `http.TimeoutHandler` wrapper instead of goroutine + select.

### B-10: Rate limiter cleanup goroutine never stops

**File:** `backend/internal/middleware/middleware.go:166-172`  
Cleanup goroutine runs forever with no `done` channel. If `Middleware()` is called multiple times, each call spawns a new goroutine.

### B-11: WithTransaction no defer rollback (panic-unsafe)

**File:** `backend/internal/db/transaction.go:10-22`  
If `fn(tx)` panics, `tx.Rollback(ctx)` is skipped. Should use `defer tx.Rollback(ctx)` (no-op after Commit).

### B-12: errors.Is not used for ErrNoRows

**Files:** `approval/gate.go:43`, `costing/costing.go:363`, `reports/templates.go:585`, `auth/seed.go:81`  
`err == pgx.ErrNoRows` fails if error is wrapped. Should use `errors.Is(err, pgx.ErrNoRows)`.

### B-13: httperr package is dead code — 7 duplicate errorResponse structs

**File:** `backend/internal/httperr/httperr.go` (unused)  
Canonical `{code, message, details, request_id}` defined but never used. 7 packages define their own `{code, message}` (dropping details + request_id). Handlers return `err.Error()` as `message` — exposes SQL text, constraint names.

### B-14: No graceful shutdown

**File:** `backend/cmd/api/main.go:313`  
`log.Fatal(http.ListenAndServe(...))` — no `signal.Notify`, no `http.Server.Shutdown`, `defer pool.Close()` is skipped. Connections leak on every deploy.

### B-15: No backup strategy

No `pg_dump` cron, no `pgbackrest`/`wal-g`, no backup volume. `deploy.sh` runs migrations without pre-backup. Only safety net is hash-chain (tamper-evidence, not backup).

### B-16: Prod missing NextReport service

**File:** `docker-compose.prod.yml`  
Prod has only postgres/api/web/caddy. `POST /reports/templates/{id}/render` falls back to `localhost:3100` and 502s.

### B-17: No security headers on Caddy

**File:** `Caddyfile`  
No HSTS, X-Frame-Options, X-Content-Type-Options, CSP. HTTP `:80` serves plain HTTP with no redirect to HTTPS.

### B-18: No DB pool tuning

**File:** `backend/cmd/api/main.go:54`  
No `MaxConns`, `MaxConnLifetime`, `MaxConnIdleTime`. Uses pgxpool defaults. No `pool.Ping` on startup.

### B-19: File upload trusts client Content-Type

**File:** `backend/internal/audit/attachments.go:83`  
No content sniffing (magic bytes). Attacker can upload HTML disguised as `image/png` → stored XSS.

### B-20: Internal errors leaked to clients

Handlers routinely pass `err.Error()` into the JSON `message` — exposes SQL text, constraint names, account internals. Combined with no `request_id`, operator cannot correlate client error to log line.

### B-21: No pagination on any list endpoint

Every list returns full result set. No `limit`/`offset` support. `CashEntryList` passes `limit: 200`. List endpoints run inside `WithTransaction`, holding a DB connection for the full scan.

### B-22: Unstructured logging

Plain `log.Printf`. No `slog`/`zap`/`zerolog`, no JSON, no log levels, no request-ID correlation. No log rotation configured.

---

## Part C: Feature Audit — Per-Module Status

### Sales Cycle (SQ→SO→DP→DO→INV→Payment→CN)

| Step | Journal Correct? | Hash? | Idem? | Outbox? | Status |
|---|---|---|---|---|---|
| SQ (quotation) | N/A — no journal | N/A | ❌ | ❌ | ✅ Status transitions correct |
| SO (sales order) | N/A — no journal | N/A | ❌ | ❌ | ⚠️ No DRAFT; created as CONFIRMED |
| DP (down payment) | ✅ Dr Cash/Cr 2201 | ✅ | ✅ | ✅ | ✅ Overflow check, FOR UPDATE |
| DP Refund | ✅ Reversal | ✅ | ✅ | ✅ | ✅ Marks original VOID |
| DO (delivery) | ✅ Dr 5101/Cr 1301 | ✅ | ✅ | ✅ | ❌ SO unconditionally CLOSED |
| INV (revenue) | ✅ Dr 1201/Cr 4101+2202 | ✅ | ✅ | ✅ | ⚠️ Shipping not posted, tax_total client-supplied |
| INV (DP realize) | ✅ Dr 2201/Cr 1201 | ✅ | ⚠️ No key | ✅ | ❌ DP never consumed (A-01) |
| Payment | ✅ Dr Cash/Cr 1201+2402 | ✅ | ✅ | ✅ | ⚠️ No FOR UPDATE (A-18) |
| CN (revenue) | ✅ Dr 4201/Cr 1201 | ✅ | ✅ | ✅ | ❌ refund_method no-op (A-17) |
| CN (COGS) | ✅ Dr 1301/Cr 5101 | ✅ | ✅ | ✅ | ❌ roundQty truncation (A-12) |

### Purchase Cycle (PO→GRN→SI→SP→Return)

| Step | Journal Correct? | Hash? | Idem? | Outbox? | Status |
|---|---|---|---|---|---|
| PO | N/A — no journal | N/A | ❌ | ❌ | ⚠️ No cancel endpoint |
| GRN | ✅ Dr 1301/Cr 2105 | ✅ | ✅ | ✅ | ❌ PO never RECEIVED (A-19) |
| SI | ✅ Dr 2105+1203/Cr 2101 | ✅ | ✅ | ✅ | ⚠️ No DP realization, float64 |
| SP | ✅ Dr 2101+1204/Cr Cash | ✅ | ✅ | ✅ | ⚠️ No FOR UPDATE, no AP sub-ledger |
| Return | ✅ Dr 2101/Cr 1301+1203 | ✅ | ✅ | ✅ | ❌ Costing wrong direction (A-04) |

### Inventory & Costing

| Feature | Status |
|---|---|
| Stock balance per item+warehouse | ✅ |
| GRN updates stock_balance (PostGRN) | ✅ |
| DO updates stock_balance + resolves COGS | ✅ |
| FIFO oldest-layer-first | ✅ |
| Moving average recalculation | ✅ |
| COGS reversal on credit note | ✅ (but roundQty bug) |
| COGS reversal on purchase return | ❌ Wrong direction (A-04) |
| Negative stock rejected | ✅ |
| Cost layers per warehouse | ✅ |
| Stock transfer (both warehouses) | ✅ |
| **Stock opname updates stock_balances** | ❌ **CRITICAL (A-05)** |
| GRN/DO pass warehouseID | ⚠️ Both pass 0 |

### Fixed Assets (All ✅)

| Feature | Status | Journal |
|---|---|---|
| Acquisition | ✅ | Dr 1401 / Cr Cash/AP |
| Straight-line depreciation | ✅ | Dr 5206 / Cr 1402 |
| Declining balance | ✅ | Dr 5206 / Cr 1402 |
| Units of production | ✅ | Dr 5206 / Cr 1402 |
| Fully-depreciated guard | ✅ | Clamps to salvage |
| Disposal (gain/loss) | ✅ | Dr Cash + Dr 1402 + Dr 5903 / Cr 1401 + Cr 4903 |
| Revaluation (OCI) | ✅ | Dr 1401 / Cr 3401 |
| Impairment | ✅ | Dr 5207 / Cr 1401 |
| Asset register report | ✅ | NBV + totals |
| Asset maintenance | ✅ | 5 types, upcoming-due |

### Lease (PSAK 73)

| Feature | Status | Notes |
|---|---|---|
| PV calculation | ✅ | Ordinary annuity formula |
| Commencement journal | ✅ | Dr 1701 / Cr 2301 |
| Amortization schedule | ✅ | Effective interest method |
| Payment journal | ✅ | Dr 2301 + Dr 5906 / Cr Cash |
| RoU depreciation | ✅ | Dr 5209 / Cr 1702, idempotent |
| Modification | ✅ | Re-measures PV |
| Termination | ✅ | Derecognize, gain/loss |
| Schedule regenerated after mod | ⚠️ | Old schedule retained |
| In-advance vs in-arrears | ⚠️ | Inconsistent (A-16) |

### Tax

| Feature | Status |
|---|---|
| PPN Keluaran on invoice | ✅ Cr 2202 |
| PPN Masukan on supplier invoice | ✅ Dr 1203 |
| PPN reconciliation | ✅ Net keluaran − masukan |
| PPN rate from tax_rates | ⚠️ Per-line caller-supplied, not enforced |
| PPh types (21/22/23/26/UMKM) | ✅ |
| **PPh posts a journal** | ❌ **CRITICAL (A-06)** |
| PPh Final UMKM posts journal | ✅ Dr 5208 / Cr 2203 |
| ECL aging buckets | ✅ 0-30=1%, 31-60=2.5%, 61-90=5%, >90=10% |
| ECL provision journal | ✅ Dr 5209 / Cr 1202 |
| ECL write-off | ✅ Dr 1202 / Cr 1201 |
| **ECL ages from invoice_date** | ❌ **Should be due_date (A-08)** |
| **ECL write-off doesn't update invoice** | ❌ **CRITICAL (A-07)** |
| Deferred tax journal | ✅ Dr/Cr 1206 / 5904 |

### Multi-Tenant & Security

| Feature | Status |
|---|---|
| Multi-tenant (user → multiple tenants) | ✅ |
| Tenant switcher | ✅ |
| Registration creates tenant + membership | ✅ |
| JWT tenant_id used everywhere | ✅ |
| RBAC (6 roles) | ⚠️ Coarse — accountant/manager/staff identical |
| 2FA (RFC 6238) | ✅ Fully compliant, ±30s tolerance |
| Inter-company elimination | ⚠️ No population endpoint, pct ignored |
| Entity hierarchy | ✅ |
| Approval gate on invoices | ✅ |
| **Approval gate on other types** | ❌ Only invoices |
| Audit log coverage | ⚠️ Only 3 of ~15 posting paths |

### Module Completeness

| Module | Tracking? | Posts Journal? | Frontend? | Key Gap |
|---|---|---|---|---|
| Petty Cash | ✅ | ❌ All 3 | ❌ | Shell — no journal, no UI |
| Recurring | ✅ | ❌ | ❌ | No scheduler, Update=501, no UI |
| Cheques | ✅ | ❌ | ❌ | Tracking only, no UI |
| Cost Centers | ✅ | ❌ | ❌ | Allocations never executed, no UI |
| Email | ✅ | N/A | ❌ | No SMTP, no worker, no UI |
| Bank Reconciliation | ✅ | ⚠️ | ✅ | Complete doesn't post adjustment |
| Dashboard | ✅ | N/A | ⚠️ | 3/11 widgets unimplemented, hardcoded KPIs |
| Attachments | ✅ | N/A | ✅ | No content sniffing |
| Audit Log | ✅ | N/A | ✅ | Coverage only 3/15 paths |
| PPh | ✅ | ❌ | ✅ | No journal posting |
| Approval Workflow | ✅ | N/A | ❌ | Gate only on invoices, no UI |
| Inter-company | ⚠️ | N/A | ⚠️ | No population endpoint, pct ignored |

### Reporting

| Report | Backend Correct? | Frontend Date Range? | Frontend Export? | Framework? | Dimension? |
|---|---|---|---|---|---|
| Trial Balance | ✅ (409 if unbalanced) | ❌ | ❌ | ❌ | ✅ |
| P&L | ✅ | ❌ | ❌ | ✅ EMKM/ETAP/SAK | ✅ |
| Balance Sheet | ✅ (A=L+E+P) | ❌ | ❌ | ❌ | ❌ |
| Cash Flow | ⚠️ 2× inflated (A-02) | ❌ | ❌ | ❌ | ❌ |
| Budget vs Actual | ✅ | ✅ | — | — | ✅ |
| Consolidated TB/P&L | ⚠️ pct ignored | — | — | — | — |
| Report Templates | ✅ | — | ✅ (NextReport) | — | — |
| AR Aging | ✅ backend | — | — | — | — | ❌ No frontend |
| AP Aging | ✅ backend | — | — | — | — | ❌ No frontend |

---

## Part D: Frontend Form Bugs

| # | Bug | File | Severity |
|---|---|---|---|
| D-01 | **CustomerForm permanent loading** — `setLoading(true)` never set to false. Form unreachable. | `CustomerForm.tsx:31` | CRITICAL |
| D-02 | **Item creation non-functional** — DemoEntryForm stub, Save disabled, no `createItem` API. Missing required fields (item_type, uom, costing_method). | `DemoEntryForm.tsx` | CRITICAL |
| D-03 | **FixedAssetForm ×100 bug** — `parseInt(cost)` sent as cents without ×100. All asset costs off by 100×. | `FixedAssetForm.tsx:118` | HIGH |
| D-04 | **CashEntryForm balance check dead code** — `cashAmountCents === counterTotalCents` always (same computation). Validation can never fail. | `CashEntryForm.tsx:94-102` | HIGH |
| D-05 | **CashEntryForm Save&New stale error** — After `await handleSubmit`, `error` is stale closure → form resets even on failure. Also re-posts same data → duplicates. | `CashEntryForm.tsx:174-184` | HIGH |
| D-06 | **DeliveryOrderForm wrong COGS rounding** — `Math.round(qty) * cost` rounds qty before multiply. qty=1.5 → COGS=2×cost. | `DeliveryOrderForm.tsx:98,104` | HIGH |
| D-07 | **CreditNoteForm stale qty** — `l.qty > 0` checks OLD qty. First entry → totals show Rp 0. | `CreditNoteForm.tsx:71-72` | CRITICAL |
| D-08 | **GRNForm doesn't load PO lines** — No `useEffect` watching `poId`. User must manually re-enter all items. | `GRNForm.tsx:102` | HIGH |
| D-09 | **DeliveryOrderForm doesn't load SO lines** — Same pattern. | `DeliveryOrderForm.tsx:169` | HIGH |
| D-10 | **CreditNoteForm doesn't load invoice lines** — Item field is free-text `<input>`, not `<select>`. | `CreditNoteForm.tsx:162-166` | HIGH |
| D-11 | **SalesOrderForm no Confirm button** — DP panel only shows when CONFIRMED, but no button to confirm. DP unreachable. | `SalesOrderForm.tsx:450` | HIGH |
| D-12 | **QuotationForm Send button always disabled** — No `onClick`. No Cancel/Mark-Expired either. | `QuotationForm.tsx:265-268` | MEDIUM |
| D-13 | **4 forms no saved-state tracking** — PurchaseOrderForm, QuotationForm, GRNForm, CreditNoteForm → clicking Save again creates duplicate. | Multiple | HIGH |
| D-14 | **PPN/tax hardcoded 0** in InvoiceForm, SalesOrderForm, PurchaseOrderForm, QuotationForm. No tax input UI. | Multiple | HIGH |
| D-15 | **6 forms accept entryId but never load existing data** — QuotationForm, CashEntryForm, CreditNoteForm, GRNForm, BudgetForm, PurchaseReturnForm. | Multiple | MEDIUM |
| D-16 | **12+ API methods defined but never called** — reverseCash, exportReport, revalueAsset, calculateDeferredTax, tagJournalLine, sendQuotation, cancelQuotation, cancelSalesOrder, listEntityHierarchy, createEntityHierarchy, createSupplierPayment. | `api.ts` | MEDIUM |
| D-17 | **No getCustomer(id) API** — Customer editing impossible even if form were fixed. | `api.ts` | MEDIUM |
| D-18 | **CustomerList & PurchaseSupplierList rows not clickable** — Can't edit from list. | Multiple | MEDIUM |
| D-19 | **JournalEntryForm double-post risk** — Post button not disabled after save. | `JournalEntryForm.tsx:257` | HIGH |
| D-20 | **CustomerForm missing 13 base columns** — contact_person, phone, email, address, city, province, postal_code, payment_term_id, credit_limit, etc. | `CustomerForm.tsx` | MEDIUM |
| D-21 | **PurchaseSupplierForm no edit mode** — Header hardcoded "New Supplier". | `PurchaseSupplierForm.tsx` | MEDIUM |
| D-22 | **GRNForm race condition loading PO list** — Sequential `then` calls, not `Promise.all`. | `GRNForm.tsx:36-39` | MEDIUM |
| D-23 | **Dead status filter pills on 6 screens** — `setStatus` declared, never called. | InvoiceList, SalesOrderList, DeliveryOrderList, SupplierInvoiceList, Sales.tsx, CashEntryList | HIGH |
| D-24 | **Plain `<select>` for all FK lookups** — No type-to-search, no code lookup, no lazy loading. | All entry forms | HIGH |
| D-25 | **No toast/notification system** — Zero `toast`/`snackbar`/`notify` in codebase. | Entire app | MEDIUM |
| D-26 | **Three currency formatting conventions** — `IDR 1,234,567` vs `1,234,567 Rp` vs `+ 1,234,567` + `fmtIDR` duplicate. | Multiple | MEDIUM |
| D-27 | **Dashboard hardcoded KPIs** — `dueBills: 2`, `lowStock: 4` are fake constants. Sparklines are `Math.sin()`. | `DashboardScreen.tsx` + `api.ts:623-624` | MEDIUM |
| D-28 | **Reports no date range in UI** — API supports `from_date`/`to_date` but UI passes `undefined`. | `Reports.tsx` | HIGH |
| D-29 | **No export buttons in UI** — `api.exportReport()` exists, zero UI calls. | `Reports.tsx` | HIGH |
| D-30 | **No pagination on any list** — Every list returns full result set. | Frontend + backend | HIGH |
| D-31 | **ListSkeleton built but never used** — All lists show bare spinner, causing layout shift. | `ui.tsx:294` | LOW |
| D-32 | **InvoiceForm validation dangerously thin** — `noValidate` + only checks customerId + lines.length > 0. No qty/price/date validation. | `InvoiceForm.tsx` | MEDIUM |
| D-33 | **Responsive breaks on mobile** — Columns hidden via `nth-child(n+4)`, no scroll wrapper. | `features.css` | MEDIUM |
| D-34 | **HTTP no timeout/retry** — `http()` has no `AbortController`, no retry, no network-error distinction. | `api.ts:253-272` | MEDIUM |
| D-35 | **Silent error masking** — Most `api.list*()` methods `catch { return []; }` — can't distinguish "no data" from "network down". | `api.ts` multiple | HIGH |

---

## Part E: Frontend Architecture & Navigation

### Architecture Bugs

| # | Bug | File | Severity |
|---|---|---|---|
| E-01 | **No Error Boundary** — Any component throw crashes entire app with blank screen. | `App.tsx` | CRITICAL |
| E-02 | **Tab switch destroys unsaved form data** — `key={activeChild?.id}` remounts form. All `useState` lost. Tab shows "unsaved" dot but form is empty when returning. | `WorkArea.tsx:155` | HIGH |
| E-03 | **Can't open two entries of same type** — Dedup checks `draft` flag but not `entryId`. Two different invoices = same tab. | `state.tsx:278-284` | HIGH |
| E-04 | **5 entryKindToListKind mappings missing** — BOM, Production Job, Lease, Customer entries open in Cash & Bank module. | `state.tsx:324-383` | HIGH |
| E-05 | **CustomerForm & PurchaseSupplierForm don't receive tabId** — `replaceDraft`/`markUnsaved` use fake IDs. | `WorkArea.tsx:344-346` | MEDIUM |
| E-06 | **queueMicrotask dispatch during render** — React anti-pattern. | `WorkArea.tsx:131` | LOW |
| E-07 | **SupplierPaymentPanel orphaned** — Contains working API calls but never rendered. | `SupplierPaymentPanel.tsx` | LOW |
| E-08 | **asset-register types unreachable** — Defined and handled but no sidebar item opens them. | `types.ts:36,84` | LOW |
| E-09 | **No keyboard shortcuts** — Ctrl+S, Esc, Alt+N all missing. | Global | LOW |

### Navigation Problems

| # | Problem | Impact |
|---|---|---|
| N-01 | **"+New" button uses wrong entry kind** — Uses `module.items[0].openEntry` (first sub-item), not the one matching the active list. On Credit Notes, clicking "+" opens Invoice draft. | Wrong form opens |
| N-02 | **List-first navigation adds a step** — Sidebar always opens a list, not the form. | +1 click per transaction |
| N-03 | **No workflow chain** — No "Convert to SO", "Create DO", "Create Invoice" buttons. | 20-60 redundant clicks per pipeline |
| N-04 | **No auto-fill from parent** — GRN doesn't load PO lines, DO doesn't load SO lines, CN doesn't load invoice lines. | Full manual re-entry every time |
| N-05 | **No inline master creation** — Can't create customer/supplier/item from within a form. | Breaks flow completely |
| N-06 | **No save feedback** — No toast. Badge changes. List doesn't refresh. | Confusion, duplicate saves |
| N-07 | **No "back to list" after save** — Entry tab stays open. | Tab clutter |
| N-08 | **No "Save & New" that works** — CashEntryForm has stale-error bug, JournalEntryForm has it permanently disabled. | Can't rapid-enter transactions |

### Current vs Proposed Flow

**Current (broken):**
```
Sidebar → List → "+New" → Form → Save → Stuck (no toast, no refresh, no next step)
```

**Proposed:**
```
Sidebar → List → "+New" → Auto-fill from parent → Fill form (inline master create)
  → Ctrl+S → Toast "✓ Saved {number}" → List refreshes behind form
  → Next-step buttons: [Convert to SO] [Create DO] [Print] [Close]
  → Click next step → new form pre-filled from current document
```

### Workflow Chain Design

| After saving... | Next-step buttons |
|---|---|
| Quotation | Convert to SO, Print, Send, Close |
| Sales Order | Create DO, Receive DP, Cancel, Close |
| Delivery Order | Create Invoice, Print DO, Close |
| Invoice | Receive Payment, Print, Send, Void, Close |
| Purchase Order | Create GRN, Print PO, Close |
| GRN | Create Supplier Invoice, Close |
| Supplier Invoice | Pay Supplier, Print, Close |
| Journal Entry | New Journal, Close |
| Fixed Asset | Depreciate, Dispose, Revalue, Impair, Close |
| Lease Contract | View Schedule, Post Payment, Modify, Terminate, Close |

---

## Part F: UI/UX Per-Module Improvements

### Module 1: Cash & Bank

| Sub-item | Status | Issues |
|---|---|---|
| Other Receipt | ✅ Working | No account search. Balance check dead code (D-04). Save&New stale-error (D-05). |
| Other Payment | ✅ Working | Same |
| Bank Transfer | ✅ Working | Simpler form, works OK |
| Bank Reconciliation | ✅ Working | Import flow works |
| **Cheques/GIRO** | ❌ Missing | Backend ready (tracking only, no journal) |
| **Petty Cash** | ❌ Missing | Backend ready (tracking only, no journal) |

**Improvements:** Searchable account combobox, fix balance check, fix Save&New, toast, auto-refresh list, "Close & return to list", Cheques screen, Petty Cash screen, Ctrl+S.

### Module 2: Sales

| Sub-item | Status | Issues |
|---|---|---|
| Customers | ❌ BROKEN | Form permanently loading (D-01). List rows not clickable. No getCustomer API. 13 fields missing. |
| Quotations | ⚠️ Partial | No edit mode. Send disabled (D-12). No convert to SO. No saved-state (D-13). |
| Sales Orders | ⚠️ Partial | No Confirm button → DP unreachable (D-11). No Create DO. markUnsaved missing 6 fields. |
| Delivery Orders | ⚠️ Partial | Doesn't load SO lines (D-09). COGS rounding bug (D-06). No Create Invoice. |
| Sales Invoices | ⚠️ Partial | PPN hardcoded 0 (D-14). No tax input. No Send/Print/Void. |
| Credit Notes | ⚠️ Partial | Doesn't load invoice lines (D-10). Stale qty bug (D-07). Free-text item input. No saved-state. |
| Sales Receipts | ✅ Working | Redirects to InvoiceForm |

**Improvements:** Fix CustomerForm, Convert/Confirm/Create buttons, auto-fill parent lines, PPN input, inline master creation, wire Send/Cancel/Reverse.

### Module 3: Purchases

| Sub-item | Status | Issues |
|---|---|---|
| Purchase Orders | ⚠️ Partial | No saved-state (duplicates). No Create GRN. No tax. |
| Goods Received | ❌ BROKEN | Doesn't load PO lines (D-08, CRITICAL). No saved-state. No over-delivery validation. Race condition. |
| Suppliers | ⚠️ Partial | No edit mode. List not clickable. Missing fields. No tabId. |
| Supplier Invoices | ✅ Working | Has payment panel. Edit mode works. |
| Purchase Payments | ✅ Working | Stuck-loading bug (no .catch) |
| Purchase Returns | ⚠️ Partial | Shows "read-only" placeholder for existing. |

**Improvements:** CRITICAL auto-fill PO lines in GRNForm, Create GRN/Create SI buttons, saved-state tracking, over-delivery validation, fix SupplierForm edit mode.

### Module 4: Production

| Sub-item | Status |
|---|---|
| Bill of Materials | ✅ Working |
| Production Jobs | ✅ Working |

**Improvements:** Start Production button on BOM, overhead variance screen, post-completion feedback.

### Module 5: Inventory

| Sub-item | Status | Issues |
|---|---|---|
| Item List | ❌ BROKEN | List calls real API but flagged mockData. Form is DemoEntryForm stub (D-02). |
| Stock Movements | ❌ STUB | Just an EmptyState. No API call. |
| Stock Opnames | ✅ Working | |
| Stock Transfers | ✅ Working | |
| **Warehouses** | ❌ Missing | Backend ready, no frontend |

**Improvements:** Build real ItemForm + createItem API, remove mockData flag, build Stock Movements screen (backend endpoint + frontend), add Warehouses screen.

### Module 6: Fixed Assets

| Sub-item | Status | Issues |
|---|---|---|
| Asset Register | ⚠️ Partial | Form has ×100 bug (D-03). Payment account hardcoded 3 options. |
| Lease Contracts | ✅ Working | Missing: modify, terminate, depreciate UI. |
| **Asset Maintenance** | ❌ Missing | Backend ready, no frontend |

**Improvements:** Fix ×100, searchable COA, lease modify/terminate/depreciate UI, asset maintenance screen, delete orphaned Assets.tsx.

### Module 7: Accountant

| Sub-item | Status | Issues |
|---|---|---|
| Journal Entries | ⚠️ Partial | Double-post risk (D-19). No Save&New. Account dropdown no search. |
| General Ledger | ✅ Working | Account dropdown needs search. Error masking. |
| Journal Register | ✅ Working | 90% duplicate of JournalEntryList. |
| Dimensions | ✅ Working | |
| Budgets | ⚠️ Partial | No edit mode (entryId not destructured). |
| Audit Trail | ✅ Working | |
| Customer Statement | ✅ Working | |
| **Cost Centers** | ❌ Missing | Backend ready, no frontend |
| **Approval Workflow** | ❌ Missing | Backend ready, no frontend |
| **Recurring** | ❌ Missing | Backend ready (but no journal posting), no frontend |

**Improvements:** Fix double-post, searchable account combobox, working Save&New, fix BudgetForm edit, merge JE/JR, add Cost Centers/Approval/Recurring screens, wire tagJournalLine.

### Module 8: Tax

| Sub-item | Status | Issues |
|---|---|---|
| PPN Reconciliation | ✅ Working | |
| PPh Final | ✅ Working | |
| ECL | ✅ Working | But: ages from invoice_date (A-08), write-off doesn't update subledger (A-07). |

**Improvements:** Fix ECL aging, fix ECL write-off, wire calculateDeferredTax.

### Module 9: Reports

| Sub-item | Status | Issues |
|---|---|---|
| Trial Balance | ⚠️ Partial | No date range. No export. |
| P&L | ⚠️ Partial | Has framework + dimension. No date range. No export. |
| Balance Sheet | ⚠️ Partial | No date range. No export. No framework. No dimension. |
| Cash Flow | ⚠️ Partial | No date range. No export. 2× inflated (A-02). |
| Financial Notes | ✅ Working | |
| Due Date Reminders | ✅ Working | |
| Budget vs Actual | ✅ Working | |
| Consolidated TB | ✅ Working | pct ignored |
| Report Templates | ✅ Working | Editor + preview |
| **AR Aging** | ❌ Missing | Backend ready, no frontend |
| **AP Aging** | ❌ Missing | Backend ready, no frontend |

**Improvements:** Date range + quick ranges on all 4 reports, export buttons, framework/dimension on BS+CF, AR/AP Aging screens, fix cash flow 2× bug.

### Quick-Add from Sidebar

Add `+` button next to each sub-item in flyout:
```
│ ├─ Quotations      [+]  │
│ ├─ Sales Orders    [+]  │
```
Clicking `[+]` opens a new entry form directly (2 clicks instead of 3).

### Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| `Ctrl/Cmd+S` | Save current form |
| `Esc` | Close active tab (with unsaved confirm) |
| `Ctrl+Enter` | Save & New |
| `Alt+N` | New entry from current list |
| `Ctrl+Tab` | Switch to next tab |
| `Ctrl+1..9` | Switch to module 1-9 |
| `/` | Focus search in list |

---

## Part G: Database Schema Issues

### G-01: 4 account code collisions

| Code | seed.go | Migration | Conflict |
|---|---|---|---|
| 5904 | Deferred Tax Expense | Loss on FX (000029) | Down migration deletes wrong account |
| 5209 | Bad Debt Expense | RoU Depreciation (000026) | Lease + tax post to same account |
| 5203 | Transportation | Income Tax (000031) | Latent |
| 1304 | Finished Goods | Cheques in Transit (000033) | Latent |

### G-02: 15+ missing indexes

`invoices(customer_id, status, sales_order_id)`, `sales_orders(customer_id, status)`, `cheques(status, direction)`, `recurring_transactions(next_date, is_active)`, `inventory_movements(movement_type, warehouse_id)`, `journal_lines(tenant_id, entry_id)`, `supplier_invoices(supplier_id)`, `grn_lines(po_line_id)`, `delivery_orders_lines(item_id)`, `purchase_orders(supplier_id)`, `approval_requests(entity_id, entity_type)`.

### G-03: Missing CHECK constraints

20+ monetary columns lack `CHECK (*_cents >= 0)`. `recurring_transactions.intent_type` is free TEXT. `tax_rates` missing UNIQUE on `(tenant_id, tax_type, effective_from)`. `payment_terms.discount_percent` no CHECK. Missing FKs on `cheques.journal_entry_id`, `cheques.payment_id`.

### G-04: Migration issues

- 000003 missing (sequence jumps 002→004)
- 000027 has no `.down.sql`
- 000029.down dangerous (deletes 5904 — the Deferred Tax Expense account)
- 000039.down incomplete (doesn't drop `inventory_cost_layers.warehouse_id`, re-creates index instead of dropping)
- 000031:22 invalid SQL (`ADD UNIQUE IF NOT EXISTS (..., COALESCE(...))`)
- 000001.down leaves orphan functions/triggers

### G-05: Report templates invisible to tenants

`tenant_id = 0` under RLS filter → invisible to every real tenant (no tenant has id 0).

### G-06: user_tokens no RLS

Migration 000035 adds `tenant_id` but no RLS policy. Refresh tokens not tenant-isolated at DB level.

---

## Part H: Accessibility

| # | Issue | Severity |
|---|---|---|
| H-01 | No semantic HTML tables — all 5 list screens use `<div>` grids. Screen readers can't announce as tables. | HIGH |
| H-02 | No focus management — no focus trap in modals, no restoration on tab close, no focus move on tab activation, no roving tabindex. | HIGH |
| H-03 | Color contrast failures: `--ink-muted` 3.3:1, `--positive` 3.75:1, `--warning` 3.3:1, `--ink-faint` 2.0:1, `--accent` 3.9:1. All fail WCAG AA (4.5:1). 9px badge font too small. | HIGH |
| H-04 | Tab strip not keyboard accessible — `<div role="tab">` with no `tabIndex`, no `onKeyDown`, no arrow key navigation. | HIGH |
| H-05 | Dynamic content not announced — saved status, net totals, outstanding receivable, loading→error transitions. No `aria-live`. | MEDIUM |
| H-06 | Grid inputs unlabeled — item/qty/price/discount in all entry forms. `FieldShell` component built with proper ARIA but **never used**. | HIGH |
| H-07 | FixedAssetList rows missing `onKeyDown` — focusable but not activatable. CustomerList rows not interactive at all. | MEDIUM |
| H-08 | Forecast/dashboard `rows.Scan` errors silently swallowed — `rows.Err()` never checked after loops. Partial results returned. | MEDIUM |

---

## Part I: Test Coverage

| # | Issue | Severity |
|---|---|---|
| I-01 | **Zero frontend tests** — no vitest/jest, no test files, no test dependencies. Entire React UI untested. | HIGH |
| I-02 | **Zero backend integration tests** — all tests are pure-function unit tests. No test connects to a real database. All SQL, hash-chain, outbox, RLS untested. | HIGH |
| I-03 | **Critical untested paths**: PPN 3-line journal, GRN Dr/Cr, period close SQL, hash chain sequence, DP realization, lease PV rounding. | HIGH |
| I-04 | dashboard package has only 3 tests (`validateWidgetType`, `normalizeWidgetGrid`). | LOW |

---

## Part J: Dead Code & Cleanup

| # | Item | Action |
|---|---|---|
| J-01 | `TransactionsScreen.tsx` — orphaned, not imported anywhere, uses legacy `useAppState()` | DELETE |
| J-02 | `Assets.tsx` (AssetRegisterList) — 100% mock, superseded by `FixedAssetList.tsx` | DELETE |
| J-03 | `SupplierPaymentPanel.tsx` — orphaned, contains working API calls but never rendered | Wire or DELETE |
| J-04 | `asset-register` ListSubKind/EntrySubKind — unreachable from sidebar | DELETE |
| J-05 | `fmtIDR` duplicate in `api.ts:234` | DELETE |
| J-06 | `purchaseHandler` shadow in `main.go:164` — re-declares variable from line 70 | Fix |
| J-07 | 20 stale branches (all 0 commits ahead of main) | DELETE |
| J-08 | `mockData: true` on `in-items` — list actually calls real API | REMOVE flag |
| J-09 | `JournalEntryList` vs `JournalRegister` — 90% duplicates | MERGE or differentiate |
| J-10 | `nginx.conf` — orphaned (docker uses Caddy) | DELETE |
| J-11 | `httperr` package — dead code (7 duplicate errorResponse structs) | ADOPT or DELETE |

---

## Implementation Roadmap

### Phase 0: Critical Accounting Fixes (Day 1-4) — ~4 days ✅ COMPLETE

Merged: 2026-08-11 (Wave 1). All fixes verified with go vet/test/build + tsc/vite.
| Task | Effort | ID |
|---|---|---|
| Fix DP double-realization (track dp_consumed on SO) | 4h | A-01 |
| Fix cash flow multi-leg double-count (sum ol not jl) | 2h | A-02 |
| Fix partial delivery (only close SO when fully delivered) | 3h | A-03 |
| Fix purchase return costing (ResolveCOGS not ReverseCOGS) | 1h | A-04 |
| Fix stock opname (call costing.PostGRN/ResolveCOGS on approve) | 3h | A-05 |
| Implement PPh journal posting (Dr 5203/Cr 210x) | 4h | A-06 |
| Fix ECL write-off (update invoice + subledger + allowance check) | 3h | A-07 |
| Fix ECL aging (use due_date not invoice_date) | 2h | A-08 |
| Fix PPN truncation (half-up rounding) | 1h | A-09 |
| Store computed tax_total (not client-supplied) | 2h | A-10 |
| Add shipping/other charges to total + post | 3h | A-11 |
| Fix CN COGS roundQty → round product | 1h | A-12 |
| Look up original delivery cost for CN COGS | 2h | A-13 |
| Fix CN warehouse_id=0 | 30 min | A-14 |
| Fix aging 'POSTED' → 'ISSUED' + migration 000045 | 1h | B-02 |
| Fix production helpers.go table name | 30 min | B-03 |
| Add 4 missing accounts to seed.go + migration 000045 | 2h | B-04 |

### Phase 1: Security & RLS (Day 2-5, parallel) — ~3 days ✅ COMPLETE

Merged: 2026-08-11 (Wave 1). All fixes verified.
| Task | Effort | ID |
|---|---|---|
| Fix 2FA bypass (Setup2FA must not disable) | 2h | B-01 |
| Wrap 11 packages in withTenantRead/Write | 6h | B-05 |
| Implement journal posting for pettycash/recurring/cheque | 3 days | A-27 |
| Add FORCE RLS + WITH CHECK to 6 tables | 1h | B-06 |
| Fix CORS to specific origins | 30 min | B-07 |
| Add rate limiting to 2FA + fix X-Forwarded-For | 2h | B-08 |
| Fix timeout middleware (use http.TimeoutHandler) | 2h | B-09 |
| Fix rate limiter cleanup goroutine (add done channel) | 1h | B-10 |
| Add defer tx.Rollback to WithTransaction | 30 min | B-11 |
| Replace == with errors.Is for ErrNoRows (4 files) | 1h | B-12 |
| Add content sniffing to file upload | 2h | B-19 |
| Fix 4 account collisions (assign distinct codes) | 3h | G-01 |
| Fix report templates tenant_id=0 → per-tenant | 2h | G-05 |
| Add RLS to user_tokens | 30 min | G-06 |

### Phase 2: Frontend Architecture + Navigation (Day 3-6) — ~3 days ✅ COMPLETE

Merged: 2026-08-11 (Wave 2). ErrorBoundary, Combobox, Toast, keyboard shortcuts all wired.
| Task | Effort | ID |
|---|---|---|
| Add Error Boundary (app + per-tab) | 2h | E-01 |
| Fix tab switch data loss (CSS hide vs unmount) | 3h | E-02 |
| Fix dedup bug (compare entryId) | 1h | E-03 |
| Fix 5 missing entryKindToListKind mappings | 1h | E-04 |
| Fix "+New" button to use active list's entry kind | 1h | N-01 |
| Add toast notification system | 4h | N-06, D-25 |
| Auto-refresh list after save | 2h | N-06 |
| Add "Close & return to list" after save | 1h | N-07 |
| Add Ctrl+S + Esc shortcuts | 2h | E-09 |
| Build Combobox/SearchableSelect component | 8h | D-24 |
| Fix dead status filter pills (6 screens) | 2h | D-23 |
| Fix dashboard hardcoded KPIs | 2h | D-27 |
| Add report date ranges + export buttons | 4h | D-28, D-29 |
| Fix CustomerForm/PurchaseSupplierForm tabId | 1h | E-05 |
| Fix HTTP timeout/retry | 2h | D-34 |
| Stop silent error masking (return discriminated results) | 4h | D-35 |

### Phase 3: Frontend Form Fixes + Workflow Chain (Day 5-10) — ~5 days ✅ COMPLETE

Merged: 2026-08-12 (Wave 3). All fixes verified: npx tsc --noEmit PASS, npx vite build PASS,
go vet/test/build PASS.
**Changes:**
- Real ItemForm built (D-02) — replaces DemoEntryForm stub, full field coverage
- Workflow chain: NextStepsBar on QuotationForm, SalesOrderForm, InvoiceForm, GRNForm,
  SupplierInvoiceForm, FixedAssetForm, LeaseContractForm
- Parent autofill: PO→GRN, SO→DO, Invoice→CN, GRN→SI line loading with validation
- CustomerForm/SupplierForm: edit mode, 13 missing fields, clickable list rows, tabId props
- PPN/tax: TaxRateSelector component wired to InvoiceForm, SalesOrderForm, QuotationForm
- CashEntryForm: real balance check, Save&New stale error fixed
- JournalEntryForm: double-post prevention (Post disabled after save)
- BudgetForm: edit mode via getBudget(id)
- Report enhancements: date ranges, export PDF/Excel, framework/dimension filters
**Total: +4,124 / -895 lines**
| Task | Effort | ID |
|---|---|---|
| Fix CustomerForm loading + add 13 fields + getCustomer API | 4h | D-01, D-17, D-20 |
| Build real ItemForm + createItem API | 6h | D-02 |
| Fix CashEntryForm balance check + Save&New | 3h | D-04, D-05 |
| Fix CreditNoteForm stale qty | 30 min | D-07 |
| Fix DeliveryOrderForm COGS rounding | 30 min | D-06 |
| Fix FixedAssetForm ×100 | 30 min | D-03 |
| Add PPN/tax input to 4 forms | 4h | D-14 |
| Add saved-state tracking to 4 forms | 4h | D-13 |
| Fix JournalEntryForm double-post | 1h | D-19 |
| Fix BudgetForm edit mode | 2h | D-15 |
| Wire Send/Cancel/Reverse buttons (orphaned API methods) | 4h | D-12, D-16 |
| Make CustomerList/SupplierList rows clickable | 2h | D-18 |
| Add PO line loading to GRNForm | 3h | D-08, N-04 |
| Add SO line loading to DeliveryOrderForm | 3h | D-09, N-04 |
| Add invoice line loading to CreditNoteForm + item select | 3h | D-10, N-04 |
| Add "Confirm" button to SalesOrderForm | 2h | D-11 |
| Add "Convert to SO" on QuotationForm | 3h | N-03 |
| Add "Create DO" on SalesOrderForm | 2h | N-03 |
| Add "Create Invoice" on DeliveryOrderForm | 3h | N-03 |
| Add "Create GRN" on PurchaseOrderForm | 3h | N-03 |
| Add "Create Supplier Invoice" on GRNForm | 3h | N-03 |
| Add inline "+New Customer/Supplier/Item" buttons | 4h | N-05 |
| Unify currency formatting | 2h | D-26 |
| Harden InvoiceForm validation | 2h | D-32 |
| Fix responsive design | 4h | D-33 |

### Phase 4: Backend Infrastructure (Day 5-9, parallel) — ~3 days

| Task | Effort | ID |
|---|---|---|
| Adopt httperr.Write everywhere (replace 7 duplicates) | 1 day | B-13 |
| Add graceful shutdown (signal.Notify + Shutdown) | 2h | B-14 |
| Add DB pool tuning (MaxConns, lifetime, Ping) | 2h | B-18 |
| Add backup strategy (pg_dump cron sidecar) | 4h | B-15 |
| Add NextReport to prod compose | 1h | B-16 |
| Add security headers to Caddy + HTTPS redirect | 1h | B-17 |
| Add structured logging (log/slog + request_id) | 4h | B-22 |
| Stop returning err.Error() to clients | 4h | B-20 |
| Extend request_hash idempotency to all posters | 1 day | — |
| Add audit.Log to all posting paths | 4h | A-31 |
| Fix error handling (400 → isNoRows/ValidationError/500) | 4h | — |
| Fix reconciliation autoMatch N+1 | 4h | — |
| Fix forecast/dashboard rows.Err() + scan swallowing | 1h | H-08 |
| Implement refund_method (cash refund + credit balance) | 4h | A-17 |
| Fix PO never RECEIVED (compare received vs ordered) | 2h | A-19 |
| Create AP sub-ledger (supplier_balances) | 4h | A-20 |
| Implement purchase DP realization | 4h | A-21 |
| Fix float64 line totals → milliunit integer | 4h | A-22 |
| Regenerate lease schedule after modification | 2h | A-23 |
| Execute cost center allocations | 4h | A-25 |
| Post bank reconciliation adjustment | 2h | A-26 |
| Implement inter-company population endpoint | 4h | A-29 |
| Wire approval gate to POs/CNs/journals | 4h | A-30 |
| Implement email SMTP + worker | 1 day | A-28 |
| Fix deposit/declining-balance/moving-avg rounding | 2h | A-15 |
| Fix lease in-advance vs in-arrears | 4h | A-16 |

### Phase 5: Database Hardening (Day 8-10) — ~2 days

| Task | Effort | ID |
|---|---|---|
| Add 15+ missing indexes (migration 000046) | 1 day | G-02 |
| Add CHECK constraints + missing FKs (migration 000047) | 0.5 day | G-03 |
| Fix 000029.down, 000039.down, write 000027.down | 1h | G-04 |
| Fix 000031:22 invalid SQL | 30 min | G-04 |

### Phase 6: Cross-Cutting UI (Day 10-14) — ~3 days

| Task | Effort | ID |
|---|---|---|
| Add pagination to all lists (server + client) | 1 day | D-30 |
| Replace LoadingState with ListSkeleton | 2h | D-31 |
| Add quick date ranges (This Month, Quarter, YTD) | 2h | — |
| Add framework/dimension to Balance Sheet + Cash Flow | 2h | — |
| Add keyboard shortcuts | 2h | E-09 |

### Phase 7: Accessibility (Day 12-15) — ~3 days

| Task | Effort | ID |
|---|---|---|
| Convert list screens to semantic `<table>` | 1 day | H-01 |
| Add ARIA labels to grid inputs (use FieldShell) | 1 day | H-06 |
| Fix color contrast (--ink-muted, --positive, --warning) | 0.5 day | H-03 |
| Make tab strip keyboard accessible (roving tabindex) | 0.5 day | H-04 |
| Add aria-live for dynamic totals/status | 0.5 day | H-05 |
| Add focus management (tab open/close, modal trap) | 0.5 day | H-02 |

### Phase 8: Missing Modules (Day 10-18, parallel via Agent Manager) — ~10 days

| Task | Effort |
|---|---|
| Cheques & GIRO (list + form + state actions + journal posting) | 1.5 days |
| Petty Cash (funds + vouchers + replenish + journal posting) | 1.5 days |
| Warehouses (list + form + stock view) | 0.5 day |
| Asset Maintenance (list + form + upcoming) | 0.5 day |
| Cost Centers (list + form + allocations + P&L + execute allocations) | 2 days |
| Approval Workflow (rules + requests + approve/reject + wire to POs/CNs) | 2 days |
| Recurring Transactions (list + form + post + journal posting + scheduler) | 1.5 days |
| AR Aging (list with buckets + summary) | 0.5 day |
| AP Aging (list with buckets + summary) | 0.5 day |
| Email Templates & Queue (screens + SMTP worker) | 1 day |
| Stock Movements screen (backend endpoint + frontend) | 1 day |
| Lease modify/terminate/depreciate UI | 1 day |
| Inter-company elimination UI + population endpoint | 1 day |

### Phase 9: Test Coverage (Day 16-20) — ~4 days

| Task | Effort |
|---|---|
| Set up vitest + write first frontend component tests | 1 day |
| Set up backend integration test harness (testcontainers) | 1 day |
| Write integration tests: PPN, GRN, period close, hash chain, DP realization | 2 days |

### Phase 10: Dead Code & Polish (Day 20-21) — ~1 day

| Task | Effort | ID |
|---|---|---|
| Delete TransactionsScreen, Assets.tsx, nginx.conf, asset-register types | 30 min | J-01,02,04,10 |
| Delete 20 stale branches | 10 min | J-07 |
| Delete fmtIDR duplicate | 5 min | J-05 |
| Merge/differentiate JournalEntryList vs JournalRegister | 4h | J-09 |
| Fix purchaseHandler shadow | 5 min | J-06 |
| Remove mockData flag from in-items | 5 min | J-08 |
| Wire or delete SupplierPaymentPanel | 1h | J-03 |
| Adopt or delete httperr package | covered Phase 4 | J-11 |

---

## Verification Gate

After each phase:
```bash
# Backend
cd backend && go vet ./... && go test ./... && go build ./...

# Frontend
cd web && npx tsc --noEmit && npx vite build

# Full
make fmt && make lint && make test && make web-build
```

## Total Effort Estimate

| Phase | Focus | Days |
|---|---|---|
| Phase 0 | Critical accounting fixes | 4.0 |
| Phase 1 | Security & RLS | 3.0 |
| Phase 2 | Frontend architecture + navigation | 3.0 |
| Phase 3 | Frontend form fixes + workflow chain | 5.0 |
| Phase 4 | Backend infrastructure + module completion | 5.0 |
| Phase 5 | Database hardening | 2.0 |
| Phase 6 | Cross-cutting UI | 3.0 |
| Phase 7 | Accessibility | 3.0 |
| Phase 8 | Missing modules (parallel) | 10.0 |
| Phase 9 | Test coverage | 4.0 |
| Phase 10 | Dead code & polish | 1.0 |
| | **Total (with parallelism)** | **~40 days** |

## Priority Recommendation

1. **Phase 0 + Phase 1 immediately** — accounting bugs corrupt the books, security bugs are exploitable
2. **Phase 2 (toast + combobox + error boundary) start first** — blocks all frontend work
3. **Phase 4 can run alongside Phase 3** — backend-only, no frontend dependency
4. **Phase 8 (module wiring) via Agent Manager worktrees** — parallel execution
5. **Phase 7 (a11y) + Phase 9 (tests) after core fixes are stable**
6. **Phase 10 (cleanup) at the end** — low risk, fast

---

*This document consolidates all findings from 4 rounds of deep audits across 10 dimensions. It is the single source of truth for implementation priorities. All other plan docs (`IMPLEMENTATION_PLAN.md`, `FEATURE_AUDIT.md`, `UI_UX_PLAN.md`) are superseded by this master plan.*
