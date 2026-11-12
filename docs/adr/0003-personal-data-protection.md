# ADR-0003: Personal data protection in the accounts API

- **Status:** Proposed
- **Date:** 2026-11-12
- **Deciders:** @pyshx

## Context

#374 stopped the GraphQL API from returning other users' email. The review that followed found weaknesses that could let a similar exposure happen again, or go unnoticed:

1. Anonymous access is an allowlist, `bypassedFields` in `internal/app/auth.go`. A query name added there by mistake becomes public, and the schema does not show which fields are public.
2. `findByID`, `findByIDs`, `findByAlias` and `findUsersByIDsWithPagination` are anonymous and return workspace members or users.
3. `X-Internal-Service` is a header the caller chooses. Nothing proves who the caller is.
4. Each resolver hides personal fields in its own code. Nothing forces a new field to declare who may see it.
5. CMS and Dashboard read the accounts MongoDB directly through reearthx `accountmongo`, so no accounts rule applies to them. If accounts moves to PostgreSQL, they read stale data without an error.
6. User lookups have no rate limit.
7. Most logs are printf-style text. They have no viewer, operation or trace ID, so access to user data cannot be queried or alerted on.
8. Tracing covers HTTP, GraphQL and MongoDB, but not PostgreSQL, Cerbos or Auth0. `pkg/gqlclient` does not propagate trace context.

Already in place:

- Introspection is on only in development.
- GraphQL has a complexity limit.
- Email matches only as a whole address, on every backend (#374).
- A client test runs `pkg/gqlclient` against the server (#374).

## Decision

1. **Fields are private unless the schema marks them public.** A `@public` directive allows a field to be called without a token. The auth middleware builds its allowlist from the schema, and `bypassedFields` is deleted. A test calls every other field without a token and expects `401`.
2. **Anonymous reads use a public view.** A new anonymous query returns a workspace's name and alias, and its members' names, aliases and photos. It leaves out roles and emails. Once anonymous callers have moved to it, the four queries in Context item 2 lose `@public`.
3. **Service identity follows ADR-0002.** `X-Internal-Service` is a log label only. Accounts accepts traffic only from internal networks and its load balancer.
4. **Every personal field declares its visibility.** Fields that carry user data take `@visibility(level: PUBLIC | SELF | WORKSPACE)`, and a directive resolver enforces it. A schema test fails when such a field has no `@visibility`.
5. **Other services stop reading the accounts database.** They move to the accounts API, using the service identities from ADR-0002. Until then, each reader gets its own read-only database user, limited to the collections it reads. Accounts does not switch to PostgreSQL in an environment where a service still reads its MongoDB.
6. **User lookups are rate limited.** Limits apply per IP at the load balancer and per viewer in accounts. The minimum keyword length and the result cap stay.
7. **GraphQL queries have a depth limit.** It applies in addition to the complexity limit.
8. **Privacy tests derive from the schema.** A generated test runs every query anonymously and as an unrelated signed-in user. It fails if another user's `SELF` or `WORKSPACE` field comes back. The client test also runs the oldest `pkg/gqlclient` version still in use.
9. **Logs are structured.**
   - Every log call uses reearthx `log` with typed fields, named from a fixed list.
   - Each REST or GraphQL request writes one entry with its route or operation, top-level fields, status, latency, viewer ID and calling service.
   - No entry carries variables, bodies or personal data. A test enforces this.
   - reearthx adds Cloud Logging trace fields and an `httpRequest` object, and fixes the misspelled `letency` key.
10. **Traces cover every hop.** PostgreSQL, Cerbos and Auth0 calls get spans. `pkg/gqlclient` sends W3C trace context. Log entries carry trace and span IDs. Spans carry no variables, returned values or personal data.
11. **Request logs are retained and watched.** Deployments export request logs to long-term storage, because MongoDB does not audit reads. Alerts fire on anonymous calls to user queries, and when one viewer or IP looks up an unusual number of users.
12. **Sensitive code has a named reviewer.**
    - CODEOWNERS requires @pyshx for `internal/app/auth.go`, `schemas/` and the user resolvers.
    - The PR template asks: "Who can call this, and what personal data does it return?"
    - `docs/runbooks/api-contract-rollout.md` sets the rollout order for a contract change: deploy the clients, confirm they are serving, then deploy accounts.

## Consequences

- Decision 2 breaks anonymous callers until they move to the public query. `@public` comes off the old queries only after those callers have deployed the change.
- Decision 4 changes how every resolver hides data. Email visibility does not change.
- Decision 6 can reject a legitimate burst, so the limits run in log-only mode first.
- Decision 9 changes log message text. Queries and dashboards that match today's text must move to the new fields. Anything that reads `letency` must move to the corrected key.
- Decisions 10 and 11 add trace ingestion and log storage cost.
- Services get trace context when they upgrade `pkg/gqlclient`.
- Work lands as one PR per decision, in this order: 12, 1, 9 and 10, 7, 6, 4, 8, 2. Decisions 3, 5 and 11 also need infrastructure changes. Decision 9 starts with a reearthx PR.

## Open question

None.
