# Atlas

A personal Platform-as-a-Service for homelab deployments. Atlas automates the workflow of shipping applications to a Kubernetes cluster — starting with app registration and growing toward builds, deploys, and observability.

Inspired by Render, Heroku, and Railway — not a clone.

## Status

**Phase 2 complete:** Git source linking and GitHub webhooks.

| Phase | Feature | Status |
|-------|---------|--------|
| 0 | Health check, dev tooling | Done |
| 1 | Apps CRUD + Postgres | Done |
| 2 | Git source + webhooks | Done |
| 3+ | Builds, registry, k3s runtime | Planned |

## Prerequisites

- Go 1.25+
- Docker + Docker Compose
- `make` (optional but recommended)

## Quick start

```bash
cp .env.example .env

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

Link a GitHub repo to an app:

```bash
curl -X PUT http://localhost:8080/apps/{id}/repo \
  -H "Content-Type: application/json" \
  -d '{"url":"https://github.com/you/portfolio","branch":"main"}'
```

### GitHub webhooks

Atlas uses one webhook secret for the whole instance. Reuse the same value from `ATLAS_WEBHOOK_SECRET` on every GitHub webhook you create.

1. Generate a production secret: `openssl rand -hex 32`
2. Set `ATLAS_WEBHOOK_SECRET` in `.env`
3. In GitHub → repo → Settings → Webhooks → Add webhook:
   - **Payload URL:** `https://<your-atlas-host>/webhooks/github`
   - **Content type:** `application/json`
   - **Secret:** same value as `ATLAS_WEBHOOK_SECRET`
   - **Events:** Just the push event

Pushes to a linked repo and branch return `202 Accepted`. Unlinked repos, wrong branches, and non-push events are ignored with `204 No Content`.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness check |
| `GET` | `/apps` | List all apps |
| `POST` | `/apps` | Create app (`{"name":"..."}`) |
| `GET` | `/apps/{id}` | Get app by ID |
| `DELETE` | `/apps/{id}` | Delete app |
| `PUT` | `/apps/{id}/repo` | Link git repo (`{"url":"...","branch":"main"}`) |
| `GET` | `/apps/{id}/repo` | Get linked repo |
| `DELETE` | `/apps/{id}/repo` | Unlink repo |
| `POST` | `/webhooks/github` | GitHub push webhook (signed) |

## Project layout

```
Atlas/
├── api/              # HTTP server and handlers
├── app/              # App and Repo resource types
├── webhook/          # GitHub webhook verification and parsing
├── store/            # Postgres persistence and migrations
├── cmd/api/          # API binary entrypoint
├── hack/             # Migration scripts
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
| `ATLAS_PORT` | `8080` | HTTP listen port |
| `ATLAS_DATABASE_URL` | `postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable` | Postgres DSN |
| `ATLAS_WEBHOOK_SECRET` | — | HMAC secret for GitHub webhooks (required for webhook verification) |
| `ATLAS_DB_PASSWORD` | `atlas` | Compose Postgres password |
| `ATLAS_DB_PORT` | `5432` | Compose host port |
| `ATLAS_TEST_DATABASE_URL` | same as above | Postgres DSN for integration tests |

`make run` loads `.env` if present. Docker Compose reads `.env` automatically.

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
