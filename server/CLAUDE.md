# Claude Code Configuration - Re:Earth Accounts Server

This file contains server-specific information and preferences for Claude Code development tasks.

## Purpose & Background

This repository serves as the centralized account management system for Re:Earth's microservice-based architecture. It handles **users, workspaces, roles, and permission evaluations** across all services.

Instead of each microservice managing user and authorization logic independently, this service consolidates identity and access management, reducing duplication and enforcing consistent permission checks.

## System Overview

- Exposes GraphQL APIs that other services call for user, workspace, role, permittable and permission evaluation
- Manages CRUD for users, workspaces, roles, and their assignments
- Evaluates permissions using Cerbos authorization engine

## DDD (Domain-Driven Design) Architecture

This application follows a well-structured DDD architecture with clear separation of concerns:

### Directory Structure
```
server/
├── cmd/                     # Application entry points
├── pkg/                     # Domain layer (core business logic)
├── internal/
│   ├── usecase/            # Application layer (use cases)
│   ├── infrastructure/     # Infrastructure layer
│   ├── adapter/            # Presentation layer (GraphQL)
│   └── app/                # Application configuration
├── schemas/                # GraphQL schema definitions
└── e2e/                    # End-to-end tests
```

### Layer Responsibilities

**Domain Layer (`pkg/`)**
- Contains domain entities: User, Workspace, Role, Permittable
- Pure business logic with no external dependencies
- Repository interfaces defined here
- Value objects and domain services
- Builder pattern for complex object construction

**Application Layer (`internal/usecase/`)**
- Orchestrates domain objects
- Implements business use cases via interactors (User, Workspace, Role, Permittable, cerbos)
- Uses dependency injection for repository interfaces
- Input/Output DTOs for external contracts
- Gateway interfaces for external services

**Infrastructure Layer (`internal/infrastructure/`)**
- MongoDB repository implementations (`mongo/`)
- In-memory implementations for testing (`memory/`)
- External service integrations (Auth0, Cerbos)
- Database migrations and connection management

**Adapter Layer (`internal/adapter/`)**
- GraphQL resolvers using gqlgen
- Generated models and dataloaders
- Request/response handling and validation

### Key DDD Patterns Implemented
- ✅ Aggregate Roots with clear boundaries (User, Workspace, Role, Permittable)
- ✅ Repository Pattern with domain interfaces
- ✅ Domain Services for complex business logic
- ✅ Dependency Inversion (high-level modules independent of low-level)
- ✅ Gateway Pattern for external services (Auth0, Cerbos)
- ✅ Builder/Factory patterns for object construction
- ✅ Bounded Contexts (identity management, authorization, workspace management)

## Notes
- Uses GraphQL schema-first development
- Authentication via JWT tokens (Auth0)
- Multi-language support (currently English and Japanese)
- Centralized authorization for all Re:Earth microservices
- DDD architecture with proper domain separation
- Automatic database migrations on startup

## Key Components
- Echo HTTP framework for GraphQL endpoint
- MongoDB for data persistence
- Auth0 JWT authentication
- Cerbos authorization engine
- gqlgen for schema-first GraphQL development

## Development Guidelines
- Follow Go conventions and DDD patterns
- Use context for request handling
- Implement proper error handling
- Use schema-first approach for GraphQL development
- Follow domain-driven design principles

## Authentication & Authorization
- JWT authentication via Auth0
- Permission evaluation via Cerbos
- Centralized authorization for all Re:Earth services
- Role-based access control (RBAC)

## Development Commands (via Makefile)

### Core Development
- `make dev` - Run with hot reloading using Air
- `make run-app` - Run the application directly
- `make run-cerbos` - Start Cerbos authorization server

### Testing
- `make test` - Run unit tests
  - Use `REEARTH_DB=mongodb://localhost:27017` for custom MongoDB URL
  - Use `TARGET_TEST=./pkg/user` for specific package testing

### Code Quality
- `make generate` - Run go generate for all packages
- `make gql` - Generate GraphQL code and dataloaders

### External Dependencies
- `make run-cerbos` - Start Cerbos authorization server via Docker Compose

## Database Management
- MongoDB with automatic migrations
- Migrations located in `internal/infrastructure/mongo/migration/`
- Naming pattern: `YYMMDDHHMMSS_description.go`
- Migrations run automatically on startup
- Schema management: See [internal/infrastructure/mongo/SCHEMA.md](internal/infrastructure/mongo/SCHEMA.md)

### MongoDB Schema Changes

When adding or modifying MongoDB collection schemas, follow these steps:

#### 1. Update the Go struct (mongodoc)

Add or modify the document struct in `internal/infrastructure/mongo/mongodoc/`:

```go
type UserDocument struct {
    ID    string `json:"id" jsonschema:"required,description=User ID (ULID format)"`
    Name  string `json:"name" jsonschema:"required,description=User display name"`
    Email string `json:"email" jsonschema:"description=User email address. Default: \"\""`
}
```

- Use `json` tag for field name (lowercase)
- Use `jsonschema` tag with:
  - `required` - marks the field as required in the schema
  - `description=` - field documentation
- Include default values in description for optional fields: `Default: ""`

#### 2. Register schema (if adding new collection)

Add the schema registration in `tools/cmd/mongoschemagen/registry.go`:

```go
func RegisterSchemas(g *Generator) {
    g.RegisterSchema(
        "newcollection",                              // collection name
        mongodoc.NewCollectionDocument{},             // Go struct type
        "NewCollection Collection Schema",            // title
        "Schema for newcollection documents",         // description
    )
}
```

Note: `required` fields are automatically extracted from struct tags (`jsonschema:"required,..."`)

#### 3. Generate JSON schema files

```bash
make update-schema-json
```

This command:
- Generates JSON schema files from Go structs (`mongoschemagen`)
- Updates the ER diagram (`ergen`)

#### 4. Create migration file

Create `internal/infrastructure/mongo/migration/YYMMDDHHMMSS_description.go`:

```go
package migration

import "context"

func ApplyNewCollectionSchema(ctx context.Context, c DBClient) error {
    return ApplyCollectionSchemas(ctx, []string{"newcollection"}, c)
}
```

Register in `migrations.go`:

```go
var migrations = migration.Migrations[DBClient]{
    YYMMDDHHMMSS: ApplyNewCollectionSchema,
}
```

#### Related Files
- Schema JSON files: `internal/infrastructure/mongo/schema/*.json`
- ER diagram: `internal/infrastructure/mongo/schema/ER.md`
- Schema generator: `tools/cmd/mongoschemagen/`
- ER diagram generator: `tools/cmd/ergen/`

## GraphQL Schema Management
- Schema files in `schemas/` directory
- Combined schema generation via `gqlgen.yml`
- Automatic code generation for resolvers and models
- Dataloader generation for efficient N+1 query resolution

## Configuration
- Environment-based configuration using `envconfig`
- See `internal/app/config.go` for available options
- Support for development and production environments

## Code Organization Rules

### Alphabetical Ordering
- **Function names**: Define functions in alphabetical order within files
- **Object properties**: Order struct fields alphabetically
- **Interface methods**: List methods alphabetically in interface definitions
- **Import statements**: Keep imports in alphabetical order (Go convention)
- **GraphQL schema files**: Schema files are organized alphabetically in `schemas/` directory

### Examples
```go
// Struct fields in alphabetical order
type Container struct {
    Cerbos      Cerbos
    Permittable Permittable
    Role        Role
    User        User
    Workspace   Workspace
}

// Domain entity fields in alphabetical order
type User struct {
    auths         []Auth
    email         string
    host          string
    id            ID
    metadata      Metadata
    name          string
    password      EncodedPassword
    passwordReset *PasswordReset
    verification  *Verification
    workspace     WorkspaceID
}

// Functions in alphabetical order
func (u *User) Alias() string { ... }
func (u *User) Email() string { ... }
func (u *User) ID() ID { ... }
func (u *User) Name() string { ... }
```

### GraphQL Schema Organization
- **Schema files**: Organized by domain (`user.graphql`, `workspace.graphql`, `role.graphql`, etc.)
- **Type definitions**: Follow alphabetical ordering within each schema file
- **Shared types**: Common types and interfaces in `_shared.graphql`
- **Schema combination**: All schemas combined via `gqlgen.yml` configuration

### Domain Layer Organization
- **Package structure**: Each domain entity has its own package (`pkg/user/`, `pkg/workspace/`)
- **Builder pattern**: Each domain entity has a corresponding builder (`user_builder.go`, `workspace_builder.go`)
- **Value objects**: Related value objects grouped within entity packages
- **ID types**: Domain-specific ID types defined in `pkg/id/`

### Use Case Layer Organization
- **Interfaces**: Use case interfaces defined in `internal/usecase/interfaces/`
- **Interactors**: Business logic implementation in `internal/usecase/interactor/`
- **Repositories**: Repository interfaces in `internal/usecase/repo/`
- **Gateways**: External service interfaces in `internal/usecase/gateway/`

## Context Handling Rules

### Context Value Access
- **Adapter Layer**: Access context values (user, operator, auth info) in the adapter layer (GraphQL resolvers)
- **Explicit Parameters**: Pass extracted values as explicit parameters to lower layers
- **Clean Dependencies**: Avoid context value reading in use cases and domain layers
- **Improved Testability**: Makes functions easier to test and understand

### Context Functions
```go
// Context attachment functions (adapter layer)
func AttachUser(ctx context.Context, u *user.User) context.Context
func AttachOperator(ctx context.Context, o *usecase.Operator) context.Context
func AttachUsecases(ctx context.Context, u *interfaces.Container) context.Context

// Context extraction functions (adapter layer)
func User(ctx context.Context) *user.User
func Operator(ctx context.Context) *usecase.Operator
func Usecases(ctx context.Context) *interfaces.Container
```

### Examples
```go
// ✅ GOOD: Extract in adapter layer, pass as parameter
func (r *Resolver) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
    operator := adapter.Operator(ctx)
    usecases := adapter.Usecases(ctx)

    return usecases.User.CreateUser(ctx, operator, input)
}

// ❌ BAD: Reading context in use case layer
func (u *User) CreateUser(ctx context.Context, input CreateUserInput) error {
    operator := adapter.Operator(ctx) // Don't do this
    // ...
}
```

### Benefits
- **Clearer Dependencies**: Function signatures show what data is needed
- **Better Testability**: No need to mock context values in unit tests
- **Easier Tracing**: Data flow is explicit and traceable
- **Layer Separation**: Maintains clean architecture boundaries

## Testing Strategy

### Test Coverage Guidelines
- **Domain Layer (`pkg/`)**: Aim for high test coverage (80%+)
  - Test all business logic and domain rules
  - Test aggregate behavior and invariants
  - Test value object validation
  - Cover edge cases and domain constraints

- **Use Case Layer (`internal/usecase/`)**: Aim for high test coverage (80%+)
  - Test all business scenarios and workflows
  - Test error handling and validation
  - Use in-memory implementations for testing
  - Cover complex business logic paths

- **Adapter Layer (`internal/adapter/`)**: Focus on GraphQL resolver behavior
  - Test GraphQL request/response flows
  - Test authentication and authorization
  - Test error handling and validation
  - Integration with use case layer

### Testing Patterns
```go
// Use case testing with in-memory repositories
func TestUser_CreateUser(t *testing.T) {
    r := memory.New()
    uc := NewUser(r, nil, "", "")
    
    ctx := context.Background()
    operator := &usecase.Operator{...}
    
    user, err := uc.CreateUser(ctx, operator, input)
    assert.NoError(t, err)
    assert.NotNil(t, user)
}

// Domain testing
func TestUser_Validation(t *testing.T) {
    user := &user.User{...}
    
    err := user.Validate()
    assert.Error(t, err)
}
```

### Testing Infrastructure
- **In-Memory Repositories**: Use `internal/infrastructure/memory` for testing
- **Testcontainers**: Use for E2E tests with real MongoDB
- **No External Dependencies**: Tests should be self-contained

## Common Patterns
- GraphQL resolver pattern: `usecases(ctx).User.Method()` and `getOperator(ctx)` for dependency access
- Domain object construction: `user.New().ID(id).Name(name).Build()` with validation
- Use case execution: `Run0`, `Run1`, `Run2`, `Run3` functions for transaction management
- Permission checking: `Usecase().WithReadableWorkspaces()` for authorization control
- Error handling: `rerror.NewE(i18n.T("message"))` for internationalized domain errors
- Repository pattern: Interface segregation for each domain (`repo.User`, `repo.Workspace`)
- Model conversion: `gqlmodel.ToUser(domainUser)` for GraphQL response transformation
- Context management: `AttachUser()`, `AttachOperator()` for request context
- Testing strategy: Use `memory.New()` for in-memory testing, testcontainers for E2E
- ID generation: `id.NewUserID()`, `id.NewWorkspaceID()` for type-safe identifiers
- Configuration: Environment-based config with `envconfig` for service setup
- Authorization: Delegate permission decisions to Cerbos service via GraphQL
- Schema-first development: GraphQL schema definitions drive code generation

## Collaboration Workflow
- When resolving bugs, document findings in code comments and commit messages
- Always run tests and linting before suggesting code complete
- For complex issues, break down analysis into steps in todo lists
- Reference specific files and line numbers when explaining problems
- Include error location context (file:line) in all debugging discussions

## Communication Style
- Provide concise technical explanations
- Include relevant code snippets with context
- Always suggest testing steps for fixes
- Document any assumptions made during problem-solving
- Focus on GraphQL-specific patterns and DDD principles

## GraphQL APIs for User/Workspace/Role/Permittable

### Overview & Flow

`reearth-accounts` exposes **GraphQL APIs** to centrally manage key domain models:

- `user` - User management and authentication
- `workspace` - Workspace management and membership
- `role` - Role definitions and assignments
- `permittable` - Role assignment relationships

These APIs are defined in the `schemas/` directory as `.graphql` files and are the only entry point for creating, updating, or querying these resources.

> Note: These domain models are **owned and managed solely within `reearth-accounts`**. Other services must not manipulate them directly. Currently, some operations are in migration — full centralization is the intended goal.

### Example Usage in Other Services

For instance, the `dashboard` service contains a `getMe` use case that needs workspace information. Instead of maintaining local workspace logic, it sends a **GraphQL request to `reearth-accounts`** to fetch it.

This approach ensures:
- Domain ownership and consistency
- Clean separation of responsibilities
- Easier auditing and security enforcement

All other services are expected to adopt the same model: rely on GraphQL APIs from `reearth-accounts` when interacting with user/workspace/role/permittable data.

## Permission Evaluation

### Overview & Flow

Each service performs **authorization via a GraphQL request** to the `reearth-accounts` service.

1. The target user's `Permittable` (role bindings) is retrieved from the DB
2. Roles are used to construct a Cerbos `Principal`
3. The request includes:
    - `service` (e.g. `"dashboard"`)
    - `resource` (e.g. `"user"`)
    - `action` (e.g. `"read"`)
4. A `Resource` is constructed and sent to Cerbos for evaluation
5. Cerbos responds with `ALLOW` or `DENY`

This way, the calling service **only needs to send the service/resource/action**, and `reearth-accounts` performs the actual evaluation.

```
  ┌────────────────────┐
  │ Other Microservices│
  └────────┬───────────┘
           │ GraphQL
           ▼
  ┌────────────────────┐
  │  reearth-accounts  │
  └────────┬───────────┘
           │ gRPC
           ▼
  ┌────────────────────┐
  │      cerbos        │
  └────────────────────┘
```

### Resource Definitions in Other Services

- Each microservice defines its resource/action matrix in `definition.go`
- Uses `generator.ResourceBuilder` to register resources and actions
- These definitions are synced to GCS via GitHub Actions
- `reearth-accounts` loads the synced policy for evaluation

Example from `dashboard` service:

```go
builder.AddResource("user", []ActionDefinition{
    NewActionDefinition("read", []string{"self"}),
})
```

### Related References

- `internal/usecase/interactor/cerbos.go`: Permission checking logic
- Each service's `definition.go`: Resource-action mapping
