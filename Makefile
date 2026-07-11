.DEFAULT_GOAL := help

BINARY := atlas-api
CMD := ./cmd/api

.PHONY: help build run test db-up db-down db-logs

help:
	@echo "Targets:"
	@echo "  build    Build the API binary"
	@echo "  run      Run the API server"
	@echo "  test     Run Go tests"
	@echo "  db-up    Start Postgres via Docker Compose"
	@echo "  db-down  Stop Postgres"
	@echo "  db-logs  Follow Postgres logs"

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
