# Conventional Commits

All commit messages and PR titles **MUST** follow the [Conventional Commits 1.0.0](https://www.conventionalcommits.org/ja/v1.0.0/) specification.

## Format

```
<type>[optional scope]: <description>
```

## Types

Choose the type based on **what the commit actually does**, not what the overall PR is about:

| Type       | When to use                                                                              | SemVer |
| ---------- | ---------------------------------------------------------------------------------------- | ------ |
| `feat`     | Adds a **new feature** for the user (new API endpoint, new UI component, new capability) | MINOR  |
| `fix`      | Fixes a **bug**                                                                          | PATCH  |
| `docs`     | Documentation only (README, comments, JSDoc, OpenAPI specs)                              | PATCH  |
| `refactor` | Code change that neither fixes a bug nor adds a feature (restructuring, renaming)        | PATCH  |
| `test`     | Adding or updating tests only                                                            | PATCH  |
| `chore`    | Build process, CI config, dependencies, tooling (Makefile, CI workflows, package.json)   | PATCH  |
| `style`    | Formatting, whitespace, linting (no logic change)                                        | PATCH  |
| `perf`     | Performance improvement                                                                  | PATCH  |
| `ci`       | CI/CD configuration changes (.github/workflows/, Makefile CI targets)                    | PATCH  |
| `build`    | Build system or external dependency changes                                              | PATCH  |

> **Note:** In this repository, our release automation is configured with `default_bump: patch`, so all commit types will result in at least a PATCH version bump.

## Decision Guide

Ask yourself: **"What is the primary purpose of this change?"**

- Adding a demo video to README -> `docs`
- Fixing a form field default value -> `fix`
- Adding a new API endpoint -> `feat`
- Updating CI workflow -> `ci`
- Adding unit tests -> `test`
- Restructuring code without behavior change -> `refactor`
- Updating dependencies -> `chore`

### Common Mistakes to Avoid

- `feat: add demo video to README` -> `docs: add demo video to README`
- `feat: update CI workflow` -> `ci: update CI workflow`
- `feat: fix connection form defaults` -> `fix: change connection form to use placeholders`
- `feat: add unit tests` -> `test: add unit tests for connection handler`
- `fix: refactor auth module` -> `refactor: restructure auth module`

## Breaking Changes

Use `!` after the type/scope to indicate a breaking change:

```
feat!: remove deprecated API endpoints
fix(api)!: change response format for /users
```

Or use `BREAKING CHANGE:` in the commit body footer.

## Scope (Optional)

Use a scope to specify the area of the codebase:

```
feat(frontend): add organization member list
fix(backend): handle null password in connection form
ci(e2e): deploy Playwright report to GitHub Pages
docs(readme): add demo GIF
```

## PR Titles

PR titles follow the **same format** as commit messages. Since PRs are squash-merged, the PR title becomes the commit message on main:

```
<type>[optional scope]: <description>
```

The PR title type should reflect the **primary change** in the PR. If a PR contains multiple types of changes (e.g., docs + fix), use the most significant type.

## Examples

```
feat: add MySQL connection support
feat(api): add organization members endpoint
fix: change Host/Port to use placeholders instead of defaults
fix(auth): handle expired JWT tokens gracefully
docs: add demo GIF to README
docs(api): update Swagger annotations
test: add Playwright E2E test suite
test(backend): add connection handler integration tests
ci: add dedicated E2E workflow
ci(e2e): deploy Playwright report to GitHub Pages
chore: update dependencies
refactor(backend): extract DDD layers
style: fix linting errors in frontend
perf(api): optimize table introspection query
```
