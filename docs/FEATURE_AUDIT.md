# Feature Audit — Comprehensive Per-Feature Matrix

**Audit Date:** 2026-08-11  
**Method:** Read every backend handler, trace every journal posting, verify every status transition, check every accounting formula. 3 parallel audits: sales/purchase cycles, inventory/assets/lease/tax, multi-tenant/reporting/modules.

---

## Executive Summary

| Domain | Features Checked | ✅ Working | ⚠️ Partial | ❌ Broken/Missing |
|---|---|---|---|---|
| Sales cycle (7 steps) | 7 | 5 | 1 | 1 (partial delivery) |
| Purchase cycle (5 steps) | 5 | 3 | 1 | 1 (costing bug) |
| Inventory & costing | 12 | 10 | 1 | 1 (opname) |
| Fixed assets | 10 | 10 | 0 | 0 |
| Lease (PSAK 73) | 7 | 5 | 2 | 0 |
| Tax (PPN/PPh/ECL/Deferred) | 12 | 7 | 3 | 2 (PPh posting, ECL write-off) |
| Multi-tenant & security | 10 | 7 | 2 | 1 (inter-company) |
| Approval workflow | 4 | 3 | 1 | 0 |
| Recurring | 3 | 0 | 1 | 2 (no posting, no scheduler) |
| Cheques & GIRO | 2 | 1 | 1 | 0 |
| Petty cash | 3 | 0 | 1 | 2 (no journal) |
| Cost centers | 3 | 2 | 1 | 0 |
| Email | 3 | 1 | 0 | 2 (no SMTP) |
| Bank reconciliation | 5 | 4 | 1 | 0 |
| Dashboard | 4 | 3 | 1 | 0 |
| Reporting (7 types) | 14 | 11 | 3 | 0 |
| **Total** | **97** | **72** | **18** | **7** |

**Critical bugs found: 7** | **Major gaps: 18** | **Working correctly: 72**

---

## Part 1: Sales Cycle (SQ→SO→DP→DO→INV→Payment→CN)

### Journal Postings Matrix

| Step | Intent | Dr Account | Cr Account | Balances? | Hash? | Idem? | Outbox? |
|---|---|---|---|---|---|---|---|
| SQ | (none) | — | — | N/A | N/A | ❌ | ❌ |
| SO | (none) | — | — | N/A | N/A | ❌ | ❌ |
| DP | SALES_DOWN_PAYMENT | Cash/Bank | 2201 Deposit | ✅ | ✅ | ✅ | ✅ |
| DP Refund | SALES_DP_REFUND | 2201 Deposit | Cash/Bank | ✅ | ✅ | ✅ | ✅ |
| DO | SALES_DELIVERY | 5101 COGS | 1301 Inventory | ✅ | ✅ | ✅ | ✅ |
| INV (revenue) | SALES_INVOICE | 1201 AR (DPP+PPN) | 4101 Rev + 2202 VAT | ✅ | ✅ | ✅ | ✅ |
| INV (DP realize) | SALES_DP_REALIZE | 2201 Deposit | 1201 AR | ✅ | ✅ | ⚠️ No key | ✅ |
| Payment | SALES_RECEIPT | Cash/Bank | 1201 AR + 2402 OP | ✅ | ✅ | ✅ | ✅ |
| CN (revenue) | SALES_RETURN | 4201 Returns | 1201 AR | ✅ | ✅ | ✅ | ✅ |
| CN (COGS) | COGS_REVERSAL | 1301 Inventory | 5101 COGS | ✅ | ✅ | ✅ (derived) | ✅ |

### Critical Bugs

| # | Bug | File:line | Severity |
|---|---|---|---|
| **S-01** | **Partial deliveries broken** — SO unconditionally set to CLOSED after any delivery. Second partial delivery fails because SO is already CLOSED. | `delivery.go:302-307` | CRITICAL |
| **S-02** | **DP never consumed** — `dp_received_cents` not decremented on invoice realization. Multiple invoices for same SO each apply full DP. | `invoices.go:200-203` | CRITICAL |
| **S-03** | **refund_method is a no-op** — Validated and stored but has zero effect on journal. `refund` and `credit_balance` methods are unimplemented. | `credit_notes.go:394-397` | MAJOR |
| **S-04** | **No concurrency lock on invoice** — Payment reads invoice without `FOR UPDATE`. Two concurrent payments could both claim full AR. | `payments.go:100-103` | MAJOR |
| **S-05** | **CN COGS uses roundQty** — `roundQty(2.5)=3` truncates fractional quantities. COGS reversal wrong for any fractional return. | `credit_notes.go:194` | MAJOR |
| **S-06** | **CN doesn't check item_type** — Service items get COGS reversal (Dr 1301/Cr 5101), but services have no inventory. | `credit_notes.go:179-205` | MEDIUM |
| **S-07** | **No over-delivery check** — DO doesn't compare delivered qty vs SO line qty. Can deliver more than ordered. | `delivery.go` | MEDIUM |
| **S-08** | **SQ→SO conversion is blind** — No validation that quotation exists, belongs to same customer, or is in convertible state. | `orders.go:223-230` | MEDIUM |
| **S-09** | **DP realization journal has no idempotency_key** — INSERT omits the column. | `invoices.go:316-322` | MEDIUM |
| **S-10** | **No validation that DO/CN items belong to SO/invoice** — Can deliver/return items not on the order. | `delivery.go`, `credit_notes.go` | MEDIUM |

### Status Transitions

| Document | Statuses | Transitions |
|---|---|---|
| SQ | DRAFT→SENT→CONVERTED/EXPIRED/CANCELLED | ✅ Correct |
| SO | CONFIRMED→CLOSED/CANCELLED | ⚠️ No DRAFT; created directly as CONFIRMED |
| DO | SHIPPED/RETURNED/CANCELLED | ✅ |
| INV | DRAFT/ISSUED/PARTIALLY_PAID/PAID/VOID | ✅ Created as ISSUED |
| CN | DRAFT/APPLIED/VOID | ✅ Created as APPLIED |

---

## Part 2: Purchase Cycle (PO→GRN→SI→SP→Return)

### Journal Postings Matrix

| Step | Intent | Dr Account | Cr Account | Balances? | Hash? | Idem? | Outbox? |
|---|---|---|---|---|---|---|---|
| PO | (none) | — | — | N/A | N/A | ❌ | ❌ |
| GRN | PURCHASE_RECEIPT | 1301 Inventory | 2105 Uninvoiced | ✅ | ✅ | ✅ | ✅ |
| SI | SUPPLIER_INVOICE | 2105 + 1203 VAT | 2101 AP | ✅ | ✅ | ✅ | ✅ |
| SP | SUPPLIER_PAYMENT | 2101 AP + 1204 OP | Cash/Bank | ✅ | ✅ | ✅ | ✅ |
| Return | PURCHASE_RETURN | 2101 AP | 1301 Inv + 1203 VAT | ✅ | ✅ | ✅ | ✅ |

### Critical Bugs

| # | Bug | File:line | Severity |
|---|---|---|---|
| **P-01** | **Purchase return costing bug** — `costing.ReverseCOGS` INCREASES `qty_on_hand` for purchase returns. Should DECREASE (goods go back to supplier). Stock sub-ledger diverges from GL. | `purchase_returns.go:307-310` | CRITICAL |
| **P-02** | **PO never reaches RECEIVED** — `poStatusAfterGRN` only returns PARTIALLY_RECEIVED. No logic to detect all lines fully received. | `grn.go:480-486` | MAJOR |
| **P-03** | **No AP sub-ledger** — No `supplier_balances` table. AP tracked only at invoice level, unlike AR's `customer_balances`. | — | MAJOR |
| **P-04** | **Purchase DP realization not implemented** — Supplier invoice hardcodes `dpApplied = 0`. | `supplier_invoices.go:252` | MAJOR |
| **P-05** | **No concurrency lock on SI row** — Supplier payment and purchase return read SI without FOR UPDATE. | `supplier_payments.go:104-107` | MAJOR |
| **P-06** | **refund_method is a no-op** — Same as sales CN. | `purchase_returns.go:335-338` | MAJOR |
| **P-07** | **No PO cancel endpoint** — `poStatusCancelled` is defined and checked by GRN, but no route exists. | — | MEDIUM |
| **P-08** | **Float64 line totals** — PO, GRN, SI all use `float64` math that can truncate, unlike sales side's milliunit integer approach. | `grn.go:476`, `supplier_invoices.go:509` | MEDIUM |
| **P-09** | **No over-delivery check** — GRN doesn't compare received vs ordered qty. | `grn.go` | MEDIUM |
| **P-10** | **No validation that SI/return items match GRN/invoice** | `supplier_invoices.go`, `purchase_returns.go` | MEDIUM |

---

## Part 3: Inventory & Costing

### Feature Matrix

| Feature | Status | Notes |
|---|---|---|
| Stock balance per item+warehouse | ✅ | `(tenant, item, warehouse)` unique |
| GRN updates stock_balance | ✅ | `PostGRN` adds qty + creates FIFO layer |
| DO updates stock_balance + resolves COGS | ✅ | `ResolveCOGS` consumes layers |
| FIFO oldest-layer-first | ✅ | `ORDER BY created_at, id FOR UPDATE` |
| Moving average recalculation | ✅ | Atomic SQL UPSERT |
| COGS reversal on credit note | ✅ | `ReverseCOGS` restores layers |
| COGS reversal on purchase return | ❌ | **WRONG DIRECTION** — increases stock instead of decreasing |
| Negative stock rejected | ✅ | `ErrInsufficientStock` |
| Cost layers per warehouse | ✅ | `warehouse_id` on all tables |
| Stock transfer (both warehouses) | ✅ | `ResolveCOGS` + `PostGRN` |
| **Stock opname updates stock_balance** | ❌ | **CRITICAL — never calls costing. stock_balances diverges from inventory_movements after every opname.** |
| GRN/DO pass warehouseID | ⚠️ | Both pass `warehouseID=0`. Only transfer uses real IDs. |

### Critical Bug: Stock Opname

**File:** `backend/internal/inventory/stock_opname.go`

Stock opname posts the adjustment journal (Dr/Cr 1301 + adjustment gain/loss) and records `inventory_movements`, but **never calls `costing.PostGRN` (surplus) or `costing.ResolveCOGS` (shortage)**. After every approved opname:
- `stock_balances.qty_on_hand` is stale
- `avg_unit_cost_cents` is stale
- FIFO layers are not adjusted
- Subsequent COGS resolution uses wrong stock balance → false `ErrInsufficientStock` or allows negative stock

**Fix:** Call `costing.PostGRN` for surplus lines and `costing.ResolveCOGS` for shortage lines inside `ApproveStockOpname`.

---

## Part 4: Fixed Assets

### Feature Matrix (All ✅)

| Feature | Status | Journal |
|---|---|---|
| Acquisition | ✅ | Dr 1401 / Cr Cash/AP |
| Straight-line depreciation | ✅ | Dr 5206 / Cr 1402 |
| Declining balance | ✅ | Dr 5206 / Cr 1402 (rate × book value) |
| Units of production | ✅ | Dr 5206 / Cr 1402 (per-unit × usage) |
| Fully-depreciated guard | ✅ | Clamps to salvage |
| Disposal (gain/loss) | ✅ | Dr Cash + Dr 1402 + Dr 5903 / Cr 1401 + Cr 4903 |
| Revaluation (OCI) | ✅ | Dr 1401 / Cr 3401 (up) or reverse (down) |
| Impairment | ✅ | Dr 5207 / Cr 1401 |
| Asset register report | ✅ | NBV + totals |
| Asset maintenance tracking | ✅ | 5 types, upcoming-due planning |

**Minor issues:** Depreciation truncates (should use `math.Round`); straight-line extends depreciation past useful life by 1 month for non-divisible bases.

---

## Part 5: Lease (PSAK 73)

### Feature Matrix

| Feature | Status | Notes |
|---|---|---|
| PV calculation | ✅ | `PV = payment × [1−(1+r)^−n] / r` |
| Commencement journal | ✅ | Dr 1701 RoU / Cr 2301 Liability |
| Amortization schedule | ✅ | Effective interest method, all payments generated |
| Payment journal | ✅ | Dr 2301 + Dr 5906 / Cr Cash |
| RoU depreciation | ✅ | Dr 5209 / Cr 1702, straight-line, idempotent |
| Modification | ✅ | Re-measures PV, adjusts RoU + liability |
| Termination | ✅ | Derecognize, gain/loss 4903/5903 |
| **Schedule regenerated after modification** | ⚠️ | Old schedule retained — stale principal/interest splits |
| **Rate=0 PV guard** | ⚠️ | Code handles it but validation rejects `rate <= 0` — unreachable |

---

## Part 6: Tax

### Feature Matrix

| Feature | Status | Notes |
|---|---|---|
| PPN Keluaran on invoice | ✅ | Cr 2202 when PPN > 0 |
| PPN Masukan on supplier invoice | ✅ | Dr 1203 when VAT > 0 |
| PPN reconciliation | ✅ | Sums 2202 credits − 1203 debits |
| PPN settlement journal | ⚠️ | No auto-settlement (Dr 2202 / Cr Cash) |
| PPN rate from tax_rates | ⚠️ | Per-line caller-supplied, not enforced from table |
| PPh types (21/22/23/26/UMKM) | ✅ | All types with correct rates |
| **PPh posts a journal** | ❌ | `pph/handler.go` only flips status, returns instruction string. **No journal posted.** |
| PPh Final UMKM posts journal | ✅ | Dr 5208 / Cr 2203 |
| ECL aging buckets | ✅ | 0-30=1%, 31-60=2.5%, 61-90=5%, >90=10% |
| ECL provision journal | ✅ | Dr 5209 / Cr 1202 |
| ECL write-off | ✅ | Dr 1202 / Cr 1201 |
| **ECL ages from invoice_date** | ❌ | Should use due_date — over-provisions for in-term invoices |
| **ECL write-off doesn't update invoice** | ❌ | Invoice remains ISSUED → re-aged and re-provisioned |
| Deferred tax journal | ✅ | Dr/Cr 1206 / 5904 |
| Deferred tax auto-computed | ⚠️ | Manual temp differences input |

### Critical Bugs

| # | Bug | File:line | Severity |
|---|---|---|---|
| **T-01** | **PPh handler doesn't post journals** — Only flips status. Should post `Dr 5203 / Cr 210x PPh Payable`. | `pph/handler.go:202-221` | CRITICAL |
| **T-02** | **ECL write-off doesn't update invoice/subledger** — Invoice stays ISSUED, re-aged, re-provisioned. | `tax/ecl.go:362-430` | CRITICAL |
| **T-03** | **ECL ages from invoice_date not due_date** | `tax/ecl.go:235,262` | HIGH |

---

## Part 7: Multi-Tenant & Security

### Feature Matrix

| Feature | Status | Notes |
|---|---|---|
| Multi-tenant (user → multiple tenants) | ✅ | `user_tenants` join table |
| Tenant switcher | ✅ | `POST /auth/switch-tenant` |
| Registration creates tenant + membership | ✅ | One transaction |
| JWT tenant_id used everywhere | ✅ | `TenantIDFromContext` |
| Create additional tenants | ✅ | `POST /tenants/new` |
| RBAC (6 roles) | ⚠️ | Coarse — accountant/manager/staff are identical |
| 2FA (RFC 6238) | ✅ | Fully compliant, ±30s tolerance |
| **Inter-company elimination** | ⚠️ | No endpoint to populate `inter_company_transactions` |
| **consolidation_pct applied** | ❌ | Stored but never used in aggregation |
| **Elimination persisted** | ⚠️ | `eliminated` flag never set to true |
| Entity hierarchy | ✅ | Parent-child stored |
| Approval gate on invoices | ✅ | CheckAmount + ConsumeApprovalByAmount |
| **Approval gate on other entity types** | ❌ | Only invoices, not POs/CNs/journals |
| Audit log coverage | ⚠️ | Only 3 of ~15 posting paths call `audit.Log` |

---

## Part 8: Module Completeness

| Module | Feature | Status | Gap |
|---|---|---|---|
| **Recurring** | Frequencies (daily-weekly-yearly) | ✅ | — |
| | "Post now" posts journal | ❌ | Only advances next_date, no journal |
| | Scheduler/cron | ❌ | Comment claims scheduler exists, but it doesn't |
| | Update endpoint | ❌ | Returns 501 NOT_IMPLEMENTED |
| **Cheques** | State machine (REGISTERED→DEPOSITED→CLEARED/BOUNCED) | ✅ | — |
| | Journal on state transition | ❌ | Tracking only, no journal posted |
| **Petty Cash** | Fund creation | ❌ | No journal (returns instruction string) |
| | Voucher creation | ❌ | No journal |
| | Replenish | ❌ | No journal |
| | Imprest math | ✅ | Computation correct |
| **Cost Centers** | Hierarchy | ✅ | — |
| | Allocations stored | ✅ | — |
| | **Allocations executed** | ❌ | Never runs to post proportional journals |
| | P&L per cost center | ✅ | Via dimension_id join |
| **Email** | Template CRUD | ✅ | — |
| | Auto-trigger on events | ❌ | Manual enqueue only |
| | SMTP integration | ❌ | No `net/smtp`, no worker |
| **Bank Reconciliation** | Statement import | ✅ | JSON only, no CSV/MT940 |
| | Auto-match | ✅ | Exact amount + date ±3 days |
| | Manual match/unmatch | ✅ | — |
| | Complete posts adjustment | ❌ | No journal for residual unmatched |
| **Dashboard** | 8/11 widget types | ✅ | 3 unimplemented (budget_vs_actual, revenue_by_customer, tax_summary) |
| | Layout customization | ✅ | Per-user, persisted |
| **Attachments** | Upload/download | ✅ | 10MB limit, 3 MIME types |
| | Tenant-scoped | ✅ | — |
| | Content sniffing | ❌ | Trusts client Content-Type |
| **Audit Log** | Fields logged | ✅ | action, entity, before/after JSONB |
| | **Coverage** | ⚠️ | Only cash + period + attachment delete |

---

## Part 9: Reporting

| Report | Status | Date Range | Framework | Dimension | Export |
|---|---|---|---|---|---|
| Trial Balance | ✅ | ❌ (UI) | ❌ | ✅ | ✅ (backend) / ❌ (UI) |
| P&L | ✅ | ❌ (UI) | ✅ EMKM/ETAP/SAK | ✅ | ✅ / ❌ |
| Balance Sheet | ✅ | ❌ (UI) | ❌ (UI) | ❌ (UI) | ✅ / ❌ |
| Cash Flow | ⚠️ | ❌ (UI) | ❌ | ❌ | ✅ / ❌ |
| Budget vs Actual | ✅ | ✅ | — | ✅ | — |
| Consolidated TB/P&L | ⚠️ | — | — | — | — |
| Report Templates | ✅ | — | — | — | ✅ (via NextReport) |
| AR Aging | ❌ (no frontend) | — | — | — | — |
| AP Aging | ❌ (no frontend) | — | — | — | — |

### Cash Flow Bug (confirmed)

Multi-leg cash entries are double-counted — the cash line amount is summed once per offsetting leg. A lease payment `Dr 2301/Dr 5906/Cr Cash X` counts X as financing AND X as operating. Total cash flow = 2× actual.

---

## Part 10: Complete Critical Bug List (All Audits Combined)

### CRITICAL (Corrupts books or security)

| # | Bug | Domain | File |
|---|---|---|---|
| 1 | 2FA bypass via Setup2FA | Security | `auth.go:439-441` |
| 2 | DP double-realization | Sales | `invoices.go:200-203` |
| 3 | Cash flow 2× inflation | Reporting | `data.go:418-419` |
| 4 | Purchase return costing wrong direction | Purchase | `purchase_returns.go:307-310` |
| 5 | Stock opname doesn't update stock_balances | Inventory | `stock_opname.go` |
| 6 | PPh handler doesn't post journals | Tax | `pph/handler.go:202-221` |
| 7 | ECL write-off doesn't update invoice | Tax | `ecl.go:362-430` |
| 8 | AR aging queries 'POSTED' (nonexistent status) | Reporting | `aging/handler.go:99,130` |
| 9 | Production uses wrong table name | Production | `helpers.go:212` |
| 10 | 4 account codes missing from seed.go | Seed | `seed.go` |
| 11 | Partial deliveries broken (SO unconditionally CLOSED) | Sales | `delivery.go:302-307` |
| 12 | Timeout middleware goroutine race | Backend | `middleware.go:117-119` |
| 13 | 11 packages skip RLS | Security | Multiple |
| 14 | 6 tables RLS not FORCED | Security | `000033:232-248` |
| 15 | Wildcard CORS | Security | `middleware.go:65` |
| 16 | CustomerForm permanent loading | Frontend | `CustomerForm.tsx:31` |
| 17 | No Error Boundary | Frontend | `App.tsx` |
| 18 | 4 modules never post journals (pettycash/recurring/pph/cheque) | Backend | Multiple |

### MAJOR (Incorrect behavior, missing critical features)

| # | Bug | Domain |
|---|---|---|
| 19 | refund_method is a no-op (CN + Purchase Return) | Sales/Purchase |
| 20 | No concurrency lock on payment rows | Sales/Purchase |
| 21 | PO never reaches RECEIVED status | Purchase |
| 22 | No AP sub-ledger (supplier_balances) | Purchase |
| 23 | Purchase DP realization not implemented | Purchase |
| 24 | CN COGS roundQty truncates fractional qty | Sales |
| 25 | ECL ages from invoice_date not due_date | Tax |
| 26 | Credit note COGS trusts client cost | Sales |
| 27 | PPN truncates (should round) | Sales |
| 28 | Shipping/other charges never posted | Sales |
| 29 | Depreciation truncation extends useful life | Assets |
| 30 | Lease schedule not regenerated after modification | Lease |
| 31 | Inter-company elimination has no population endpoint | Multi-tenant |
| 32 | consolidation_pct never applied | Multi-tenant |
| 33 | Approval gate only on invoices | Multi-tenant |
| 34 | Audit log covers only 3 of ~15 posting paths | Audit |
| 35 | Idempotency gap — only cash does payload matching | Backend |
| 36 | Bank reconciliation complete doesn't post adjustment | Banking |
| 37 | Email has no SMTP/worker | Email |
| 38 | Dashboard 3/11 widgets unimplemented | Dashboard |
| 39 | httperr package is dead code (7 duplicates) | Backend |
| 40 | No graceful shutdown | Infra |
| 41 | No backup strategy | Infra |
| 42 | Prod missing NextReport | Infra |
| 43 | Internal errors leaked to clients | Backend |
| 44 | Tab switch destroys unsaved data | Frontend |
| 45 | Can't open 2 entries of same type (dedup bug) | Frontend |
| 46 | 4 forms allow duplicates (no saved-state) | Frontend |
| 47 | GRN/DO/CN don't load parent lines | Frontend |
| 48 | No FK combobox (plain select everywhere) | Frontend |
| 49 | No pagination on any list | Frontend/Backend |
| 50 | No toast/notification system | Frontend |
| 51 | Dead status filter pills on 6 screens | Frontend |
| 52 | Dashboard hardcoded KPIs | Frontend |
| 53 | Reports no date range in UI | Frontend |
| 54 | No export buttons in UI | Frontend |
| 55 | 4 account code collisions | Database |
| 56 | 15+ missing indexes | Database |
| 57 | Lease in-advance vs in-arrears inconsistency | Lease |
| 58 | Cost center allocations never executed | Cost Center |
| 59 | Recurring has no scheduler | Recurring |
| 60 | Float64 line totals in purchase cycle | Purchase |

---

## Summary

The backend accounting engine is **structurally sound** — hash chain, idempotency, outbox, RLS (where applied), and journal balance checks are all correctly implemented in the core posting paths (cash, sales, purchase, lease, assets). The **critical bugs are in edge cases** (partial deliveries, purchase returns, stock opname, ECL, multi-leg cash flow) and **infrastructure modules** (petty cash, recurring, PPh, email, cheques) that were built as tracking-only shells without journal posting.

The frontend has the most gaps — 10+ backend modules have zero frontend wiring, and the existing forms have significant UX and correctness bugs.

**Priority fix order:**
1. Critical accounting bugs (#1-18) — corrupt the books
2. Major accounting bugs (#19-30) — incorrect behavior
3. Infrastructure gaps (#31-43) — modules are shells
4. Frontend fixes (#44-54) — UX and data loss
5. Database hardening (#55-56) — performance and integrity
6. Module wiring (#57-60) — remaining gaps
