# Resource Role Assignment (Generic Per-Resource Membership)

> **Status: DRAFT.** Lets members and roles be set **per project** — and, more
> generally, per resource of any service — so that services (dashboard,
> visualizer, cms, …) can have project members without each building its own
> membership system, and **without reearth-accounts learning what a project
> is**. The first to use it is Dashboard projects. In code, a role assignment is
> represented by types and tables named `...Binding` / `resource_bindings`.
>
> Japanese version: [resource-role-binding.ja.md](resource-role-binding.ja.md)

## How to read this document

Sections from *Goals* onward include technical content. Please review in two
steps:

1. **Concept first.** Read *Background / Problem Statement* and check whether
   the concept feels right: does anything seem off, contradictory, or confusing
   for customers?
2. **Then technical adoption.** Read *Goals* and the sections after it, and
   check how difficult it would be to adopt this in each service (dashboard,
   visualizer, cms, flow) and what concerns you see. *Solution Options* holds
   the system design details and is for readers interested in the
   implementation.

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
  Why an organization using SSO wants to restrict workspace creation:
  - **Company data stays under the company's control.** If anyone can create a
    workspace, employees can put company projects and data in workspaces that
    the organization's administrators cannot see or manage.
  - **Leaving the company really cuts off access.** SSO / SCIM removes a
    departing employee from the organization's workspaces, but workspaces they
    created themselves stay outside that control, along with the data in them.
  - **Members are managed in one place.** Administrators want to manage who can
    access what in the organization's workspace, instead of chasing many
    workspaces created by individual employees.
- But restricting workspace creation raises a new problem. Today every member
  of a workspace sees every project in it, so once all employees work in one
  organization workspace, there is no way to keep teams from seeing each
  other's projects. The organization needs to decide, per project, who can see
  it — while everyone stays a member of the workspace, managed by SSO / SCIM.
- Today members can only be assigned per workspace, so customers manage members
  workspace by workspace — even though the people involved differ from project
  to project. When customers want to share a project across workspaces, they
  currently copy the project into the target workspace.
- Assigning members and permissions per project lets a workspace function as an
  organization. This is a separate feature from SSO / SCIM, but we want to make
  it implementable early so it is ready when workspace creation can be
  restricted.
- The intended model is close to GitHub's. A workspace relates to its projects
  the way a GitHub organization relates to its repositories:

  | GitHub | Re:Earth |
  |---|---|
  | Organization member | Workspace member |
  | Organization owner | Workspace owner / maintainer |
  | Organization base permission (applies to all members on every repository) | Workspace **base permission** |
  | Repository role | Project role |

  With the base permission set to "None", members see only the projects they
  were given a role on — this is how teams in one workspace are kept apart, and
  how a partner company's staff can be given just one project.
- Two deliberate differences from GitHub:
  - GitHub also has *outside collaborators* (people with access to a repository
    without being organization members). Re:Earth deliberately does not: **every
    person with access is a workspace member**, so all of them are managed in
    one place — SSO / SCIM, seats, the member list, and removal.
  - GitHub members have no per-person writer / reader distinction. In Re:Earth
    the workspace roles writer / reader stay and keep governing workspace-level
    actions (for example, creating projects); access to projects comes from the
    base permission plus project roles.

### Current situation

- Permissions across Re:Earth are decided by reearth-accounts. Each service
  (dashboard, visualizer, cms, ...) asks accounts "may this user perform this
  action?" before allowing it.
- A user's role is held **per workspace** (plus a few platform-wide roles).
  Joining a workspace gives the same role on every project in that workspace.
- A permission check therefore only knows *which workspace* the request is
  about, not *which project*. There is no way to say "user A can edit project P
  but not project Q" within the same workspace, and adding someone to a
  workspace always shows them every project in it.

### Problem

Products need **project-level membership**: "user A, a reader of the workspace,
is an editor of project P", "in this workspace, members see only the projects
they were added to, so team X cannot see team Y's projects", and "user B from a
partner company is added to the workspace but can work on project P only".
Today the finest granularity accounts can express is the workspace.

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
workspace, and this user has this role on it", plus each workspace's base
permission.

## Goals

1. **Members and roles can be set per project.** A workspace member can be
   given a role on a specific project, not only on the whole workspace. The
   model is:
   - **Workspace owners and maintainers** always have full access to every
     project in the workspace.
   - **Workspace writers and readers** get, on every project, a baseline set by
     the workspace's base permission (Goal 7). By default this is their
     workspace role, exactly as today. A project role can **raise** their access
     on that project (for example, a workspace reader who is a writer on project
     P can edit P), but never lower it.
2. **Permission checks look at the specific project.** When a service asks
   accounts "may this user do this?", the answer can take the user's role on
   that particular project into account. The check always uses the workspace
   the project really belongs to, so a caller cannot get access by naming a
   different workspace.
3. **Membership rules stay consistent.** A project always belongs to exactly
   one workspace, every project member is a member of that workspace, and once
   a project has an owner it never ends up with none — even when several people
   change members at the same time.
4. **Leaving a workspace removes project access.** When a user is removed from a
   workspace, their roles on every project in that workspace are removed
   automatically. Services do not need to clean this up themselves.
5. **Accounts stays generic.** Accounts does not know what a "project" is. Any
   service can use the same mechanism for its own resources (for example, a CMS
   model) just by adding a permission policy, with no change to accounts.
6. **Nothing existing breaks.** Current permission checks and policies keep
   working exactly as they do today. Existing workspace roles do not change,
   and the default base permission reproduces today's behavior.
7. **Each workspace has a base permission** (name tentative), like a GitHub
   organization's base permission. Its values are "Follow workspace role"
   (default — today's behavior), "None", "Reader" and "Writer". It decides what
   writers and readers of the workspace get on every project; owners and
   maintainers always have full access. Workspace owners and maintainers can
   change it, and the change takes effect immediately. "None" plus project roles
   is the way to give someone access to only some projects.
8. **Access to specific projects can be given in one step.** From the UI, an
   admin can add a person directly to a project — adding them to the workspace
   at the same time if needed — or, while adding a person to the workspace,
   choose which projects they get. Either way it is one step that either fully
   succeeds or changes nothing (see *Design → Adding people from the UI*).

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
   or "private"). See *Open Questions*; the first version decides by roles and
   the base permission only.
7. **Sending invitation emails for project membership.** The email mentions the
   project's name, which accounts does not know, so the service sends it.

## Functional Requirements

1. **Permission checks stay fast.** Checks that do not name a project cost
   exactly the same as today. Checks that name a project add only one small,
   quick lookup (which workspace the project belongs to, together with that
   workspace's base permission); the user's project roles are read together
   with the data accounts already loads.
2. **Names and IDs have size limits.** The resource type name and the resource
   ID have a maximum length and a fixed set of allowed characters, so stored
   data stays small and predictable.
3. **Designed for realistic scale.** Up to about 1,000 project role assignments
   per user, and up to about 1,000 members per project.
4. **Listing a project's members is quick.** It returns only who the members are
   and their roles; names and other details come from the service.
5. **Works with every database accounts supports.** Membership changes behave
   the same whichever database a deployment uses.
6. **Services can tell what a person may list.** For a workspace, accounts can
   tell a service whether a person sees every project (and at what level) or
   only the projects where they have a project role, so lists can be filtered
   correctly whatever the base permission.

Technical details: see *Solution Options → Option 1 → Detailed requirements*.

## Solution Options

This section describes the system design in detail and is intended for readers
interested in the implementation; the other sections can be read without it.

### Option 1 (Recommended): Resource registration + role assignments embedded in `Permittable`

Three pieces of state, all opaque to accounts:

| State | Purpose |
|---|---|
| **Resource registration** `(resourceKind, resourceID) → workspaceID, version` | Authoritative workspace scope of the resource; proof the owning service registered it; per-resource serialization point for invariants |
| **Resource role** on `Permittable` `(workspaceID, resourceKind, resourceID, roleID)` | The user's role on that resource; read together with the permittable for zero-read lookup |
| **Workspace resource settings** `workspaceID → basePermission, version` | The workspace's base permission; also the per-workspace lock (serialization point for the one-step add, workspace member add / remove, the seat limit, and base-permission changes) |

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

**Base permission.**

```go
// server/pkg/resource/settings.go
type BasePermission string

const (
    BaseFollowWorkspaceRole BasePermission = "follow_workspace_role" // default; also when no row exists
    BaseNone                BasePermission = "none"
    BaseReader              BasePermission = "reader"
    BaseWriter              BasePermission = "writer"
)

type WorkspaceSettings struct {
    basePermission BasePermission
    version        int64 // bumped by every write and by Lock
    workspaceID    workspace.ID
}

// BaselineRole returns a member's baseline role on every resource of the workspace.
// ok == false means "no baseline" (the member only gets resource roles).
func BaselineRole(workspaceRole role.RoleType, base BasePermission) (r role.RoleType, ok bool)
```

| Workspace role | Follow workspace role | None | Reader | Writer |
|---|---|---|---|---|
| `owner` / `maintainer` | same role | same role | same role | same role |
| `writer` | `writer` | — | `reader` | `writer` |
| `reader` | `reader` | — | `reader` | `writer` |

The setting is stored in accounts, in its own per-workspace record rather than
on the workspace document: the existing workspace flows save the workspace as a
snapshot (Mongo replaces the whole document), so a field there could be
reverted by a concurrent, stale workspace save. A missing record means
"Follow workspace role", so no data migration is needed. Existing workspace
roles are not changed.

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

CREATE TABLE resource_workspace_settings (
    workspace_id    text PRIMARY KEY,
    base_permission text NOT NULL DEFAULT 'follow_workspace_role'
        CHECK (base_permission IN ('follow_workspace_role', 'none', 'reader', 'writer')),
    version         bigint NOT NULL DEFAULT 0
);
```

```sql
-- 0005_resource_role_binding.down.sql
DROP TABLE IF EXISTS resource_workspace_settings;
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
- New collection `resource_workspace_settings` (`_id` = workspace ID,
  `base_permission`, `version`).

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
RemoveInWorkspace(ctx, workspaceID) error                // all registrations + role assignments

// resource.WorkspaceSettingsRepo
Find(ctx, workspaceID) (*resource.WorkspaceSettings, error) // default settings when no row
Lock(ctx, workspaceID) (*resource.WorkspaceSettings, error) // must run inside a transaction; creates the row on first use
SetBasePermission(ctx, workspaceID, base resource.BasePermission) error // after Lock
Remove(ctx, workspaceID) error

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
| `WorkspaceSettings.Lock` | in a transaction: `findOneAndUpdate {_id: ws} {$inc: {version: 1}}, upsert` (write conflict → retry) | `INSERT … ON CONFLICT DO NOTHING` then `SELECT … FOR UPDATE` |
| Registration + settings for `checkPermission` | one aggregation on `resource_registrations`: `$match` by key → `$lookup` `resource_workspace_settings` by `workspace_id` | one query: `SELECT r.*, s.base_permission FROM resource_registrations r LEFT JOIN resource_workspace_settings s USING (workspace_id) WHERE …` |

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

When `resourceId` is **absent**, behavior is unchanged — whatever the base
permission. Principal roles are the actual workspace roles, as today.

When `resourceId` is **present**:

1. Load the registration `(service:resource, resourceId)` together with its
   workspace's settings (one round-trip). **Not found → deny.**
2. If `workspaceAlias` is given and does not resolve to the registration's
   `workspaceID` → **deny**.
3. Principal roles = global roles + the user's **baseline role**
   `BaselineRole(workspace role, basePermission)` **in the registration's
   `workspaceID`** (the caller cannot choose the workspace). With "Follow
   workspace role" the baseline is the workspace role, so the roles are exactly
   what they would be today. When a workspace member has no baseline (`None`
   for a `writer` / `reader`), the evaluation-only role `workspace_member` is
   added instead (never stored; it gives Cerbos a non-empty role list and lets
   a policy recognize the case). A user with no workspace role in that
   workspace gets neither, and so is denied.
4. Principal attributes: `resourceRole` = the user's role on this resource after
   the read-time guard, or `""` if none; `workspaceRole` = the actual workspace
   role (or `""`), for policies that need it.
5. The Cerbos resource ID is the real `resourceId`.

**Default model.** Effective role on a resource = max(baseline role, resource
role). Workspace `owner` / `maintainer` always have a full baseline. For
`writer` / `reader` the baseline comes from the base permission; with `None`
they reach a resource only through a resource role. Each role means the same at
both levels (`owner` / `maintainer`: full control and member management;
`writer`: read and edit; `reader`: read), so a policy implements "max" as the
union of a role-list rule (now evaluated against the baseline role) and a
`resourceRole`-condition rule per action — Cerbos allows if either matches.
Because accounts substitutes the baseline into the principal roles, existing
role-list rules need no change: they automatically follow the base permission
for checks with a `resourceId` and keep using the actual workspace role for
checks without one.

Policies stay in each service's `definition.go`, using the conditions the
reearthx generator already supports. Example for `dashboard:project`:

```go
// dashboard: definition.go
resourceRole := func(expr string) *generator.Condition { return generator.SimpleExpr(expr) }
any := []string{"*"} // any principal role (incl. workspace_member); the condition does the gating

builder.AddResource("project", []generator.ActionDefinition{
    // baseline: with a resourceId these roles are the member's baseline role (workspace role + base permission)
    // ("workspace_member" is never listed here)
    generator.NewActionDefinition("read", []string{"owner", "maintainer", "writer", "reader"}),
    generator.NewActionDefinition("edit", []string{"owner", "maintainer", "writer"}),
    generator.NewActionDefinition("delete", []string{"owner", "maintainer"}),
    generator.NewActionDefinition("manage_members", []string{"owner", "maintainer"}),
    // project roles: add on top of the baseline; the only source of access without a baseline
    generator.NewActionDefinitionWithCondition("read", any, resourceRole(`P.attr.resourceRole != ""`)),
    generator.NewActionDefinitionWithCondition("edit", any, resourceRole(`P.attr.resourceRole in ["owner", "maintainer", "writer"]`)),
    generator.NewActionDefinitionWithCondition("delete", any, resourceRole(`P.attr.resourceRole in ["owner", "maintainer"]`)),
    generator.NewActionDefinitionWithCondition("manage_members", any, resourceRole(`P.attr.resourceRole in ["owner", "maintainer"]`)),
})
```

Because resource roles arrive as a principal **attribute**, not as principal
roles, a project `owner` is never mistaken for a workspace `owner`: a project
role counts only for the resource named in the check. Per-project visibility on
top of the base permission (e.g. a "visible to all members" project) is Open
Question 1.

**Policy note for members without a baseline.** Policies must grant
`workspace_member` nothing except through `P.attr.resourceRole` conditions.
Rules keyed to explicit role lists already exclude it (fail-closed). Rules that
grant on `*` without a `resourceRole` condition must be reviewed per service.
Workspace-level resources (e.g. `dashboard:workspace`, "create project", "list
projects") are checked without a `resourceId`, so they keep using the actual
workspace role: under `None` a workspace writer can still create a project (and
becomes its owner) and may call a list endpoint — **the service must filter the
list** (see *Listing what a user may see*).

**Listing what a user may see.** A list screen cannot call `checkPermission`
for every resource (Open Question 2). Services ask once per workspace:

```graphql
type ResourceAccess {
  workspaceId: ID!
  baselineRole: Role        # null = no baseline: only the bindings below are visible
  bindings: [ResourceBinding!]!
}
extend type Query {
  myResourceAccess(workspaceId: ID!, resourceKind: String!): ResourceAccess!
}
```

If `baselineRole` is set, the user sees every resource of the kind in the
workspace at least at that level; otherwise (a member under `None`) the service
lists only `bindings`.

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

**Membership and settings (user JWT, GraphQL)**:

```graphql
enum BasePermission { FOLLOW_WORKSPACE_ROLE NONE READER WRITER }

extend type Workspace {
  basePermission: BasePermission!   # FOLLOW_WORKSPACE_ROLE when never set
}

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
input ResourceRoleInput {
  resourceKind: String!
  resourceId: String!
  role: Role!
}
input AddWorkspaceMemberWithResourceRolesInput {
  workspaceId: ID!
  userId: ID!
  workspaceRole: Role               # used only if the user is not yet a member; default reader; not owner
  resources: [ResourceRoleInput!]!  # 0..100 entries, distinct (kind, id)
}
input UpdateWorkspaceBasePermissionInput {
  workspaceId: ID!
  basePermission: BasePermission!
}

type ResourceMember { userId: ID!, role: Role! }
type ResourceBinding { workspaceId: ID!, resourceKind: String!, resourceId: String!, role: Role! }
type AddWorkspaceMemberWithResourceRolesPayload {
  addedToWorkspace: Boolean!        # false if the user was already a member
  workspaceRole: Role!              # the user's workspace role after the call
  resources: [ResourceBinding!]!
}

extend type Query {
  resourceMembers(resourceKind: String!, resourceId: String!): [ResourceMember!]!
  # IDs only; the owning service hydrates names etc.
  myResourceBindings(resourceKind: String!, workspaceId: ID): [ResourceBinding!]!
  myResourceAccess(workspaceId: ID!, resourceKind: String!): ResourceAccess!
}

extend type Mutation {
  setResourceMember(input: SetResourceMemberInput!): ResourceMember
  removeResourceMember(input: RemoveResourceMemberInput!): Boolean!
  addWorkspaceMemberWithResourceRoles(
    input: AddWorkspaceMemberWithResourceRolesInput!
  ): AddWorkspaceMemberWithResourceRolesPayload
  updateWorkspaceBasePermission(input: UpdateWorkspaceBasePermissionInput!): Workspace
}
```

The user is identified by user ID, as in `addUsersToWorkspace`; the UI finds
them by name or email with the existing user lookup (`userByNameOrEmail` /
`searchUser`). No new email-based invitation is introduced.

"Workspace admin" below means the operator passes the existing workspace
check used by `addUsersToWorkspace` (`add_member`, i.e. workspace owner or
maintainer).

Guards (generic, no kind-specific code). Every mutation runs in **one
transaction**; locks are taken in a fixed order — workspace lock
(`WorkspaceSettings.Lock`) first where needed, then registrations sorted by
`(kind, id)` — and guards are evaluated after the locks:

| Operation | Locks | Guard |
|---|---|---|
| `setResourceMember` | the registration | registration exists; target user is a member of the registration's workspace (else `ErrNotWorkspaceMember` — use `addWorkspaceMemberWithResourceRoles`); Cerbos allows `manage_members` on `(kind, id)` for the operator; if the change downgrades the last `owner`, reject |
| `removeResourceMember` | the registration | Cerbos `manage_members`, or operator == target (self-leave); if it removes the last `owner`, reject |
| `addWorkspaceMemberWithResourceRoles` | workspace lock, then each registration | every registration exists and belongs to `workspaceId` (else the whole call is rejected); Cerbos allows `manage_members` on each for the operator. If the user is **not yet a member**: operator is a workspace admin (v1, Open Question 7); personal workspace rejected; `workspaceRole` (default `reader`) is not `owner`, as in `addUsersToWorkspace`; the seat limit (`enforceMemberCount`) passes; then `Members().Join` and the permittable workspace role via the existing `bulkUpdatePermittable` path. If the user **is already a member**: workspace membership and role are left unchanged — never downgraded (`workspaceRole` is ignored). Then the last-owner rule per resource and `UpsertResourceRole` for each entry. Any failure rolls everything back |
| `updateWorkspaceBasePermission` | workspace lock | operator passes the existing workspace `manage` check (owner / maintainer); personal workspace rejected (v1, Open Question 9); then `SetBasePermission`. Takes effect on the next `checkPermission` — nothing else is rewritten |
| `resourceMembers` | — | Cerbos `read` on `(kind, id)`; result filtered by the read-time guard |
| `myResourceBindings` | — | operator's own role assignments only, filtered by the read-time guard |
| `myResourceAccess` | — | operator's own access only; `baselineRole` from `BaselineRole(...)`, `bindings` filtered by the read-time guard |
| `Workspace.basePermission` | — | anyone who can read the workspace (`read`) |

**Last-owner rule:** a membership mutation that would take the owner count of a
resource from ≥ 1 to 0 is rejected. A resource registered without an owner (e.g.
backfill) is managed by workspace roles until an owner is set. The unregister
endpoint is exempt.

Removing a user's last resource role (`removeResourceMember`) does **not**
remove them from the workspace (Open Question 8).

**Changes to existing workspace flows** (all take the workspace lock first, so
they serialize with the one-step add):

- `addUsersToWorkspace`: unchanged apart from the lock.
- `removeUserFromWorkspace` / `removeMultipleUsersFromWorkspace`: also delete
  the users' role assignments in the workspace (`DeleteResourceRolesInWorkspace`).
- `deleteWorkspace` (`Workspace.Remove`): `RemoveInWorkspace(ws)` removes all
  registrations and role assignments of the workspace, and
  `WorkspaceSettings.Remove(ws)` removes its settings.

#### Concurrency

| Race | Mitigation |
|---|---|
| Two writers on different users of the same resource both pass the last-owner check | Both must take `LockForUpdate` on the same registration; the second waits (Postgres) or conflicts and retries (Mongo). The owner count is read after the lock. |
| Two services register the same `(kind, id)` | Primary key / unique index; the loser gets `ErrAlreadyExists`. |
| `setResourceMember` passes the workspace-membership check, then the user is removed from the workspace and the cascade runs before the role assignment is written | **Read-time guard**: a role assignment is honored only if the user currently holds a workspace role in the role assignment's workspace. The permittable is already loaded, so this costs no read. The orphaned row is harmless and is removed the next time the cascade or the unregister endpoint touches it. |
| A stale snapshot `Save` from the workspace flow overwrites a newer role assignment (or vice versa) | Snapshot saves never write `resource_roles`; role-assignment writes never write `workspace_roles` (see *Repository changes*). |
| Two admins add the same new person with `addWorkspaceMemberWithResourceRoles` at once (possibly with different projects) | Both take the workspace lock; the second waits / retries, then sees an existing member and only adds its role assignments. The workspace role is not changed and the seat is counted once. |
| Two admins add two different new people when one seat is left | The seat count is read after the workspace lock, so exactly one succeeds. |
| A user is removed from the workspace while a one-step add for them runs | Both take the workspace lock. If the removal commits second, its cascade deletes the new role assignments; if first, the add treats the user as new (admin check, seat limit). The read-time guard covers any remaining gap. |
| Two multi-resource calls on overlapping resources deadlock | Fixed lock order: workspace lock, then registrations sorted by `(kind, id)`. |
| Two admins change the base permission at once, or a change races a workspace save | The setting lives in its own record and is written under the workspace lock; the last committed value wins, and no workspace snapshot `Save` can revert it. |
| A permission check runs while the base permission changes | The check reads the setting in the same round-trip as the registration, so it sees either the old or the new value; the next check sees the new one. Accounts does not cache the setting. Services that cache `checkPermission` / `myResourceAccess` results see the change after their cache TTL, so such caches must be short (or invalidated on change). |

#### Lifecycle and cascades

| Event | Who acts | Effect on role assignments |
|---|---|---|
| Resource created | Service (M2M) | `POST /api/service/resources` with `ownerUserId` = creator |
| Resource deleted | Service (M2M) | `DELETE /api/service/resources/{kind}/{id}`. A missed call leaves an orphan registration — harmless (the resource no longer exists in the service), cleaned by an optional sweeper later |
| Existing resources (pre-feature) | Service (M2M) | One-off backfill via the same endpoint (idempotent), before the service starts sending `resourceId` |
| Person added with `addWorkspaceMemberWithResourceRoles` | Accounts | Workspace membership (if new) and role assignments written in one transaction |
| A member's last role assignment in a workspace removed | Accounts | Only that role assignment is removed; workspace membership (and the seat) stays |
| User removed from workspace | Accounts | `bulkRemovePermittable` also calls `DeleteResourceRolesInWorkspace` |
| Workspace deleted (`Workspace.Remove`) | Accounts | `RemoveInWorkspace(ws)`: all registrations and role assignments of the workspace; the settings record is removed |
| Workspace deactivated / restored | Accounts | No change (mirrors workspace roles; data kept for restore) |
| Base permission changed | Accounts | None: role assignments are untouched; only the baseline derived at check time changes |
| User deleted (`DeleteMe`) | Accounts | Today `DeleteMe` does not delete the permittable. Role assignments are unreachable once the user is gone; deleting the permittable on `DeleteMe` is proposed as a follow-up fix (independent of this feature) |

**Pros**

- Workspace scope of a resource is authoritative and server-side, so a caller
  cannot borrow roles from another workspace.
- Only M2M-authenticated services can make a resource exist in accounts, so
  users cannot claim IDs.
- Resource-wide invariants are serialized on one record per resource, and
  workspace-wide decisions (one-step add, seats, base permission) on one record
  per workspace, on both backends.
- Role-assignment lookup stays inside the already-loaded permittable; the only
  new read is a primary-key lookup of the registration joined with the
  workspace settings.
- Everyone with access is a workspace member, so seats, SSO / SCIM and removal
  keep working through the existing member flows.
- Accounts remains domain-agnostic: opaque kind/ID, semantics in policies.

**Cons**

- One extra round-trip for `checkPermission` calls that pass `resourceId`.
- Services must register resources (including a backfill) and unregister on delete.
- Mongo `Save` semantics change from replace to `$set` (touches existing code).
- Existing workspace member flows must take the workspace lock.
- Under a base permission other than the default, services must filter lists
  with `myResourceAccess` instead of assuming workspace writers / readers see
  everything.

#### Detailed requirements

The technical form of *Functional Requirements*. The items are FR-1 to FR-7 in
order; other parts of this section refer to them by those numbers.

1. `checkPermission` without `resourceId`: **no change** in reads or in
   principal roles. With `resourceId`: **one additional round-trip** — the
   resource registration by primary key, joined with the workspace's settings
   by primary key (Postgres `LEFT JOIN`, Mongo `$lookup`). The role assignment
   itself is read from the already-loaded permittable.
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
7. `myResourceAccess(ws, kind)` costs the already-loaded permittable plus one
   point read of the workspace's settings.

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
    A->>DB: Registration.FindByKey(kind, resourceId) + workspace settings  (new round-trip)
    alt not registered, or alias ≠ registration.workspace
        A-->>S: { allowed: false }
    else
        A->>A: attr.resourceRole = guarded role or ""<br/>roles = global + BaselineRole(workspace role, basePermission)<br/>(+ workspace_member if member without baseline)
        A->>C: CheckResources(principal{roles, attr}, resource{kind, resourceId}, action)
        C-->>A: ALLOW / DENY
        A-->>S: { allowed }
    end
```

##### Project creation and adding a workspace member to it

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
    U->>S: give workspace member B writer on P
    S->>A: setResourceMember(kind, P, B, writer) (U's JWT)
    A->>A: lock registration → B ∈ ws? Cerbos manage_members for U? → upsert
    A-->>S: ok
```

##### Adding a person from the UI in one step (patterns a and b)

Pattern (a) passes one resource (the project being viewed); pattern (b) passes
the projects chosen in the workspace "add member" dialog. Both call the same
mutation.

```mermaid
sequenceDiagram
    participant U as Admin
    participant S as Service UI (e.g. dashboard)
    participant A as reearth-accounts
    participant C as Cerbos

    U->>S: add D to project P1 (a)<br/>or add D to ws with projects P1, P2 (b)
    S->>A: userByNameOrEmail(...) → D's userId
    S->>A: addWorkspaceMemberWithResourceRoles(ws, D, reader, [(kind,P1,writer), ...]) (U's JWT)
    A->>A: begin tx; lock ws
    A->>A: each Pn registered in ws?
    A->>C: manage_members on each Pn for U?
    alt D ∉ ws
        A->>A: U is workspace admin? seat limit → Join(D, reader) + permittable workspace role
    else D ∈ ws
        A->>A: keep D's workspace role unchanged
    end
    A->>A: LockForUpdate P1, P2 … (sorted) → last-owner rule → UpsertResourceRole each
    A->>A: commit (any failure → roll back all)
    A-->>S: { addedToWorkspace, workspaceRole, resources }
```

##### Workspace member removal cascade

```mermaid
flowchart LR
    R[removeUserFromWorkspace / deleteWorkspace] --> B[bulkRemovePermittable]
    B --> W[RemoveWorkspaceRole ws → snapshot Save]
    B --> RR[DeleteResourceRolesInWorkspace ws, users → targeted delete]
    D[deleteWorkspace] --> RW[RemoveInWorkspace ws + settings Remove]
```

#### Technical notes: impact, tests, deployment, rollback, monitoring

##### Impact

1. **Permittable document/row size** grows by ≤ ~300 B per role assignment.
2. **Mongo `Permittable.Save`** changes from replace to `$set`; existing tests
   for permittable persistence must still pass, and a new test verifies that
   `Save` leaves `resource_roles` untouched.
3. **Cerbos resource ID change** when `resourceId` is passed: only callers that
   opt in are affected.
4. **Existing workspace member flows** take the workspace lock; member removal
   issues one additional targeted delete.
5. **Policy authoring**: role names are shared between workspace and resource
   scopes. A resource role must be checked only through `P.attr.resourceRole`
   (never as a principal role); role-list rules are evaluated against the
   baseline role when a `resourceId` is given, and `workspace_member` is never
   listed in them. Existing project policies of a service must be extended with
   the `resourceRole` rules (union = max), not replaced.
6. **gqlclient** (`server/pkg/gqlclient`) consumers need a version bump to use
   the new fields; old clients are unaffected (the workspace member list and
   the `Role` enum do not change).
7. **Base permission and lists**: services must not assume that a workspace
   `writer` / `reader` sees every resource; list endpoints use
   `myResourceAccess` and filter to `bindings` when `baselineRole` is null.
   Checks with a `resourceId` already follow the setting.
8. **Base permission UI**: the workspace settings screen shows and edits
   `Workspace.basePermission` (owners / maintainers only).

##### Tests

1. **Unit** (`pkg/resource`, `pkg/permittable`): kind/ID validation and length
   limits; one role per resource; read-time guard drops role assignments without
   a matching workspace role; `BaselineRole` for every cell of the table above
   (a missing settings record = "Follow workspace role").
2. **Interactor** (`internal/usecase/interactor`):
   - `CheckPermission`: no `resourceId` → unchanged roles whatever the setting
     (regression); unregistered resource → deny; alias of another workspace →
     deny; workspace roles taken from the registration, not the alias;
     `resourceRole` set/empty; with a `resourceId`, principal roles equal the
     baseline (`workspace_member` when none) for each base permission; a user
     with no workspace role → deny even with a stale role assignment.
   - Register: M2M only (JWT rejected); owner must be a workspace member;
     duplicate with same workspace is idempotent, different workspace is 409.
   - `setResourceMember` / `removeResourceMember`: non-member target rejected;
     denied without `manage_members`; self-leave allowed; last-owner
     protection; removing the last role assignment keeps workspace membership.
   - `addWorkspaceMemberWithResourceRoles`: a non-member joins with the given
     (default `reader`) workspace role and gets every role assignment; an
     existing member keeps their workspace role (no downgrade) and only gains
     role assignments; a resource from another workspace, an unregistered
     resource, a missing `manage_members`, a non-admin operator for a new
     person, `owner` as workspace role, a personal workspace, or the seat limit
     → whole call rejected with no membership and no role assignment left
     behind; last-owner rule applied.
   - `updateWorkspaceBasePermission`: owner / maintainer only; personal
     workspace rejected; the next check uses the new value.
   - `myResourceAccess`: `baselineRole` per setting and workspace role; null
     for a member under `None`.
   - Cascades: `RemoveUserMember` / `RemoveMultipleUserMembers` / `Remove`.
3. **Concurrency** (`make test-integration`, both backends): two goroutines
   removing the last two owners concurrently → exactly one succeeds; concurrent
   registrations of the same key → one succeeds; stale snapshot `Save` after
   `UpsertResourceRole` keeps the role assignment; two concurrent one-step adds
   of the same new person → joined once, both sets of role assignments present;
   two concurrent one-step adds of different new people with one seat left →
   exactly one succeeds.
4. **Repository**: Mongo, Postgres and memory implementations of every new
   operation; Postgres `FindByUserID` issues a single query.
5. **Migration**: `0005` up/down on an existing DB; existing permittables load
   with empty `resourceRoles`; workspaces without a settings record read as
   "Follow workspace role".
6. **E2E** (`server/e2e`): a `dashboard:project` test policy generated with the
   rules above; verify:
   - workspace `reader` without a role assignment → `read` allowed, `edit`
     denied, on every project of the workspace;
   - workspace `reader` + resource `writer` on P → `edit` allowed on P only;
   - workspace `writer` + resource `reader` on P → `edit` still allowed on P
     (a resource role never lowers access);
   - workspace `owner` → every action on every project;
   - user from another workspace → denied;
   - base permission `None`: workspace `reader` / `writer` without a role
     assignment → denied on every project; with resource `writer` on P →
     `read` / `edit` on P only; workspace `maintainer` → every action on every
     project;
   - base permission `Writer`: workspace `reader` → `edit` allowed on every
     project; base permission `Reader`: workspace `writer` → `edit` denied
     unless a resource role allows it.

##### Deployment

1. Backward compatible? **Yes.** New optional input field, new
   collections/tables, new routes and GraphQL fields and enum. Behavior is
   unchanged when `resourceId` is absent; the workspace member list and roles
   are unchanged.
2. Partial deployment? **Yes.** Order:
   1. Deploy accounts.
   2. Each service backfills registrations for existing resources.
   3. Each service starts registering on create / unregistering on delete.
   4. Each service deploys policies using `P.attr.resourceRole` and starts
      sending `resourceId`. Before step 4 the attribute is absent and conditions
      never match (fail-closed).
   5. Enable the UI for the one-step add.
   6. Services filter lists with `myResourceAccess`; only then enable the UI
      that changes the base permission. Until a workspace changes it, every
      workspace is "Follow workspace role", so steps 1–5 change nobody's access.
3. Notify: dashboard / visualizer / cms owners; whoever maintains Cerbos policies.
4. Config changes: none (reuses the existing M2M API key).
5. DDL: Postgres migration `0005` runs automatically at startup (golang-migrate).
   Mongo: collection schema + indexes via the startup migration; no backfill in accounts.

##### Rollback

1. Notify the cause in the team Slack channel.
2. Disable the one-step-add and base-permission UIs, then revert consumer
   services / policies that rely on `resourceId` / `resourceRole`. **Access
   widens** for workspaces whose base permission is not the default: the old
   version has no base permission, so their writers / readers see every project
   again. List those workspaces
   (`base_permission <> 'follow_workspace_role'`) and inform their admins before
   rolling back; prefer a fix-forward if any of them depend on `None`.
3. Roll back the accounts image. The old binary ignores the new collections,
   tables and `resource_roles` field, so **no data rollback is required**.
   Note: the old Mongo `Save` replaces the whole document and will drop
   `resource_roles` of any permittable it writes — acceptable during rollback,
   since the feature is disabled; services re-run the backfill on roll-forward.
   People added with the one-step add stay workspace members.
4. Only if the schema itself must be removed: run `0005` down (drops
   registrations, role assignments and workspace settings, including base
   permissions — take a dump first).

##### Post-deployment checks and monitoring

- Checklist
  - Create a project in dashboard staging; verify the registration and the
    creator's `owner` role assignment.
  - Give a workspace `reader` the resource role `writer` on a project; verify
    they can edit that project and still only read the others. Remove them from
    the workspace; verify all project access is lost.
  - Call `checkPermission` with the alias of another workspace; verify deny.
  - `addWorkspaceMemberWithResourceRoles` for a user outside the workspace;
    verify they become a `reader` member, use a seat, and get the role on the
    given project.
  - Verify that workspaces without a settings record report
    `FOLLOW_WORKSPACE_ROLE` and that checks for their members are unchanged.
  - `updateWorkspaceBasePermission(NONE)` on a staging workspace; verify a
    `reader` without resource roles is denied on every project,
    `myResourceAccess` returns `baselineRole: null`, a `maintainer` still has
    full access, and switching back restores access on the next check.
- Metrics
  - `checkPermission` p50/p95 latency with and without `resourceId`.
  - Error rate of register / unregister / `setResourceMember` /
    `addWorkspaceMemberWithResourceRoles` / `updateWorkspaceBasePermission`,
    including Mongo transaction retries and seat-limit rejections.
  - Count of role assignments per permittable (max / p99) to watch FR-4.
- Alerting
  - `checkPermission` error rate: Warning > 1%, Danger ≥ 5%.
  - `checkPermission` p95 latency: Warning > +20% vs baseline.

#### Technical notes on open questions

1. **Per-project visibility:** restricting members is done with the base
   permission. If a project must be visible to the whole workspace even under
   `None`, the proposal is an optional `attributes: JSON` on
   `CheckPermissionInput`, passed through to the Cerbos resource (e.g.
   `visibility`), and a rule such as `read` for `workspace_member` when
   `R.attr.visibility == "workspace"`; services would then also have to merge
   such projects into lists built from `myResourceAccess`.
2. **Batch checks:** a `checkPermissions` query over many `resourceId`s for list
   screens. Cerbos `CheckResources` already supports multiple resources.
3. **Per-service M2M keys:** v1 accepts the existing shared M2M API key for
   register/unregister (`POST` / `DELETE /api/service/resources`). Option: scope
   each key to the kinds of its own service (`dashboard:*`).
4. **Orphan sweeper:** would remove registrations of resources deleted without
   calling the unregister endpoint.
5. **Integration members:** M2M tokens as resource principals (possibly needed
   by cms).
6. **UI placement:** no accounts change either way. Both patterns call
   `addWorkspaceMemberWithResourceRoles`, from whichever service hosts the
   screen; giving an existing member a project role can also use
   `setResourceMember`.
7. **Default workspace role and who may add new people:** the default is the
   `workspaceRole` default in `addWorkspaceMemberWithResourceRoles` (`reader`).
   Letting project `manage_members` holders add new people would drop the
   workspace-admin check for the "not yet a member" branch.
8. **Remove from the workspace with the last project role:** if decided,
   `removeResourceMember` (and unregister) would, in the same transaction and
   under the workspace lock, remove a member whose last role assignment in the
   workspace was removed — only when that member has no baseline (`None`), via
   the existing workspace member removal path.
9. **Base permission on personal / Free workspaces:** v1 rejects
   `updateWorkspaceBasePermission` on personal workspaces. Restricting it by
   plan would add a plan check (the same source `enforceMemberCount` uses) to
   that guard.
10. **`Maintainer` as a base permission value:** add `MAINTAINER` to the enum
    and a column to the `BaselineRole` table; nothing else changes. It would let
    every member manage every project's members, so it is left out of v1.
11. **Scope of the setting:** proposed platform-wide per workspace (one record
    per workspace, applied to every resource kind). A per-service setting would
    key the record by `(workspaceID, service)` and look it up by the
    registration's kind prefix; the round-trip in FR-1 stays the same.

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

### Alternative considered and rejected: outside collaborators

GitHub-style outside collaborators (people with project roles who are not
workspace members) were considered and rejected: they would need a second kind
of membership, separate seat counting and their own removal and listing flows,
and they would sit outside SSO / SCIM management. With the base permission set
to "None", adding a person as a workspace member with only some project roles
gives the same access while keeping everyone in one member list.

## Design

**For each project, accounts keeps only two facts: which workspace it belongs
to, and who holds which role on it.** For each workspace, it also keeps one
setting: the base permission. Everything else about the project stays in the
service.

### Workspace members, roles and the base permission

- **Everyone with access to a project is a workspace member.** People are
  added to the workspace as today (by an admin, or by SSO / SCIM), use a seat as
  today, appear in the member list, and lose all access when removed.
- **Workspace roles** (owner / maintainer / writer / reader) are unchanged and
  still govern workspace-level things, such as managing members or creating
  projects.
  - **Owners and maintainers** always have full access to every project, like
    GitHub organization owners.
  - **Writers and readers** get, on every project, the level set by the
    workspace's base permission (below). A project role can raise their access
    on that project, never lower it.
- Unlike GitHub, there are no outside collaborators: a partner company's staff
  are added as workspace members too, and the base permission plus project
  roles decide what they can see.

### Base permission

Each workspace has a **base permission** (name tentative), like a GitHub
organization's base permission. It decides what writers and readers of the
workspace get on every project:

| Base permission | Writers and readers get, on every project |
|---|---|
| **Follow workspace role** (default) | Their workspace role — exactly today's behavior |
| **None** | Nothing; they see only projects where they have a project role |
| **Reader** | View |
| **Writer** | View and edit |

- Owners and maintainers always have full access, whatever the setting.
- On each project, a writer or reader gets the higher of the base permission and
  their project role.
- Only workspace owners and maintainers can change it, and the change takes
  effect immediately for all members.
- **Giving access to only some projects.** Set the base permission to "None"
  and give people project roles. For example:
  - **Teams in one SSO workspace.** When workspace creation is restricted and
    all employees work in one organization workspace, each team gets project
    roles on its own projects; employees stay workspace members managed by SSO /
    SCIM, and each sees only their own projects.
  - **A partner company's staff.** Add them to the workspace (as readers, for
    example) with a role on the one project they work on; they see nothing else.

#### Diagrams

How access to a project is decided:

```mermaid
flowchart TD
    U["A person opens project P"] --> M{"Member of P's workspace?"}
    M -- "No" --> X["No access"]
    M -- "Yes" --> R{"Workspace role?"}
    R -- "owner / maintainer" --> F["Full access to every project"]
    R -- "writer / reader" --> B["Baseline from the base permission<br/>(default: the workspace role)"]
    B --> E["Access on P = the higher of<br/>the baseline and the role on P"]
```

Example: workspace W with projects P and Q. A is a workspace reader and a writer on P.

```mermaid
flowchart LR
    subgraph D["Base permission: Follow workspace role (default)"]
        direction LR
        A1["A (reader, writer on P)"] -- "edit" --> P1["project P"]
        A1 -- "view" --> Q1["project Q"]
    end
    subgraph N["Base permission: None"]
        direction LR
        A2["A (reader, writer on P)"] -- "edit" --> P2["project P"]
        A2 -. "cannot see" .-> Q2["project Q"]
    end
```

### Checking a permission on a project

1. A service asks accounts whether a user may do something (for example, edit)
   on a specific project.
2. Accounts looks up which workspace the project belongs to. If the service
   never registered the project, or the request names a different workspace,
   the answer is "no".
3. Accounts combines the user's role in that workspace, the workspace's base
   permission and the user's role on the project (if any), and applies the
   service's permission policy:
   - a workspace **owner or maintainer** has full access;
   - a workspace **writer or reader** gets the higher of what the base
     permission gives them and their project role (with "None", only their
     project role);
   - anyone who is not a member of the workspace has no access.
4. A project role only counts while the user is still a member of the project's
   workspace.
5. Checks that do not name a project (for example "may this user create a
   project in this workspace?") still use the workspace role, as today. So when
   a service lists projects, it asks accounts whether the person sees every
   project or only those where they have a project role, and filters the list
   accordingly.

### Creating a project and adding members

1. A user creates a project in a service. The service saves it and registers it
   with accounts, naming the creator as its owner (the creator must be a
   workspace member). So even under "None", the creator can see the project.
2. Someone allowed to manage the project's members gives a workspace member a
   role on it (for example, writer). Accounts checks that the person is in the
   workspace and that the operator may manage members, then saves the role.
   People who are not yet in the workspace are added through the one-step flow
   below.
3. Member changes on the same project are processed one at a time, so the last
   owner can never be removed, even when two people act at once. Members can
   leave a project themselves, except the last owner.
4. A project registered without an owner (for example, one that existed before
   this feature) is managed through workspace roles until an owner is set.

### Adding people from the UI

An admin can give someone access to specific projects in **one step**, in
either of two ways. Either everything succeeds or nothing changes.

- **(a) From a project:** add a person directly to the project. If they are not
  yet a workspace member, they are added to the workspace at the same time —
  as a reader by default (tentative); the admin may choose another workspace
  role — and given the project role.
- **(b) From the workspace:** when adding a person to the workspace, choose
  which projects they get and with which roles.

Adding a new person to the workspace uses a seat, as today, and in the first
version only workspace owners and maintainers can do it. If the person is
already a member, they just get the project roles; their workspace role is not
changed. With the base permission "Follow workspace role", a person added as a
reader can still view every project; to limit them to the chosen projects, the
workspace uses "None".

### Removing people

- **Removing a person from the workspace** removes their workspace role and all
  their project roles in that workspace, in the same step.
- **Removing someone's last project role** does **not** remove them from the
  workspace; they stay a member (and keep using a seat) until an admin removes
  them.
- **Deleting a workspace** removes all its project registrations and project
  roles. Deactivating a workspace removes nothing, so it can be restored.

### Example: Dashboard projects

| Who | Base permission "Follow workspace role" (default) | Base permission "None" |
|---|---|---|
| Workspace owner / maintainer | View, edit and delete every project, and manage its members | Same |
| Workspace writer | View and edit every project | Only projects where they have a project role, with that role |
| Workspace reader | View every project | Only projects where they have a project role, with that role |
| A project role held by a workspace writer / reader | Adds to their workspace role on that project only — for example, a workspace reader who is a writer on a project can edit it. Never reduces access | Is their access to that project |

Project role permissions: owner / maintainer — view, edit, delete and manage
members; writer — view and edit; reader — view. With "Reader" or "Writer" as the
base permission, workspace writers and readers get that level on every project,
plus anything their project roles add. People who are not workspace members
have no access. Whether a single project can be opened to the whole workspace
even under "None" is an open question.

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
   as different things, and must keep the model: owners and maintainers see
   every project, and writers and readers get what the base permission gives
   plus their project roles.
6. **Only services that use the feature need to update.** Services that use the
   new features must update the client library they use to talk to accounts;
   services that don't update keep working unchanged.
7. **Services must not assume workspace writers and readers see everything.**
   When a workspace's base permission is not the default, project lists and
   other screens must show writers and readers only what they may see.
8. **The base permission must be visible in the UI.** The workspace settings
   screen shows it and lets owners and maintainers change it, so admins can
   tell why someone cannot see a project.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Test Plan

1. **Basic rules.** Resource type names and IDs are validated and size-limited;
   a user has at most one role per project; a project role is ignored once the
   user is no longer in the project's workspace.
2. **Permission checks.**
   - Checks that do not name a project give exactly the same results as today,
     whatever the base permission.
   - A project that was never registered → denied.
   - Naming a workspace other than the project's own → denied.
   - The user's workspace role is always taken from the project's real
     workspace, not from what the caller says.
   - With the default base permission, a workspace writer or reader without a
     project role keeps exactly the access their workspace role gives on every
     project.
   - A project role raises a writer's or reader's access on that project only (a
     workspace reader who is a project writer can edit it) and never lowers it
     (a workspace writer who is a project reader can still edit it).
3. **Registering projects.** Only services can register (a user's login is
   rejected); the owner must be a workspace member; registering the same
   project again in the same workspace is harmless, but in a different
   workspace it is rejected.
4. **Managing members.** A project role can be given only to workspace members
   (people outside are added through the one-step addition in item 9); people
   without permission to manage members are refused; members can leave a
   project themselves; the last owner cannot be removed or downgraded;
   removing someone's last project role keeps them in the workspace.
5. **Cleanup.** Removing one or several users from a workspace, or deleting the
   workspace, removes their project roles.
6. **Simultaneous changes.** Two people removing the last two owners at the same
   time → exactly one succeeds. Two services registering the same project at the
   same time → exactly one succeeds. A workspace change saved at the same time
   as a project role change does not wipe out the project role. Two admins
   adding the same new person at the same time → the person is added once and
   gets both sets of project roles. Two admins adding different new people when
   one seat is left → exactly one succeeds.
7. **Every supported database**, including the database upgrade and its undo on
   existing data, and existing users loading correctly with no project roles.
8. **End to end.** With a sample Dashboard project policy, check the results for
   a workspace reader with and without a project role, a workspace writer with
   a lower project role, a workspace owner, a user from another workspace, and
   — under "None" — a reader with and without a project role and a maintainer.
9. **Adding people in one step.**
   - (a) Adding a person who is not in the workspace to a project makes them a
     member (a reader by default) with that project role, in one step.
   - (b) Adding a person to the workspace with several projects gives exactly
     those project roles, in one step.
   - All-or-nothing: if any part fails (a project from another workspace, no
     permission, the seat limit reached, …), the person is not added and gets no
     project roles.
   - Adding a person who is already a member only adds project roles; their
     workspace role is not changed or downgraded.
   - Only workspace owners and maintainers can add new people; it uses a seat.
10. **Base permission.**
    - A workspace that never changed it behaves exactly as today.
    - With "None", writers and readers see only projects where they have a
      project role; owners and maintainers still see everything; a member who
      creates a project can see it.
    - With "Reader" or "Writer", writers and readers get that level on every
      project, plus their project roles.
    - Only owners and maintainers can change it, and the change applies to the
      next check.
    - Project lists show members under "None" only their own projects.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Deployment Plan

1. **Backward compatible? Yes.** Everything is added alongside what exists.
   Services that don't name a project in permission checks see no change, and
   workspace members and roles do not change.
2. **Can it be rolled out gradually? Yes.** Order:
   1. Deploy accounts.
   2. Each service registers its existing projects with accounts (a one-off
      backfill).
   3. Each service starts registering new projects when they are created and
      telling accounts when they are deleted.
   4. Each service turns on its project-level permission rules and starts naming
      the project in permission checks. Until this step, project roles have no
      effect, so nobody gains access by mistake (fail-closed).
   5. The UI for adding people in one step is turned on.
   6. Services filter project lists by what each person may see; then the UI
      for changing the base permission is turned on. Every workspace starts at
      "Follow workspace role", so nobody's access changes until an admin
      changes the setting — the rollout is safe.
3. **Who to notify:** the dashboard / visualizer / cms owners, and whoever
   maintains the permission policies.
4. **Configuration changes:** none.
5. **Database changes:** applied automatically when accounts starts. Accounts
   itself does not need to convert any existing data.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Rollback Plan

1. Share the cause in the team Slack channel.
2. First, turn off the UI for adding people in one step and for changing the
   base permission, and turn off project-level permission rules in the services
   that use them. **Rolling back widens access** in workspaces whose base
   permission is not the default: their writers and readers see every project
   again. Tell those workspaces' admins first, and prefer fixing forward if
   they rely on "None".
3. Roll accounts back to the previous version. The previous version simply
   ignores the new data, so **no data rollback is required**.
   Note: while the previous version runs, it may drop the project roles of users
   whose permission data it saves. This is acceptable because the feature is off
   at that point; services re-run the backfill when the feature is rolled out
   again. People added in one step stay workspace members.
4. Only if the new data structures must be removed completely: run the database
   downgrade. It deletes all project registrations, project roles and base
   permission settings, so take a backup first.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Post Deployment

- Checklist
  - Create a project in Dashboard staging; check that it is registered and that
    the creator is its owner.
  - Make a workspace reader a writer on one project; check that they can edit
    that project and can still only view the others. Remove them from the
    workspace; check that they lose access to every project.
  - Ask for a permission on the project while naming a different workspace;
    check that it is denied.
  - From the project, add a person who is not in the workspace; check that they
    become a reader member, use a seat, and get the project role.
  - From the workspace, add a person and choose two projects; check that they
    get exactly those two project roles.
  - Check that existing workspaces show "Follow workspace role" and that their
    members' access has not changed.
  - Set a staging workspace's base permission to "None"; check that a reader
    with no project roles sees no projects (also in project lists), a maintainer
    still sees all of them, and setting it back restores access immediately.
- Metrics
  - How long permission checks take (typical and slowest 5%), with and without a
    project named.
  - Error rate of registering / unregistering projects, changing members,
    adding people in one step and changing the base permission, including
    automatic retries and seat-limit rejections.
  - Number of project roles per user (maximum and top 1%), to watch the ~1,000
    per user assumption.
- Alerting
  - Permission check error rate: Warning above 1%, Danger at 5% or more.
  - Permission check response time (slowest 5%): Warning when more than 20%
    slower than the usual baseline.

Technical details: see *Solution Options → Option 1 → Technical notes: impact,
tests, deployment, rollback, monitoring*.

## Open Questions

1. **Do we need per-project visibility?** Keeping members from seeing projects
   is done with the base permission ("None" plus project roles). What remains
   is the opposite case: under "None", should a single project be openable to
   the whole workspace (a "visible to all members" project), as GitHub does
   with internal repositories? That would need accounts to know a project's
   visibility. Deferred until a product needs it.
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
   and is to be decided together with the designer. It also decides where the
   two one-step patterns in *Design → Adding people from the UI* appear: (a)
   adding a person from a project's screen, and (b) choosing projects in the
   workspace "add member" dialog.
7. **What workspace role do people added from a project get, and who may add
   them?** Tentatively reader, and the admin may choose. May a project owner who
   is not a workspace owner or maintainer add new people to the workspace this
   way, given that it uses a paid seat? (Proposed for the first version: no —
   only workspace owners and maintainers.)
8. **Should removing a person's last project role also remove them from the
   workspace?** Today it does not: under "None" such a member sees no projects
   but stays in the workspace and keeps using a seat until an admin removes
   them. Removing them automatically frees the seat but may surprise admins.
9. **Should Free plans and personal workspaces offer the base permission?** A
   personal workspace has a single member, so the first version does not offer
   it there. Whether it is a paid-plan feature is a product decision.
10. **Should "Maintainer" be allowed as a base permission value?** It would let
    every writer and reader manage every project's members. The first version
    offers only "Follow workspace role", "None", "Reader" and "Writer".
11. **Should the base permission be per service or platform-wide?** Proposed:
    one setting per workspace that applies to every service (Dashboard,
    Visualizer, CMS, Flow), so admins set it once and people see the same thing
    everywhere.

Technical details: see *Solution Options → Option 1 → Technical notes on open
questions*.

## Reviewed by

- Technical Architect: [name]
- Technical Leader: [name]
- Peers: [name]
