# Atlas

A personal Platform-as-a-Service for homelab deployments. Atlas automates the workflow of shipping applications to a Kubernetes cluster — starting with app registration and growing toward builds, deploys, and observability.

Inspired by Render, Heroku, and Railway — not a clone.

## Status

**Phase 4 in progress:** GitHub push → build → registry push → k3s Deployment, Service, Ingress, and optional TLS. Reconciliation and observability are next.

| Phase | Feature | Status |
|-------|---------|--------|
| 0 | Health check, dev tooling | Done |
| 1 | Apps CRUD + Postgres | Done |
| 2 | Git source + webhooks | Done |
| 3 | Builds (clone, docker build, push) | Done |
| 4 | k3s runtime (Deploy, Service, Ingress, TLS, Kaniko Job builds) | In progress |
| 5+ | Reconciliation, observability | Planned |

## Prerequisites

- Go 1.26+
- Docker + Docker Compose (Docker daemon required only for host-fallback builds; k8s Job builds use Kaniko)
- `git` on PATH (host-fallback clone step)
- `make` (optional but recommended)
- k3s or Kubernetes cluster + kubeconfig access (for deploys and isolated Job builds)
- Local Docker registry (required for k3s deploys — e.g. `localhost:5000`)
- DNS or `/etc/hosts` entries for `*.your-domain` (if using Ingress)

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

Poll build history after a push:

```bash
curl http://localhost:8080/apps/{id}/builds
curl http://localhost:8080/apps/{id}/builds/{build_id}
```

Example build response (after a successful push):

```json
{
  "id": "…",
  "app_id": "…",
  "status": "succeeded",
  "image": "localhost:5000/atlas/<app-id>:<build-id>",
  "created_at": "2026-07-11T12:00:00Z",
  "updated_at": "2026-07-11T12:01:30Z"
}
```

`image` is empty until the worker pushes to the registry; it holds the full registry-qualified tag used for deploy.

Check resources on k3s:

```bash
kubectl get deployments,services,ingress -n default
kubectl get pods -n default -l app=portfolio
```

Open the app (with Ingress configured):

```bash
curl http://portfolio.homelab.local
# If ATLAS_INGRESS_TLS_SECRET is set:
curl https://portfolio.homelab.local
```

### Build and deploy pipeline

On a matched GitHub push:

```text
webhook → create build (pending) → worker (async)
  → build image (k8s Job with Kaniko, or host docker build as fallback)
  → save image on build record → EnsureDeployment → EnsureService → EnsureIngress (if ATLAS_INGRESS_DOMAIN set)
  → build status: succeeded | failed
```

Atlas chooses a build strategy per build:

- **Isolated k8s Job (preferred):** when a cluster is reachable and `ATLAS_REGISTRY_URL` is set, each build runs as an `atlas-build-<build-id>` Job — an init container does a shallow `git clone`, then [Kaniko](https://github.com/GoogleContainerTools/kaniko) builds the Dockerfile and pushes to the registry. No Docker daemon on the API host required.
- **Host build (fallback):** when no cluster is available, the worker clones and runs `docker build` / `docker push` on the API host (requires `git` and `docker` on the host).

Requirements for a full deploy:

- Repo must contain a `Dockerfile` at the root
- Public git repo (private repo credentials are not supported yet)
- Container listens on port `80` (matches Service/Ingress defaults for now)
- `ATLAS_REGISTRY_URL` set so k3s can pull the image
- k3s reachable via `ATLAS_KUBECONFIG`, in-cluster config, or default `~/.kube/config`
- k3s configured to pull from your registry (e.g. insecure registry for `localhost:5000`)
- For Job builds against an insecure registry, set `ATLAS_INSECURE_REGISTRY=true`
- For registries that require auth, create a `dockerconfigjson` Secret and set `ATLAS_REGISTRY_SECRET` to its name

External access (optional):

- Set `ATLAS_INGRESS_DOMAIN` (e.g. `homelab.local`) — apps get `<app>.<domain>` (e.g. `portfolio.homelab.local`)
- Set `ATLAS_INGRESS_CLASS=traefik` on k3s (or leave empty to use cluster default)
- Point DNS or `/etc/hosts` at your k3s node IP
- For HTTPS, create a `kubernetes.io/tls` Secret in `ATLAS_K8S_NAMESPACE` and set `ATLAS_INGRESS_TLS_SECRET` to its name

Images are tagged `atlas/<app-id>:<build-id>` locally, pushed as `<registry>/atlas/<app-id>:<build-id>`, saved on the build record as `image`, and deployed as a Deployment named after the app (e.g. `portfolio`).

Example TLS secret:

```bash
kubectl create secret tls homelab-tls \
  --cert=wildcard.homelab.local.crt \
  --key=wildcard.homelab.local.key \
  -n default
```

Then set:

```bash
ATLAS_INGRESS_TLS_SECRET=homelab-tls
```

Example registry auth secret (only if your registry requires login):

```bash
kubectl create secret docker-registry registry-creds \
  --docker-server=registry.homelab.local \
  --docker-username=<user> \
  --docker-password=<pass> \
  -n default
```

Then set:

```bash
ATLAS_REGISTRY_SECRET=registry-creds
```

**Notes:**

- When a cluster is reachable, builds run as isolated k8s Jobs (Kaniko); otherwise they fall back to host `docker build`.
- The build worker waits synchronously for the Job to finish; build logs are not streamed yet.
- If the cluster is unreachable, Atlas logs a warning and skips deploy; host builds still run.
- Atlas references an existing TLS secret; it does not issue or renew certificates yet.

### GitHub webhooks

Atlas uses one webhook secret for the whole instance. Reuse the same value from `ATLAS_WEBHOOK_SECRET` on every GitHub webhook you create.

1. Generate a production secret: `openssl rand -hex 32`
2. Set `ATLAS_WEBHOOK_SECRET` in `.env`
3. In GitHub → repo → Settings → Webhooks → Add webhook:
   - **Payload URL:** `https://<your-atlas-host>/webhooks/github`
   - **Content type:** `application/json`
   - **Secret:** same value as `ATLAS_WEBHOOK_SECRET`
   - **Events:** Just the push event

Pushes to a linked repo and branch return `202 Accepted` with `app_id` and `build_id`. Unlinked repos, wrong branches, and non-push events are ignored with `204 No Content`.

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
| `GET` | `/apps/{id}/builds` | List builds for an app (newest first; includes `image`) |
| `GET` | `/apps/{id}/builds/{build_id}` | Get build by ID (includes `image`, `status`, timestamps) |
| `POST` | `/webhooks/github` | GitHub push webhook (signed; builds and deploys) |

## Project layout

```
Atlas/
├── api/              # HTTP server and handlers
├── app/              # App and Repo resource types
├── build/            # Build type, worker, clone/build/push steps
├── runtime/          # k3s client-go, Deployment, Service, Ingress
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
| `ATLAS_REGISTRY_URL` | — | Docker registry host (e.g. `localhost:5000`); empty skips push and deploy |
| `ATLAS_REGISTRY_SECRET` | — | `dockerconfigjson` Secret name for Kaniko push auth; empty if registry needs no auth |
| `ATLAS_INSECURE_REGISTRY` | `false` | Set `true` for insecure registries (e.g. `localhost:5000`) in Job builds |
| `ATLAS_KUBECONFIG` | — | Path to kubeconfig; empty uses in-cluster or `~/.kube/config` |
| `ATLAS_K8S_NAMESPACE` | `default` | Namespace for app Deployments |
| `ATLAS_INGRESS_DOMAIN` | — | Base domain for apps (`portfolio.homelab.local`); empty skips Ingress |
| `ATLAS_INGRESS_CLASS` | — | Ingress class (e.g. `traefik` on k3s) |
| `ATLAS_INGRESS_TLS_SECRET` | — | Optional TLS Secret name for HTTPS Ingress |
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
