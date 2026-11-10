# ADR-0001: REST parity with GraphQL

- **Status:** Proposed
- **Date:** 2026-11-10
- **Deciders:** @pyshx

## Context

Accounts serves two APIs: GraphQL (`server/schemas/*.graphql`, `internal/adapter/gql`) and REST (`internal/adapter/http`, `server/docs/swagger.yaml`). Some clients can only use REST. Re:Earth Serve is one: it sends `resource_id` on permission checks, and GraphQL rejects unknown input fields while the Echo binder ignores them.

A comparison of every GraphQL query and mutation with the REST routes on `main` (b7d5685) found that REST already calls the same interactor for almost every operation. The differences:

1. `mfaStatus`, `enableMFA`, `disableMFA` and `regenerateMFARecoveryCode` had no REST route.
2. GraphQL shows a user's `email` only to that user (#374). The REST user lookups returned every user's email to any signed-in caller.
3. Response shapes differed: signed photo URLs, member filtering, expansions, and the `checkPermission` error without a user.
4. `make swag` failed on `main`, so the committed spec was missing routes.

Reviewing these raised four questions: granting `owner`, `resource_id`, step-up for recovery codes, and machine-to-machine access. The first three are decided below. The fourth stays open.

## Decision

Each REST route calls the same interactor as its GraphQL resolver, so authorization and validation have one implementation. REST never returns data that GraphQL hides for the same caller. GraphQL changes only where a decision says so.

### 1. MFA is a sub-resource of the current user

| GraphQL | REST | Response |
|---|---|---|
| `mfaStatus` | `GET /api/users/me/mfa` | `200 {"enrolled": bool}` |
| `enableMFA` | `POST /api/users/me/mfa` | `200 {"enrollment_url": string}` |
| `disableMFA` | `DELETE /api/users/me/mfa` | `204` |
| `regenerateMFARecoveryCode` | `POST /api/users/me/mfa/recovery-code` | `200 {"recovery_code": string}` |

The enrollment URL and recovery code responses set `Cache-Control: no-store`. A user with no Auth0 identity gets `200 {"enrolled": false}` from `GET` and `400` (`interfaces.ErrNoAuthenticator`, same message as before) from the other three. Action routes such as `POST .../mfa/enable` were rejected, because the routes act on one thing, the MFA enrollment.

### 2. REST shows a user's email only to that user

The REST user builders take an `EmailViewer`, so every call site chooses a rule. Lookups (`GET /api/users/:id`, `/api/users?ids=`, `/search`, `/by-alias`, `/by-name-or-alias`, `/by-name-or-email`) blank other users' email to an empty string, as GraphQL does. Signup and verification return the caller's own record. `GET /api/users/all`, deactivate and restore keep the email: the interactor admits only a platform `maintainer` or `owner`, and GraphQL has no equivalent. `POST /api/users/sync-sso` shows the email only to its API key caller, because a signed-in user also passes that route's gate.

### 3. A workspace can have several owners

An owner can grant `owner` through the existing member routes and mutations. For both APIs:

1. Only an owner (the owner-like check, which asks Cerbos for `transfer_ownership`) can grant `owner`, change an owner's role, or remove an owner. The maintainer shortcut does not apply. On `main` a maintainer could demote or remove any owner, including the only one.
2. A workspace that has an active owner keeps at least one. A workspace created with `skip_owner_membership` stays valid. Every member-writing path reads the workspace with `FindByIDForUpdate`, which locks the row on PostgreSQL. On MongoDB a transaction write conflict aborts the second writer, which needs a replica set. On a standalone MongoDB, concurrent owner changes are last-write-wins.
3. An owner can step down or leave while another owner remains.
4. Integrations cannot be `owner`.
5. SCIM cannot grant `owner` and does not change an owner's role. Group sync cannot offboard an owner, so offboard owners with user-level deprovisioning (`active=false` or `DELETE /Users`).
6. `transfer-ownership` refuses the caller and disabled members as the target.

### 4. Per-project roles will read `resource_id`

`POST /api/permissions/check` keeps ignoring `resource_id` until per-project roles ship. They will read this field name, and GraphQL `CheckPermissionInput` will gain it.

### 5. Recovery codes require a recent login

`RegenerateMFARecoveryCode` refuses a login older than `REEARTH_ACCOUNTS_MFA_RECOVERY_MAX_AUTH_AGE`, for both APIs. The window defaults to `0`, which turns the check off. The login time is the claim named by `REEARTH_ACCOUNTS_AUTH_TIME_CLAIM` when it is present in the token, otherwise `auth_time`. The value must be epoch seconds. A configured claim that is present but not a number counts as too old. A reserved or refresh-stamped claim name fails startup. Auth0 does not let Actions set `auth_time`, so an Auth0 post-login Action sets a namespaced claim instead. REST answers `401` with `recent_login_required` and `WWW-Authenticate: Bearer error="insufficient_user_authentication"` (RFC 9470).

Turn the window on only after the IdP emits the claim and the web client can force a fresh login. `DisableMFA` is not covered.

### 6. REST returns the same shapes as GraphQL

1. **Photo URLs.** REST signs exactly where GraphQL signs: `GET /api/users/me` (and fails if signing fails) and every workspace response (empty on failure). The other `me` routes return the stored value, as their mutations do. Both APIs sign through one `adapter.PhotoSigner`. Signed URLs expire, so clients must not store them.
2. **Members.** REST drops members whose user no longer exists, as GraphQL does. GraphQL keeps omitting integration members, because `pkg/gqlclient`, which other services use, reads every member as a user. REST keeps returning them. This difference is intentional.
3. **Expansions.** `GET /api/users/me?expand=my_workspace` and `?expand=members.user` on workspace routes are opt-in. An embedded member user has only the fields GraphQL `User` exposes, follows the email rule, and carries the stored photo path, as GraphQL does. An unknown value returns `400`.
4. **`checkPermission` without a user.** Over HTTP both APIs already answer `401`. The unreachable GraphQL fallback now returns `ErrInvalidOperator`, as other resolvers do.

`make swag` excludes the admin API, whose `internal` package collides with `adapter/http/internal`, so the spec regenerates again.

## Consequences

- REST clients that read another user's email from the lookup routes get an empty string. LINKS-Veda reads it on its admin teams page, its admin user search and its team page, so it must switch its admin pages to `GET /api/users/all` and decide what its team page shows before decision 2 deploys, as with #374.
- Maintainers can no longer demote or remove owners.
- REST `photo_url` on `GET /api/users/me` and workspaces is a signed URL that expires.
- REST stops returning members whose user was deleted. GraphQL does not change.
- Two new optional settings, `REEARTH_ACCOUNTS_MFA_RECOVERY_MAX_AUTH_AGE` and `REEARTH_ACCOUNTS_AUTH_TIME_CLAIM`, are off by default.
- No DDL or DML. Each implementing PR can be reverted alone.

## Open question

**Machine-to-machine access.** Should accounts accept a non-user caller for workspace and member reads, and if so, with the existing API key or an OIDC client-credentials principal scoped by workspace membership? Serve authorizes service callers from its own config until accounts decides (Serve ADR-016, decision 11). This needs its own ADR.
