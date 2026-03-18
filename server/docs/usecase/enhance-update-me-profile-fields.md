# Enhance UpdateMe Mutation with Additional Profile Fields

## Document Signature

|           |                                      |
|-----------|--------------------------------------|
| Creator   | Developer                            |
| Leader    | -                                    |
| Task Link | -                                    |
| Developer | Developer                            |

## Background / Problem Statement

The current `UpdateMe` GraphQL mutation only supports updating basic user profile fields: `name`, `email`, `lang`, `theme`, `password`, and `passwordConfirmation`. Users need the ability to update additional profile information to enrich their user profiles and personal workspaces.

Current limitations:
1. Users cannot set a unique alias/username for their profile
2. No support for profile descriptions
3. No support for website URLs
4. No support for profile photo URLs

These fields are already supported in the domain model (`user.User` and `user.Metadata`) but are not exposed through the `UpdateMe` mutation.

## Goals

1. Extend the `UpdateMe` mutation to support updating `alias`, `description`, `website`, and `photoURL` fields
2. Synchronize profile metadata fields (description, website, photoURL) with the user's personal workspace
3. Validate alias uniqueness to prevent duplicate usernames
4. Maintain backward compatibility with existing API consumers

## Non-Goals

1. Adding validation rules for description/website/photoURL content (URL format validation, content length limits)
2. Adding image upload functionality - only URL references are supported
3. Modifying the user registration flow

## Functional Requirements

1. The `UpdateMe` mutation must accept the following new optional fields:
   - `alias: String` - User's unique alias/username
   - `description: String` - User profile description
   - `website: String` - User's website URL
   - `photoURL: String` - User's profile photo URL
2. Alias must be unique across all users
3. When updating description, website, or photoURL, the changes must also be applied to the user's personal workspace metadata
4. All new fields are optional and do not affect existing fields if not provided

## Solution Options

### Option 1 (Selected): Extend UpdateMeInput with new fields

1. Update GraphQL schema (`schemas/user.graphql`):
   - Add `alias`, `description`, `website`, `photoURL` fields to `UpdateMeInput`
   - Reorder fields alphabetically per project conventions

2. Update use case interfaces (`internal/usecase/interfaces/user.go`):
   - Add corresponding fields to `UpdateMeParam` struct

3. Update interactor (`internal/usecase/interactor/user.go`):
   - Add alias uniqueness validation
   - Update user fields from input parameters
   - Synchronize metadata fields with personal workspace

4. Regenerate GraphQL code and update gqlclient

**Benefits:**
- Minimal changes to existing code structure
- Follows established patterns in the codebase
- Clear separation between user-specific (alias) and shared (metadata) fields

**Drawbacks:**
- Personal workspace sync adds complexity

## Design

```
Client Request
      |
      v
+------------------+
| GraphQL Mutation |
| updateMe         |
+------------------+
      |
      v
+------------------+
| UpdateMe         |
| Interactor       |
+------------------+
      |
      +-- Validate alias uniqueness (if provided)
      |        |
      |        v
      |   +------------------+
      |   | User Repository  |
      |   | FindByAlias      |
      |   +------------------+
      |
      +-- Update User fields
      |        |
      |        v
      |   +------------------+
      |   | User Entity      |
      |   | UpdateAlias,     |
      |   | Metadata updates |
      |   +------------------+
      |
      +-- Sync Personal Workspace (if metadata changed)
      |        |
      |        v
      |   +------------------+
      |   | Workspace Repo   |
      |   | FindByID, Save   |
      |   +------------------+
      |
      v
+------------------+
| Save User        |
| User Repository  |
+------------------+
      |
      v
Response
```

## Potential Impact

1. **Database**: Additional queries for alias uniqueness check and personal workspace updates
2. **API Consumers**: New optional fields available - fully backward compatible
3. **Data Consistency**: Personal workspace metadata will be kept in sync with user metadata

## Test Plan

1. **Unit Tests** (`internal/usecase/interactor/user_test.go`):
   - Test updating each new field individually
   - Test updating multiple fields together
   - Test alias uniqueness validation (duplicate alias rejected)
   - Test personal workspace metadata synchronization

2. **E2E Tests** (`e2e/gql_user_test.go`):
   - Test GraphQL mutation with new fields
   - Verify response includes updated fields
   - Verify personal workspace is updated

## Deployment Plan

1. Does this feature have backward compatibility? **Yes** - All new fields are optional
2. Can we make partial deployment? **Yes** - Single service update
3. Who needs to be notified about this deployment? API consumers may want to know about new fields
4. What configuration changes are needed? **None**
5. Is there any DDL or DML needed before deployment? **No**

## Rollback Plan

1. Notify the cause to Slack channel
2. Stop deployment
3. Revert changes to previous version
4. No data cleanup needed as fields are additive

## Post Deployment

- Checklist
    - Verify updateMe mutation accepts new fields via GraphQL playground
    - Verify personal workspace metadata is synchronized
    - Check existing updateMe calls continue to work
- Metrics
    - Monitor API error rates
    - Track usage of new fields
- Alerting
    - Success rate:
        - Warning: < 99%
        - Danger: <= 95%

## Reviewed by

- Technical Architect: [pending]
- Technical Leader: [pending]
- Peers: [pending]