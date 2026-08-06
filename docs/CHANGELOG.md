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
