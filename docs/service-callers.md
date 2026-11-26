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

A client token without the header is anonymous. It reaches only what the next section lists for anonymous clients.

### Responses

| Request | Result |
|---|---|
| Client token, registered, `integration:<id>` allowed by `act_as` | Authorized as the integration |
| Client token, no header | Anonymous |
| Client token on a route or field not open to its subject kind | 403 |
| Client token, unregistered or with invalid claims | 401 |
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

### What is logged

Every accepted client call writes one `auth: client call` entry. That entry, every later entry of the request, and the request log line carry these fields:

- `caller_client_id`: the client ID.
- `caller_name`: the registered name.
- `acting_for`: `integration:<id>`, or `none` for an anonymous call.

Every refused call writes one `auth: rejected client call` entry with a `reason` code, the token's `issuer`, and `caller_client_id` when the token names a client. No entry contains the token, the raw header value, or an email address.

## Local development

On a laptop, a service can make client calls to accounts with tokens from a local issuer, and every accounts check runs as in a deployed environment (ADR-0002 decision 11). The local issuer is [mock-oauth2-server](https://github.com/navikt/mock-oauth2-server), pinned by version and digest in `docker-compose.dev.yml`. Its config is `server/dev/mock-oauth2/config.json`.

### Set it up

1. Start the mock issuer:

   ```sh
   docker compose -f docker-compose.dev.yml up -d mock-oauth2
   ```

   The issuer is `http://mock-oauth2:8091/accounts-local`. Compose publishes port 8091 on `127.0.0.1` only.

2. Write the keys and the accounts config:

   ```sh
   cd server && make dev-seed
   ```

   The command creates one RSA key for each local client in `server/.dev/service-callers/`, which git ignores. The default client is `cms`, with `act_as` `["integration"]`. To add clients, pass them as flags, such as `make dev-seed ARGS='-client cms=integration -client flow=integration'`. A second run keeps the existing keys. To replace them, run `make dev-seed ARGS=-rotate`.

3. Load the accounts config. The command writes `server/.dev/service-callers/accounts.env`, which sets three variables: `REEARTH_ACCOUNTS_LOCAL_ISSUER=true`, `REEARTH_AUTH` with one entry for the mock issuer, and `REEARTH_ACCOUNTS_SERVICE_CALLERS` with one entry for each client.
   - `reearth-accounts-dev` in `docker-compose.dev.yml` loads this file after `server/.env.docker` on every start once the file exists. The optional `env_file` entry needs Docker Compose 2.24 or later.
   - Accounts adds `REEARTH_AUTH` entries after the providers that the Auth0, `REEARTH_AUTH_ISS`, and CIP variables configure, so the mock issuer is added to those providers and does not replace them. A `REEARTH_AUTH` value in `.env.docker` is replaced by the one in this file.
   - For a server that runs on the host, load the file into the shell first, such as `set -a; . ./.dev/service-callers/accounts.env; set +a`. That server fetches the mock's keys by its service name, so the host also needs the hosts-file line in [Use one issuer URL](#use-one-issuer-url).
   - To turn the local issuer off, delete `server/.dev/service-callers/accounts.env` and restart accounts.

4. Turn off `REEARTH_MOCK_AUTH`. Mock auth skips token validation and has no client path, so accounts refuses every client call while it is on (decision 11.9).

5. Configure each calling service with the `servicetoken` settings that the command prints: the token URL, the client ID, the audience, and the key file. `servicetoken.Config.Key` takes a parsed key, so the service reads the key file and passes its bytes to `servicetoken.ParsePrivateKeyPEM`. A service that runs in a container mounts the key file read-only.

### Use one issuer URL

The mock builds the `iss` claim from the host of the token request, and accounts compares `iss` exactly. Accounts and every caller must therefore reach the mock as `http://mock-oauth2:8091`. A token fetched through another host name, such as `localhost`, gets `401`.

Accounts, or a caller, that runs outside compose needs this line in its hosts file:

```
127.0.0.1 mock-oauth2
```

### What the local path tests

The mock gives a client credentials token the same claims as a deployed issuer: `sub` is `<client ID>@clients`, `azp` is the client ID, `gty` is `client-credentials`, and `aud` is the requested audience. The token has no `email` and lasts one hour. An authorization code login on the same issuer gives a user token with `email` and no `gty`, which accounts treats as a user token.

The other grant types that the mock offers (`password`, `urn:ietf:params:oauth:grant-type:jwt-bearer`, `refresh_token`, and token exchange) are not mapped in the config. The mock gives them a token whose `sub` the requester chooses (or no `sub` for `refresh_token`), with no `aud`, `azp`, `gty`, or `email`. Accounts refuses these tokens with `401` on the client path and on the user path, because they have no `aud`.

The local path does not test the identity provider's check of the client assertion. The mock accepts any assertion, so any process on the laptop that reaches port 8091 can get a client token for any local client. It can also get an authorization code user token, whose `sub` is `local-user`, and tokens with a `sub` it chooses from the unmapped grant types. Accounts does not accept those tokens today only because they have no `aud`. For these reasons, compose publishes the mock on `127.0.0.1` only, and accounts trusts the mock only with `REEARTH_ACCOUNTS_LOCAL_ISSUER` (ADR-0002, accepted risk 6).

The keys are for development only. Never register them, or the mock issuer, in a deployed environment. A deployed server refuses the mock issuer. The issuer is an `http` URL on a single-label host, which passes only with `REEARTH_ACCOUNTS_LOCAL_ISSUER`, and the server refuses that switch on Cloud Run and Kubernetes.

`make test-integration` runs `TestLocalIssuer`, which starts the pinned image with this config and checks the token shape, a call that acts for an integration, the refusal without the switch, the refusal of a token fetched through another host name, a user token that is accepted as its user without `X-Reearth-Act-As` and refused with it, and the refusal of tokens from the unmapped grant types.
