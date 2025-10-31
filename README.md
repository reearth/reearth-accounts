# Re:Earth Accounts

[![GitHub stars](https://img.shields.io/github/stars/reearth/reearth-accounts?style=social)](https://github.com/reearth/reearth-accounts/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/reearth/reearth-accounts)](https://github.com/reearth/reearth-accounts/issues)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/reearth/reearth-accounts/blob/main/LICENSE)

Centralized account management and authorization service for Re:Earth's microservice architecture. This service complements the authentication functionality across Re:Earth microservices by providing unified user, workspace, role management and permission evaluation.

## Features

- **User Management** - Centralized user account management with Auth0 JWT authentication
- **Workspace Management** - Multi-tenant workspace organization and membership control
- **Role-Based Access Control** - Flexible role definitions and assignments across services
- **Centralized Authorization** - Permission evaluation using Cerbos authorization engine
- **GraphQL API** - Schema-first GraphQL APIs for all account operations
- **Multi-language Support** - Internationalization support (English, Japanese)
- **DDD Architecture** - Clean domain-driven design with clear layer separation

## Architecture

Re:Earth Accounts follows Domain-Driven Design (DDD) principles with clear layer separation:

- **Domain Layer** (`pkg/`) - Core business logic and entities
- **Application Layer** (`internal/usecase/`) - Use cases and orchestration
- **Infrastructure Layer** (`internal/infrastructure/`) - Database and external services
- **Adapter Layer** (`internal/adapter/`) - GraphQL API and request handling

## Built with

- [Go](https://golang.org/) - Backend language (1.24.2+)
- [Echo](https://echo.labstack.com/) - HTTP framework
- [gqlgen](https://gqlgen.com/) - GraphQL server library
- [MongoDB](https://www.mongodb.com/) - Database
- [Cerbos](https://cerbos.dev/) - Authorization engine
- [Auth0](https://auth0.com/) - Authentication provider
- [Docker](https://www.docker.com/) - Containerization

## GraphQL API

The service exposes GraphQL APIs for:

- User operations (create, update, delete, query)
- Workspace management
- Role definitions and assignments
- Permission evaluation

Schema files are located in the `schemas/` directory.

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.24.2 or later (for local development)
- MongoDB (included in Docker setup)

### Running with Docker Compose

This service is designed to run alongside other Re:Earth microservices within a shared Docker network. It provides centralized authentication and authorization for all Re:Earth services including reearth-visualizer, reearth-cms, reearth-flow, and others.

**Prerequisites:**

Before starting this service, ensure the following are running:
- The `reearth` Docker network
- MongoDB instance named `reearth-mongo` on that network

These are typically provided by any Re:Earth service (e.g., [reearth-visualizer](https://github.com/reearth/reearth-visualizer), [reearth-cms](https://github.com/reearth/reearth-cms), [reearth-flow](https://github.com/reearth/reearth-flow)).

Example setup with reearth-visualizer:

```bash
# Clone and start reearth-visualizer (or any other Re:Earth service)
git clone https://github.com/reearth/reearth-visualizer.git
cd reearth-visualizer/server
make run
```

**Start the service:**

Once the `reearth` network and `reearth-mongo` are available, start this service:

```bash
cd server
make run
```

This will:
- Start Cerbos authorization server on port 3593
- Start Re:Earth Accounts server on port 8090
- Attach to the external `reearth` Docker network, making the service accessible to all Re:Earth microservices
- Connect to the `reearth-mongo` database

The GraphQL endpoint will be available at:
- From host machine: `http://localhost:8090/graphql`
- From within Docker network: `http://reearth-accounts-dev:8090/graphql`

**Note:** The service uses `docker-compose.dev.yml` which declares `networks.reearth.external: true`, meaning it attaches to the existing `reearth` network rather than creating its own.

To stop the services:

```bash
cd server
make down
```

### Development Setup

For local development with hot reloading:

1. **Install development tools**

```bash
cd server
make dev-install
```

This installs:
- [Air](https://github.com/air-verse/air) - Hot reloading
- [mockgen](https://github.com/uber-go/mock) - Mock generation

2. **Start Cerbos authorization server**

```bash
make run-cerbos
```

3. **Configure environment**

Create a `.env` file in the `server` directory with your configuration:

```env
REEARTH_DB=mongodb://localhost:27017
REEARTH_DB_NAME=reearth-accounts
REEARTH_AUTH0_DOMAIN=your-auth0-domain
REEARTH_AUTH0_AUDIENCE=your-auth0-audience
# Add other environment variables as needed
```

4. **Run with hot reloading**

```bash
make dev
```

The server will automatically reload when you make changes to the code.

### Running Tests

```bash
# Run all tests
make test

# Run tests with custom MongoDB URL
REEARTH_DB=mongodb://localhost:27017 make test

# Run specific package tests
TARGET_TEST=./pkg/user make test
```

### GraphQL Code Generation

After modifying GraphQL schemas in the `schemas/` directory:

```bash
make gql
```

This generates:
- GraphQL resolvers
- Type definitions
- Dataloaders for efficient queries

### Database Migrations

Migrations run automatically on server startup. To run migrations manually:

```bash
make run-migration
```

## Environment Compatibility

### Operating Systems

| OS | Supported |
|---|---|
| macOS | ✅ |
| Linux | ✅ |
| Windows | ✅ |

### Required Tools

| Tool | Version |
|---|---|
| Go | 1.24.2+ |
| Docker | Latest |
| Docker Compose | Latest |
| MongoDB | 4.4+ |

For detailed architecture documentation, see [server/CLAUDE.md](server/CLAUDE.md).

## Contact

- Website: [https://reearth.io](https://reearth.io)
- GitHub Issues: [https://github.com/reearth/reearth-accounts/issues](https://github.com/reearth/reearth-accounts/issues)

---

Copyright © 2025 Re:Earth Contributors
