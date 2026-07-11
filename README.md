# Atlas

A personal Platform-as-a-Service for homelab deployments. Atlas automates the workflow of shipping applications to a Kubernetes cluster — starting with app registration and growing toward builds, deploys, and observability.

Inspired by Render, Heroku, and Railway — not a clone.

## Status

**Phase 1 complete:** Apps CRUD API backed by Postgres.

| Phase | Feature | Status |
|-------|---------|--------|
| 0 | Health check, dev tooling | Done |
| 1 | Apps CRUD + Postgres | Done |
| 2 | Git source + webhooks | Planned |
| 3+ | Builds, registry, k3s runtime | Planned |

## Prerequisites

- Go 1.25+
- Docker + Docker Compose
- `make` (optional but recommended)

## Quick start

```bash
# Start Postgres
make db-up

# Apply schema
make migrate

# Run the API
make run
```

Verify:

```bash
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"portfolio"}'
curl http://localhost:8080/apps
```

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness check |
| `GET` | `/apps` | List all apps |
| `POST` | `/apps` | Create app (`{"name":"..."}`) |
| `GET` | `/apps/{id}` | Get app by ID |
| `DELETE` | `/apps/{id}` | Delete app |

## Project layout

```
Atlas/
├── api/              # HTTP server and handlers
├── app/              # App resource type
├── store/            # Postgres persistence
├── cmd/api/          # API binary entrypoint
├── docker-compose.yml
├── Makefile
└── go.mod
```

## Configuration

Copy the example env file for local overrides:

```bash
cp .env.example .env
```

| Variable | Default | Purpose |
|----------|---------|---------|
| `ATLAS_ADDR` | `:8080` | HTTP listen address |
| `ATLAS_DATABASE_URL` | `postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable` | Postgres DSN |
| `ATLAS_DB_PASSWORD` | `atlas` | Compose Postgres password |
| `ATLAS_DB_PORT` | `5432` | Compose host port |

Go does not auto-load `.env` files. Export variables in your shell or use a tool like `direnv`. Docker Compose reads `.env` automatically.

## Make targets

```
make help      # Show available targets
make build     # Build atlas-api binary
make run       # Run the API server
make test      # Run Go tests
make db-up     # Start Postgres
make db-down   # Stop Postgres
make db-logs   # Follow Postgres logs
make migrate   # Apply database migrations
```

## License

Private — homelab project.
