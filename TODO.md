# Atlas — TODO / next work

Continue from this file. Context is current as of **2026-07-14**.

---

## Current platform snapshot

Atlas is a self-hosted PaaS (Go API + embedded React UI + Postgres + k3d/k3s).

**What works today**

- Apps CRUD, GitHub App install + push webhooks, builds (Kaniko Jobs), registry push
- Deploy: single container Deployment + ClusterIP Service + Traefik Ingress
- Per-app **container port** (`apps.port`, default 80; Service `80 → targetPort`)
- Public apps: `https://<name>.edwardscott.dev` via Cloudflare Tunnel (`*.edwardscott.dev → host.docker.internal:80`)
- Webhooks: `hooks.edwardscott.dev → http://api:8080` (must be listed **above** the wildcard hostname)
- Console: `http://localhost:8080` (Tailscale)

**Hard limits right now**

- One process per app (no Redis, Postgres, Nginx sidecars, or compose-style multi-service)
- No user-defined env vars on Deployments (`runtime/deploy.go` → `DeployOptions` is only `Namespace`, `Name`, `Image`, `Port`)
- Build status is coarse: `pending | running | succeeded | failed` (`store/builds.go`); UI polls every ~2.5s (`ProjectPage.tsx`)
- No log streaming API; worker only `log.Printf`s failures
- Teardown only deletes Ingress / Service / Deployment for the app name — not add-on services

**Motivating example:** `we-know-ball` listens on **8080**, needs **Redis**, homepage had a Gin `Location: ./` redirect loop (fix in that repo). Redis fails with `dial tcp [::1]:6379: connection refused` because Atlas never runs Redis.

---

## 1. Add-on / companion services (Redis, Nginx, …)

**Goal:** Declared dependencies that Atlas provisions alongside an app (like Railway plugins / Render disks+redis), not “only the Dockerfile CMD.”

### Suggested design

| Approach | Pros | Cons |
|----------|------|------|
| **A. Named add-ons** (`redis`, `postgres`, templates) | Simple UX, opinionated images | Less flexible |
| **B. Generic “services” table** (image, port, env, volumes) | Flexible (Nginx, custom) | More UI/validation |
| **C. In-repo `atlas.yaml` / compose subset** | Git-native | Parser + security surface |

**Recommended start: A → grow toward B.**

### Data model (sketch)

```sql
-- per-app companion workload
app_services (
  id UUID PK,
  app_id UUID FK apps,
  kind TEXT NOT NULL,          -- 'redis' | 'nginx' | 'custom'
  name TEXT NOT NULL,          -- k8s name suffix, e.g. we-know-ball-redis
  image TEXT NOT NULL,
  port INT NOT NULL,
  env JSONB NOT NULL DEFAULT '{}',
  status TEXT,
  unique (app_id, name)
)
```

### Runtime work (`runtime/`)

- `EnsureAddonDeployment` + `EnsureAddonService` (ClusterIP only; not on Ingress by default)
- Wire DNS for the app: inject env like `REDIS_URL=redis://<app>-redis:6379`
- On app delete: tear down addons too (`api/apps.go` delete path)
- Optional: Nginx as TLS/static front — usually Traefik already covers HTTP; Nginx add-on is for app-specific reverse proxy / static

### UI / API

- Settings → **Services** → “Add Redis”
- `POST /apps/{id}/services` `{ "kind": "redis" }`
- Redeploy (or hot-apply) so main Deployment gets connection env

### Decisions to make later

- Shared Redis vs per-app Redis (start **per-app**)
- Persistence (`emptyDir` vs PVC)
- Resource limits / same namespace vs `atlas-addons`

---

## 2. Configurable env vars for deployed projects

**Goal:** User-defined `KEY=VALUE` on the app Deployment (and optionally build Jobs).

### Data model (sketch)

```sql
app_env_vars (
  app_id UUID FK,
  key TEXT NOT NULL,
  value TEXT NOT NULL,       -- consider encryption at rest later
  secret BOOLEAN NOT NULL DEFAULT false,
  PRIMARY KEY (app_id, key)
)
```

### Runtime

- Extend `runtime.DeployOptions` with `Env []corev1.EnvVar`
- `build/worker.go` `deployApp`: load vars from store; merge with system vars (`PORT`, addon URLs)
- Optional: Kaniko/build Job env for build-time only (separate flag)
- Never log secret values

### API / UI

- `GET/PUT /apps/{id}/env` 
- Settings panel: key/value editor; mask secrets
- Changing env should prompt **Redeploy** (or auto-rollout Deployment)

### Precedence

1. Atlas system (`PORT`, `ATLAS_*`)
2. Addon-injected (`REDIS_URL`, …)
3. User env (user wins if keyed same? **or** forbid overrides of system keys)

---

## 3. Better deploy loading UX

**Goal:** Honest progress while a build is `pending`/`running`, not a bar that finishes early then hangs until succeed/fail.

### Current UI behavior

- `ProjectPage.tsx`: Deploy sets `busy`; polls builds every **2.5s** while any build is `pending`/`running`
- `StatusBadge` shows `pending` / `running` / `succeeded` / `failed`
- No real phase progress from the API today (status flips to `running` for the whole clone→build→push→deploy)

### Suggested UX

- Replace indeterminate/fake bar with **phase steps** driven by backend:
  1. Queued  
  2. Cloning  
  3. Building image  
  4. Pushing  
  5. Deploying  
  6. Healthy / Failed  
- Keep polling or upgrade to SSE/WebSocket (ties to #4)
- Disable Deploy while active; show elapsed time; link to failing step + logs

### Backend needed

- Finer build status or `build_events` / `phase` column updated in `build/worker.go` (`execute`, `runJobBuild`, `deployApp`)
- Alternatively: derive phase from Job pod phase + Deployment rollout (more k8s-y, less precise for host builds)

**Minimum:** `builds.phase TEXT` + `UpdateBuildPhase` calls at each worker step; UI step list reads `phase`.

---

## 4. Pass-through logging (live deploy logs in UI)

**Goal:** Stream Kaniko/build Job + app deploy logs into the console during/after a build.

### Sources

| Phase | Source |
|-------|--------|
| Build (Kaniko) | `kubectl logs job/atlas-build-<id> -f` / pod logs |
| Host build (rare) | worker stdout captured to store/stream |
| Deploy / runtime | `kubectl logs deploy/<app> -f` |

### API sketch

- `GET /apps/{id}/builds/{buildId}/logs` — SSE (`text/event-stream`) or WebSocket
- Query: `?follow=1` for live; without follow return stored ring buffer
- Auth later; for now same trust model as console (private Tailscale)

### Implementation notes

- Prefer **SSE** from API: worker or handler watches pod logs via client-go
- Persist last N KB in Postgres or object storage so refresh still works
- UI: terminal-like panel under Deployments; auto-scroll; filter stderr
- Register route **before** SPA catch-all in `api/ui.go` / mux order
- Vite proxy: add `/apps` already proxies — ensure EventSource works through proxy (`web/vite.config.ts`)

### Security

- Logs may contain secrets from build args — redact env values marked secret
- Scope logs to the requesting app’s namespace/name prefix

---

## Suggested implementation order

1. **Env vars (#2)** — smallest runtime change; unblocks Redis URL once Redis exists manually  
2. **Deploy phases + loading UI (#3)** — improves UX immediately; small schema change  
3. **Log streaming (#4)** — depends on knowing which Job/Deployment; natural fit after phases  
4. **Add-ons (#1)** — largest; injects env from #2; Redis for `we-know-ball`

---

## Key files to touch

| Area | Files |
|------|--------|
| Deploy | `runtime/deploy.go`, `runtime/service.go`, `build/worker.go` |
| Store | `store/apps.go`, new migrations, new `store/env.go` / `store/services.go` |
| API | `api/apps.go`, new `api/env.go`, `api/services.go`, `api/logs.go` |
| UI | `web/src/pages/ProjectPage.tsx`, `ProjectSettingsPage.tsx`, `web/src/api/client.ts` |
| Mux / proxy | `api/server.go`, `api/ui.go`, `web/vite.config.ts` |

---

## Related recent work (don’t regress)

- GitHub App: `docs/github-app.md` — **Setup URL** required; sync via `POST /github/installations/sync`
- Tunnel: `hooks.edwardscott.dev` must win over `*.edwardscott.dev`
- Container port: migration `006_app_port.sql`; Settings → Container port; Service `port=80` / `targetPort=<app.port>`
- Tests write to `ATLAS_TEST_DATABASE_URL` (defaults to same DB as Atlas — avoid polluting `github_installations` with fake IDs)

---

## Out of scope / later

- Multi-tenant auth on the console  
- Preview environments / PR deploys  
- Custom domains per app beyond `ATLAS_INGRESS_DOMAIN`  
- Encrypting env-at-rest, sealed secrets  
- Full docker-compose import  

---

## Quick verification after implementing

```bash
# env
curl -s localhost:8080/apps/$ID/env

# redis addon → from app pod
kubectl exec deploy/$NAME -- wget -qO- redis://$NAME-redis:6379

# logs stream
curl -N localhost:8080/apps/$ID/builds/$BUILD/logs?follow=1

# UI: Deploy shows phases; log panel scrolls; Settings edits env + add Redis
```
