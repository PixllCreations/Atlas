.DEFAULT_GOAL := help

BINARY := atlas-api
CMD := ./cmd/api

.PHONY: help build run test db-up db-down db-logs migrate

help:
	@echo "Targets:"
	@echo "  build    Build the API binary"
	@echo "  run      Run the API server"
	@echo "  test     Run Go tests"
	@echo "  db-up    Start Postgres via Docker Compose"
	@echo "  db-down  Stop Postgres"
	@echo "  db-logs  Follow Postgres logs"
	@echo "  migrate  Apply pending database migrations"

build:
	go build -o $(BINARY) $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

db-up:
	docker compose up -d

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

ifeq ($(OS),Windows_NT)
migrate:
	powershell -ExecutionPolicy Bypass -File hack/migrate.ps1
else
migrate:
	bash hack/migrate.sh
endif
