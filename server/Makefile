default: help

help:
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  dev-install       Install tools for make dev - behavior not confirmed in windows."
	@echo "  dev               Run the application with hot reloading"
	@echo "  run-app           Run the application"
	@echo "  run-cerbos       Run the Cerbos server"
	@echo "  gql               Generate GraphQL code include dataloader"


AIR_BIN := $(shell which air)
dev-install:
ifndef AIR_BIN
	@echo "reflex is not installed. Installing..."
	@go install github.com/air-verse/air@v1.61.5
else
	@echo "air is already installed."
endif

dev: dev-install
	air

run-app:
	go run ./cmd/reearth-dashboard

run-cerbos:
	docker compose -f ../docker-compose.yml up -d reearth-cerbos

gql:
	go generate ./internal/adapter/gql

.PHONY: dev-install dev run-app run-cerbos gql
