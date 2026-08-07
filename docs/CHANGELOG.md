# Changelog

## Unreleased

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
