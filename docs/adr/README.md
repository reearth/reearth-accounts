# Architecture decision records

An ADR records one significant decision: the context that forced it, what was decided, and what follows from it. Write one before the code for a change that adds or changes an API contract, an authorization rule, a data model or schema, a cross-service integration, or a behavior other teams depend on. Bug fixes and refactors that keep behavior need none.

## How to write one

1. Copy the template below into `docs/adr/NNNN-short-title.md`, using the next free number.
2. Open it as its own PR, or as the first PR of the stack that implements it, with **Status: Proposed**.
3. Implementing PRs cite it as `ADR-NNNN decision N`.
4. Set **Status: Accepted** when it merges. To change an accepted decision, write a new ADR that supersedes it and set the old one to **Superseded by ADR-NNNN**.

Keep it short. State the decision and its consequences, not the history of the discussion.

## Index

| ADR | Title | Status |
|---|---|---|
| [0001](0001-rest-parity-with-graphql.md) | REST parity with GraphQL | Proposed |
| [0003](0003-personal-data-protection.md) | Personal data protection in the accounts API | Accepted |

## Template

```markdown
# ADR-NNNN: Title

- **Status:** Proposed | Accepted | Superseded by ADR-NNNN
- **Date:** YYYY-MM-DD
- **Deciders:** @handle

## Context

The problem, the constraints, and the facts that force a decision.

## Decision

What is decided, as numbered decisions that implementing PRs can cite.

## Consequences

What changes for clients, operators and maintainers, including what breaks and what to do first.

## Open question

Anything deliberately left undecided, and who decides it.
```
