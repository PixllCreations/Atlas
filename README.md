# Atlas

**A self-hosted, config-driven Kubernetes PaaS** — declare what an app needs in `atlas.yaml`, connect a GitHub repo, and ship with a deploy button or a single `git push`.

Inspired by Render, Heroku, and Railway. Built from scratch in Go to show end-to-end platform engineering: declarative config, async build pipelines, dependency provisioning, and Kubernetes reconciliation.

Access the console on **Tailscale** (`http://<host>:8080`). Public apps and webhooks go through **Cloudflare Tunnel** on `*.edwardscott.dev` / `hooks.edwardscott.dev`.

---

## Highlights

- **Config-driven deploys** — each repo ships an `atlas.yaml` (port + managed dependencies); Atlas is the control plane that turns intent into Kubernetes resources
- **Per-project namespaces** — workloads land in `atlas-<project>` with ownership labels; build Jobs stay in the system namespace
- **Managed Redis** — declared in config, provisioned as Deployment + ClusterIP Service, injected as `REDIS_URL` / `PORT`
- **Plan → reconcile** — pure `DeploymentPlan` then idempotent apply; removed dependencies are pruned without touching unmanaged objects
- **Console UI** — React SPA (embedded in the API) for projects, GitHub linking, deploy history, and read-only infrastructure view
- **GitHub App** — install once; push webhooks and private clones without per-repo hook setup
- **Isolated builds** — Kaniko Jobs (fallback: host Docker) → registry → Deploy / Service / Ingress

## Tech Stack

| Layer | Technologies |
|-------|--------------|
| Language | Go 1.26 |
| UI | React, TypeScript, Vite (embedded into the API binary) |
| API | `net/http` (Go 1.22+ routing), REST |
| Config | `atlas.yaml` via `gopkg.in/yaml.v3` |
| Database | PostgreSQL, pgx/v5, SQL migrations |
| GitHub | GitHub App (JWT + installation tokens) |
| Builds | Kaniko (K8s Jobs); Docker host fallback |
| Orchestration | Kubernetes via **k3d** (local) or k3s, client-go v0.36 |
| Ingress | Traefik (bundled with k3d/k8s) |
| Access | Tailscale (console); Cloudflare Tunnel (public apps + webhooks) |

## Architecture

```text
Repository
    │
    ▼
atlas.yaml  →  parse / validate  →  DeploymentPlan
    │                                       │
    ▼                                       ▼
Dockerfile → Kaniko Job / host build → project namespace
                                            │
                    ┌───────────────────────┼───────────────────────┐
                    ▼                       ▼                       ▼
              Deployment/app          Deployment/redis         Ingress/app
              Service/app             Service/redis      <name>.edwardscott.dev
                    │
                    └── env: PORT, REDIS_URL, …
```

GitHub App push → `hooks.edwardscott.dev` → Atlas API → create build → reconcile.

## Console

```bash
make up                  # k3d + Postgres + API + Cloudflare Tunnel
```

Open `http://localhost:8080` (or `http://<tailscale-hostname>:8080`).

1. **New project** — name becomes the ingress host (`<name>.edwardscott.dev`) and namespace `atlas-<name>`
2. **Connect GitHub** — install the Atlas GitHub App, then pick a repo + branch
3. **Deploy** — Atlas clones, reads `atlas.yaml`, builds, provisions deps, rolls out the app
4. **Push to re-deploy** — App webhooks fire automatically
5. **Overview** — dependencies from the latest successful plan; Settings for GitHub/unlink/delete

UI routes use `/projects/...` and `/system` so they do not collide with the JSON API (`/apps`, `/status`).

## Project Status

| Phase | Feature | Status |
|-------|---------|--------|
| 0 | Health check, Compose/k3d tooling | Done |
| 1 | Apps CRUD + Postgres | Done |
| 2 | Git source + GitHub App webhooks | Done |
| 3 | Builds (clone, Kaniko/Docker, push) | Done |
| 4 | k3s Deploy/Service/Ingress + console | Done |
| 5 | `atlas.yaml`, project namespaces, Redis provisioner, plan reconcile | Done |
| 6 | Deploy phases + live build log streaming (SSE) | Done |
| 7 | Runtime pod logs per workload/service | Done |
| 8+ | User env / secrets UI, more deps (Postgres…) | Planned — see [TODO.md](TODO.md) |

## Repository requirements

At the **root** of every deployed repo:

```yaml
# atlas.yaml
version: 1

app:
  port: 8080

dependencies:
  redis:
    type: redis   # optional; only redis is implemented today
```

Plus:

- A root `Dockerfile` (Atlas builds that context)
- App listens on `app.port` (Atlas also injects `PORT`)
- Apps that use Redis should read `REDIS_URL` (e.g. `redis://redis:6379`)

Missing or invalid `atlas.yaml` fails the build with a clear error.

## Quick Start

**Prerequisites:** Docker, [k3d](https://k3d.io/), [kubectl](https://kubernetes.io/docs/tasks/tools/), domain on Cloudflare DNS.

### 1. Cloudflare Tunnel + DNS (one time)

Full walkthrough: **[docs/cloudflare.md](docs/cloudflare.md)**

1. Zero Trust → **Tunnels** → Create → Docker → copy token to `.env` as `CLOUDFLARE_TUNNEL_TOKEN`
2. Add public hostnames (**order matters** — put `hooks` above the wildcard):

| Hostname | Service URL |
|----------|-------------|
| `hooks.edwardscott.dev` | `http://api:8080` |
| `*.edwardscott.dev` | `http://host.docker.internal:80` |

3. Keep apex / `www` on GitHub Pages if you use them
4. SSL/TLS mode: **Full**

### 2. GitHub App (recommended)

See **[docs/github-app.md](docs/github-app.md)**.

### 3. Run Atlas

```bash
brew install k3d kubectl   # once
cp .env.example .env       # CLOUDFLARE_TUNNEL_TOKEN, ATLAS_WEBHOOK_SECRET, GitHub App vars

make up                    # k3d + Postgres + API + tunnel
make logs                  # optional
```

- Console: `http://localhost:8080`
- Example project `we-know-ball` → `https://we-know-ball.edwardscott.dev` (namespace `atlas-we-know-ball`)
- Webhooks → `https://hooks.edwardscott.dev/webhooks/github`

```bash
make down
```

### Legacy: manual repo webhooks

If the GitHub App is not configured, link a repo URL and add a push webhook:

1. `openssl rand -hex 32` → `ATLAS_WEBHOOK_SECRET`
2. Repo → Settings → Webhooks → `https://hooks.edwardscott.dev/webhooks/github`, secret above, event **Push**

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness check |
| `GET` | `/status` | Runtime hints (registry, k8s, ingress, GitHub App) |
| `GET` | `/apps` | List apps |
| `POST` | `/apps` | Create app (`{"name":"..."}`) |
| `GET` | `/apps/{id}` | Get app |
| `DELETE` | `/apps/{id}` | Delete project namespace + DB row |
| `GET` | `/apps/{id}/infrastructure` | Latest deployment snapshot (deps, host, port) |
| `PUT` | `/apps/{id}/repo` | Link repo (URL or GitHub App install) |
| `GET` | `/apps/{id}/repo` | Get linked repo |
| `DELETE` | `/apps/{id}/repo` | Unlink repo |
| `GET` | `/apps/{id}/builds` | List builds |
| `POST` | `/apps/{id}/builds` | Trigger build/deploy |
| `GET` | `/apps/{id}/builds/{build_id}` | Build status and image |
| `GET` | `/auth/github/install` | Start GitHub App install |
| `GET` | `/auth/github/callback` | Install callback |
| `GET` | `/github/installations` | List installations |
| `POST` | `/github/installations/sync` | Sync from GitHub API |
| `GET` | `/github/installations/{id}/repos` | List installation repos |
| `POST` | `/webhooks/github` | GitHub push / App webhook |

## Project Layout

```text
Atlas/
├── api/           # HTTP server, handlers, SPA routes
├── app/           # Domain types (App, Repo)
├── build/         # Worker: clone → config → build → plan → reconcile
├── config/        # atlas.yaml parse + validation
├── plan/          # Pure DeploymentPlan builder
├── dependency/    # Provisioner registry (Redis)
├── github/        # GitHub App auth
├── runtime/       # client-go Ensure*/Delete* helpers, namespaces
├── webhook/       # HMAC verify + push parse
├── store/         # Postgres + migrations
├── web/           # React console (Vite); dist embedded at build
├── cmd/api/       # Process entrypoint
├── docs/          # Cloudflare, GitHub App, ADRs
├── hack/          # k3d + entrypoint + migrate
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── TODO.md
└── go.mod
```

Package boundaries: `config` and `plan` never call Kubernetes; `dependency` provisioners use `runtime`; `build.Worker` orchestrates.

## Configuration

Copy `.env.example` to `.env`. Key variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `ATLAS_PORT` | `8080` | HTTP listen port (UI + API) |
| `ATLAS_DATABASE_URL` | local Postgres DSN | Control-plane DB |
| `ATLAS_WEBHOOK_SECRET` | — | GitHub webhook HMAC (+ install state) |
| `ATLAS_WEBHOOK_PUBLIC_URL` | — | Public webhook URL shown in UI |
| `ATLAS_GITHUB_APP_*` | — | App ID, slug, private key, OAuth client |
| `ATLAS_REGISTRY_URL` | `atlas-registry:5000` | In-cluster registry |
| `ATLAS_INSECURE_REGISTRY` | `true` | Kaniko insecure registry flags |
| `ATLAS_KUBECONFIG` | — | Kubeconfig path |
| `ATLAS_K8S_NAMESPACE` | `default` | **System** namespace for Kaniko Jobs (not app workloads) |
| `ATLAS_INGRESS_DOMAIN` | `edwardscott.dev` | Base domain for app hosts |
| `ATLAS_INGRESS_CLASS` | `traefik` | Ingress class |
| `CLOUDFLARE_TUNNEL_TOKEN` | — | Required for `make up` |

## Development

```bash
make up        # full stack
make down
make logs
make test      # go test ./...
make web-dev   # Vite (API on :8080)
```

- [docs/cloudflare.md](docs/cloudflare.md) — tunnel + DNS  
- [docs/github-app.md](docs/github-app.md) — GitHub App registration  
- [docs/atlas-yaml.md](docs/atlas-yaml.md) — repository config schema  
- [TODO.md](TODO.md) — next features  

## Resume bullets

- Built a self-hosted Kubernetes PaaS in Go that turns repository `atlas.yaml` into per-project namespaces, managed Redis, and Traefik Ingress
- Designed a plan/reconcile control plane (parse → validate → `DeploymentPlan` → idempotent client-go apply) with dependency provisioners and prune-on-remove
- Implemented GitHub App install flow, Kaniko Job builds, Postgres control-plane storage, and an embedded React operator console behind Cloudflare Tunnel + Tailscale

## License

Private — personal portfolio project.
