# Development Workflow

**Version:** 0.1.0  
**Status:** Draft  
**Owner:** Engineering

## AI Agent Rules

- Read root `AGENTS.md` and the relevant specs before editing.
- Claim one task in `docs/TASK_LEDGER.md` before editing.
- Stay within owned paths.
- Add tests with code changes.
- Update `docs/CHANGELOG.md` and an ADR for shared-contract changes.
- Run the required verification commands before marking a task done.

## Ponytail

- Source: `https://github.com/DietrichGebert/ponytail`
- Project installation path: `.agents/skills/ponytail` plus companion skills.
- Installation command: `npx skills add https://github.com/DietrichGebert/ponytail -y`.
- Default intent: reduce unnecessary code without removing validation, security, accessibility, or tests.
- Feature sessions may activate the installed skill but must not reinstall it.

## Validation

```text
make fmt
make lint
make test
TEST_DATABASE_URL=postgres://finance_test:finance_test@localhost:5432/finance_accounting_test make test-integration
make db-reset
make db-migrate
make web-build
```

Integration tests require a dedicated non-superuser database role. PostgreSQL superusers bypass RLS and must not be used to validate tenant isolation. The role must have access to the schema but must not have `BYPASSRLS`.
