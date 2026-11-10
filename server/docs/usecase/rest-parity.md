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
3. A few response fields differ in shape. They are listed under Known divergences and are not changed here.

The full gap table is in the appendix.

## Goals

1. Every GraphQL operation has a REST route that calls the same interactor, so authorization and validation have one implementation.
2. REST never returns data that GraphQL hides for the same caller.
3. GraphQL behavior does not change.

## Non-Goals

1. Granting the `owner` role directly. The interactor refuses it for both APIs (`ErrCannotChangeRoleToOwner`), and only ownership transfer can make an owner. Changing that is a rule change, listed under Open Questions.
2. A machine-to-machine path for workspace and member management. Listed under Open Questions.
3. Signed `photo_url` values and the other shape differences under Known divergences.
4. The two GraphQL stubs `removeMultipleUsersFromWorkspace` and `removeIntegrationsFromWorkspace`, which return null. REST already implements both.

## Functional Requirements

1. Each new route uses `RequiredAuth` and passes `httpinternal.Operator(c)` to the interactor, as the GraphQL resolver passes `getOperator(ctx)`.
2. Errors render as the existing `ErrorResponse` through `CustomHTTPErrorHandler`.
3. `docs/swagger.yaml` and `docs/swagger.json` are regenerated with `make swag`.

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

Error mapping. The interactor returns `rerror.NewE(i18n.T("no authenticator found"))` when the user has no Auth0 identity. REST maps unknown errors to 500. The fix moves that error to a sentinel, `interfaces.ErrNoAuthenticator`, with the same message, so GraphQL output does not change, and maps it to `400`. Missing user maps to `401` and interactor not-found to `404`, as today.

### Email visibility

**Option 1 (chosen).** The REST user response takes the viewer. `httpmodel.NewUserResponse` gains a viewer argument and blanks `email` unless the viewer is that user, which is the rule in `userResolver.Email`. Every call site must pass a viewer, so a new route cannot forget the rule. Email is kept as an empty string, not omitted, which matches GraphQL and the gqlclient change in #374.

| Route | Email shown |
|---|---|
| `GET /api/users/:id`, `GET /api/users?ids=`, `GET /api/users/search`, `GET /api/users/by-alias`, `GET /api/users/by-name-or-alias`, `GET /api/users/by-name-or-email` | own record only |
| `GET /api/users/me`, `PATCH /api/users/me`, signup routes, `find-or-create`, `sync-sso` | always (own record) |
| `GET /api/users/all`, deactivate, restore, by-sub routes | always (maintainer or owner gated) |

`GET /api/users/search` still matches by email, so a caller who already knows an address can still find its owner. GraphQL `searchUser` behaves the same way.

**Option 2.** Blank emails in each handler after the response is built. Rejected because each new handler would need to remember the step.

## Design

```
client --REST--> handler --> same interactor as resolver --> repo
                    |
                    +--> httpmodel.NewUserResponse(u, viewer)   (email rule)
```

## Known divergences

These stay as they are in this change. Each needs its own decision.

1. GraphQL `photoURL` on `me` and on workspaces is a signed URL. REST returns the stored value. Storage is built only in `internal/app/graphql.go`, so signing in REST needs the storage gateway wired into the REST router.
2. GraphQL workspace `members` drops users whose record no longer exists and drops integration members. REST returns every member, integrations included.
3. GraphQL can expand `myWorkspace` on `me` and `user` on a member. REST returns ids, and the client reads `GET /api/workspaces/:id` or `GET /api/users?ids=`.
4. GraphQL `checkPermission` without a user returns not found. REST returns `401`.

## Open Questions

1. **Granting `owner`.** Should an owner be able to add a second owner, through `POST /api/workspaces/:id/members` and `PATCH /api/workspaces/:id/members/:user_id` and the matching GraphQL mutations? Today only transfer can, and transfer demotes the caller to maintainer. Serve treats accounts as the owner of this rule and does not need a change to ship.
2. **Machine-to-machine access.** Accounts has an API key on `find-or-create`, `sync-sso` and `permissions/check`, and `permissions/check` still needs a user. Should accounts accept a non-user caller for workspace and member reads, and if so, which: the existing API key, or an OIDC client-credentials principal? Serve authorizes service callers from its own config until accounts decides (Serve ADR-016, decision 11).
3. **`resource_id` on `POST /api/permissions/check`.** Serve already sends it on project checks, and accounts ignores it. If per-project roles ship, will they read this field name?
4. **Recovery code step-up.** `RegenerateMFARecoveryCode` needs only a session (the SEC-04 note in `interactor/user.go`). The REST route inherits that. Should both APIs require a recent login first?

## Potential Impact

1. REST clients that read another user's `email` from the lookup routes get an empty string. Serve does not read it. Other REST clients should check before this ships.
2. GraphQL `enableMFA`, `disableMFA` and `regenerateMFARecoveryCode` keep the same error message for a user with no Auth0 identity.

## Test Plan

1. Handler tests in `internal/adapter/http/handlers` for each MFA route with a mock authenticator. Cover success, no user (401) and no Auth0 identity (400).
2. Handler and e2e tests (`e2e/rest_user_test.go`) that read another user through each lookup route and assert an empty `email`, and read the caller's own record and assert the address.
3. `go test ./...`, `golangci-lint run`, `make generate` and `make swag` leave a clean tree.

## Deployment Plan

1. Backward compatible for MFA (new routes). Email visibility removes data from REST responses.
2. Partial deployment is possible. Each PR ships alone.
3. Notify owners of REST clients that read other users' email.
4. No configuration change.
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
| `me` | operator user | `GET /api/users/me` | covered, divergence 1 and 3 |
| `mfaStatus` | `User.GetMFAStatus` | none | gap, this change |
| `user`, `findUsersByIDs`, `findUsersByIDsWithPagination`, `findUserByAlias`, `searchUser`, `userByNameOrAlias`, `userByNameOrEmail` | `User.Fetch*`, `User.SearchUser` | `GET /api/users/:id`, `/api/users`, `/search`, `/by-alias`, `/by-name-or-alias`, `/by-name-or-email` | covered, email gap, this change |
| `findByID`, `findByIDs`, `findByName`, `findByAlias`, `findByUser`, `findByUserWithPagination` | `Workspace.Fetch*` | `GET /api/workspaces/:id`, `GET /api/workspaces?ids|name|alias|user_id` | covered, divergence 1 and 2 |
| `signup`, `signupOIDC`, `findOrCreate`, `verifyUser`, `createVerification`, `startPasswordReset`, `passwordReset` | `User.*` | `POST /api/users/signup`, `/signup-oidc`, `/find-or-create` (API key or JWT), `/verify`, `/verifications`, `/password-reset/start`, `/password-reset` | covered |
| `updateMe`, `deleteMe`, `removeMyAuth`, `logout` | `User.*` | `PATCH`, `DELETE /api/users/me`, `DELETE /api/users/me/auths/:sub`, `POST /api/auth/logout` | covered |
| `enableMFA`, `disableMFA`, `regenerateMFARecoveryCode` | `User.EnableMFA`, `DisableMFA`, `RegenerateMFARecoveryCode` | none | gap, this change |
| `createWorkspace`, `updateWorkspace`, `deleteWorkspace` | `Workspace.Create`, `Update`, `Remove` | `POST /api/workspaces`, `PATCH`, `DELETE /api/workspaces/:id` | covered |
| `addUsersToWorkspace`, `updateUserOfWorkspace`, `removeUserFromWorkspace` | `Workspace.AddUserMember`, `UpdateUserMember`, `RemoveUserMember` | `POST /api/workspaces/:id/members`, `PATCH`, `DELETE /api/workspaces/:id/members/:user_id` | covered, owner refused in both (Open Question 1) |
| `transferWorkspaceOwnership` | `Workspace.TransferOwnership` | `POST /api/workspaces/:id/transfer-ownership` | covered |
| `addIntegrationToWorkspace`, `updateIntegrationOfWorkspace`, `removeIntegrationFromWorkspace` | `Workspace.*Integration*` | `/api/workspaces/:id/integrations(/:integration_id)` | covered |
| `removeMultipleUsersFromWorkspace`, `removeIntegrationsFromWorkspace` | GraphQL stub | `DELETE /api/workspaces/:id/members`, `DELETE /api/workspaces/:id/integrations` | REST ahead |

REST-only routes, with no GraphQL equivalent. `GET /api/users/all`, `GET /api/workspaces/all`, user and workspace `deactivate` and `restore`, `PATCH /api/users/by-sub/:sub`, `PUT /api/users/by-sub/:sub/platform-roles`, the workspace SCIM admin routes under `/api/workspaces/:id/scim`, `PATCH /api/service/workspaces/:id/members/:user_id`, `POST /api/users/sync-sso`, and `skip_owner_membership` on `POST /api/workspaces`. The SCIM protocol itself lives in `internal/adapter/scim`.
