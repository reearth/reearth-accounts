# Token issuer configuration

## Startup check on trusted URLs

The server refuses to start when a URL that it trusts for tokens is not an `https` URL on a DNS name. The check implements ADR-0002 decision 11, items 6 and 7, and runs every time the server reads its configuration, in every build.

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

