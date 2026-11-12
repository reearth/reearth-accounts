# ADR-0003: Personal data protection in the accounts API

- **Status:** Proposed
- **Date:** 2026-11-12
- **Deciders:** @pyshx

## Context

#374 fixed a leak of users' email through the GraphQL API. The review after it found weaknesses that let such a leak happen and kept it from being noticed:

1. Anonymous access is an allowlist, `bypassedFields` in `internal/app/auth.go`. A new query is public if someone adds its name there by mistake. Nothing in the schema shows which fields are public.
2. Four queries are still anonymous: `findByID`, `findByIDs` and `findByAlias` return a workspace with its members, and `findUsersByIDsWithPagination` returns users. Only Dashboard's public pages and Flow's anonymous path need them.
3. `X-Internal-Service` is a label the caller chooses. Accounts only logs it and never authorizes on it, but nothing proves who the caller is.
4. Hiding a personal field is code in each resolver. Nothing forces a new field to choose who may see it.
5. CMS and Dashboard read the accounts MongoDB directly through reearthx `accountmongo`, so no accounts rule applies to them. The CMS leak happened there. If accounts switches to PostgreSQL, those readers will read stale data with no error.
6. Lookups have no rate limit. A keyword shorter than 3 characters returns nothing and a search returns at most 10 users, but one caller can repeat the search without limit.
7. Logs are mostly text. reearthx `log` already writes JSON with `severity` on GCP and supports typed fields (`Infow`, `With`), and its access log has `request_id`, `route`, `status` and latency. But accounts logs through 132 printf-style calls, so their content cannot be queried or alerted on. reearthx does not add Cloud Logging's trace and span fields or an `httpRequest` object, so logs do not link to traces. Requests are logged without the operation, the fields or the viewer, and logs are kept too briefly. Five months of access could not be checked after #374.
8. OpenTelemetry tracing is on in every environment (`otel_enabled` in `reearth-infrastructure`), with spans for HTTP, GraphQL operations and MongoDB. PostgreSQL, Cerbos and Auth0 calls have no spans. Logs carry no trace ID. `pkg/gqlclient` sends no trace context, so a service's call to accounts starts a new trace.

Already in place on `main` (ae7edfe):

- Introspection is on only in development (`internal/app/graphql.go`).
- A complexity limit applies, the `GraphQL.ComplexityLimit` setting, default 6000.
- Email matches only as a whole address, on MongoDB, PostgreSQL and memory, with conformance tests (#374).
- `e2e/gql_user_email_visibility_test.go` runs the in-repo `pkg/gqlclient` against the server (#374).

## Decision

### 1. Fields are private unless the schema marks them public

A `@public` directive on a `Query` or `Mutation` field allows a caller with no token. The auth middleware builds its allowlist from the schema at startup, and `bypassedFields` is deleted. A test runs every unmarked field with no token and expects `401`.

### 2. Anonymous reads get a public query with public fields only

A new anonymous query returns a workspace's public view: name, alias, and each member's name, alias and photo. Member roles are left out, because they tell an anonymous caller who owns or administers the workspace. They are added only if Dashboard's public pages need them. Dashboard's public pages and Flow's anonymous path move to it. Then `findByID`, `findByIDs`, `findByAlias` and `findUsersByIDsWithPagination` lose `@public`.

### 3. Service identity follows ADR-0002

How a service proves its identity is decided in ADR-0002. `X-Internal-Service` stays a log label and is never an authorization input. Accounts' Cloud Run ingress is restricted to internal traffic and the load balancer, as for the CMS worker.

### 4. Every personal field declares who can see it

A `@visibility(level: PUBLIC | SELF | WORKSPACE)` directive goes on every field of `User`, `Me`, `UserMetadata` and any type that carries user data. A directive resolver enforces it, and `email` moves from resolver code to `@visibility(level: SELF)`. A schema test fails when a field of those types has no `@visibility`.

### 5. Other services stop reading the accounts database

CMS and Dashboard move from `accountmongo` to the accounts API, once ADR-0002 gives them an identity. Until then, each gets its own MongoDB user, read-only, on only the collections it reads. Accounts does not switch to PostgreSQL in an environment while any service reads its MongoDB there.

### 6. Lookups are rate limited

User lookups are limited per IP at the load balancer (Cloud Armor) and per viewer in accounts. They cover GraphQL `searchUser`, `user`, `findUserByAlias`, `findUsersByIDs` and `findUsersByIDsWithPagination`, and the REST lookups. The existing minimum keyword length and result cap stay.

### 7. GraphQL queries have a depth limit

A depth limit applies next to the complexity limit.

### 8. Tests derive from the schema

1. A generated test runs every query, with no token and as a signed-in user with no shared workspace, and fails if another user's `SELF` or `WORKSPACE` field comes back.
2. The client test also runs the oldest `pkg/gqlclient` version that any service still deploys, so a server change that breaks it fails CI.

### 9. Logs are structured, and every request logs what it did

Accounts keeps reearthx `log` and stops using printf-style calls.

1. Every log call uses typed fields (`Infow`, `Errorw`, `With`), through the request-scoped logger reearthx attaches to the context.
2. Field names are fixed in one place: `request_id`, `viewer_id`, `caller`, `operation`, `fields`, `route`, `status`, `latency_ms`, `error`.
3. Every request, REST and GraphQL, writes one request entry with its route or operation name, top-level fields, status, latency, viewer ID and calling service.
4. No entry carries variables, request or response bodies, or personal data. A test fails if an email or other `SELF` field value reaches a log entry.
5. reearthx gains, in its own PR so every service gets them:
   1. `logging.googleapis.com/trace`, `logging.googleapis.com/spanId` and `logging.googleapis.com/trace_sampled` fields taken from the OpenTelemetry span in the context.
   2. An `httpRequest` object on the access log entry, written once per request.
   3. The access log's `letency` key fixed.

   Accounts then upgrades reearthx.

### 10. Traces cover every hop and join the logs

1. PostgreSQL (pgx), Cerbos (gRPC) and Auth0 (HTTP) calls get spans.
2. `pkg/gqlclient` sends W3C trace context, so a caller's trace continues into accounts.
3. Every log entry in a request carries its trace and span IDs (decision 9).
4. Spans carry the operation name, the top-level fields, the viewer ID and the calling service. They never carry variables, returned values or personal data.

### 11. Logs are kept and watched

Infrastructure changes in `reearth-infrastructure`:

1. A log sink exports accounts request logs to BigQuery or Cloud Storage for 3 years. MongoDB does not audit reads, so these logs are the record of who read user data.
2. An alert fires on anonymous calls to user queries.
3. An alert fires when one viewer or IP looks up an unusual number of users.

### 12. Sensitive code needs a named reviewer

1. CODEOWNERS makes @pyshx the required reviewer for `internal/app/auth.go`, `schemas/` and the user resolvers.
2. The PR template asks: "Who can call this, and what personal data does it return?"
3. `docs/runbooks/api-contract-rollout.md` sets the order for an API contract change, per environment:
   1. Deploy client updates.
   2. Confirm each client's new revision is serving.
   3. Deploy accounts.

## Consequences

- Decision 2 breaks Dashboard and Flow until they move to the public query, so `@public` comes off the old queries only after both have deployed.
- Decision 4 changes how every resolver hides data. Email visibility keeps its current behavior.
- Decision 6 can reject a legitimate burst. Limits start in log-only mode and are enforced after a week of data.
- Decision 11 adds BigQuery or Cloud Storage cost for 3 years of logs. Decision 10 adds spans, which count against Cloud Trace ingestion at the configured sampling ratio.
- Services pick up decision 10's trace context when they upgrade `pkg/gqlclient`.
- Decision 9 rewrites accounts' log calls. Queries and dashboards that match today's message text must move to the new fields. The reearthx change reaches other services when they upgrade it. Anything reading the `letency` key must move to the corrected one.
- Accounts work lands as one PR per decision, in this order: 12, 1, 9 with 10, 7, 6, 4, 8, 2. Decision 9 starts with a reearthx PR. Decisions 3, 5 and 11 need `reearth-infrastructure`, CMS and Dashboard PRs.

## Open questions

None.
