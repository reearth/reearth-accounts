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

TARGET_TEST :=./...
REEARTH_DB := mongodb://localhost
test:
	REEARTH_DB=${REEARTH_DB} go test ./... -run ${TARGET_TEST}

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
	air

run-app:
	go run ./cmd/reearth-accounts

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

.PHONY: dev-install dev run down run-app run-cerbos run-migration gql update-schema-json
