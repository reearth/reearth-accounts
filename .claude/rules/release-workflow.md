# Release Workflow

## Overview

Releases are automated via GitHub Actions using `Stage` and `Release` workflows. Version numbers are determined automatically based on commit messages - no manual version input required.

## How to Release

1. Open the Actions tab on GitHub
2. Run the "Stage" workflow (merges `main` branch into `release` branch)
3. Run the "Release" workflow (creates tag, GitHub Release, and Docker build)

## Automatic Versioning

`mathieudutour/github-tag-action` analyzes commit messages and determines the version bump according to Semantic Versioning:

| Commit Type | Version Bump | Example |
| ----------- | ------------ | ------- |
| `feat:` | MINOR | 1.0.0 -> 1.1.0 |
| `fix:` | PATCH | 1.0.0 -> 1.0.1 |
| `perf:` | PATCH | 1.0.0 -> 1.0.1 |
| `BREAKING CHANGE` or `!` | MAJOR | 1.0.0 -> 2.0.0 |
| Others (`docs:`, `chore:`, etc.) | PATCH (default) | 1.0.0 -> 1.0.1 |

## Release Process

### Stage Workflow

- Merges `main` branch into `release` branch
- Conflicts are resolved by preferring `main` content (`-X theirs`)

### Release Workflow

1. **Tag Creation** - Creates and pushes a new version tag
2. **GitHub Release** - Auto-generates release notes and creates a GitHub Release
3. **Docker Build** - Builds and pushes Docker image to DockerHub
   - `reearth/reearth-accounts-api:<version>`
   - `reearth/reearth-accounts-api:latest`

## Tips

- Ensure commit messages follow Conventional Commits format before releasing
- For breaking changes, include `BREAKING CHANGE:` in the commit body or add `!` after the type (e.g., `feat!:`)
- Pre-releases (e.g., `v1.0.0-beta.1`) are automatically marked as prerelease when the tag contains `-`
