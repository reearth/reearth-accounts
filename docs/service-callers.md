# Token issuer configuration

## Startup check on trusted URLs

Outside dev mode, the server refuses to start when a URL that it trusts for tokens is not an `https` URL on a DNS name. The check implements ADR-0002 decision 11.6 and runs when the server reads its configuration.

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

## Dev mode

Dev mode turns the check off. Dev mode is on when `REEARTH_DEV` is `true` or when the server is built without the `release` build tag. The release image is built with that tag. `REEARTH_ACCOUNTS_DEV` and `REEARTH_MOCK_AUTH` do not turn the check off.

Do not set `REEARTH_DEV` in a deployed environment to get past this check. Dev mode also makes the GraphQL API accept the `X-Reearth-Debug-*` headers, which name a user without a token. Fix the URL instead.

## Registering a service caller

A service caller is an OAuth client that calls accounts with a client credentials token. Accounts accepts such a token only from a client listed in `REEARTH_ACCOUNTS_SERVICE_CALLERS`, and only as the subjects that the entry allows (ADR-0002 decisions 3, 5 and 7).

### The registry entry

`REEARTH_ACCOUNTS_SERVICE_CALLERS` is a JSON array. Each entry has exactly these keys:

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

The server refuses to start when an entry has a missing or unknown key, a duplicate `act_as` value, an invalid `name`, or a repeated `(issuer, client_id)` pair.

**Breaking change.** The earlier shape, `{"issuer","client_id","integration_id"}`, is no longer accepted. An entry with `integration_id` stops startup with an error that names `act_as`. To migrate, replace `integration_id` with `"name"` and `"act_as": ["integration"]`, and make the client send `X-Reearth-Act-As: integration:<integration ID>` on every call.

### The act-as header

A client names the subject of each call with one header:

```
X-Reearth-Act-As: integration:<integration ID>
```

Accounts then authorizes the call as that integration: the workspaces where the integration is a member, with the integration's role in each. An integration with no memberships is accepted and sees no workspaces.

`user:<user ID>` is parsed, but accounts refuses it for now, even when `act_as` lists `"user"`. Acting for a user opens once every route closed to client tokens is enforced.

A client token without the header is anonymous. It gets what a request with no token gets.

### Responses

| Request | Result |
|---|---|
| Client token, registered, `integration:<id>` allowed by `act_as` | Authorized as the integration |
| Client token, no header | Anonymous |
| Client token, unregistered or with invalid claims | 401 |
| Client token from an unconfigured issuer, with the wrong audience, or expired | 401 |
| Header with a user token or with no token | 401 |
| Header empty, repeated, malformed, or with an unknown kind | 401 |
| Subject kind not in `act_as` | 401 |
| `user:<id>` | 401 |
| Header with `REEARTH_MOCK_AUTH` on | 401 |
| User token without the header | Unchanged |

REST routes that read no credentials at all, such as `GET /api/auth/config` and the password reset routes, do not check the header yet.

### What is logged

Every accepted client call writes one `auth: client call` entry. That entry, every later entry of the request, and the request log line carry these fields:

- `caller_client_id`: the client ID.
- `caller_name`: the registered name.
- `acting_for`: `integration:<id>`, or `none` for an anonymous call.

Every refused call writes one `auth: rejected client call` entry with a `reason` code, the token's `issuer`, and `caller_client_id` when the token names a client. No entry contains the token, the raw header value, or an email address.
