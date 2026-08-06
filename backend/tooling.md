# Backend Tooling

- Go module: `go 1.24`
- PostgreSQL driver: `github.com/jackc/pgx/v5 v5.7.4`
- sqlc: `v1.29.0`, invoked through `make sqlc-generate`

## Current Environment Note

The sqlc binary installation was attempted but the local filesystem ran out of space while compiling sqlc dependencies. Query contracts and `sqlc.yaml` are committed; generate code after freeing disk space.
