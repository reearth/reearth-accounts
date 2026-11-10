# REST Parity with GraphQL

## Document Signature

|           |        |
|-----------|--------|
| Creator   | @pyshx |
| Leader    | TBD    |
| Task Link | TBD    |
| Developer | @pyshx |

## Background / Problem Statement

Accounts serves two APIs. GraphQL (`schemas/*.graphql`, `internal/adapter/gql`) and REST (`internal/adapter/http`, `docs/swagger.yaml`). Some clients can only use REST. Re:Earth Serve is one: it sends a `resource_id` field on permission checks ahead of per-project roles, and GraphQL rejects unknown input fields, while the Echo binder ignores them.

A comparison of every GraphQL query and mutation with the REST routes on `main` shows that REST already covers almost all of them, and calls the same interactor for each. Three differences remain.

1. `mfaStatus`, `enableMFA`, `disableMFA` and `regenerateMFARecoveryCode` have no REST route.
2. GraphQL shows a user's `email` only to that user (#374). The REST user lookups return every user's email to any signed-in caller. This is a data exposure, not only a parity gap.
3. A few response fields differ in shape. Decision 4 closes them.

The full gap table is in the appendix.

## Goals

1. Every GraphQL operation has a REST route that calls the same interactor, so authorization and validation have one implementation.
2. REST never returns data that GraphQL hides for the same caller.
3. GraphQL behavior changes only where a decision below says so.

## Non-Goals

1. A machine-to-machine path for workspace and member management. Listed under Open Questions.
2. The two GraphQL stubs `removeMultipleUsersFromWorkspace` and `removeIntegrationsFromWorkspace`, which return null. REST already implements both.

## Functional Requirements

1. Each new route uses `RequiredAuth` and passes `httpinternal.Operator(c)` to the interactor, as the GraphQL resolver passes `getOperator(ctx)`.
2. Errors render as the existing `ErrorResponse` through `CustomHTTPErrorHandler`.
3. `docs/swagger.yaml` and `docs/swagger.json` are regenerated with `make swag`. That target fails on `main` because two packages are named `internal`, so the stack fixes it first.

## Solution Options

### MFA routes

**Option 1 (chosen).** MFA is a sub-resource of the current user.

| GraphQL | REST | Response |
|---|---|---|
| `mfaStatus` | `GET /api/users/me/mfa` | `200 {"enrolled": bool}` |
| `enableMFA` | `POST /api/users/me/mfa` | `200 {"enrollment_url": string}` |
| `disableMFA` | `DELETE /api/users/me/mfa` | `204` |
| `regenerateMFARecoveryCode` | `POST /api/users/me/mfa/recovery-code` | `200 {"recovery_code": string}` |

All four use `RequiredAuth` and call `User.GetMFAStatus`, `User.EnableMFA`, `User.DisableMFA` and `User.RegenerateMFARecoveryCode`. Response bodies use `Cache-Control: private` like every REST route, and the recovery code also gets `Cache-Control: no-store`.

**Option 2.** Action routes, such as `POST /api/users/me/mfa/enable` and `POST /api/users/me/mfa/disable`. The repo uses this style for `deactivate`, `restore` and `transfer-ownership`. It was rejected because enable, status and disable act on one thing, the user's MFA enrollment, and the sub-resource needs one path instead of three.

Error mapping. The interactor returns `rerror.NewE(i18n.T("no authenticator found"))` when the user has no Auth0 identity. REST maps unknown errors to 500. The fix moves that error to a sentinel, `interfaces.ErrNoAuthenticator`, with the same message, so GraphQL output does not change, and maps it to `400` for `POST` and `DELETE /api/users/me/mfa` and the recovery-code route. `GET /api/users/me/mfa` does not error for a user with no Auth0 identity: `User.GetMFAStatus` returns `200 {"enrolled": false}`, as GraphQL `mfaStatus` does. Missing user maps to `401` and interactor not-found to `404`, as today.

### Email visibility

**Option 1 (chosen).** The REST user response takes the viewer. `httpmodel.NewUserResponse`, `NewUserResponses` and `NewSimpleUserResponse` (used by `by-name-or-email`) gain a viewer argument and blank `email` unless the viewer is that user, which is the rule in `userResolver.Email`. Routes that return the caller's own record without an authenticated viewer (signup, verification) pass an explicit own-record viewer, as GraphQL marks those responses with `ToOwnUser`. Every call site must pass a viewer, so a new route cannot forget the rule. Email is kept as an empty string, not omitted, which matches GraphQL and the gqlclient change in #374.

| Route | Email shown |
|---|---|
| `GET /api/users/:id`, `GET /api/users?ids=`, `GET /api/users/search`, `GET /api/users/by-alias`, `GET /api/users/by-name-or-alias`, `GET /api/users/by-name-or-email` | own record only |
| `GET /api/users/me`, `PATCH /api/users/me`, signup routes, `POST /api/users/verify` | always (own record) |
| `GET /api/users/all`, deactivate, restore | always. The interactor admits only callers with a platform `maintainer` or `owner` role, and GraphQL has no equivalent. Admin tooling such as the LINKS-Veda user list reads the email here |
| `POST /api/users/sync-sso` | always with its API key. A signed-in user also passes this route's gate and can name another user's `sub`, so a JWT caller gets the lookup rule |

`find-or-create` and the by-sub routes return `204` with no body.

`GET /api/users/search` still matches by email, so a caller who already knows an address can still find its owner. GraphQL `searchUser` behaves the same way.

**Option 2.** Blank emails in each handler after the response is built. Rejected because each new handler would need to remember the step.

## Design

```
client --REST--> handler --> same interactor as resolver --> repo
                    |
                    +--> httpmodel.NewUserResponse(u, viewer)   (email rule)
```

## Decisions

Recorded 2026-11-10 after review of the open questions.

### 1. A workspace can have several owners

An owner can grant `owner` through `POST /api/workspaces/:id/members`, `PATCH /api/workspaces/:id/members/:user_id`, `addUsersToWorkspace` and `updateUserOfWorkspace`. No new route. The rules, for both APIs:

1. Only an owner can grant `owner`, change an owner's role, or remove an owner. "Owner" here is the existing owner-like check (`checkOwnerLikePermission`: workspace owner, or a caller Cerbos allows). The maintainer shortcut (`IsMaintainingWorkspace`) does not apply to these actions. Error: `403`.
2. A workspace that has an active owner keeps at least one. Demoting or removing the last owner fails, whoever the caller is. A workspace created with `skip_owner_membership` starts with no owner and stays valid: the rule only stops a change that would remove the last owner, and adding members to it works as before. Error: `403` (`ErrCannotChangeOwnerRole`, `ErrOwnerCannotLeaveTheWorkspace`).
3. An owner can demote themselves or leave while another owner remains. Today an owner can never change their own role.
4. Unchanged: personal workspaces cannot change members, integrations cannot be `owner`, SCIM group mapping cannot grant `owner` (#368), `PATCH /api/service/workspaces/:id/members/:user_id` cannot grant `owner`, and `transfer-ownership` still demotes the caller to maintainer.

Rule 1 is also a fix. On `main` a maintainer can demote or remove an owner who is not themselves, including the only owner, which leaves a workspace with no owner.

### 2. Per-project roles will read `resource_id`

`POST /api/permissions/check` keeps ignoring `resource_id` until per-project roles ship. When they do, they read this field name, and GraphQL `CheckPermissionInput` gains the same field. No code change now.

### 3. Recovery codes require a recent login

`RegenerateMFARecoveryCode` refuses a session whose login is older than a configured window, for both APIs. The adapter reads the `auth_time` claim from the validated token and passes it to the interactor. Error: `interfaces.ErrRecentLoginRequired`, mapped to `401` in REST, with the same message in GraphQL. A token without the claim counts as too old.

The window is `REEARTH_ACCOUNTS_MFA_RECOVERY_MAX_AUTH_AGE` (a duration). It defaults to `0`, which turns the check off. Turn it on only after two things are in place, since otherwise no one can regenerate a code:

1. The identity provider puts `auth_time` in the token accounts receives. Firebase (CIP) ID tokens carry it. Auth0 access tokens need a post-login Action to add it. Verify the claim on the tenant before turning the check on.
2. The web client can force a fresh login before the call (Auth0 `max_age` or `prompt=login`).

`DisableMFA` (SEC-03) is not covered by this decision.

### 4. REST and GraphQL return the same shapes

The four divergences found in the first comparison are closed. Each change names which side moves.

1. **Signed photo URLs. REST moves.** `photo_url` on `GET /api/users/me` and on every workspace response is the signed URL GraphQL returns. The app layer passes the storage gateway into `RouterConfig`, and the REST builders call `GetSignedURL` as `gqlmodel.ToWorkspace` and the `me` resolver do. A signing failure keeps the stored value empty, as GraphQL does.
2. **Workspace members. Both move.**
   - REST drops user members whose user record no longer exists, as GraphQL does through its `exists` map.
   - GraphQL returns integration members as `WorkspaceIntegrationMember`. The schema has always declared that type in the `WorkspaceMember` union, and `ToWorkspace` reserves room for integrations but never adds them (since #103). REST already returns them, and REST and GraphQL both manage integrations, so dropping them from REST would leave no way to list them.
3. **Expansions. REST moves.** REST gains an opt-in `expand` query parameter, so nothing extra is fetched by default:
   - `GET /api/users/me?expand=my_workspace` embeds the personal workspace as `my_workspace`.
   - Workspace routes accept `?expand=members.user`, which embeds each user member's `UserResponse` as `user`. Embedded users follow the email rule above.
4. **`checkPermission` without a user. GraphQL moves.** GraphQL returns `interfaces.ErrInvalidOperator`, the error other resolvers return when there is no operator, instead of `not found`. REST already returns `401`.

## Open Questions

1. **Machine-to-machine access.** Accounts has an API key on `find-or-create`, `sync-sso` and `permissions/check`, and `permissions/check` still needs a user. Should accounts accept a non-user caller for workspace and member reads, and if so, which: the existing API key, or an OIDC client-credentials principal? Serve authorizes service callers from its own config until accounts decides (Serve ADR-016, decision 11).

## Potential Impact

1. REST clients that read another user's `email` from the lookup routes get an empty string. Serve does not read it. LINKS-Veda does: its admin teams page and admin user search read member emails from `GET /api/users?ids=` and `GET /api/users/search`, and its team page shows member emails to team members the same way. LINKS-Veda must move the admin pages to `GET /api/users/all` and decide what its team page shows before this ships, as with #374.
2. GraphQL `enableMFA`, `disableMFA` and `regenerateMFARecoveryCode` keep the same error message for a user with no Auth0 identity.
3. Maintainers can no longer demote or remove owners (Decision 1, rule 1). Both APIs change.
4. GraphQL `members` starts returning `WorkspaceIntegrationMember` entries (Decision 4.2). A client that reads every entry as a user member without checking `__typename` breaks. Check the GraphQL clients of accounts (visualizer, dashboard, CMS) before this ships.
5. GraphQL `checkPermission` without a user returns `invalid operator` instead of `not found` (Decision 4.4).
6. REST `photo_url` values become signed URLs that expire. A client that stores them should store the value it uploaded, not the one it read.
7. With the step-up window set, `regenerateMFARecoveryCode` and `POST /api/users/me/mfa/recovery-code` fail for an old session. The window is off by default.

## Test Plan

1. Handler tests in `internal/adapter/http/handlers` for each MFA route with a mock authenticator. Cover success, no user (401), and no Auth0 identity (`200 {"enrolled": false}` on `GET`, 400 on the other three).
2. E2e tests that read another user through each lookup route and assert an empty `email`, and read the caller's own record and assert the address. A handler test that calls `sync-sso` as a signed-in user with another user's `sub` and asserts an empty `email`.
3. `go test ./...`, `golangci-lint run`, `make generate` and `make swag` leave a clean tree.

## Deployment Plan

1. Backward compatible for MFA (new routes). Email visibility removes data from REST responses.
2. Partial deployment is possible. Each PR ships alone.
3. Notify owners of REST clients that read other users' email.
4. One new optional setting, `REEARTH_ACCOUNTS_MFA_RECOVERY_MAX_AUTH_AGE`, off by default (Decision 3).
5. No DDL or DML.

## Rollback Plan

1. Revert the PR. No data changes.

## Post Deployment

- Checklist
    - Call each MFA route as a signed-in Auth0 user.
    - Read another user through `GET /api/users/:id` and confirm `email` is empty.

## Reviewed by

- Technical Architect: TBD
- Technical Leader: TBD

## Appendix. Gap table

Measured against `main` at b7d5685. Auth is `RequiredAuth` unless noted, and the interactor enforces the rest.

| GraphQL | Interactor | REST | Status |
|---|---|---|---|
| `node`, `nodes` | `User.FetchByID`, `Workspace.FetchByID` | `GET /api/users/:id`, `GET /api/workspaces/:id`, `?ids=` | covered |
| `authConfig` | config | `GET /api/auth/config` (public) | covered |
| `checkPermission` | `Cerbos.CheckPermission` | `POST /api/permissions/check` (JWT or API key, user required) | covered |
| `me` | operator user | `GET /api/users/me` | covered, shape fixed by Decision 4.1 and 4.3 |
| `mfaStatus` | `User.GetMFAStatus` | none | gap, this change |
| `user`, `findUsersByIDs`, `findUsersByIDsWithPagination`, `findUserByAlias`, `searchUser`, `userByNameOrAlias`, `userByNameOrEmail` | `User.Fetch*`, `User.SearchUser` | `GET /api/users/:id`, `/api/users`, `/search`, `/by-alias`, `/by-name-or-alias`, `/by-name-or-email` | covered, email gap, this change |
| `findByID`, `findByIDs`, `findByName`, `findByAlias`, `findByUser`, `findByUserWithPagination` | `Workspace.Fetch*` | `GET /api/workspaces/:id`, `GET /api/workspaces?ids`, `?name`, `?alias`, `?user_id` | covered, shape fixed by Decision 4.1 and 4.2 |
| `signup`, `signupOIDC`, `verifyUser`, `createVerification`, `startPasswordReset`, `passwordReset` | `User.*` | `POST /api/users/signup`, `/signup-oidc`, `/verify`, `/verifications`, `/password-reset/start`, `/password-reset` | covered |
| `findOrCreate` | none (`User.FindOrCreate` is not on `interfaces.User`) | `POST /api/users/find-or-create` (API key or JWT) | stub on both: GraphQL returns null, REST validates and returns `204` |
| `updateMe`, `deleteMe`, `removeMyAuth`, `logout` | `User.*` | `PATCH`, `DELETE /api/users/me`, `DELETE /api/users/me/auths/:sub`, `POST /api/auth/logout` | covered |
| `enableMFA`, `disableMFA`, `regenerateMFARecoveryCode` | `User.EnableMFA`, `DisableMFA`, `RegenerateMFARecoveryCode` | none | gap, this change |
| `createWorkspace`, `updateWorkspace`, `deleteWorkspace` | `Workspace.Create`, `Update`, `Remove` | `POST /api/workspaces`, `PATCH`, `DELETE /api/workspaces/:id` | covered |
| `addUsersToWorkspace`, `updateUserOfWorkspace`, `removeUserFromWorkspace` | `Workspace.AddUserMember`, `UpdateUserMember`, `RemoveUserMember` | `POST /api/workspaces/:id/members`, `PATCH`, `DELETE /api/workspaces/:id/members/:user_id` | covered, owner refused in both (Open Question 1) |
| `transferWorkspaceOwnership` | `Workspace.TransferOwnership` | `POST /api/workspaces/:id/transfer-ownership` | covered |
| `addIntegrationToWorkspace`, `updateIntegrationOfWorkspace`, `removeIntegrationFromWorkspace` | `Workspace.*Integration*` | `/api/workspaces/:id/integrations(/:integration_id)` | covered |
| `removeMultipleUsersFromWorkspace`, `removeIntegrationsFromWorkspace` | GraphQL stub | `DELETE /api/workspaces/:id/members`, `DELETE /api/workspaces/:id/integrations` | REST ahead |

REST-only routes, with no GraphQL equivalent. `GET /api/users/all`, `GET /api/workspaces/all`, user and workspace `deactivate` and `restore`, `PATCH /api/users/by-sub/:sub`, `PUT /api/users/by-sub/:sub/platform-roles`, the workspace SCIM admin routes under `/api/workspaces/:id/scim`, `PATCH /api/service/workspaces/:id/members/:user_id`, `POST /api/users/sync-sso`, and `skip_owner_membership` on `POST /api/workspaces`. The SCIM protocol itself lives in `internal/adapter/scim`.
