# Add UpdateMeOIDC GraphQL Mutation

## Document Signature

|           |        |
|-----------|--------|
| Creator   | [name] |
| Leader    | [name] |
| Task Link | [url]  |
| Developer | [name] |

## Background / Problem Statement

Currently, the `updateMe` mutation in the reearth-accounts service handles user profile updates for both local authentication and external OIDC providers (Auth0). However, there is a need to differentiate behavior between these two authentication flows:

1. **Current Situation**: The existing `updateMe` mutation validates the current password before allowing updates (specifically for password changes). This is appropriate for locally authenticated users but creates friction for OIDC-authenticated users who may not have a local password set.

2. **Problem**: OIDC-authenticated users (e.g., those using Auth0, Google, or other identity providers) should be able to update their profile information without the current password validation requirement. The Auth0 provider handles password changes independently, so the local password validation step is unnecessary and potentially blocking for OIDC users.

3. **Impact**: Without a dedicated OIDC update endpoint, OIDC users may be unable to update their profile information properly, or the system needs complex conditional logic within a single mutation.

## Goals

1. Provide a dedicated GraphQL mutation `updateMeOIDC` specifically designed for OIDC-authenticated users
2. Allow OIDC users to update profile fields (name, email, lang, theme, password) without current password validation
3. Synchronize profile changes with Auth0 when the user has an auth0 provider configured
4. Maintain consistency with the existing `updateMe` mutation behavior for shared functionality (workspace renaming, metadata updates)
5. Ensure backward compatibility - existing `updateMe` mutation continues to work unchanged

## Non-Goals

1. Migration of existing users from `updateMe` to `updateMeOIDC` - clients decide which endpoint to use
2. Support for identity providers other than Auth0 in the initial implementation
3. Automatic detection of OIDC vs local users at the API level - client determines the appropriate mutation
4. Email verification flow changes - handled separately by Auth0

## Functional Requirements

1. **Input Fields**: The mutation accepts the following optional fields:
   - `name`: User display name (String)
   - `email`: User email address (String)
   - `lang`: Preferred language (Lang enum)
   - `theme`: UI theme preference (Theme enum)
   - `password`: New password (String)
   - `passwordConfirmation`: Password confirmation (String)

2. **Validation**:
   - Password and passwordConfirmation must match when provided
   - No current password validation required (unlike `updateMe`)

3. **Auth0 Synchronization**:
   - When name, email, or password is updated, sync changes to Auth0 if user has auth0 provider
   - Only call Auth0 API when relevant fields are changed

4. **Workspace Update**:
   - When name changes, update personal workspace name if it matches the old user name

## Solution Options

### Option 1: Separate UpdateMeOIDC Mutation (Chosen)

Create a new dedicated mutation `updateMeOIDC` that handles OIDC user profile updates without current password validation.

**Benefits**:
- Clear separation of concerns between local and OIDC authentication flows
- No conditional logic complexity in a single mutation
- Explicit API contract for OIDC clients
- Easy to extend with OIDC-specific logic in the future

**Drawbacks**:
- Code duplication with `updateMe` for shared logic (workspace updates, metadata)
- Two mutations to maintain

### Option 2: Add OIDC Flag to Existing UpdateMe Mutation

Add an optional `skipPasswordValidation` or `isOIDC` flag to the existing `updateMe` mutation.

**Benefits**:
- Single mutation to maintain
- Smaller API surface

**Drawbacks**:
- Mixed concerns in a single mutation
- Potential security risk if flag is misused
- More complex conditional logic

**Selected Option**: Option 1 - Separate mutations provide clearer security boundaries and simpler implementation.

## Design

### Sequence Diagram

```
┌────────┐          ┌─────────────────┐          ┌──────────┐          ┌───────┐
│ Client │          │ GraphQL Resolver│          │ User UC  │          │ Auth0 │
└───┬────┘          └────────┬────────┘          └────┬─────┘          └───┬───┘
    │                        │                        │                    │
    │ updateMeOIDC(input)    │                        │                    │
    ├───────────────────────>│                        │                    │
    │                        │                        │                    │
    │                        │ UpdateMeOIDC(param)    │                    │
    │                        ├───────────────────────>│                    │
    │                        │                        │                    │
    │                        │                        │ FindByID(userID)   │
    │                        │                        ├──────────────┐     │
    │                        │                        │              │     │
    │                        │                        │<─────────────┘     │
    │                        │                        │                    │
    │                        │                        │ Validate passwords │
    │                        │                        ├──────────────┐     │
    │                        │                        │              │     │
    │                        │                        │<─────────────┘     │
    │                        │                        │                    │
    │                        │                        │ Update user fields │
    │                        │                        ├──────────────┐     │
    │                        │                        │              │     │
    │                        │                        │<─────────────┘     │
    │                        │                        │                    │
    │                        │                        │ UpdateUser(auth0)  │
    │                        │                        ├───────────────────>│
    │                        │                        │                    │
    │                        │                        │      OK/Error      │
    │                        │                        │<───────────────────│
    │                        │                        │                    │
    │                        │                        │ Save user & workspace
    │                        │                        ├──────────────┐     │
    │                        │                        │              │     │
    │                        │                        │<─────────────┘     │
    │                        │                        │                    │
    │                        │      Updated User      │                    │
    │                        │<───────────────────────│                    │
    │                        │                        │                    │
    │  UpdateMePayload       │                        │                    │
    │<───────────────────────│                        │                    │
    │                        │                        │                    │
```

### Files Changed

| File | Change Type | Description |
|------|-------------|-------------|
| `schemas/user.graphql` | Modified | Added `UpdateMeOIDCInput` type and `updateMeOIDC` mutation |
| `internal/adapter/gql/generated.go` | Generated | Auto-generated GraphQL resolver code |
| `internal/adapter/gql/gqlmodel/models_gen.go` | Generated | Auto-generated input model |
| `internal/adapter/gql/resolver_mutation_user.go` | Modified | Added `UpdateMeOidc` resolver method |
| `internal/usecase/interfaces/user.go` | Modified | Added `UpdateMeOIDCParam` struct and interface method |
| `internal/usecase/interactor/user.go` | Modified | Implemented `UpdateMeOIDC` use case logic |
| `internal/usecase/interactor/user_signup_test.go` | Modified | Added comprehensive tests for `UpdateMeOIDC` |
| `internal/usecase/proxy/user.go` | Modified | Added proxy method for cross-service calls |
| `internal/usecase/proxy/operations.graphql` | Modified | Added GraphQL operation for proxy |
| `internal/usecase/proxy/generated.go` | Generated | Auto-generated proxy code |

### GraphQL Schema Addition

```graphql
input UpdateMeOIDCInput {
  name: String
  email: String
  lang: Lang
  theme: Theme
  password: String
  passwordConfirmation: String
}

extend type Mutation {
  updateMeOIDC(input: UpdateMeOIDCInput!): UpdateMePayload
}
```

### Key Implementation Details

1. **No Current Password Validation**: Unlike `updateMe`, the `updateMeOIDC` mutation does not require `oldPassword` validation.

2. **Password Confirmation Only**: When updating password, only `password` and `passwordConfirmation` must match.

3. **Auth0 Sync**: Changes to name, email, or password trigger an Auth0 API call to synchronize the user profile.

4. **No Local Password Storage**: Password is NOT stored locally for OIDC users. Password updates are only sent to Auth0 API. This is intentional as OIDC users authenticate through their identity provider, not through local credentials.

## Potential Impact

1. **Auth0 API Calls**: Increased API calls to Auth0 when users update name, email, or password through this endpoint.

2. **No Local Password for OIDC Users**: OIDC users will not have passwords stored locally. They must authenticate through their identity provider (Auth0).

3. **Client Migration**: Clients using Auth0/OIDC should migrate from `updateMe` to `updateMeOIDC` for proper handling.

## Test Plan

### Unit Tests Added

| Test Case | Description | Expected Result |
|-----------|-------------|-----------------|
| Update name only | Update user name with Auth0 provider | Name updated, Auth0 called, workspace renamed |
| Update email only | Update user email with Auth0 provider | Email updated, Auth0 called |
| Update password | Update password with matching confirmation | Password sent to Auth0, NOT stored locally |
| Update lang and theme | Update metadata fields only | Metadata updated, Auth0 NOT called |
| Password mismatch | Password and confirmation don't match | `ErrUserInvalidPasswordConfirmation` returned |
| No Auth0 provider | Update user without auth0 provider | Updates applied, Auth0 NOT called |
| Authenticator error | Auth0 returns an error | Error propagated to client |
| Invalid operator | No user in operator | `ErrInvalidOperator` returned |

### Manual Testing

1. Call `updateMeOIDC` mutation with JWT from OIDC user
2. Verify profile changes reflected in database
3. Verify Auth0 Management API received update call
4. Verify workspace name updated when user name changes

## Deployment Plan

1. **Backward Compatibility**: Yes - existing `updateMe` mutation unchanged
2. **Partial Deployment**: Yes - new mutation is additive
3. **Notification**: Frontend teams should be notified about the new mutation availability
4. **Configuration Changes**: None required
5. **DDL/DML**: None required

## Rollback Plan

1. Notify the issue to Slack channel
2. Revert the code changes (the mutation is additive, so removal is safe)
3. Frontend teams revert to using `updateMe` mutation
4. No data migration required as user data structure unchanged

## Post Deployment

- **Checklist**:
  - Verify `updateMeOIDC` mutation accessible via GraphQL Playground
  - Test with OIDC-authenticated user
  - Verify Auth0 synchronization working

- **Metrics**:
  - Number of `updateMeOIDC` calls vs `updateMe` calls
  - Auth0 API call success/failure rate
  - Latency of mutation (should be similar to `updateMe`)

- **Alerting**:
  - Auth0 API failure rate:
    - Warning: > 1%
    - Danger: > 5%
  - Mutation error rate:
    - Warning: > 1%
    - Danger: > 5%

## Reviewed by

- Technical Architect: [name]
- Technical Leader: [name]
- Peers: [name]
- Peers: [name]
- Peers: [name]
- Peers: [name]
