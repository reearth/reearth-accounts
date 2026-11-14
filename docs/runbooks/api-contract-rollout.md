# Rolling out an API contract change

Use this runbook when a change to reearth-accounts alters what a client sends or receives: a field becomes private, a query is removed or loses `@public`, a response field changes shape, or a client must send a new header. It implements [ADR-0003](../adr/0003-personal-data-protection.md) decision 12.

The rule is: clients first, accounts last. A client that tolerates both the old and the new contract can be deployed at any time. Accounts can change the contract only after every client tolerates the new one.

## Before you start

1. List every client of the changed field or route. Search the organisation's repositories for the field name and for `reearth-accounts/server/pkg/gqlclient` imports.
2. For each client, confirm that a released version accepts the new contract. If none does, change the client first and release it.
3. Record the client versions in the accounts PR description.

## Per environment

Repeat these steps in each environment, starting with the least critical one. Finish one environment before you start the next.

1. Deploy each client at the version that accepts the new contract.
2. Confirm that each client's new revision serves traffic. Check that the old revision receives no traffic and that the new revision's error rate is normal.
3. Deploy accounts with the contract change.
4. Watch the error rate of accounts and of each client for at least 30 minutes. Look for new 4xx responses and GraphQL errors on the changed field.

## If something breaks

Roll back accounts first. The clients accept both contracts, so they can stay on their new version. Do not roll back a client while accounts still serves the new contract.
