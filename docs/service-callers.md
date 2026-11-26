# Token issuer configuration

## Startup check on trusted URLs

The server refuses to start when a URL that it trusts for tokens is not an `https` URL on a DNS name. The check implements ADR-0002 decision 11.6 and runs every time the server reads its configuration, in every build.

The check covers these URLs:

- The issuer of each auth provider. The providers come from `REEARTH_AUTH0_DOMAIN`, `REEARTH_AUTH_ISS`, `REEARTH_ACCOUNTS_CIP_PROJECT_ID`, and each entry of `REEARTH_AUTH`, in that order.
- The `JWKSURI` of each `REEARTH_AUTH` entry that sets one. The server fetches signing keys from this URL instead of from the issuer.
- The issuer of each entry in `REEARTH_ACCOUNTS_SERVICE_CALLERS`.

A URL passes only when all of these are true:

- The scheme is `https`. The scheme is compared after parsing, so `HTTP://` counts as `http`.
- The URL has no user name or password.
- The host is not an IP address, IPv6 included.
- The host has at least one dot, ignoring one trailing dot, and no empty label.
- Every label uses only ASCII letters, digits, and hyphens. Write an internationalized name in its `xn--` form.
- The last label starts with a letter. This rejects numeric forms that some resolvers read as an IPv4 address, such as `127.1` and `127.0.0.0x1`.
- No label is `localhost`. This rejects `localhost`, names under it, and names such as `localhost.localdomain`.

The check looks at the URL, not at the address the name resolves to. A DNS name that resolves to a private address still passes.

When the check fails, the error lists every failing URL by its source, such as `auth provider 2 jwks uri` or `service caller "<client id>" issuer`, with the reason. The provider number follows the order above. The errors from this check never include a user name, password, path, or query.

## Local issuers

A local identity provider, such as a mock OAuth server in Docker Compose, often runs over `http` on a loopback address or a service name. To trust such a provider on a laptop, set `REEARTH_ACCOUNTS_LOCAL_ISSUER=true`. The default is `false`.

With the switch on, a URL that fails the check above still passes when all of these are true:

- The scheme is `http` or `https`.
- The URL has no user name or password.
- The host is one of these:
  - A loopback IP address, such as `127.0.0.1`, any other `127.x.y.z` address, `::1`, or `::ffff:127.0.0.1`. The addresses `0.0.0.0` and `::`, private ranges, link-local addresses, and metadata addresses do not pass.
  - The name `localhost`, in any case, with or without one trailing dot. Names under it, such as `app.localhost`, do not pass.
  - A name with one label that starts with an ASCII letter and uses only ASCII letters, digits, and hyphens, such as `mock-oauth2`. Short numeric forms such as `127.1` do not pass. The names `metadata` and `instance-data` do not pass, because cloud resolvers expand them to the metadata address.

Every other URL must still be an `https` URL on a DNS name. For example, `http://idp.example.com/` and `https://10.0.0.1/` fail with the switch on.

When the switch is off and a failing URL would pass with the switch on, the error says so.

The switch is for a developer machine. Do not set it in a deployed environment:

- The server refuses to start with the switch on when it runs on Cloud Run or Kubernetes. It detects them by the `K_SERVICE` and `KUBERNETES_SERVICE_HOST` variables that those platforms set. A single-label host resolves to in-cluster services there.
- The server logs a warning at startup whenever the switch is on.
- Only the variable `REEARTH_ACCOUNTS_LOCAL_ISSUER` sets the switch. A value that is not a boolean stops startup.

`REEARTH_DEV`, `REEARTH_ACCOUNTS_DEV`, and `REEARTH_MOCK_AUTH` have no effect on this check, and neither does the build tag. A local setup that runs with `REEARTH_MOCK_AUTH` and a placeholder issuer, such as `http://localhost:8080/`, needs the switch too, or an `https` issuer.

### Difference from ADR-0002

ADR-0002 decisions 11.6 and 11.7 say that the check is off when dev mode is on. The server is stricter: dev mode has no effect, and only the local issuer switch relaxes the check. The reason is that a deployed development environment can run with dev mode on.

## Registering a service caller

A service caller is an OAuth client that calls accounts with a client credentials token. Accounts accepts such a token only from a client listed in `REEARTH_ACCOUNTS_SERVICE_CALLERS`, and only as the subjects that the entry allows (ADR-0002 decisions 3, 5 and 7).

### The registry entry

`REEARTH_ACCOUNTS_SERVICE_CALLERS` must be a JSON array. An unset or blank variable registers no caller; `null`, an object, a string, or a number stops startup. Each entry is an object with exactly these keys, each written once, in lower case:

```json
[
  {
    "issuer": "https://idp.example.com/",
    "client_id": "abc123",
    "name": "example-app",
    "act_as": ["integration"]
  }
]
```

- `issuer` is the exact `iss` of the client's tokens. It must also be the issuer of a configured auth provider.
- `client_id` is the client ID that the token carries in `azp` or `client_id`. One client ID may appear once per issuer.
- `name` is the name that accounts logs for the client. It must match `^[a-z0-9][a-z0-9-]{0,62}$`.
- `act_as` lists the kinds of subject the client may act for: `"integration"`, `"user"`, or both. The key is required. An empty list, `[]`, registers a client that can only make anonymous calls.

Keys are case-sensitive: `"ACT_AS"` or `"Name"` is an unknown key. The server refuses to start when an entry has a missing, unknown, or repeated key, a duplicate `act_as` value, an invalid `name`, or a repeated `(issuer, client_id)` pair.

**Breaking change.** The earlier shape, `{"issuer","client_id","integration_id"}`, is no longer accepted. An entry with `integration_id` stops startup with an error that names `act_as`. To migrate, replace `integration_id` with `"name"` and `"act_as": ["integration"]`, and make the client send `X-Reearth-Act-As: integration:<integration ID>` on every call.

### The act-as header

A client names the subject of each call with one header:

```
X-Reearth-Act-As: integration:<integration ID>
```

Accounts then authorizes the call as that integration: the workspaces where the integration is a member, with the integration's role in each. An integration with no memberships is accepted and sees no workspaces.

`user:<user ID>` is parsed, but accounts refuses it for now, even when `act_as` lists `"user"`. Acting for a user opens once every route closed to client tokens is enforced.

A client token without the header is anonymous. It reaches only what the next section lists for anonymous clients.

A Go client that uses `pkg/servicetoken` names the subject on each request's context, and `servicetoken.Transport` sends the header:

```go
ctx = servicetoken.WithActAs(ctx, servicetoken.IntegrationSubject(integrationID))
```

The transport sends the header only with the service token. A request that already carries a user token gets neither.

### Migrating

Change a caller in this order. First, make the caller send `X-Reearth-Act-As` on every call. Then change its registry entry. A caller that sends no header becomes anonymous as soon as the new entry is in place, and anonymous calls get 403 on every route and field that is not open to anonymous clients.

### Responses

| Request | Result |
|---|---|
| Client token, registered, `integration:<id>` allowed by `act_as` | Authorized as the integration |
| Client token, no header | Anonymous |
| Client token on a route or field not open to its subject kind | 403 |
| Client token, unregistered or with invalid claims, with or without the header | 401, also when no caller is registered |
| Client token from an unconfigured issuer, with the wrong audience, or expired | 401 |
| Header with a user token or with no token | 401 |
| Header empty, repeated, malformed, or with an unknown kind | 401 |
| Subject kind not in `act_as` | 401 |
| `user:<id>` | 401 |
| Header with `REEARTH_MOCK_AUTH` on | 401 |
| User token without the header | Unchanged |

Every REST route under `/api` checks the header, including the routes that read no credentials, such as `GET /api/auth/config` and the password reset routes.

### What a client token can reach

Every REST route and every GraphQL Query or Mutation field is closed to client tokens unless it is listed. A route or field added later is closed until someone opens it. The rule does not depend on whether the operator has a user.

- REST: the table `clientRoutes` in `server/internal/adapter/http/client_routes.go` lists each open route as `"METHOD /path"`, with the path as the route pattern in `router.go`, and the subject kinds it admits. The server refuses to start when a key names a route that does not exist.
- GraphQL: the directive `@client(as: [INTEGRATION])` on a root field opens it to clients acting for an integration. A client token without the header may call the `@public` Query fields. No client token may call a `@public` Mutation, so signup, verification, and password reset are closed to client tokens.

To open a route or field, add the table entry or the directive and run `go test ./internal/app -run TestClientSurface -update`. The test writes `server/internal/app/testdata/client_surface.golden`, which lists every open route and field with the kinds it admits. Review the diff of that file with the change.

Today a client acting for an integration may use the workspace routes and fields that a service caller could use before this rule, still limited by the integration's role in each workspace. The SCIM token and SCIM config routes stay closed: a client cannot create a workspace credential. Signup, verification, password reset, `/users/me`, MFA, account deletion, login removal, the user directory, and permission checks are closed to every client token.

A refused REST call gets `403` with the message `forbidden: this route is not open to client tokens`. A refused GraphQL operation gets HTTP `403` and one error with the code `FORBIDDEN`. The gate checks every root field of the selected operation, including fields inside fragments and behind aliases, and refuses the whole operation before any resolver runs. `__typename` is allowed. The introspection fields are closed to client tokens.

A route mounted outside the `/api` REST group does not pass through the gate. A client token on such a route resolves to no user and no operator, so a route that requires authentication answers 401.

### The acting integration's workspaces

Two endpoints let an app work with the integration it acts for without reading the accounts database. Neither takes an integration ID: the integration is the one in `X-Reearth-Act-As`, so a client cannot read or change another integration's memberships.

Only a client token acting for an integration may call them. Without a token the response is 401. A user's own token gets 401 on REST and an `invalid operator` error on GraphQL. A client token without the header gets 403.

To list the workspaces and the integration's role in each:

```
GET /api/integrations/self/workspaces
```

```json
{
  "integration_id": "<integration ID>",
  "workspaces": [{"id": "<workspace ID>", "name": "...", "alias": "...", "role": "writer"}]
}
```

The GraphQL query is `actingIntegration { id workspaces { role workspace { id name alias } } }`. `workspace` shows the same fields as `findByID` does for the integration.

To remove the integration from every workspace it belongs to, for example when the integration is deleted:

```
DELETE /api/integrations/self/workspaces
```

```json
{"removed_workspace_ids": ["<workspace ID>"], "failed_workspace_ids": []}
```

The GraphQL mutation is `leaveAllWorkspaces { removedWorkspaceIds failedWorkspaceIds }`. Leaving needs no role, because it lowers the integration's access. Accounts leaves each workspace in its own transaction. When a workspace fails, accounts lists it in `failed_workspace_ids`, continues with the others, and still answers 200. A retry is safe: it removes only the memberships that are left, and when none are left it returns two empty lists. A 5xx means accounts could not read the memberships and changed nothing.

### What is logged

The client gate writes one entry for every valid client call: `auth: client call` when the route or field admits it, and `auth: rejected client call` with the reason `not_open_to_client` when it answers 403. That entry, every later entry of the request, and the request log line carry these fields, on a 403 too:

- `caller_client_id`: the client ID.
- `caller_name`: the registered name.
- `acting_for`: `integration:<id>`, or `none` for an anonymous call.

A call refused with 401 writes one `auth: rejected client call` entry with a `reason` code and `caller_client_id` when the token names a client. A refusal made while reading the token or the header also carries the token's `issuer`. No call is logged both as accepted and as refused.

The request log line of any request made with a client token carries `caller_client_id`, also when the call is refused or the route reads no credentials. No entry contains the token, the raw header value, or an email address.
