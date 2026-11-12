# ADR-0002: Service callers for the accounts API

- **Status:** Proposed
- **Date:** 2026-11-10
- **Deciders:** @pyshx

## Context

Every call to the accounts API today carries an end user's token. Flow, Visualizer, CMS, LINKS-Veda and the Serve adapter forward the caller's JWT. Accounts cannot represent a caller that is a service, so work with no user behind it goes around accounts:

1. Flow, Visualizer, CMS, reearth-cloud and the LINKS-Veda scripts read the accounts database directly, which skips every authorization rule.
2. Flow's scheduled triggers arrive with no user, so the accounts permission check fails them.

Accounts already has two static keys, `REEARTH_ACCOUNTS_REST_API_KEY` and `REEARTH_ACCOUNTS_SYNC_SSO_API_KEY`. No repository we could read calls them. A static key is one long-lived shared secret with no identity, no scope and no expiry.

The callers do not share a platform. Flow, Visualizer, CMS and reearth-cloud run on Cloud Run. Re:Earth Serve runs on Cloudflare Workers, and anyone can deploy it as open source against their own OIDC provider.

Auth0 is already managed in terraform in every environment (`reearth-infrastructure`): the accounts API audience (`auth0_resource_server`), machine-to-machine applications (`auth0_client`, `app_type = "non_interactive"`), grants (`auth0_client_grant`) and client authentication (`auth0_client_credentials`). The provider supports `private_key_jwt` with a registered public key.

ADR-0001 left machine-to-machine access to accounts open. Services calling other services, such as Serve and the internal gRPC APIs that reearth-dashboard calls, have the same problem. They are out of scope here and can reuse this pattern in their own ADRs.

## Decision

### 1. A service authenticates with Auth0 client credentials and a private key

A calling service gets an access token for the accounts API audience from Auth0 with the client credentials grant. It authenticates to Auth0 with `private_key_jwt` (RFC 7523): it signs the token request with its own private key, and Auth0 holds only the public key. A `client_secret` is not accepted. The service sends the token as `Authorization: Bearer`, as a user token is sent, and caches it until it expires.

Service tokens use the accounts audience that user tokens already use. No new audience or API is created.

An open-source deployment uses its own OIDC issuer in the same way.

### 2. Accounts verifies the token strictly

Accounts accepts a service token only when every check passes:

1. The signature verifies against the configured issuer's published keys.
2. The audience is the accounts audience.
3. The token is unexpired.
4. It is a client-credentials token, with no user.
5. Its client ID is registered (decision 3).

A request carries a user token or a service token, never both. A service caller cannot act as a user, and the debug impersonation headers stay off outside development.

### 3. Everything is in terraform, per environment

For each calling service in each environment, terraform creates:

1. An `auth0_client` with `app_type = "non_interactive"`.
2. An `auth0_client_credentials` with `authentication_method = "private_key_jwt"` and the public key from a `tls_private_key`.
3. An `auth0_client_grant` for the accounts audience only.
4. The private key as a secret for that service: Secret Manager on Cloud Run, a Worker secret for Serve.
5. An entry in accounts' `REEARTH_ACCOUNTS_SERVICE_CALLERS` setting that maps the client ID to an accounts integration, built from the client's output.

The existing `auth0_m2m` module grows inputs for the audience, the grant and `private_key_jwt`, so a new caller is one module block. Nothing is created by hand, and no token is stored anywhere. Registration matches client IDs exactly, with no wildcards. An unregistered client gets `401`. Rotating a key means replacing the `tls_private_key` and applying: the public key on Auth0 and the secret change in the same apply.

### 4. Workspace membership scopes a service

A registered client is an accounts integration. Its operator has no user and holds the workspaces where that integration is a member, with that membership's role. The existing integration member routes and mutations grant and revoke that access. A service with no membership sees nothing.

### 5. Start with reads on REST

Service callers can use the REST workspace and member reads first: `GET /api/workspaces/:id`, `GET /api/workspaces`, and the member data they return. Each write needs a later decision that names it. GraphQL follows in a separate change, because its auth middleware is separate.

### 6. Every service call is attributable

Accounts logs the caller's client ID and integration on every service call.

### 7. Static keys and direct database access are retired

1. The two static keys are removed once this lands, since nothing calls them.
2. Each direct accounts database reader moves to the API in its own repository, one at a time, starting with reearth-cloud, which has no other route.

## Consequences

- Each calling service needs one module block per environment. Terraform owns the Auth0 client, the key and the grant.
- The private key is in terraform state, so the state bucket stays restricted to the infrastructure team.
- Accounts verifies the issuer's public keys and caches them. It stores no secret for service callers.
- Each caller needs one token, for one audience, cached for its lifetime. Auth0 counts these tokens against the tenant's monthly quota, so the volume stays small as long as callers cache.
- Removing the static keys breaks any unknown caller of them. The removal PR names the keys and waits one release.
- Data that today comes from direct database reads gains authorization checks, so a migrating service can see less than before. That is the point.

## Alternatives considered

1. **Google runtime identity for Cloud Run services.** No secret to store on GCP, but Serve on Cloudflare cannot use it, so it would mean two issuers and two registration schemes.
2. **The existing static keys.** No identity, scope or expiry.
3. **Direct database access.** Skips every authorization rule. A leaked credential reads and writes every tenant.
4. **A self-hosted Ory Hydra as the issuer.** No token quota, but a service with a database to run in every environment. Auth0 already does this job and is already in terraform. Revisit if token volume or cost grows.
5. **SPIFFE/SPIRE.** Workload identity without stored secrets, but infrastructure to run in every environment and on Cloudflare.

## Open question

Which writes, if any, should service callers get, and in what order? Flow's scheduler is the first candidate.
