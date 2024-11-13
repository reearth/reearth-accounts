run-app:
	go run ./cmd/reearth-dashboard

run-cerbos:
	docker compose -f ../docker-compose.yml up -d reearth-cerbos

gql:
	go generate ./internal/adapter/gql

.PHONY: run-app run-cerbos gql
