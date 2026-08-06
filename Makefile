help:
	@printf '%s\n' 'make backend-build backend-test web-install web-build'

backend-build:
	cd backend && go build ./...

backend-test:
	cd backend && go test ./...

web-install:
	cd web && npm install --include=dev

web-build:
	cd web && npm install --include=dev && npm run build

fmt:
	cd backend && gofmt -w $$(find . -name '*.go')

lint:
	cd backend && go vet ./...

test: backend-test

test-integration:
	@test -n "$${TEST_DATABASE_URL}" || (printf '%s\n' 'TEST_DATABASE_URL is required for integration tests' >&2; exit 1)
	cd backend && TEST_DATABASE_URL="$${TEST_DATABASE_URL}" go test ./internal/db -run TestMVPDatabaseInvariants -count=1

require-database:
	@test -n "$${DATABASE_URL}" || (printf '%s\n' 'DATABASE_URL is required' >&2; exit 1)
	@command -v psql >/dev/null || (printf '%s\n' 'psql is required' >&2; exit 1)

db-migrate: require-database
	psql "$${DATABASE_URL}" -v ON_ERROR_STOP=1 -f backend/migrations/000001_mvp_foundation.up.sql

db-migrate-test:
	@test -n "$${TEST_DATABASE_URL}" || (printf '%s\n' 'TEST_DATABASE_URL is required' >&2; exit 1)
	psql "$${TEST_DATABASE_URL}" -v ON_ERROR_STOP=1 -f backend/migrations/000001_mvp_foundation.up.sql

sqlc-generate:
	@command -v sqlc >/dev/null || (printf '%s\n' 'sqlc is required; install the pinned version from backend/tooling.md' >&2; exit 1)
	cd backend && sqlc generate

db-reset: require-database
	@printf '%s\n' 'This target is intentionally not destructive yet; use an isolated development database and run the down migration explicitly.'
	@exit 1

