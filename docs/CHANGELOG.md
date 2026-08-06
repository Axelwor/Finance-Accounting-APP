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
