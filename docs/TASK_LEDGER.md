# Task Ledger

Durable task registry for all AI Agent sessions.

| ID | Status | Owner | Scope / Owned Paths | Acceptance | Changed Files | Verification | Follow-up |
|---|---|---|---|---|---|---|---|
| B0-001 | DONE | current-session | Repository bootstrap; root files, backend/, web/, docs/ | Base structure exists; backend and frontend build | Root governance, skeleton backend/frontend, Makefile, Docker Compose, CI, contracts, skills | `make fmt`, `make lint`, `make test`, `make backend-build`, `make web-build` pass | B1 toolchain/CI and B3 database foundation |
| B1-001 | DONE | current-session | Toolchain/CI; Makefile, .github/workflows/ci.yml, web/package.json, backend/cmd/api | Reproducible dependency install; backend unit test; CI build | package pinning, health test, npm ci CI | `make fmt`, `make lint`, `make test`, `make backend-build`, `npm ci --include=dev`, `npm run build` pass | B2 contracts; B3 database foundation |
| B2-001 | DONE | current-session | Shared MVP contracts; docs/API_CONTRACT.md, docs/ADR/0001-mvp-boundary.md | API and ownership boundary recorded before feature implementation | API contract, MVP boundary ADR | Read-through complete | B3 database foundation |
| B3-001 | DONE | current-session | MVP database and pure engine; backend/migrations, backend/internal/accounting, backend/queries, backend/internal/db | Database invariants and pure engine tests pass; integration harness passes on VPS | Pure engine types/tests; MVP foundation migration; SQL enforcement/RLS; sqlc.yaml; journal queries; pgx transaction helper; pgx integration harness with transaction-local tenant context | Local `make test` and VPS `TestMVPDatabaseInvariants` (tenant isolation, balance, immutability, idempotency) pass as non-superuser | Generate sqlc code once disk allows; wire HTTP services on top of transaction boundary |
| B3-002 | DONE | current-session | M2 COA/category backend; backend/internal/coa, backend/cmd/api/main.go | COA list/create, deactivate (reject on journal history), category list/create, report-mapping create; RLS via transaction-local `app.tenant_id`; unit tests for validation | backend/internal/coa/{helpers,accounts,categories,report_mappings,helpers_test}.go; backend/cmd/api/main.go (routes only) | `gofmt`, `go vet ./...`, `go test ./...`, `go build ./...` pass | Replace X-Tenant-ID header with JWT context tenant; add integration tests for RLS paths |
| M1-001 | DONE | current-session | JWT auth backend; backend/internal/auth, backend/cmd/api/main.go | Register, login, refresh with HS256 JWT; middleware injects tenant/user context; unit tests for token issue/parse | backend/internal/auth/{auth,helpers,auth_test}.go; backend/cmd/api/main.go | `go vet ./...`, `go test ./...`, `go build ./...` pass | Add refresh-token rotation/revocation table; link tenant membership on register |
| M1-002 | DONE | current-session | M4 reporting backend; backend/internal/reporting, backend/cmd/api/main.go | Trial balance, profit/loss, balance sheet, and cash flow endpoints behind auth middleware | backend/internal/reporting/{handler,helpers}.go; backend/cmd/api/main.go | `go vet ./...`, `go test ./...`, `go build ./...` pass | Add date/period filtering |
| M1-003 | DONE | current-session | Swap tenant source from X-Tenant-ID header to JWT context across cash and coa | All tenant-scoped endpoints read tenant from auth middleware context; no X-Tenant-ID header remains in handlers | backend/internal/cash/{handler,helpers,handler_test}.go; backend/internal/coa/helpers.go; backend/cmd/api/main.go; backend/internal/auth/auth.go | `go vet ./...`, `go test ./...`, `go build ./...` pass | Add refresh-token rotation table |
| M3-001 | DONE | current-session | M3 cash backend; backend/internal/cash, backend/cmd/api/main.go | cash-in/out, transfer, opening balance, reverse with idempotency key, single-transaction journal + chain head + outbox | backend/internal/cash/{handler,journal,helpers,handler_test}.go; backend/cmd/api/main.go | `go vet ./...`, `go test ./...`, `go build ./...` pass | Swap X-Tenant-ID to JWT context; add DB integration test for posting flow |
| M5-001 | DONE | current-session | M1 frontend; web/src, web/package.json | Login/register, onboarding wizard, dashboard, transaction forms, typed API stub, responsive + a11y | web/src/**/*, web/index.html, web/package.json, web/package-lock.json | `npm run build` passes (tsc clean, vite build) | Point API stub at real backend; add frontend tests |

## Rules

- One task must be claimed before editing.
- Each task declares owned paths and acceptance criteria.
- A task is `DONE` only after verification commands pass.
- Shared contracts require a separate coordination task.
- No commit or push without explicit approval.
