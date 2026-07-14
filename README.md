# Atlas

**A self-hosted Platform-as-a-Service for Kubernetes** — register apps in a Railway-like console, connect GitHub repos, and ship containerized workloads to a k3s cluster with a deploy button or a single `git push`.

Inspired by Render, Heroku, and Railway. Built from scratch in Go to demonstrate end-to-end platform engineering: API design, async build pipelines, container orchestration, and infrastructure automation.

Access the console on **Tailscale** (`http://<host>:8080`). Public apps and webhooks go through **Cloudflare Tunnel** on `*.edwardscott.dev` / `hooks.edwardscott.dev`.

---

## Highlights

- **Console UI** — React SPA (embedded in the API) for projects, GitHub linking, deploy, build history, and teardown
- **GitHub App** — Connect GitHub once; push events and private clones without per-repo webhook setup
- **Git-to-deploy pipeline** — Manual deploy or push webhooks → clone → Kaniko build → registry push → Kubernetes rollout
- **Isolated builds** — Kaniko Jobs for sandboxed image builds without a host Docker daemon
- **Declarative runtime** — Idempotent Deployment, Service, and Ingress via client-go; delete tears them down
- **Configurable container port** — Service stays on `:80` and forwards to the app’s listen port
- **REST API** — Full app lifecycle with Postgres-backed persistence and SQL migrations

## Tech Stack

| Layer | Technologies |
|-------|--------------|
| Language | Go 1.26 |
| UI | React, TypeScript, Vite (embedded into the API binary) |
| API | `net/http` (Go 1.22+ routing), REST |
| Database | PostgreSQL, pgx/v5, hand-written SQL migrations |
| GitHub | GitHub App (JWT + installation tokens) |
| Container builds | Kaniko (K8s Jobs); Docker host fallback |
| Orchestration | Kubernetes via **k3d** (local) or k3s, client-go v0.36 |
| Ingress | Traefik (bundled with k3d/k3s) |
| Access | Tailscale (console); Cloudflare Tunnel (public apps + webhooks) |

## Architecture

```text
Mac / Desktop (Tailscale)
    │
    ▼
┌──────────────────────────────────┐
│  Atlas :8080  (UI + API)         │
│  React console  ·  REST handlers │
└──────────────┬───────────────────┘
               │
     ┌─────────┴─────────┐
     ▼                   ▼
 Postgres          Build worker
                         │
           ┌─────────────┴─────────────┐
           ▼                           ▼
    Kaniko Job / host build      Docker registry
           │                           │
           └───────────┬───────────────┘
                       ▼
              k3s Deployment → Service → Ingress
                       │
                       ▼
              https://app.edwardscott.dev
```

GitHub App push events → `hooks.edwardscott.dev` → Atlas API → create build.

## Console

```bash
make up                  # k3d + Postgres + API + Cloudflare Tunnel
```

Open `http://localhost:8080` (or `http://<tailscale-hostname>:8080`).

1. **New project** — name becomes the K8s resource / ingress host (`<name>.edwardscott.dev`)
2. **Connect GitHub** — install the Atlas GitHub App, then pick a repo + branch
3. **Deploy** — `POST /apps/{id}/builds` (same pipeline as webhooks)
4. **Push to re-deploy** — App webhooks fire automatically (no per-repo webhook config)
5. **Settings** — container port, unlink repo, manage GitHub access, or delete the project

Status (**Status** in the sidebar) shows registry, Kubernetes, webhook, and GitHub App hints.

UI routes use `/projects/...` and `/system` so they do not collide with the JSON API (`/apps`, `/status`).

## Project Status

| Phase | Feature | Status |
|-------|---------|--------|
| 0 | Health check, dev tooling | Done |
| 1 | Apps CRUD + Postgres | Done |
| 2 | Git source + webhooks | Done |
| 3 | Builds (clone, Kaniko/Docker, push) | Done |
| 4 | k3s runtime + console UI + GitHub App | Done |
| 5+ | Add-ons (Redis…), env vars, log streaming, deploy phases | Planned — see [TODO.md](TODO.md) |

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

3. Keep `edwardscott.dev` / `www` on GitHub Pages (unchanged)
4. SSL/TLS mode: **Full** (wait for Universal SSL / Edge Certificates if the zone is new)

### 2. GitHub App (recommended)

See **[docs/github-app.md](docs/github-app.md)**. You need App ID, slug, private key, and webhook secret matching `ATLAS_WEBHOOK_SECRET`. Set the app **Setup URL** to the same callback as install redirects.

### 3. Run Atlas

```bash
brew install k3d kubectl   # once
cp .env.example .env       # CLOUDFLARE_TUNNEL_TOKEN, ATLAS_WEBHOOK_SECRET, GitHub App vars

make up                    # k3d + Postgres + API + tunnel
make logs                  # optional
```

- Console: `http://localhost:8080` (or Tailscale)
- Deploy app `portfolio` → `https://portfolio.edwardscott.dev`
- GitHub App webhook → `https://hooks.edwardscott.dev/webhooks/github`

```bash
make down                  # stop everything
```

### Deploy requirements

- `Dockerfile` at repo root
- Container listens on port **80** by default, or set **Container port** in project Settings (e.g. `8080`)
- Public or private GitHub repos (private requires the GitHub App)
- App name becomes the hostname: `<name>.edwardscott.dev`

### Legacy: manual repo webhooks

If the GitHub App is not configured, link a repo URL and add a push webhook yourself:

1. `openssl rand -hex 32` → `ATLAS_WEBHOOK_SECRET`
2. Repo → Settings → Webhooks → `https://hooks.edwardscott.dev/webhooks/github`, secret above, event **Push**

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness check |
| `GET` | `/status` | Runtime hints (registry, k8s, ingress, GitHub App) |
| `GET` | `/apps` | List all apps |
| `POST` | `/apps` | Create app (`{"name":"..."}`) |
| `GET` | `/apps/{id}` | Get app by ID |
| `PATCH` | `/apps/{id}` | Update app (`{"port":8080}`) |
| `DELETE` | `/apps/{id}` | Teardown K8s resources and delete app |
| `PUT` | `/apps/{id}/repo` | Link repo (URL or `github_full_name` + `installation_id`) |
| `GET` | `/apps/{id}/repo` | Get linked repo |
| `DELETE` | `/apps/{id}/repo` | Unlink repo |
| `GET` | `/apps/{id}/builds` | List builds (newest first) |
| `POST` | `/apps/{id}/builds` | Trigger a manual build/deploy |
| `GET` | `/apps/{id}/builds/{build_id}` | Get build status and image |
| `GET` | `/auth/github/install` | Start GitHub App install |
| `GET` | `/auth/github/callback` | GitHub App setup/install callback |
| `GET` | `/github/installations` | List stored installations |
| `POST` | `/github/installations/sync` | Sync installations from GitHub API |
| `GET` | `/github/installations/{id}/repos` | List repos for an installation |
| `POST` | `/webhooks/github` | GitHub App / push webhook |

## Project Layout

```
Atlas/
├── api/              # HTTP server, handlers, SPA embedding
├── app/              # Domain types (App, Repo)
├── build/            # Build worker, clone/build/push pipeline
├── github/           # GitHub App JWT, tokens, install state
├── runtime/          # client-go — Deployments, Services, Ingress, Kaniko Jobs
├── webhook/          # GitHub HMAC verification and payload parsing
├── store/            # Postgres persistence and SQL migrations
├── web/              # React console (Vite); dist embedded at build time
├── cmd/api/          # API binary entrypoint
├── docs/             # Cloudflare + GitHub App setup
├── hack/             # k3d cluster + Docker entrypoint
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── TODO.md           # Next features (add-ons, env, logs, …)
└── go.mod
```

## Configuration

Copy `.env.example` to `.env` for local overrides. Key variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `ATLAS_PORT` | `8080` | HTTP listen port (UI + API) |
| `ATLAS_DATABASE_URL` | `postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable` | Postgres DSN |
| `ATLAS_WEBHOOK_SECRET` | — | HMAC secret for GitHub webhooks (and install state) |
| `ATLAS_WEBHOOK_PUBLIC_URL` | — | Public webhook URL shown in the UI |
| `ATLAS_GITHUB_APP_ID` | — | GitHub App ID |
| `ATLAS_GITHUB_APP_SLUG` | — | App slug (`https://github.com/apps/<slug>`) |
| `ATLAS_GITHUB_APP_PRIVATE_KEY_HOST` | — | Host path to `.pem` (Compose bind-mount) |
| `ATLAS_GITHUB_APP_PRIVATE_KEY` | — | PEM contents (alternative to file path) |
| `ATLAS_REGISTRY_URL` | `atlas-registry:5000` | Registry host (k3d in-cluster name; host port defaults to `localhost:5001`) |
| `ATLAS_REGISTRY_SECRET` | — | `dockerconfigjson` Secret for Kaniko push auth |
| `ATLAS_INSECURE_REGISTRY` | `true` (`.env.example`) | Allow insecure registries in Job builds |
| `ATLAS_KUBECONFIG` | — | Path to kubeconfig; empty uses in-cluster or `~/.kube/config` |
| `ATLAS_K8S_NAMESPACE` | `default` | Namespace for Deployments and build Jobs |
| `ATLAS_INGRESS_DOMAIN` | `edwardscott.dev` | Base domain (`portfolio.edwardscott.dev`) |
| `ATLAS_INGRESS_CLASS` | `traefik` | Ingress class |
| `ATLAS_INGRESS_TLS_SECRET` | — | Optional in-cluster TLS (usually empty; Cloudflare terminates TLS) |
| `CLOUDFLARE_TUNNEL_TOKEN` | — | **Required** for `make up` — Cloudflare Tunnel Docker token |

## Development

```bash
make up        # full stack
make down      # tear down
make logs      # api / postgres / tunnel
make test      # Go tests
make web-dev   # UI hot reload (API on :8080)
```

- [docs/cloudflare.md](docs/cloudflare.md) — tunnel + DNS  
- [docs/github-app.md](docs/github-app.md) — GitHub App registration  
- [TODO.md](TODO.md) — planned work (Redis add-ons, env vars, log streaming, deploy phases)

## License

Private — personal portfolio project.
