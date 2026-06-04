default: help

help:
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  dev-install       Install tools for make dev - behavior not confirmed in windows."
	@echo "  dev               Run the application with hot reloading"
	@echo "  run               Run reearth-cerbos and reearth-accounts via docker-compose with hot reload"
	@echo "  down              Stop and remove containers started by make run"
	@echo "  run-app           Run the application"
	@echo "  run-cerbos        Run the Cerbos server"
	@echo "  run-migration     Run database migrations"
	@echo "  gql               Generate GraphQL code include dataloader"
	@echo "  generate          Run go generate"
	@echo "  sqlc              Regenerate sqlc query code for the postgres backend"
	@echo "  test-integration  Run testcontainers integration tests (postgres + mongo)"
	@echo "  gen-policies      Generate Cerbos policy files"

TARGET_TEST :=./...
REEARTH_DB := mongodb://localhost
test:
	REEARTH_DB=${REEARTH_DB} go test ./... -run ${TARGET_TEST}

# Integration tests that spin real databases (postgres + mongo) via
# testcontainers. Requires a running Docker daemon. -p 1 serializes packages so
# their testcontainers reapers don't collide; the -run filter skips e2e tests
# that need a separate mongo service.
test-integration:
	go test -p 1 -tags integration -timeout 900s \
		./internal/infrastructure/postgres/... \
		./internal/infrastructure/conformance/... \
		./e2e/... \
		-run 'Conformance|TestMigrate|TestTransaction|TestResolveDBTX|TestPostgresE2E_GetAndUpdateMe'

# Regenerate the sqlc query package for the PostgreSQL backend after editing
# any *.sql under internal/infrastructure/postgres/sqlc. Requires sqlc v1.27+.
sqlc:
	sqlc generate

DOCKER_COMPOSE := docker compose -f ../docker-compose.yml
DOCKER_COMPOSE_DEV := docker compose -f ../docker-compose.dev.yml

AIR_BIN := $(shell which air)
MOCKGEN_BIN := $(shell which mockgen)
dev-install:
ifndef AIR_BIN
	@echo "reflex is not installed. Installing..."
	@go install github.com/air-verse/air@v1.61.5
else
	@echo "air is already installed."
endif
ifndef MOCKGEN_BIN
	@echo "mockgen is not installed. Installing..."
	@go install go.uber.org/mock/mockgen@v0.5.0
else
	@echo "mockgen is already installed."
endif

dev: dev-install
	op run --env-file ./app.env -- air

run-app:
	op run --env-file ./app.env -- go run ./cmd/reearth-accounts

run:
	${DOCKER_COMPOSE_DEV} up reearth-accounts-dev

down:
	${DOCKER_COMPOSE_DEV} down

run-cerbos:
	${DOCKER_COMPOSE} up -d reearth-cerbos

generate: dev-install
	go generate ./...

gen-policies:
	go run ./cmd/policy-generator

gql:
	go generate ./internal/adapter/gql

run-migration:
	RUN_MIGRATION=true go run ./cmd/reearth-accounts

update-schema-json:
	go run ./tools/cmd/mongoschemagen -output ./internal/infrastructure/mongo/schema
	go run ./tools/cmd/ergen -schema ./internal/infrastructure/mongo/schema -mongodoc ./internal/infrastructure/mongo/mongodoc -output ./internal/infrastructure/mongo/schema/ER.md

swag:
	go run github.com/swaggo/swag/cmd/swag init --generalInfo internal/adapter/http/doc.go --dir ./,internal/adapter/http --output ./docs --parseDependency --parseInternal

wire-admin:
	go run github.com/goforj/wire/cmd/wire gen ./internal/reearth-accounts-admin/di
	@# Strip the generated `//go:generate ... -mod=mod ... wire` directive: `-mod=mod`
	@# is incompatible with this repo's Go workspace mode and would break
	@# `go generate ./...` in CI. Regenerate via this target instead.
	sed -i.bak '/go:generate go run -mod=mod github.com\/goforj\/wire/d' internal/reearth-accounts-admin/di/wire_gen.go && rm -f internal/reearth-accounts-admin/di/wire_gen.go.bak

swag-admin:
	go run github.com/swaggo/swag/cmd/swag init -g main.go -d cmd/reearth-accounts-admin,internal/reearth-accounts-admin -o docs/admin --parseInternal --useStructName

run-admin:
	go run ./cmd/reearth-accounts-admin

.PHONY: dev-install dev run down run-app run-cerbos run-migration gql gen-policies update-schema-json test test-integration sqlc swag wire-admin swag-admin run-admin
