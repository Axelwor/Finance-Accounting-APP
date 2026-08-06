# Task Ledger

Durable task registry for all AI Agent sessions.

| ID | Status | Owner | Scope / Owned Paths | Acceptance | Changed Files | Verification | Follow-up |
|---|---|---|---|---|---|---|---|
| B0-001 | DONE | current-session | Repository bootstrap; root files, backend/, web/, docs/ | Base structure exists; backend and frontend build | Root governance, skeleton backend/frontend, Makefile, Docker Compose, CI, contracts, skills | `make fmt`, `make lint`, `make test`, `make backend-build`, `make web-build` pass | B1 toolchain/CI and B3 database foundation |
| B1-001 | DONE | current-session | Toolchain/CI; Makefile, .github/workflows/ci.yml, web/package.json, backend/cmd/api | Reproducible dependency install; backend unit test; CI build | package pinning, health test, npm ci CI | `make fmt`, `make lint`, `make test`, `make backend-build`, `npm ci --include=dev`, `npm run build` pass | B2 contracts; B3 database foundation |
| B2-001 | DONE | current-session | Shared MVP contracts; docs/API_CONTRACT.md, docs/ADR/0001-mvp-boundary.md | API and ownership boundary recorded before feature implementation | API contract, MVP boundary ADR | Read-through complete | B3 database foundation |
| B3-001 | DONE | current-session | MVP database and pure engine; backend/migrations, backend/internal/accounting, backend/queries, backend/internal/db | Database invariants and pure engine tests pass; integration harness passes on VPS | Pure engine types/tests; MVP foundation migration; SQL enforcement/RLS; sqlc.yaml; journal queries; pgx transaction helper; pgx integration harness with transaction-local tenant context | Local `make test` and VPS `TestMVPDatabaseInvariants` (tenant isolation, balance, immutability, idempotency) pass as non-superuser | Generate sqlc code once disk allows; wire HTTP services on top of transaction boundary |

## Rules

- One task must be claimed before editing.
- Each task declares owned paths and acceptance criteria.
- A task is `DONE` only after verification commands pass.
- Shared contracts require a separate coordination task.
- No commit or push without explicit approval.
