# Resource Role Assignment (Generic Per-Resource Membership)

> **Status: DRAFT.** Lets members and roles be set **per project** — and, more
> generally, per resource of any service — so that services (dashboard,
> visualizer, cms, …) can have project members without each building its own
> membership system, and **without reearth-accounts learning what a project
> is**. The first to use it is Dashboard projects. In code, a role assignment is
> represented by types and tables named `...Binding` / `resource_bindings`.
>
> Japanese version: [resource-role-binding.ja.md](resource-role-binding.ja.md)

## Document Signature

|           |                |
|-----------|----------------|
| Creator   | soneda-yuya    |
| Leader    | TBD            |
| Task Link | TBD            |
| Developer | TBD            |

## Background / Problem Statement

### Motivation (product context)

- While introducing SSO, we discussed whether users should be able to create
  workspaces freely, and there was a request to make workspace creation
  restrictable via an option. With that restriction, a single workspace would
  have to hold many projects.
- Today members can only be assigned per workspace, so customers manage members
  workspace by workspace — even though the people involved differ from project
  to project. When customers want to share a project across workspaces, they
  currently copy the project into the target workspace.
- Assigning members and permissions per project lets a workspace function as an
  organization. This is a separate feature from SSO / SCIM, but we want to make
  it implementable early so it is ready when workspace creation can be
  restricted.
- The intended model is close to GitHub's: a workspace relates to its projects
  the way a GitHub organization relates to its repositories. Just as GitHub has
  both organization members and repository members, Re:Earth would have both
  workspace members and project members.

### Current situation

- Permissions across Re:Earth are decided by reearth-accounts. Each service
  (dashboard, visualizer, cms, ...) asks accounts "may this user perform this
  action?" before allowing it.
- A user's role is held **per workspace** (plus a few platform-wide roles).
  Joining a workspace gives the same role on every project in that workspace.
- A permission check therefore only knows *which workspace* the request is
  about, not *which project*. There is no way to say "user A can edit project P
  but not project Q" within the same workspace.

### Problem

Products need **project-level membership**: "user A is an editor of project P,
user B (same workspace) cannot see P". Today the finest granularity accounts
can express is the workspace.

If each service implements project membership on its own:

1. Invite / role-change / remove APIs and UI are duplicated per service.
2. Each service must react to **workspace membership changes** (user removed from
   workspace, workspace deleted) to clean up its project members. Only accounts
   owns that information, so every service needs an event/sync path, and a miss
   leaves a user with access to a project in a workspace they have left.
3. "A project member must be a member of the workspace" cannot be validated
   locally by the service without calling accounts anyway.
4. Permission rules end up scattered across every service, instead of being
   decided in one place (accounts).

### Constraint (the reason for this design's shape)

Accounts must **not** take over what a *project* is. Whether a project exists,
its name, visibility, content and lifecycle stay with each service. Accounts
only keeps what it needs to decide permissions: "this project belongs to this
workspace, and this user has this role on it".

## Goals

1. **Members and roles can be set per project.** A workspace member can be given
   a role on a specific project (for example, editor on project P but no access
   to project Q), not only on the whole workspace.
2. **Permission checks look at the specific project.** When a service asks
   accounts "may this user do this?", the answer can take the user's role on
   that particular project into account. The check always uses the workspace
   the project really belongs to, so a caller cannot get access by naming a
   different workspace.
3. **Membership rules stay consistent.** A project always belongs to exactly
   one workspace, and once it has an owner it never ends up with none — even
   when several people change members at the same time.
4. **Leaving a workspace removes project access.** When a user is removed from a
   workspace, their roles on every project in that workspace are removed
   automatically. Services do not need to clean this up themselves.
5. **Accounts stays generic.** Accounts does not know what a "project" is. Any
   service can use the same mechanism for its own resources (for example, a CMS
   model) just by adding a permission policy, with no change to accounts.
6. **Nothing existing breaks.** Current permission checks and policies keep
   working exactly as they do today.

## Non-Goals

1. **Storing project details in accounts.** Names, visibility, owners, dates and
   other information about a project stay in the service that owns it.
2. **Checking that a project really exists.** Accounts trusts the service that
   registers the project. Only services can register projects, not users.
3. **Noticing on its own when a project is deleted.** The service that owns the
   project must tell accounts when it is deleted.
4. **Nested resources and inherited permissions** (for example folder → project
   → layer). These could be added later through permission policies.
5. **Integrations as project members.** In the first version only people (user
   accounts) can be project members, not integrations or API tokens.
6. **Using project properties in permission decisions** (for example "public"
   or "private"). See *Open Questions*; the first version decides by roles only.
7. **Sending invitation emails for project membership.** The email mentions the
   project's name, which accounts does not know, so the service sends it.

## Functional Requirements

1. **Permission checks stay fast.** Checks that do not name a project cost
   exactly the same as today. Checks that name a project add only one small,
   quick lookup (which workspace the project belongs to); the user's project
   roles are read together with the data accounts already loads.
2. **Names and IDs have size limits.** The resource type name and the resource
   ID have a maximum length and a fixed set of allowed characters, so stored
   data stays small and predictable.
3. **Designed for realistic scale.** Up to about 1,000 project role assignments
   per user, and up to about 1,000 members per project.
4. **Listing a project's members is quick.** It returns only who the members are
   and their roles; names and other details come from the service.
5. **Works with every database accounts supports.** Membership changes behave
   the same whichever database a deployment uses.

Technical details: see *Solution Options → Option 1 → Detailed requirements*.

## Solution Options

This section describes the system design in detail and is intended for readers
interested in the implementation; the other sections can be read without it.

### Option 1 (Recommended): Resource registration + role assignments embedded in `Permittable`

Two pieces of state, both opaque to accounts:

| State | Purpose |
|---|---|
| **Resource registration** `(resourceKind, resourceID) → workspaceID, version` | Authoritative workspace scope of the resource; proof the owning service registered it; per-resource serialization point for invariants |
| **Resource role** on `Permittable` `(workspaceID, resourceKind, resourceID, roleID)` | The user's role on that resource; read together with the permittable for zero-read lookup |

#### Domain

```go
// server/pkg/resource/registration.go (new package, no service-specific code)
type Registration struct {
    createdAt   time.Time
    kind        Kind   // validated per FR-3
    resourceID  ID     // validated per FR-3
    version     int64  // bumped by every membership mutation of this resource
    workspaceID workspace.ID
}

// server/pkg/permittable/permittable.go
type Permittable struct {
    id             ID
    resourceRoles  []ResourceRole // NEW
    roleIDs        []role.ID
    updatedAt      time.Time
    userID         user.ID
    workspaceRoles []WorkspaceRole
}

type ResourceRole struct {
    resourceID   resource.ID
    resourceKind resource.Kind
    roleID       role.ID
    workspaceID  workspace.ID // copied from the registration
}
```

`ResourceRole(kind, id)` returns the user's role on a resource **only if the
user still holds a workspace role in `workspaceID`** (read-time guard, see
*Concurrency*). One role per `(kind, id)` per user. No `switch` on `kind`
anywhere in accounts.

Roles reuse the existing `roles` table (`owner` / `maintainer` / `writer` /
`reader`). Their meaning per resource kind is defined solely by that kind's
Cerbos policy.

#### Storage

**PostgreSQL** — migration `0005_resource_role_binding.up.sql`:

```sql
CREATE TABLE resource_registrations (
    resource_kind text NOT NULL CHECK (octet_length(resource_kind) <= 64),
    resource_id   text NOT NULL CHECK (octet_length(resource_id) <= 128),
    workspace_id  text NOT NULL,
    version       bigint NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (resource_kind, resource_id)
);
CREATE INDEX resource_registrations_workspace_idx
    ON resource_registrations (workspace_id);

CREATE TABLE permittable_resource_roles (
    permittable_id text NOT NULL REFERENCES permittables(id) ON DELETE CASCADE,
    resource_kind  text NOT NULL,
    resource_id    text NOT NULL,
    workspace_id   text NOT NULL,
    role_id        text NOT NULL,
    PRIMARY KEY (permittable_id, resource_kind, resource_id),
    FOREIGN KEY (resource_kind, resource_id)
        REFERENCES resource_registrations (resource_kind, resource_id) ON DELETE CASCADE
);
CREATE INDEX permittable_resource_roles_resource_idx
    ON permittable_resource_roles (resource_kind, resource_id);
CREATE INDEX permittable_resource_roles_workspace_idx
    ON permittable_resource_roles (workspace_id, permittable_id);
```

```sql
-- 0005_resource_role_binding.down.sql
DROP TABLE IF EXISTS permittable_resource_roles;
DROP TABLE IF EXISTS resource_registrations;
```

**MongoDB**

- New collection `resource_registrations` with a unique index on
  `{ resource_kind: 1, resource_id: 1 }` and an index on `{ workspace_id: 1 }`
  (schema registered via `mongoschemagen`, migration added per
  `server/CLAUDE.md`).
- New optional array `resource_roles` on the permittable document, with indexes
  `{ "resource_roles.resource_kind": 1, "resource_roles.resource_id": 1 }` and
  `{ "resource_roles.workspace_id": 1 }`.

#### Repository changes (targeted writes, never snapshot writes)

The existing snapshot `Save` / `SaveMany` must not be able to overwrite or
resurrect resource roles:

- **Mongo** `Permittable.Save` switches from document replacement to
  `$set` of `role_ids`, `workspace_roles`, `updated_at` only. `resource_roles`
  is written exclusively by the targeted operations below.
- **Postgres** `Permittable.Save` already only rewrites
  `permittable_workspace_roles`; it is left untouched for resource roles.
- **Postgres** `hydrate` is replaced by a single query returning the base row
  plus `json_agg` sub-selects of workspace roles and resource roles (FR-2).

New repository interfaces:

```go
// resource.RegistrationRepo
FindByKey(ctx, kind, id) (*resource.Registration, error)
Create(ctx, r resource.Registration) error              // ErrAlreadyExists on duplicate
LockForUpdate(ctx, kind, id) (*resource.Registration, error) // must run inside a transaction
Remove(ctx, kind, id) error                              // also removes all role assignments

// permittable.Repo (additions)
FindResourceMembers(ctx, kind, id) ([]permittable.ResourceMember, error) // projection: userID + roleID
CountResourceOwners(ctx, kind, id, ownerRoleID) (int, error)
UpsertResourceRole(ctx, userID, r permittable.ResourceRole) error
DeleteResourceRole(ctx, userID, kind, id) error
DeleteResourceRolesInWorkspace(ctx, workspaceID, userIDs) error
```

| Operation | Mongo | Postgres |
|---|---|---|
| `UpsertResourceRole` | single-document update: `$pull` the `(kind,id)` element then `$push` the new one (one `updateOne` with an aggregation pipeline) | `INSERT … ON CONFLICT (permittable_id, resource_kind, resource_id) DO UPDATE SET role_id = …` |
| `DeleteResourceRole` | `$pull` | `DELETE … WHERE permittable_id=$1 AND resource_kind=$2 AND resource_id=$3` |
| `DeleteResourceRolesInWorkspace` | `updateMany` + `$pull {workspace_id}` | `DELETE … WHERE workspace_id=$1 AND permittable_id = ANY($2)` |
| `FindResourceMembers` | aggregation `$match` → `$unwind` → `$match` → `$project {user_id, role_id}` | `SELECT p.user_id, r.role_id FROM permittable_resource_roles r JOIN permittables p …` |
| `LockForUpdate` | in a transaction: `findOneAndUpdate {$inc: {version: 1}}` (concurrent transactions on the same registration hit a write conflict and are retried) | `SELECT … FOR UPDATE` |

#### Authorization check

`CheckPermissionInput` gains one optional field:

```graphql
input CheckPermissionInput {
  service: String!
  resource: String!
  action: String!
  workspaceAlias: String
  resourceId: String   # NEW
}
```

When `resourceId` is **absent**, behavior is unchanged.

When `resourceId` is **present**:

1. Load the registration `(service:resource, resourceId)`. **Not found → deny.**
2. If `workspaceAlias` is given and does not resolve to the registration's
   `workspaceID` → **deny**.
3. Principal roles = global roles + the user's workspace roles **in the
   registration's `workspaceID`** (the caller cannot choose the workspace).
4. Principal attribute `resourceRole` = the user's assigned role on this
   resource, or `""` if none (or if the read-time guard drops it).
5. The Cerbos resource ID is the real `resourceId`.

Policies stay in each service's `definition.go`, using the conditions the
reearthx generator already supports. Example for `dashboard:project`:

```go
// dashboard: definition.go
resourceRole := func(expr string) *generator.Condition { return generator.SimpleExpr(expr) }
any := []string{"*"} // any principal role; the condition does the gating

builder.AddResource("project", []generator.ActionDefinition{
    // workspace admins keep full access to every project in their workspace
    generator.NewActionDefinition("read", []string{"owner", "maintainer"}),
    generator.NewActionDefinition("edit", []string{"owner", "maintainer"}),
    generator.NewActionDefinition("delete", []string{"owner", "maintainer"}),
    generator.NewActionDefinition("manage_members", []string{"owner", "maintainer"}),
    // project members
    generator.NewActionDefinitionWithCondition("read", any, resourceRole(`P.attr.resourceRole != ""`)),
    generator.NewActionDefinitionWithCondition("edit", any, resourceRole(`P.attr.resourceRole in ["owner", "writer"]`)),
    generator.NewActionDefinitionWithCondition("delete", any, resourceRole(`P.attr.resourceRole == "owner"`)),
    generator.NewActionDefinitionWithCondition("manage_members", any, resourceRole(`P.attr.resourceRole == "owner"`)),
})
```

Because resource roles arrive as a principal **attribute**, not as principal
roles, a project `owner` is never mistaken for a workspace `owner`. Whether
workspace roles *add to* or are *restricted by* project membership is a
per-product policy decision.

#### Membership management API

**Lifecycle (M2M only)** — REST service routes, authenticated **only** by the
M2M API key (a user JWT is rejected). Registration is how accounts learns that a
resource exists and which workspace it belongs to; restricting it to services
removes any way for a user to claim an unregistered ID.

```
POST   /api/service/resources
       { workspaceId, resourceKind, resourceId, ownerUserId? }
       → creates the registration and, if ownerUserId is given, an `owner`
         role assignment, in one transaction. ownerUserId must be a workspace member.
         409 if already registered with a different workspace; idempotent
         (200) if registered with the same workspace.
DELETE /api/service/resources/{resourceKind}/{resourceId}
       → removes the registration and every role assignment on the resource.
```

**Membership (user JWT, GraphQL)**:

```graphql
input SetResourceMemberInput {
  resourceKind: String!
  resourceId: String!
  userId: ID!
  role: Role!
}
input RemoveResourceMemberInput {
  resourceKind: String!
  resourceId: String!
  userId: ID!
}

type ResourceMember { userId: ID!, role: Role! }
type ResourceBinding { workspaceId: ID!, resourceKind: String!, resourceId: String!, role: Role! }

extend type Query {
  resourceMembers(resourceKind: String!, resourceId: String!): [ResourceMember!]!
  # IDs only; the owning service hydrates names etc.
  myResourceBindings(resourceKind: String!, workspaceId: ID): [ResourceBinding!]!
}

extend type Mutation {
  setResourceMember(input: SetResourceMemberInput!): ResourceMember
  removeResourceMember(input: RemoveResourceMemberInput!): Boolean!
}
```

Guards (generic, no kind-specific code). Every mutation runs in one transaction
that starts with `LockForUpdate` on the registration:

| Operation | Guard (evaluated after the lock) |
|---|---|
| `setResourceMember` | registration exists; target user is a member of the registration's workspace; Cerbos allows `manage_members` on `(kind, id)` for the operator; if the change downgrades the last `owner`, reject |
| `removeResourceMember` | Cerbos `manage_members`, or operator == target (self-leave); if it removes the last `owner`, reject |
| `resourceMembers` | Cerbos `read` on `(kind, id)`; result filtered by the read-time guard |
| `myResourceBindings` | operator's own role assignments only, filtered by the read-time guard |

**Last-owner rule:** a membership mutation that would take the owner count of a
resource from ≥ 1 to 0 is rejected. A resource registered without an owner (e.g.
backfill) is managed by workspace roles until an owner is set. The unregister
endpoint is exempt.

#### Concurrency

| Race | Mitigation |
|---|---|
| Two writers on different users of the same resource both pass the last-owner check | Both must take `LockForUpdate` on the same registration; the second waits (Postgres) or conflicts and retries (Mongo). The owner count is read after the lock. |
| Two services register the same `(kind, id)` | Primary key / unique index; the loser gets `ErrAlreadyExists`. |
| `setResourceMember` passes the workspace-membership check, then the user is removed from the workspace and the cascade runs before the role assignment is written | **Read-time guard**: a role assignment is honored only if the user currently holds a workspace role in the role assignment's workspace. The permittable is already loaded, so this costs no read. The orphaned row is harmless and is removed the next time the cascade or the unregister endpoint touches it. |
| A stale snapshot `Save` from the workspace flow overwrites a newer role assignment (or vice versa) | Snapshot saves never write `resource_roles`; role-assignment writes never write `workspace_roles` (see *Repository changes*). |

#### Lifecycle and cascades

| Event | Who acts | Effect on role assignments |
|---|---|---|
| Resource created | Service (M2M) | `POST /api/service/resources` with `ownerUserId` = creator |
| Resource deleted | Service (M2M) | `DELETE /api/service/resources/{kind}/{id}`. A missed call leaves an orphan registration — harmless (the resource no longer exists in the service), cleaned by an optional sweeper later |
| Existing resources (pre-feature) | Service (M2M) | One-off backfill via the same endpoint (idempotent), before the service starts sending `resourceId` |
| User removed from workspace | Accounts | `bulkRemovePermittable` also calls `DeleteResourceRolesInWorkspace` |
| Workspace deleted (`Workspace.Remove`) | Accounts | Same path via `bulkRemovePermittable`; registrations of the workspace are removed too |
| Workspace deactivated / restored | Accounts | No change (mirrors workspace roles; data kept for restore) |
| User deleted (`DeleteMe`) | Accounts | Today `DeleteMe` does not delete the permittable. Role assignments are unreachable once the user is gone; deleting the permittable on `DeleteMe` is proposed as a follow-up fix (independent of this feature) |

**Pros**

- Workspace scope of a resource is authoritative and server-side, so a caller
  cannot borrow roles from another workspace.
- Only M2M-authenticated services can make a resource exist in accounts, so
  users cannot claim IDs.
- Resource-wide invariants are serialized on one record per resource on both
  backends.
- Role-assignment lookup stays inside the already-loaded permittable; the only new read
  is a primary-key lookup of the registration.
- Accounts remains domain-agnostic: opaque kind/ID, semantics in policies.

**Cons**

- One extra point read for `checkPermission` calls that pass `resourceId`.
- Services must register resources (including a backfill) and unregister on delete.
- Mongo `Save` semantics change from replace to `$set` (touches existing code).

#### Detailed requirements

The technical form of *Functional Requirements*. The items are FR-1 to FR-6 in
order; other parts of this section refer to them by those numbers.

1. `checkPermission` without `resourceId`: **no change** in reads.
   With `resourceId`: **one additional indexed point read** (the resource
   registration, by primary key). The role assignment itself is read from the
   already-loaded permittable.
2. On PostgreSQL, `Permittable.FindByUserID` loads the base row, workspace roles
   and resource roles in **one round-trip** (aggregated sub-selects), so adding
   resource roles does not add a query to `checkPermission`.
3. Field limits, enforced in the domain layer and as DB constraints:
   - `resourceKind`: ≤ 64 bytes, `^[a-z0-9-]{1,32}:[a-z0-9_-]{1,31}$`
   - `resourceID`: ≤ 128 bytes, `^[A-Za-z0-9_.:-]{1,128}$`
4. Expected scale: ≤ ~1,000 role assignments per user (≤ ~300 B each with the limits
   above, i.e. ≤ ~300 KB per Mongo document, well under 16 MB) and ≤ ~1,000
   members per resource.
5. Listing members of one resource is index-backed and returns only
   `(userId, role)` for that resource — it never hydrates whole aggregates.
6. All mutations work on both MongoDB and PostgreSQL backends and in the
   `memory` repo.

Scope notes (technical form of *Non-Goals*):

- Resource existence is not validated. Accounts trusts the
  **M2M-authenticated** service that registers a resource, and relies on that
  service to call the unregister endpoint when the resource is deleted (see
  *Lifecycle and cascades*).
- v1 role assignments are for **user principals only**; integration (M2M)
  principals cannot hold resource roles.
- v1 `checkPermission` passes no resource attributes (e.g. `visibility`) to
  Cerbos; policies use roles only (see *Technical notes on open questions*).

#### Request flows (sequence diagrams)

##### Permission check with a resource ID

```mermaid
sequenceDiagram
    participant S as Service (e.g. dashboard)
    participant A as reearth-accounts
    participant DB as Mongo/Postgres
    participant C as Cerbos

    S->>A: checkPermission(service, resource, action, workspaceAlias?, resourceId)
    A->>DB: Permittable.FindByUserID(user)  (existing read, one round-trip)
    A->>DB: Registration.FindByKey(kind, resourceId)  (new point read)
    alt not registered, or alias ≠ registration.workspace
        A-->>S: { allowed: false }
    else
        A->>A: roles = global + workspace roles in registration.workspace<br/>attr.resourceRole = guarded assigned role or ""
        A->>C: CheckResources(principal{roles, attr}, resource{kind, resourceId}, action)
        C-->>A: ALLOW / DENY
        A-->>S: { allowed }
    end
```

##### Project creation and member invite

```mermaid
sequenceDiagram
    participant U as User
    participant S as Service
    participant A as reearth-accounts

    U->>S: createProject
    S->>S: persist project P
    S->>A: POST /api/service/resources {ws, "dashboard:project", P, owner=U} (M2M key)
    A->>A: U ∈ ws? → create registration + owner role assignment (one tx)
    A-->>S: 201
    U->>S: invite B as writer
    S->>A: setResourceMember(kind, P, B, writer) (U's JWT)
    A->>A: lock registration → B ∈ ws? Cerbos manage_members for U? → upsert
    A-->>S: ok
```

##### Workspace member removal cascade

```mermaid
flowchart LR
    R[removeUserFromWorkspace / deleteWorkspace] --> B[bulkRemovePermittable]
    B --> W[RemoveWorkspaceRole ws → snapshot Save]
    B --> RR[DeleteResourceRolesInWorkspace ws, users → targeted delete]
```

#### Technical notes: impact, tests, deployment, rollback, monitoring

##### Impact

1. **Permittable document/row size** grows by ≤ ~300 B per role assignment.
2. **Mongo `Permittable.Save`** changes from replace to `$set`; existing tests
   for permittable persistence must still pass, and a new test verifies that
   `Save` leaves `resource_roles` untouched.
3. **Cerbos resource ID change** when `resourceId` is passed: only callers that
   opt in are affected.
4. **Workspace member removal** issues one additional targeted delete.
5. **Policy authoring**: role names are shared between workspace and resource
   scopes; policies must gate resource access on `P.attr.resourceRole`, not on
   principal roles.
6. **gqlclient** (`server/pkg/gqlclient`) consumers need a version bump to use
   the new fields; old clients are unaffected.

##### Tests

1. **Unit** (`pkg/resource`, `pkg/permittable`): kind/ID validation and length
   limits; one role per resource; read-time guard drops role assignments without a
   matching workspace role.
2. **Interactor** (`internal/usecase/interactor`):
   - `CheckPermission`: no `resourceId` → unchanged roles (regression);
     unregistered resource → deny; alias of another workspace → deny; workspace
     roles taken from the registration, not the alias; `resourceRole` set/empty.
   - Register: M2M only (JWT rejected); owner must be a workspace member;
     duplicate with same workspace is idempotent, different workspace is 409.
   - `setResourceMember` / `removeResourceMember`: non-member target rejected;
     denied without `manage_members`; self-leave allowed; last-owner protection.
   - Cascades: `RemoveUserMember` / `RemoveMultipleUserMembers` / `Remove`.
3. **Concurrency** (`make test-integration`, both backends): two goroutines
   removing the last two owners concurrently → exactly one succeeds; concurrent
   registrations of the same key → one succeeds; stale snapshot `Save` after
   `UpsertResourceRole` keeps the role assignment.
4. **Repository**: Mongo, Postgres and memory implementations of every new
   operation; Postgres `FindByUserID` issues a single query.
5. **Migration**: `0005` up/down on an existing DB; existing permittables load
   with empty `resourceRoles`.
6. **E2E** (`server/e2e`): a `dashboard:project` test policy generated with the
   conditions above; verify member / non-member / workspace-owner /
   other-workspace outcomes.

##### Deployment

1. Backward compatible? **Yes.** New optional input field, new collection/tables,
   new routes and GraphQL fields. Behavior is unchanged when `resourceId` is absent.
2. Partial deployment? **Yes.** Order:
   1. Deploy accounts.
   2. Each service backfills registrations for existing resources.
   3. Each service starts registering on create / unregistering on delete.
   4. Each service deploys policies using `P.attr.resourceRole` and starts
      sending `resourceId`. Before step 4 the attribute is absent and conditions
      never match (fail-closed).
3. Notify: dashboard / visualizer / cms owners; whoever maintains Cerbos policies.
4. Config changes: none (reuses the existing M2M API key).
5. DDL: Postgres migration `0005` runs automatically at startup (golang-migrate).
   Mongo: collection schema + indexes via the startup migration; no backfill in accounts.

##### Rollback

1. Notify the cause in the team Slack channel.
2. Revert consumer services / policies that rely on `resourceId` / `resourceRole` first.
3. Roll back the accounts image. The old binary ignores the new collection,
   tables and `resource_roles` field, so **no data rollback is required**.
   Note: the old Mongo `Save` replaces the whole document and will drop
   `resource_roles` of any permittable it writes — acceptable during rollback,
   since the feature is disabled; services re-run the backfill on roll-forward.
4. Only if the schema itself must be removed: run `0005` down (drops
   registrations and role assignments — take a dump first).

##### Post-deployment checks and monitoring

- Checklist
  - Create a project in dashboard staging; verify the registration and the
    creator's `owner` role assignment.
  - Invite a workspace member; verify access. Remove them from the workspace;
    verify project access is lost.
  - Call `checkPermission` with the alias of another workspace; verify deny.
- Metrics
  - `checkPermission` p50/p95 latency with and without `resourceId`.
  - Error rate of register / unregister / `setResourceMember`, including
    Mongo transaction retries.
  - Count of role assignments per permittable (max / p99) to watch FR-4.
- Alerting
  - `checkPermission` error rate: Warning > 1%, Danger ≥ 5%.
  - `checkPermission` p95 latency: Warning > +20% vs baseline.

#### Technical notes on open questions

1. **Resource attributes in `checkPermission`:** proposal — an optional
   `attributes: JSON` on `CheckPermissionInput`, passed through to the Cerbos
   resource (e.g. `visibility`).
2. **Batch checks:** a `checkPermissions` query over many `resourceId`s for list
   screens. Cerbos `CheckResources` already supports multiple resources.
3. **Per-service M2M keys:** v1 accepts the existing shared M2M API key for
   register/unregister (`POST` / `DELETE /api/service/resources`). Option: scope
   each key to the kinds of its own service (`dashboard:*`).
4. **Orphan sweeper:** would remove registrations of resources deleted without
   calling the unregister endpoint.
5. **Integration members:** M2M tokens as resource principals (possibly needed
   by cms).

### Option 2: Role assignments in their own collection/table (no embedding)

Same registration and API, but role assignments live in a standalone
`resource_bindings (kind, id, userID, role)` collection/table instead of on the
permittable.

**Pros:** no permittable growth; member listing is a plain index scan; snapshot
`Save` semantics do not need to change.

**Cons:** `checkPermission` needs a second extra read (the role assignment) on top of
the registration; workspace cascades touch two stores.

**Decision:** Option 1. Revisit Option 2 if per-user role-assignment counts approach
FR-4 or if the Mongo `Save` change proves risky.

### Alternative considered and rejected: membership in each service

Rejected because of the duplication and cascade problems in *Problem* (1)–(4).
The variant "services keep members but pass the role to Cerbos as an attribute"
solves only (4).

## Design

**For each project, accounts keeps only two facts: which workspace it belongs
to, and who holds which role on it.** Everything else about the project stays in
the service.

### Checking a permission on a project

1. A service asks accounts whether a user may do something (for example, edit)
   on a specific project.
2. Accounts looks up which workspace the project belongs to. If the service
   never registered the project, or the request names a different workspace,
   the answer is "no".
3. Accounts combines the user's role in that workspace with the user's role on
   the project (if any) and applies the service's permission policy.
4. A project role only counts while the user is still a member of the project's
   workspace.

### Creating a project and inviting members

1. A user creates a project in a service. The service saves it and registers it
   with accounts, naming the creator as its owner (the creator must be a
   workspace member).
2. The owner invites another workspace member with a role (for example,
   writer). Accounts checks that the invited person is in the workspace and that
   the inviter is allowed to manage members, then saves the role.
3. Member changes on the same project are processed one at a time, so the last
   owner can never be removed, even when two people act at once. Members can
   leave a project themselves, except the last owner.
4. A project registered without an owner (for example, one that existed before
   this feature) is managed through workspace roles until an owner is set.

### Removing someone from a workspace

When a user is removed from a workspace, or the workspace is deleted, accounts
removes their workspace role and all their project roles in that workspace in
the same step. When a workspace is deleted, its project registrations are
removed too. Deactivating a workspace removes nothing, so it can be restored.

### Example: Dashboard projects

| Who | What they can do |
|---|---|
| Workspace owner / maintainer | View, edit and delete every project in the workspace, and manage its members |
| Project owner | View, edit and delete the project, and manage its members |
| Project writer | View and edit the project |
| Other project roles (maintainer / reader) | View the project |
| Workspace member with no role on the project | No access to the project (unless they are a workspace owner / maintainer) |

Whether workspace roles *add to* project membership or are *restricted by* it is
decided per product in its permission policy.

Technical details: see *Solution Options → Option 1 → Request flows (sequence
diagrams)*.

## Potential Impact

1. **Slightly more stored data per user.** Each project role adds a few hundred
   bytes to the user's permission data.
2. **A change in how accounts saves permission data.** In one of the databases
   accounts supports, the way permission data is saved changes so that
   workspace changes and project role changes never overwrite each other. This
   is covered by the existing tests plus a new one.
3. **Only services that opt in are affected.** Permission checks change only for
   services that start naming a specific project; everything else behaves as
   today.
4. **Removing a workspace member does a little more work**, because their
   project roles in that workspace are removed in the same step.
5. **Role names are shared between workspaces and projects.** "Owner",
   "maintainer", "writer" and "reader" exist at both levels. Whoever writes a
   service's permission policy must treat "project owner" and "workspace owner"
   as different things.
6. **Only services that use the feature need to update.** Services that use the
   new features must update the client library they use to talk to accounts;
   services that don't update keep working unchanged.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Test Plan

1. **Basic rules.** Resource type names and IDs are validated and size-limited;
   a user has at most one role per project; a project role is ignored once the
   user is no longer in the project's workspace.
2. **Permission checks.**
   - Checks that do not name a project give exactly the same results as today.
   - A project that was never registered → denied.
   - Naming a workspace other than the project's own → denied.
   - The user's workspace role is always taken from the project's real
     workspace, not from what the caller says.
   - The user's project role is applied when they have one, and not otherwise.
3. **Registering projects.** Only services can register (a user's login is
   rejected); the owner must be a workspace member; registering the same
   project again in the same workspace is harmless, but in a different
   workspace it is rejected.
4. **Managing members.** Someone outside the workspace cannot be added; people
   without permission to manage members are refused; members can leave a
   project themselves; the last owner cannot be removed or downgraded.
5. **Cleanup.** Removing one or several users from a workspace, or deleting the
   workspace, removes their project roles.
6. **Simultaneous changes.** Two people removing the last two owners at the same
   time → exactly one succeeds. Two services registering the same project at the
   same time → exactly one succeeds. A workspace change saved at the same time
   as a project role change does not wipe out the project role.
7. **Every supported database**, including the database upgrade and its undo on
   existing data, and existing users loading correctly with no project roles.
8. **End to end.** With a sample Dashboard project policy, check the results for
   a project member, a non-member, a workspace owner, and a user from another
   workspace.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Deployment Plan

1. **Backward compatible? Yes.** Everything is added alongside what exists.
   Services that don't name a project in permission checks see no change.
2. **Can it be rolled out gradually? Yes.** Order:
   1. Deploy accounts.
   2. Each service registers its existing projects with accounts (a one-off
      backfill).
   3. Each service starts registering new projects when they are created and
      telling accounts when they are deleted.
   4. Each service turns on its project-level permission rules and starts naming
      the project in permission checks. Until this step, project roles have no
      effect, so nobody gains access by mistake (fail-closed).
3. **Who to notify:** the dashboard / visualizer / cms owners, and whoever
   maintains the permission policies.
4. **Configuration changes:** none.
5. **Database changes:** applied automatically when accounts starts. Accounts
   itself does not need to convert any existing data.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Rollback Plan

1. Share the cause in the team Slack channel.
2. First, turn off project-level permission rules in the services that use them.
3. Roll accounts back to the previous version. The previous version simply
   ignores the new data, so **no data rollback is required**.
   Note: while the previous version runs, it may drop the project roles of users
   whose permission data it saves. This is acceptable because the feature is off
   at that point; services re-run the backfill when the feature is rolled out
   again.
4. Only if the new data structures must be removed completely: run the database
   downgrade. It deletes all project registrations and project roles, so take a
   backup first.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Post Deployment

- Checklist
  - Create a project in Dashboard staging; check that it is registered and that
    the creator is its owner.
  - Invite a workspace member to the project; check that they can access it.
    Remove them from the workspace; check that they lose access to the project.
  - Ask for a permission on the project while naming a different workspace;
    check that it is denied.
- Metrics
  - How long permission checks take (typical and slowest 5%), with and without a
    project named.
  - Error rate of registering / unregistering projects and changing members,
    including automatic retries.
  - Number of project roles per user (maximum and top 1%), to watch the ~1,000
    per user assumption.
- Alerting
  - Permission check error rate: Warning above 1%, Danger at 5% or more.
  - Permission check response time (slowest 5%): Warning when more than 20%
    slower than the usual baseline.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Open Questions

1. **Should permission decisions use project properties?** Rules like "private
   projects are visible only to their members, public ones to the whole
   workspace" need to know whether a project is public. Deferred until a
   product needs it.
2. **Should list screens be able to check many projects at once?** A screen that
   lists projects needs a permission answer for each one; one combined check
   would be faster.
3. **Should each service's credentials be limited to its own resources?** In the
   first version, all services register projects with the same shared service
   credential. Should each service be able to register only its own kinds of
   resources (for example, Dashboard only Dashboard projects)?
4. **Should we build an automatic cleanup** for projects that a service deleted
   without telling accounts, or rely on services always telling accounts?
5. **Do integrations need to be project members** (for example, in cms)?
6. **Where are members managed in the UI?** Should project membership be managed
   only in the Dashboard, or also within each service's own screens
   (visualizer / cms / flow)? This does not change accounts (every screen uses
   the same membership functions), but it affects which services need UI work
   and is to be decided together with the designer.

Technical details: see *Solution Options → Option 1 → Technical notes on open
questions*.

## Reviewed by

- Technical Architect: [name]
- Technical Leader: [name]
- Peers: [name]
