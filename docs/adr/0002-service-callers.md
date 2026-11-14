# ADR-0002: Service callers for the accounts API

- **Status:** Accepted
- **Date:** 2026-11-10
- **Deciders:** @pyshx

## Context

Accounts accepts only end-user tokens. Work with no user behind it therefore goes around accounts:

1. Some services read the accounts database directly, which skips every authorization rule.
2. Flow's scheduled triggers arrive with no user, so the accounts permission check fails them.

Accounts has two static keys, `REEARTH_ACCOUNTS_REST_API_KEY` and `REEARTH_ACCOUNTS_SYNC_SSO_API_KEY`. A static key is one long-lived shared secret with no identity, no scope and no expiry.

Callers run on different platforms: Cloud Run, and Cloudflare Workers for Re:Earth Serve. An open-source deployment can use its own OIDC provider. The identity provider is configured in terraform for each environment.

ADR-0001 left machine-to-machine access open. Calls between other services are out of scope and can reuse this pattern.

Options considered and rejected:

- **Cloud runtime identity, such as Google ID tokens.** It does not work on Cloudflare, so there would be two schemes.
- **The static keys.** They have no identity, scope or expiry.
- **Direct database access.** It skips authorization, and one leaked credential exposes every tenant.
- **A self-hosted issuer, such as Ory Hydra.** It is one more stateful service to run in every environment.
- **SPIFFE/SPIRE.** It needs its own infrastructure on every platform.

## Decision

### 1. A service authenticates with OAuth client credentials and a private key

A service gets an access token for the accounts audience with the client credentials grant. It authenticates to the identity provider with `private_key_jwt` (RFC 7523), and a client secret is not accepted.

The service fetches the token on its first call that has no user behind it, and caches it in memory until it expires. It sends the token as `Authorization: Bearer`. Calls made for a user keep forwarding the user's token.

### 2. Accounts verifies the token strictly

Accounts accepts a service token only when all of these checks pass:

1. `iss` exactly matches the configured issuer.
2. The signature verifies against that issuer's published keys.
3. The audience is the accounts audience.
4. The token is unexpired.
5. The token was issued by the client credentials grant and carries no user.
6. Its client ID is registered (decision 3).

A request carries a user token or a service token, never both. A service caller cannot act as a user.

### 3. Callers are registered in terraform

For each calling service, in each environment that needs it, terraform creates:

1. The identity provider client.
2. The client's private key.
3. A grant for the accounts audience only.
4. The private key as a secret for the service.
5. An entry in `REEARTH_ACCOUNTS_SERVICE_CALLERS` that maps the client ID to an accounts integration.

Client IDs match exactly, with no wildcards, and an unregistered client gets `401`. No token is stored.

Keys rotate in stages:

1. Terraform registers the new public key alongside the old one.
2. Callers switch to the new private key and redeploy.
3. Terraform removes the old public key.

### 4. Workspace membership scopes a service

A registered client acts as an accounts integration. It can reach only the workspaces where that integration is a member, with that membership's role. A service with no membership sees nothing.

### 5. Service callers use REST and GraphQL, for reads and writes

Service callers are accepted on both REST and GraphQL, for every read and write. The integration's role in each workspace decides what a call may do, as it does for a user (decision 4).

### 6. Every service call is attributable

Accounts logs the client ID and integration on every service call.

### 7. Static keys and direct database access are retired

1. The two static keys are removed one release after this lands.
2. Each direct database reader moves to the API in its own repository.

## Consequences

- Adding a caller means adding terraform for each environment it runs in. An environment with no caller gets no client and no secret.
- Private keys are in terraform state, so access to the state stays restricted.
- Accounts stores no secret for service callers. It caches the issuer's public keys.
- Identity providers can meter client credentials tokens. Lazy fetching and per-instance caching keep the volume low. A token shared across instances is the fallback if a tenant's quota is exceeded.
- Removing the static keys breaks any unknown caller of them.
- A service that moves off direct database reads can see less data than before, because it is now authorized.
- Both auth middlewares, REST and GraphQL, accept service tokens. A write by a service is limited by its integration's workspace role, as a user's write is.

## Open question

None.
