# Project Guidance

## Project Overview
Finance Accounting App: Go modular monolith backend, pure double-entry accounting engine, PostgreSQL persistence, and React/Vite SPA. The root specification files are the domain source of truth; implementation lives in `backend/` and `web/`.

## Source of Truth
1. `ACCOUNTING_ENGINE.md` — accounting invariants, posting behavior, and journal effects.
2. `PRD.md` — product scope and delivery roadmap.
3. `DATA_MODEL.md` — persistence constraints that must enforce engine invariants.
4. `ARCHITECTURE.md` — technical implementation, which must not redefine accounting behavior.
5. `USER_STORIES.md` — acceptance criteria and backlog traceability.
6. `GLOSSARY.md` — UI and accounting terminology.
7. `GAP_ANALYSIS.md` — competitive prioritization only.

When documents conflict, preserve accounting invariants first, then align implementation notes and acceptance criteria to the applicable source. Update cross-references and versions whenever a specification changes.

## Multi-Agent Workflow

- Every AI Agent reads this file and relevant specifications before editing.
- Every task must be claimed in `docs/TASK_LEDGER.md` before editing.
- Each task declares owned paths; agents do not edit outside them.
- Add or update tests with every code change.
- Record changed files, decisions, migrations, and verification in the task ledger.
- Update `docs/CHANGELOG.md` for user-visible, schema, API, or accounting changes.
- Shared files (`AGENTS.md`, API contracts, migrations index, task ledger, changelog, shared types, UI contract) require a coordination task.
- Do not commit or push unless explicitly requested.

## Architecture Notes
- Backend: Go modular monolith; `backend/internal/accounting` is a pure package without IO.
- Frontend: React + TypeScript + Vite SPA; no Next.js/SSR.
- Database: PostgreSQL with sqlc/pgx, ACID transactions, tenant-scoped RLS.
- Posted journals are immutable; corrections use reversal/correction entries and audit logs.
- Outbox events are written in the same transaction as the journal.
- One tenant has one OPEN period; periods cannot overlap.
- System posting commands require idempotency keys.
- Hash-chain head is serialized per tenant.

## Skills

- UI: `.agents/skills/design-taste-frontend`; see `docs/UI_CONTRACT.md`.
- Coding workflow: `.agents/skills/ponytail`; see `docs/DEV_WORKFLOW.md`.
- Do not reinstall skills in feature sessions.

## Verification

Before completing a task, run the relevant checks. Base gate:

```text
make fmt
make lint
make test
make test-integration
make db-reset
make db-migrate
make web-build
```

A task is DONE only when acceptance criteria pass, tests pass, migrations are verified, the task ledger is updated, and no unrelated files changed.
