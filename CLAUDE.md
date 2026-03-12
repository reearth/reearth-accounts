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
```

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
