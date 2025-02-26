default: help

help:
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  dev-install       Install tools for make dev - behavior not confirmed in windows."
	@echo "  dev               Run the application with hot reloading"
	@echo "  run-app           Run the application"
	@echo "  run-cerbos        Run the Cerbos server"
	@echo "  gql               Generate GraphQL code include dataloader"
	@echo "  generate          Run go generate"

TARGET_TEST :=./...
REEARTH_DB := mongodb://localhost
test:
	REEARTH_DB=${REEARTH_DB} go test ./... -run ${TARGET_TEST}

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

run-cerbos:
	docker compose -f ../docker-compose.yml up -d reearth-cerbos

generate: dev-install
	go generate ./...

gql:
	go generate ./internal/adapter/gql

.PHONY: dev-install dev run-app run-cerbos gql
