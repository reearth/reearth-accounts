# CLAUDE.md

This file provides guidance for Claude Code when working with this repository.

## Project Overview

reearth-accounts is an account management service for Re:Earth platform.

## Project Structure

```
.
├── server/          # Go backend (GraphQL API)
├── policies/        # Authorization policies
├── docs/            # Documentation
└── .github/         # GitHub Actions workflows
```

## Development Commands

```bash
# Run server
cd server && go run ./cmd/reearth-accounts

# Run tests
cd server && go test ./...

# Generate GraphQL code
cd server && go generate ./...

# Regenerate the sqlc query package after editing any *.sql under
# internal/infrastructure/postgres/sqlc (requires sqlc v1.27+)
cd server && make sqlc        # == sqlc generate (reads sqlc.yaml)

# Run testcontainers integration tests (postgres + mongo; needs Docker)
cd server && make test-integration
```

### Persistence backends

The server selects its persistence backend at startup from `REEARTH_ACCOUNTS_DB`:

- `mongodb://…` (default) → MongoDB (unchanged behavior)
- `postgres://…` / `postgresql://…` → PostgreSQL backend
- `REEARTH_ACCOUNTS_DB_DRIVER=postgres|mongo` overrides scheme inference

The PostgreSQL backend lives in `internal/infrastructure/postgres/`. SQL queries
are in `internal/infrastructure/postgres/sqlc/queries/`; the schema mirror used
for codegen is `internal/infrastructure/postgres/sqlc/schema/schema.sql`; the
generated package (`sqlc/gen`, committed) must be regenerated with `make sqlc`
whenever those `*.sql` files change. Runtime schema migrations are embedded SQL in
`internal/infrastructure/postgres/migration/` and run automatically on startup
(golang-migrate). A local Postgres for development is available via the `postgres`
service in `docker-compose.dev.yml`.

## Rules

The following rules are available in `.claude/rules/`:

- [conventional-commits.md](.claude/rules/conventional-commits.md) - Commit message format following Conventional Commits 1.0.0
- [release-workflow.md](.claude/rules/release-workflow.md) - How to release using Stage and Release workflows

## Key Workflows

| Workflow | Purpose |
| -------- | ------- |
| `ci_server.yml` | Run tests and linting on PRs |
| `staging.yml` | Merge main into release branch |
| `release.yml` | Create tag, GitHub Release, and Docker image |
| `build_server.yml` | Build server Docker image |
| `deploy_server_nightly.yml` | Nightly deployment |
