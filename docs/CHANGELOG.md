# Changelog

## Unreleased

- **Audit Fixes Phase C — RLS Activation (in progress; plan `1787414685590-audit-fix-implementation-plan.md`):**
  - **C1 batch 1:** new `db.WithTenantData` helper (tenant-scoped transaction via `set_config('app.tenant_id', …, true)`) — every RLS policy is fail-closed, so pool-direct queries return zero rows for a restricted role without it. Reporting (all report fetchers + exports) and the whole dashboard package (layout CRUD, widget CRUD, widget data fetchers) now run inside tenant transactions. No schema/policy changes; remaining packages tracked in TASK_LEDGER before the restricted-role shadow run and cutover.

- **Audit Fixes Phase D — Observability Baseline (plan `1787414685590-audit-fix-implementation-plan.md`):**
  - **pg_stat_statements:** Postgres container now preloads `pg_stat_statements` (one-time `CREATE EXTENSION` applied on the server) so slow-query baselines can be inspected live.
  - **Pool stats in health:** `/healthz/detail` now reports a `pool` block (`max_conns`, `total_conns`, `acquired_conns`, `idle_conns`, `acquire_count`, `acquire_duration_ms`) from `pgxpool.Stat()`.
  - **Scheduler pass duration:** recurring-post scheduler summary logs now include `duration_ms` per pass.
  - **Request-log sampling:** access logs are sampled — errors (status ≥ 400), slow requests (> 300 ms), and ~1% of fast successes are kept at 100%/1% respectively instead of logging every request synchronously.

- **Audit Fixes Phase B — Auth & Worker Hardening (F-06, F-08, F-09, F-12, F-13, F-14, F-15; plan `1787414685590-audit-fix-implementation-plan.md`):**
  - **Refresh-token rotation atomic + replay detection:** rotation now runs inside one transaction with `FOR UPDATE`; presenting an already-revoked refresh token (replay/theft evidence) revokes the ENTIRE token family and answers the same generic 401 `INVALID_REFRESH`. Applies to `/auth/refresh` and `/auth/switch-tenant`. Concurrent double-rotation yields exactly one winner. Affected users are forced back to login by design.
  - **Dashboard fails visibly:** widget data fetchers (cash balance, P&L snapshot, AR/AP aging, low stock, recent transactions, period status, outstanding invoices, tax summary) now answer 503 `WIDGET_DATA_UNAVAILABLE` on query failure instead of silently rendering zeros/empty lists; the partial AR/AP "simple total" fallbacks were removed because a partial number reads as real data. No open accounting period remains a legitimate empty state.
  - **Cash flow multi-cash-leg guard:** split payments across cash+bank (e.g. Dr Expense 10.000 / Cr Cash 6.000 / Cr Bank 4.000) now count each offsetting line exactly once — operating outflow is 10.000, previously double-counted as 20.000; pure cash↔bank transfers contribute zero to all activities. Dimension filters on cash flow now target the offsetting leg.
  - **SMTP deadlines:** dial timeout 10s + whole-session deadline 60s (configurable via `SMTPConfig`) so one unresponsive MX cannot freeze the sequential delivery worker; timeouts feed the existing retry queue.
  - **JWT algorithm pinning:** tokens signed with any algorithm other than HS256 (RS256/none confusion) are rejected at parse time.
  - **CORS cache safety:** allowed CORS responses carry `Vary: Origin`.
  - **Register role consistency:** registration now issues `role: owner` claims/tokens matching its `'owner'` membership row (was `admin`).
  - Removed stray orphan file cleanup target (`CashEntryForm.tsx.new` was already absent).

- **Audit Fixes Phase A — Correctness & Security (F-02, F-03, F-04, F-05, F-07; plan `1787414685590-audit-fix-implementation-plan.md`):**
  - **Trial balance POSTED-only (Critical):** `fetchTrialBalance` LEFT JOINs converted to INNER JOINs — lines belonging to non-POSTED (VOID/unposted) journal entries no longer leak into totals when no date filter is supplied. Regression-tested with a live-DB integration test (POSTED + VOID entry on the same account → only posted amounts counted).
  - **Stored-XSS hardening (High):** Email template HTML preview is now sanitized client-side via DOMPurify (`<script>`, inline event handlers, and `javascript:` URLs stripped before render); all `/email/templates` + `/email/queue` endpoints moved from the staff-writable group to the Owner/Admin-only RBAC group (staff/accountant/manager/viewer now receive 403).
  - **HTTP server timeouts (High):** `http.Server` now sets ReadHeaderTimeout=10s, ReadTimeout=60s, WriteTimeout=65s (deliberately above the 60s per-request handler timeout so exports are never truncated mid-write), IdleTimeout=120s.
  - **Global body size cap (High):** new `middleware.LimitBody(8 MiB)` applied to every route — oversized request bodies are rejected; per-route tighter caps (e.g. attachments) still win.
  - **Register rate-limited (Medium):** `POST /auth/register` now shares the login limiter (5 req/min/IP), returning 429 when exceeded.

- **Frontend Visual & UX Enhancement Wave (FE-VIS-002 - Plan fe-vis-002-visual-ux-implementation.md):**
  - **Design System Tokens & Utilities:** High-contrast focus rings (`:focus-visible`), accounting double-underline rules (`.total-rule-top`, `.total-double`), soft Debit/Credit tinting (`.cell-debit`, `.cell-credit`) with dark mode support, computed field styling (`.input-computed`), sticky table headers, and standardized semantic status badges.
  - **Dashboard Analytics Widgets:** New `SegmentedAgingBar` (stacked horizontal progress with aging buckets 0-30, 31-60, 61-90, >90 days), `TrendPill` (semantic change indicators), and `QuickRatioGauge` (tri-zone liquidity gauge) integrated into Dashboard KPI and Aging matrices.
  - **Power-User Entry Forms (CashEntryForm & JournalEntryForm):** Real keyboard shortcuts (Ctrl+S to save/post, Esc to close tab), draft autosave/restore (localStorage debounced), period-lock fiscal warning banner, inline duplicate reference number warnings, arrow-key grid navigation, live debit/credit micro-balance gauge, and official double-underline totals.
  - **Global Command Palette (Ctrl+K / Cmd+K):** Instant modal search across all registered accounting modules, reports, list views, and quick entry actions with keyboard navigation (↑/↓/Enter/Esc) and TopBar launcher.
  - **Official Print Voucher Engine:** Enhanced `@media print` layout featuring official 3-box signature voucher sign-offs (Dibuat Oleh, Diperiksa Oleh, Disetujui Oleh) and full interactive control hiding.

- **Frontend Total Overhaul (Accounting-First Architecture v2.0 - Plan 1787391592524):**
  - **Zero-Glitch SVG System:** Migrasi 100% dari web component font ligature `@material/web` ke Pure SVG Icons (`lucide-react`). Menghilangkan total bug teks font ligature (`ala`, `sel`, `let`, `go`).
  - **High-Density App Shell:** Slim Navigation Rail (64px) dengan instant flyout submenu, responsive TopBar dengan Quick Action Menu (`+ Buat Baru`), dan browser-style tabstrip dengan dirty state tracking (`●`).
  - **4-Tier Financial Health Dashboard:** 5 KPI Likuiditas Utama (Kas Likuid, Laba MTD, AR, AP, Quick Ratio), Cashflow matrix, AR/AP aging buckets, live journal feed, dan Quick Action Dock.
  - **Enterprise 3-Zone Form Architecture:** Standardisasi seluruh form input transaksi (Zone 1 Header, Zone 2 Dynamic Body dengan live debit-credit balancing preview, Zone 3 Sticky Summary & Action Footer).
  - **Reusable DataTable Engine:** Search bar cepat, sortable header, status pills, responsive overflow, dan footer summary baris.
  - **Print Engine:** Professional print stylesheet (`@media print`) untuk dokumen dan laporan keuangan dengan legal sign-off area.
  - **Pure CSS Token Rebuild:** Menghilangkan specificity conflicts antar stylesheet lama menjadi token sistem modular bersih.

- **Audit trail, idempotency & error-handling fixes (A-31, M-023, B-03):**
  - `audit.Log` now covers all critical posting paths (previously only cash journals, period close, and attachment delete): sales invoices, customer payments, credit notes, delivery orders, down payments (receive + refund with before-state void snapshot), GRN, supplier invoices, supplier payments, purchase returns, asset depreciation/revaluation/disposal/impairment, lease payments and RoU depreciation, stock opname approval, production job completion, ECL provision/write-off, PPN reconciliation, PPh Final recognition + payment, deferred tax calculation, recurring transaction create/deactivate/postnow, petty cash fund/voucher/replenish, and cheque register/deposit/clear/bounce status transitions. All entries are written inside the posting transaction (roll back together with the journal) or state-mutation transaction.
  - Idempotency payload matching extended beyond cash journals: invoices, payments, credit notes, GRN, and supplier invoices now store `request_hash` (sha256 of the request body, column from migration 000032) and reject replays that reuse an idempotency key with a different body with 409 `IDEMPOTENCY_KEY_REUSE`.
  - Error classification (B-03): new `httperr.Classify` helper maps `pgx.ErrNoRows`→404, unique violation→409, FK violation→400, everything else→500; applied to invoice/payment/credit-note/GRN/supplier-invoice handlers and DP refund (which previously returned 400 for internal DB errors).
  - Status-transition mutations (cheque, petty cash, recurring) wrapped in transactions with `audit.Log` capturing before-state snapshots where relevant (e.g., cheque status: REGISTERED→DEPOSITED/CLEARED/BOUNCED; recurring deactivate: is_active=true→false).

- **Audit & Implementation Sprint 1-8 (2026-08-10/11):** Comprehensive correctness audit (`audit-report.md`, 128 findings: 4 Critical, 28 Major, 22 Minor, 16 Info) — ALL resolved or documented. Highlights:

  **Security (Critical):**
  - RBAC enforcement: `RequireRole` middleware + Role claim in JWT (owner/admin/accountant/manager/staff/viewer). Period close/unlock admin-only; write ops require accountant+; read-only for viewer.
  - JWT secret fail-fast: no fallback secret; `log.Fatal` if missing or <32 chars.
  - 2FA TOTP (RFC 6238, stdlib-only): `POST /auth/2fa/setup|verify|disable`; login enforces `totp_code` when enabled. RFC known vectors tested.
  - Rate limiting login (5 req/min/IP) + Recover/RequestLogger/CORS/Timeout middleware.

  **Accounting Integrity:**
  - Hash chain single source of truth: exported `accounting.HashJournal`; all 11 package-local hash functions now delegate (no duplication drift risk). Sort stability fix (SourceLineRef + AccountID + DebitCents tiebreaker).
  - `finalize` no longer pre-computes hash with "genesis" placeholder (posting layer computes after chain-head lock).
  - `Account.Type` populated in posting path (`isCashOrBank` works for real DB accounts).
  - Integer-cents math: `lineTotalCents` milliunits integer math (no float drift), production cost rounding fixed.
  - Credit note COGS journal idempotency key; CN total validated vs invoice receivable.
  - Period close `loadPLBalances` filters by period date range.
  - Cash flow classified operating/investing/financing; Trial Balance returns 409 when unbalanced.
  - PPN posted on invoice (Dr AR gross / Cr Revenue DPP / Cr 2202 VAT); idempotency payload-match (409 `IDEMPOTENCY_KEY_REUSE` on differing payload).
  - Unified error format (`httperr` package: code/message/details/request_id).
  - Seed COA completed (3105 Suspense, 4902 Applied Overhead, 1302 Raw Material, 4908/5908 variance accounts).

  **New Backend Modules (150+ endpoints total, see `docs/API_CONTRACT.md`):**
  - Multi-tenant: switch tenant, add tenant, `/tenants/me`; tenant switcher UI.
  - AR sub-ledger: `customer_balances` maintained on invoice/payment/CN; `/customers/ar-balances` with GL reconciliation + `/customers/{id}/statement`.
  - Approval workflow: rules per entity_type + min amount; gate on invoice posting (409 `APPROVAL_REQUIRED`), consume-on-post.
  - Lease PSAK 73 lifecycle: modification (re-measure PV, adjust RoU+liability) + termination (derecognise + gain/loss 4903/5903).
  - Inter-company elimination extended: SALE↔PURCHASE, LOAN↔LOAN, INTEREST↔INTEREST, DIVIDEND↔DIVIDEND.
  - Multi-warehouse: warehouse master, `stock_balances` per warehouse, stock transfer moves stock between warehouses.
  - Asset maintenance tracking + asset register report with NBV.
  - 2FA TOTP, petty cash (imprest), cheques/GIRO, recurring transactions, cash flow forecast, budget vs actual, email templates/queue, AR/AP aging.
  - Report templates (YAML) CRUD + render PDF/HTML via NextReport sidecar; report template editor UI with live preview.
  - Dashboard per-user widgets (layout + widget CRUD + data endpoints).
  - COA CSV export; COA export uses tenant-scoped RLS read path (`withTenantRead`).

  **Tests & Migrations:**
  - ~500 test functions across 35 packages (assets, tax, period, reporting, inventory, costing, purchase, lease, approval gate, TOTP, etc.); 35 packages pass / 0 FAIL.
  - Migrations 000025-000044 added (report templates, dashboard, warehouses, approval, asset maintenance, 2FA, customer balances, inter-company indexes, etc.).

  **Frontend:**
  - styles.css (2,922 lines) modularized into 6 ordered files under `web/src/styles/`.
  - MockEntryForm renamed DemoEntryForm; slugify Unicode-safe.
  - Customer statement, AR balances, approval screens wired.

  **Docs & Deployment:**
  - `docs/API_CONTRACT.md` synced with all 150+ endpoints.
  - `docs/DEPLOYMENT.md` deployment guide + `deploy.sh` one-command script.
  - `audit-report.md` + `implementation-tracker.md` track all 128 findings → resolved.

- US-090A + US-093: Report framework selection (EMKM/ETAP/SAK Umum) + dimensions + budget vs actual. Added a new `budget` backend package with: report framework config per tenant (`GET/POST /report-frameworks` — same posted data, different presentation); dimensions master data (`POST/GET /dimensions`, `POST /journal-lines/{id}/dimensions` — cabang/proyek/departemen/cost center tags on journal lines); and budgets (`POST/GET /budgets`, `GET /budgets/{id}`, `GET /budgets/{id}/vs-actual` — compares planned `budget_lines` against actual posted journal movements per account/month). The existing `reporting` package was extended: all four reports (`/reports/trial-balance|profit-loss|balance-sheet|cash-flow`) now accept `framework` and `dimension_id` query params — the dimension filter joins `journal_line_dimensions` to narrow aggregation, and the framework regroups the P&L by `account_type` with EMKM (2-section simplest), ETAP, and SAK_UMUM (full PSAK breakdown) labels. New migration `000022_report_frameworks_budget` (report_frameworks, dimensions, journal_line_dimensions, budgets, budget_lines + RLS + indexes). Frontend: `DimensionList` (create/list dimensions), `BudgetList` (budget headers with totals), `BudgetForm` (monthly lines per account picker), `BudgetVsActual` (report with budget picker + KPI stats + variance table), and a framework selector + dimension filter dropdown added to the P&L report toolbar. New types + API methods. Wired into Accountant (Dimensions, Budgets) and Reports (Budget vs Actual) modules. `go vet/test/build` clean; `gofmt` clean; `npm run build` clean (92 modules, JS 584.73 kB, CSS 56.88 kB).
- P3-022: US-110 Konsolidasi Multi-Entitas (PSAK 65) + US-111 Sewa (PSAK 73). Added the `backend/internal/lease` package with four files. **Lease contracts (US-111):** `POST /lease-contracts` registers a lease, calculates the present value of lease payments (`PV = payment * [1 - (1+r)^-n] / r`), and posts the initial journal `Dr 1701 Right-of-Use Asset / Cr 2301 Lease Liability` (intent `LEASE_INITIAL`) with hash-chain + outbox + idempotency. A full amortization schedule (effective interest method) is generated and stored in `lease_payments`. `POST /lease-contracts/{id}/payments/{payment_no}/post` posts a single lease payment: `Dr 2301 Lease Liability (principal) + Dr 5906 Interest Expense / Cr Cash` (intent `LEASE_PAYMENT`). `GET /lease-contracts` lists; `GET /lease-contracts/{id}` returns contract + schedule. **Consolidation (US-110):** `POST /entity-hierarchy` sets parent-child tenant relationships. `GET /consolidated-reports/trial-balance` aggregates journal lines across parent + child tenants and eliminates inter-company transactions (matched SALE/PURCHASE pairs net to zero by subtracting both entries' journal lines). `GET /consolidated-reports/profit-loss` returns the consolidated P&L with elimination. New migration `000024_consolidation_lease` creates `entity_hierarchy`, `inter_company_transactions`, `lease_contracts`, `lease_payments` with FK constraints, RLS (FORCE), and indexes; seeds accounts 1701/2301/5906 for existing tenants. `seed.go` updated to seed 1701/2301/5906 for new tenants. Routes wired in `main.go` after tax routes. 6 backend unit tests (PV, discount rate parse, monthly+quarterly schedule, frequency, format). Frontend: 4 new screens — `LeaseContractList` (listtab with ROU asset column), `LeaseContractForm` (registration form with live PV preview + existing-contract detail view), `LeasePaymentSchedule` (schedule table with post buttons), `ConsolidatedReport` (toggleable TB/P&L with elimination summary). New types + API methods. Wired into `modules.ts` (Fixed Assets + Reports), `WorkArea.tsx`, `MockEntryForm.tsx`, `types.ts`. `go vet/test/build` clean; `gofmt` clean; `npm run build` clean (93 modules, JS 582.31 kB, CSS 56.88 kB).
- P2-007: Purchase Phase 2A backend + frontend. Added Suppliers (CRUD), Purchase Orders (PO — no journal, commitment only), and Goods Received Notes (GRN — posts `Dr 1301 Inventory / Cr 2105 Uninvoiced Payables`, intent_type `GRN`). GRN records inventory movements (GRN, qty positive = stock in), validates over-delivery against PO, updates PO status to PARTIALLY_RECEIVED/RECEIVED. New migration `000011_purchase_flow` (suppliers, purchase_orders, po_lines, goods_received_notes, grn_lines + accounts 1205/2105/1203 for existing tenants + RLS). `seed.go` updated to seed 1205/2105/1203 for new tenants. Frontend: Purchases module expanded with Suppliers, Purchase Orders, and GRN sub-items. `PurchaseOrderList`/`PurchaseOrderForm` (supplier picker, item lines), `GRNList`/`GRNForm` (PO picker, item lines with unit cost), `PurchaseSupplierList`/`PurchaseSupplierForm`. New types + API methods. Routes wired in `main.go`. `go vet/test/build` clean; `make web-build` clean (62 modules, JS 406.24 kB).
- P2-006: Sales Phase 2F backend + frontend. Added Credit Note (CN / Sales Return) — the final step in the sales flow. CN posts two journals in one transaction: (1) Revenue reversal `Dr 4201 Sales Returns / Cr 1201 AR` (intent_type `SALES_RETURN`); (2) COGS reversal `Dr 1301 Inventory / Cr 5101 COGS` (intent_type `COGS_REVERSAL`) per returned item. Inventory movements recorded (SALES_RETURN, qty positive = stock in). Invoice `receivable_cents` increased; PAID→PARTIALLY_PAID if applicable. `refund_method` supports deduct/refund/credit_balance. New migration `000010_credit_note` (credit_notes, credit_note_lines + 4201 Sales Returns account for existing tenants + RLS). `seed.go` updated to seed 4201 for new tenants. Frontend: new **Credit Notes** sub-item in the Sales module, `CreditNoteList` (Accurate listtab), `CreditNoteForm` (invoice picker, return lines with unit price + unit cost + COGS reversal per line). New API methods `listCreditNotes/getCreditNote/createCreditNote`. Routes wired in `main.go`. `go vet/test/build` clean; `make web-build` clean (56 modules, JS 382.09 kB).
- P2-005: Sales Phase 2E backend + frontend. Added Invoice Payment (Pelunasan) — the final step in the SQ→SO→DP→DO→INV→Pelunasan sales flow. Payment posts a journal: `Dr Cash/Bank / Cr 1201 AR` (intent_type `SALES_RECEIPT`) with hash-chain, outbox, and idempotency. Overpayment (amount > receivable) credits `2402 Customer Overpayment` in the same journal. After payment, the invoice's `receivable_cents` is reduced and `status` becomes `PARTIALLY_PAID` or `PAID`. New migration `000009_invoice_payment` (invoice_payments + 2402 Customer Overpayment account for existing tenants + RLS). `seed.go` updated to seed 2402 for new tenants. Frontend: `InvoiceForm` gains an inline payment panel (payment list + receive-payment form with cash account picker, amount, date, description) that updates the invoice's receivable and status in real time. New API methods `createInvoicePayment/listInvoicePayments`. Routes wired in `main.go`. `go vet/test/build` clean; `make web-build` clean (54 modules, JS 371.07 kB).
- P2-004: Sales Phase 2D backend + frontend. Added Invoice (INV) to the sales flow. INV posts two journals in one transaction: (1) Revenue `Dr 1201 AR / Cr 4101 Revenue` (intent_type `SALES_INVOICE`); (2) DP realization `Dr 2201 Customer Deposit / Cr 1201 AR` (intent_type `SALES_DP_REALIZE`) — only when the linked SO has `dp_received_cents > 0`. `dp_applied_cents` is clamped to `min(dp_received, total)`; `receivable_cents = total - dp_applied`. Both journals carry hash-chain, outbox events, and idempotency. Accounts 1201/4101/2201 are resolved by code from the seeded COA. New migration `000008_invoice` (invoices, invoice_lines + RLS). Frontend: **Sales Invoices** sub-item replaced mock data with real backend — `InvoiceList` (Accurate listtab with DP Applied + Receivable columns, outstanding receivable footer), `InvoiceForm` (customer/SO picker, item lines, shows DP applied + receivable breakdown on saved invoices). New API methods `listInvoices/getInvoice/createInvoice`. `sales-invoice` module sub-item `mockData` flag removed. Routes wired in `main.go`. `go vet/test/build` clean; `make web-build` clean (54 modules, JS 367.40 kB).
- P2-003: Sales Phase 2C backend + frontend. Added Delivery Order (DO) to the sales flow. DO posts a COGS journal: `Dr 5101 COGS / Cr 1301 Inventory` per item delivered (intent_type `SALES_DELIVERY`) with hash-chain, outbox event, and idempotency. Only goods items can be delivered (services rejected); each item must have `inventory_account_id` and `cogs_account_id`. An `inventory_movements` row is recorded per line (movement_type `DO`, qty negative = stock out). The linked SO status is set to `CLOSED` after delivery. New migration `000007_delivery_order` (delivery_orders, delivery_orders_lines, inventory_movements + `delivered_qty` on sales_orders_lines + RLS). Frontend: new **Delivery Orders** sub-item in the Sales module, real `DeliveryOrderList` (Accurate listtab with Total COGS column), and `DeliveryOrderForm` (SO picker, item picker, unit cost + COGS per line). New API methods `listDeliveryOrders/getDeliveryOrder/createDeliveryOrder`. Routes wired in `main.go`. `go vet/test/build` clean; `make web-build` clean (52 modules, JS 355.43 kB).
- P2-002: Sales Phase 2B backend + frontend. Added Sales Order (SO) and Down Payment (DP) to the sales flow. SO is a commitment only — it posts no journal (per ACCOUNTING_ENGINE.md §7). DP posts a journal: `Dr Cash/Bank / Cr 2201 Customer Deposit` (intent_type `SALES_DOWN_PAYMENT`) with hash-chain, outbox event, and idempotency. DP validation rejects amounts exceeding the remaining order total (`DP_EXCEEDS_ORDER`). Multiple DPs per SO are supported; `dp_received_cents` is tracked on the order. DP refund reverses the original journal (intent_type `SALES_DP_REFUND`), marks it `VOID`, and reduces the order's received total. New migration `000006_sales_order_dp` (sales_orders, sales_orders_lines, sales_down_payments + 2201 Customer Deposit account for existing tenants + RLS). `seed.go` updated to seed 2201 for new tenants. SQ→SO conversion marks the quotation `CONVERTED`. Frontend: new **Sales Orders** sub-item in the Sales module, real `SalesOrderList` (Accurate listtab with DP received + total columns), and `SalesOrderForm` with customer/item pickers + inline DP receive/refund panel. New API methods `listSalesOrders/getSalesOrder/createSalesOrder/cancelSalesOrder/createDownPayment/listDownPayments/refundDownPayment`. Routes wired in `main.go`. `go vet/test/build` clean; `make web-build` clean (50 modules, JS 343.73 kB).
- P2-001: Sales Phase 2A backend + frontend. Added customer master data (`GET/POST /customers`, `GET /customers/{id}`, deactivate), payment terms (`GET/POST /payment-terms`), item master data (`GET/POST /items`, deactivate, `GET/POST /items/{id}/prices`, goods/service policy validation + FIFO/avg costing), and sales quotations (`POST /quotations` with lines, list/get, send/cancel/mark-expired; SQ posts **no journal** — it is a commitment only, per ACCOUNTING_ENGINE.md). New migration `000005_customer_item_sales` (payment_terms, customers, items, item_price_lists, sales_quotations, sales_quotations_lines, RLS) verified on PostgreSQL. Routes wired in `main.go`. Frontend: new **Quotations** sub-item in the Sales module, real `QuotationList` (Accurate listtab) fetching `/quotations`, and a `QuotationForm` (customer + item pickers from live API, posts no journal). New API methods `listCustomers/listItems/listQuotations/getQuotation/createQuotation/sendQuotation/cancelQuotation`. `go vet/test/build` clean; migration applied+rolled back clean; `make web-build` clean (48 modules, JS 326.81 kB).
- VIS-002: applied DBG-UI-002/003 polish to Auth, Onboarding, and Ledger screens. Auth and Onboarding adopt compact density (form padding 24px → 16px, gap 16px → 12px, input height 40px → 32px) without changing structure or copy. Ledger (`/transactions`) now uses the Accurate-style listtab layout (filter pill, action toolbar with reload-style buttons and search, 6-column ruled table, net total footer) matching Cash & Bank; the 5-col "LEDGER · X of Y" meta header is gone. Tokens stay on the live Accurate-blue palette; `make web-build` passes (CSS 57.08 kB, JS 314.36 kB).
- SPEC-001: synced durable design docs to the live Accurate-inspired corporate direction. `docs/UI_CONTRACT.md` bumped to v0.4.0 with the actual `web/src/styles.css` token set (canvas `#f5f7fa`, accent `#2f80ed`, navy ink, Inter + IBM Plex Mono). `.commandcode/design/brief.md` rewritten to mark the previous Wave-teal / cream / Source Serif / ruled-sheet direction as superseded and to record Accurate Online as the authoritative layout reference. `.commandcode/design/handoff.md` heading and "Expected absent" tokens updated. No source-code change; `web/src/styles.css` was already the source of truth and remains unchanged.
- Bootstrapped the Go backend and React/Vite frontend skeleton.
- Added durable AI Agent task ledger and repository governance.
- Installed project-level Taste Skill and Ponytail skills.
- Added base Makefile, Docker Compose, CI workflow, UI contract, and development workflow.
- Verified backend build/test and frontend production build.

### Base B0 Verification

- `make fmt` — passed
- `make lint` — passed
- `make test` — passed
- `make backend-build` — passed
- `make web-build` — passed
- Database persistence integration/sqlc — pending B3 follow-up
- Pinned frontend dependency ranges and switched CI to `npm ci --include=dev`.
- Added backend `/healthz` unit test.
- Added the first pure accounting engine MVP contract with golden tests for cash in/out, transfer, opening balance, reversal, balance, and deterministic hashing.
- Added the MVP PostgreSQL foundation migration with RLS, period constraints, tenant composite keys, idempotency indexes, reversal metadata, immutable journal triggers, deferred balance validation, and hash-chain head.
- Verified the migration on an isolated local PostgreSQL database: balanced atomic journal commit and unbalanced rollback both pass.
- Added a database integration test harness and updated `make test-integration` to require a non-superuser `TEST_DATABASE_URL`.
- Prepared VPS and Docker integration: added `postgres-test` dev service to `docker-compose.yml` (with migration volume mount and memory limits), added non-superuser `TEST_DATABASE_URL` to `.env.example`, created VS Code Remote SSH alias `finance-accounting-vps` with SSH config, and added `db-migrate-test` Makefile target.
- Verified integration tests on the VPS against a dedicated non-superuser `app_test` role: tenant isolation, balanced journal commit, unbalanced rollback, posted-journal immutability, and idempotency uniqueness all pass. The harness sets tenant context transaction-locally to match production RLS middleware.
- M1: added JWT auth backend (register/login/refresh, middleware) and reporting endpoints (trial balance, profit/loss).
- M2: added COA/category/report-mapping backend endpoints with RLS-scoped transactions.
- M3: added cash service backend (cash-in/out, transfer, opening balance, reverse) with idempotency keys, single-transaction journaling, chain head locking, and outbox events.
- M5: added the M1 frontend (login/register, onboarding wizard, dashboard, transaction forms) with a typed API stub, responsive layout, and accessibility states.
- Swapped tenant sourcing from the temporary `X-Tenant-ID` header to the JWT auth middleware context across cash and COA endpoints; all tenant-scoped routes now sit behind auth middleware.
- Added balance-sheet and cash-flow report endpoints (trial balance and profit/loss already present).
- Added refresh-token rotation and revocation: login returns access+refresh, refresh rotates within the same family and revokes the old token, logout revokes; new `user_tokens` migration (000002) applied and verified on the VPS test database with integration invariants still passing.
- Deployed to the VPS with Docker: `api` (Go), `web` (nginx static), and `postgres` (16) containers; application is live at `http://119.28.116.123` with `/healthz`, register, and login verified externally. Production secrets live in `.env.prod` on the VPS and are not committed.
- Frontend register/login/logout now call the backend API (`POST /api/v1/auth/*`); dashboard and transactions still use local mock data until the corresponding backend endpoints are wired into the UI.
- Added Caddy reverse proxy (HTTP on :80, HTTPS-ready with domain); removed the `postgres-test` dev container from the VPS.
- Registration with `tenant_name` now creates the tenant and owner membership in one transaction; JWT carries `tenant_id`; login resolves the user's default tenant. Frontend accounts, categories, transactions, and reports call the real API with Bearer token and idempotency keys, with mock fallback on network failure.
- New tenant registrations now receive a seeded default chart of accounts (17 core accounts) and 9 UI categories inside the same transaction.
- Production domains configured in Caddyfile: `tikuma.net`, `www.tikuma.net`, `accounting.tikuma.net`; automatic Let's Encrypt certificates verified for the first two (the `accounting` subdomain needs a DNS A record).
- Fixed cash posting path issues found during API testing: ledger chain head is seeded on first posting, tenants get an open accounting period on registration, and reversal now marks the original journal VOID with audit metadata and creates a REVERSAL journal linked via `reversal_of_id` (migration 000004).
- End-to-end API verification via HTTPS passed: register → cash-in/out → transfer → reports (balanced trial balance) → idempotent replay (same journal returned) → reversal (original VOID, linked reversal).
- Balance sheet now includes current-period profit (revenue − expense) in equity, so `asset = liability + equity + profit` holds before the period is closed; verified all four reports consistent on `accounting.tikuma.net` (P&L profit 289k, balance-sheet balanced, trial balance 1,011,000 = 1,011,000, cash flow net 289k).
- Added `POST /api/v1/periods/close`: posts the closing entry (P&L → 3301 → 3201 retained earnings) and locks the period in one transaction; verified end-to-end — period CLOSED, balance-sheet balanced with equity 289k, P&L 0 after close, trial balance 1,989,000 = 1,989,000.
- Added `POST /api/v1/periods/unlock`: reopens a closed period by posting a PERIOD_REOPEN reversal of the closing entry (linked via `reversal_of_id`) and restoring the period to OPEN; verified end-to-end — P&L restored to 289k, balance-sheet balanced.
- Opening balances now resolve the seeded equity account (3101) server-side when `equity_account_id` is omitted, so onboarding clients don't need tenant account ids; added unit tests. Frontend added "Tutup Buku"/"Buka Periode" buttons on the dashboard and fixed the onboarding opening-balance payload to fetch real account ids.

## DBG-UI-003 — Compact density + filled active tabs + accent-strip headers

Audit-driven refresh of Cash & Bank visual hierarchy to match the Accurate Online reference (Accurate-inspired corporate direction, SPEC-001). Three user-confirmed decisions drove the work: Save button = `#2f80ed` brand blue, density = compact, active tab = filled `--accent` background.

- **Compact density**: form padding 20px → 16px, section gap 16px → 12px, field input 36px → 32px, label uppercase tightened, detail-grid row padding 8/12 → 4/12 with 32px min-height and hover accent-soft, module/content gap 16px → 8px, workarea padding 24px → 16px.
- **Active tab filled**: top-level `.tabpill.is-active` becomes `var(--accent)` background + white text; nested-tabpill gets the same treatment for visual continuity. Tabstrip height 40px → 44px. Inactive `.tabpill__kind`, `.tabpill__status`, `.tabpill__close` recolor to white. `.tabpill__title::after` (unsaved dot) stays warning amber for legibility on the blue background.
- **Accent-strip header**: `.entrytab__head` and `.listtab__head` get a 3px `--accent` top border so the form/list is unmistakably identified. Status badge thickens (1px → 2px border, sans font, 700 weight). Number becomes an accent-soft pill with accent border. Date stamp is a chip on panel background.
- **Detail-grid + ledger-table grid**: detail-grid header switches from gray panel to accent-soft with accent-deep text + 2px accent bottom border. Ledger-table rebuilt with full outer border, accent-soft header band with 2px accent underline, row cells now carry right-rule grid lines (Accurate-style full grid), hover uses accent-soft.
- **Action rail**: width 76px → 96px, button min-height 56px, larger 20px icons. Save primary brand blue with 1px accent-deep box-shadow; Save & New warning amber. SVG stroke forced white for icon legibility on colored backgrounds.
- **Filter pills**: border upgraded to `--rule-strong`, label uppercase 2xs, value bold ink-deep, caret in accent, hover accent-soft background.
- **Nested-tabstrip**: replaced margin-bottom 16px with a 2px `--rule-strong` bottom border to match the main tabstrip's rhythm.
- Verified live on `accounting.tikuma.net` (commit `cb2723a`): bundle `index-CMVDfZ7T.css` (56.9 kB) + `index-BulEM3pP.js` (314 kB). Both form and list views match the Accurate Online reference's hierarchy.

## DBG-UI-002 — Cash & Bank layout = Accurate Online pattern

- **Form layout (CashEntryForm + MockEntryForm)** rewritten to mirror the Accurate Online reference:
  - 2-column header grid: left = Cash/Bank (or party) + Date; right = Auto-number toggle + Document number + Ambil button.
  - Full-width Keterangan / Description field below the header.
  - "Cari/Pilih Akun Perkiraan..." search bar above the detail grid (filters the account picker in the grid).
  - Detail grid simplified to 3 columns (Akun | Nama Akun | Nilai) — the cash side is implied from the header; the grid renders counter lines only. Multi-counter still works (1+ lines, sum equals cash amount).
  - Bottom-right "Nilai" total shows the running sum.
  - Right-side vertical action rail: Save (primary, posts to backend), Save & New (resets for the next entry), Document / Attach / More (placeholders, disabled).
  - Transfers render only the header (From + To + Amount) — the detail grid is hidden.
- **List view (CashEntryList)** rewritten to the same pattern:
  - Filter pill row (Tanggal: Semua · Kas/Bank: Semua · more filters toggle).
  - Action toolbar: + Tambah, Reload, Export ↓, Print 🖨, Settings ⚙, Search "Ketik dan [Enter]", count badge.
  - 6-column table: Nomor # | Tanggal | Kas/Bank | No Cek # | Keterangan | Nilai.
  - Empty state "Belum ada data".
- **CSS additions**: `.entrytab--accurate`, `.entrytab__header-grid`, `.entrytab__search`, `.entrytab__detail`, `.detail-grid` (3 cols), `.entrytab__total`, `.action-rail` + variants, `.listtab--accurate`, `.filter-pill`, `.listtab__actions`, `.listtab__search`, `.listtab__count`, `.btn--icon`, `.ledger-table` 6-column grid, `.listtab__demo`.
- Verified live on `accounting.tikuma.net` (commit `883ff0c`): bundle `index-CVnFSe5x.css` (52.9 kB) + `index-DwI7s6rY.js` (314 kB). Multi-counter verified end-to-end: 2 counter lines (300k + 200k) → HTTP 201, journal 16 posted with hash chain.

## DBG-UI-001 — UI/UX debug pass + multi-counter + nested tabs

- **Multi-counter line support (backend)**: `CashIntent` now accepts `CounterLines []CounterLine`. `CashIn`/`CashOut` distribute the counter side across one or more accounts when provided; the single `CounterAccount` field is preserved as a fallback. New request shape `counter_lines: [{account_id, amount_cents, description}]` for `POST /cash-in` and `POST /cash-out`; validation enforces that the sum of `counter_lines[].amount_cents` equals `amount_cents` and that each line has a positive account_id and amount. Ledger hash chain and balance invariant unchanged. New unit tests for split-counter success, amount mismatch, non-postable account, and CashOut debit-side distribution.
- **Nested tab model (frontend)**: the workbench now has a two-level tab model — top-level tabs are the Dashboard (pinned) and one module parent per sidebar group; each parent owns a list of child tabs (list views + entry forms) rendered as a sub-strip inside the work area. The sub-strip has `+ New entry` affordance and per-tab close. Clicking a sidebar sub-item opens the module parent and the matching child atomically. Session-storage key bumped to `v2` (old single-level tabs from prior sessions are discarded).
- **Dashboard pinned**: the Dashboard tab no longer renders a close button, and the reducer's `close` action silently no-ops for `id === "tab-dashboard"`. The pinned tab is enforced by `ensure-dashboard` on every load.
- **Cash entry form follows the header**: the first row of the multi-line grid is locked to the Cash/Bank account (or From account for transfers) picked in the Header, with a read-only amount derived from the counter rows. Counter rows can be added/removed (min 1) and each carries its own account, description, and amount; the running `D / C / Diff` totals compare the cash side against the sum of counter amounts. Transfers use two locked rows that share a single amount on the Header.
- **Mock entry form aligns with the same locked-row pattern** (single counter editable, demo banner preserved).
- **Visual fixes**: `.spark` sparkline opacity removed (the dashboard trend graphic now renders), `.page-head__meta` no longer uppercases the date (still mono, sentence-case) — restores the Accurate-style calm to the page-head chrome.
- **API client**: `postCashIn`/`postCashOut` now accept an optional `counter_lines` array and send it instead of the legacy `counter_account_id` when provided.

## Ledgerly v0.2.0 — English UI + Accurate-style redesign

- App rebranded to **Ledgerly** (was "Pembukuan Mudah"): new wordmark, HTML title, and meta description.
- UI language switched from Indonesian to English across all screens, components, API error strings, mock data, formatting (`en-US`), and seed data.
- Code identifiers, types, routes (`/transactions`, `/record/:kindParam`), and localStorage keys renamed to English (`ledgerly.m1.v1`, `ledgerly.tokens`).
- Redesigned in the style of Accurate Online: cool light-gray background (`#f5f7fa`) with a single blue accent (`#1e6fd9`), fixed left sidebar navigation (collapsible slide-over under 900px), tighter radii, and a blue-tinted auth hero.
- Backend seed COA and categories now use English names (applies to new tenants; existing tenant data is unchanged).
- Specs updated to match: `docs/UI_CONTRACT.md` v0.2.0 (English, Accurate-style tokens, sidebar layout), `GLOSSARY.md` v1.3 (English two-layer term map), and `README.md`.
- Known notes: no i18n framework (hardcoded English, per zero-dep frontend); existing tenants keep Indonesian COA/category names in the DB; users lose cached local state once after deploy due to new storage keys.
