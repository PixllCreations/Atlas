# Atlas — TODO / next work

Context as of **2026-07-15**.

---

## Current platform snapshot

Atlas is a self-hosted, config-driven PaaS (Go API + embedded React UI + Postgres + k3d/k3s).

**What works today**

- Apps CRUD, GitHub App install + push webhooks, Kaniko Job builds, registry push
- Per-project namespaces (`atlas-<name>`), Ownership labels, delete-by-namespace teardown
- Root `atlas.yaml` → parse/validate → `DeploymentPlan` → reconcile Deploy/Service/Ingress
- Managed **Redis** dependency (ClusterIP only, injects `REDIS_URL` + `PORT`)
- Infrastructure snapshot API + read-only Dependencies panel on the project page
- Build phases (`queued` → `cloning` → `building` → `pushing` → `deploying`) + project page step UX
- Live build logs over SSE (`GET /apps/{id}/builds/{build_id}/logs?follow=1`) + log panel
- Runtime pod logs per service (`GET /apps/{id}/workloads`, `.../workloads/{name}/logs?follow=1`)
- Public apps via Cloudflare Tunnel (`*.edwardscott.dev`); webhooks on `hooks.edwardscott.dev`
- Console on Tailscale / localhost `:8080`

**Example:** [`we-know-ball`](https://github.com/PixllCreations/we-know-ball) — port `8080`, Redis, root Dockerfile with embedded SPA.

**Still coarse**

- No user-defined secrets/env in the console (only atlas/plan-injected vars)
- Only Redis is provisioned; Postgres/NATS types are rejected
- Secret redaction in logs deferred until user secrets exist

---

## 1. User env vars / secrets

**Goal:** Let operators set extra `KEY=VALUE` on the app Deployment (and mark secrets).

- Store encrypted-at-rest or at least masked in UI
- Merge into plan env with deterministic precedence: system (`PORT`) → dependency URLs → user env
- Forbid overriding Atlas/dependency keys
- Settings UI + redeploy prompt

---

## 2. More dependency types

**Goal:** Grow the provisioner registry without changing the worker switch.

Candidates:

| Type | Notes |
|------|-------|
| `postgres` | Pin image; PVC optional phase-2 |
| `nats` | ClusterIP only |

Keep opinionated: reject unsupported options rather than half-configured templates.

---

## 3. Deploy phases + loading UX — done

Show Cloning → Building → Pushing → Deploying via `builds.phase`, step list + elapsed time, Deploy disabled while active.

---

## 4. Live build / runtime logs — done

Stream build logs via SSE. Runtime tab streams selected workload pod logs (`app`, `redis`, …) from the project namespace.

---

## Suggested order

1. ~~Deploy phases (#3)~~  
2. ~~Log streaming (#4)~~  
3. User env (#1) — unlocks app-specific config without new deps  
4. Postgres provisioner (#2) — next managed dependency  

---

## Out of scope / later

- Multi-tenant console auth  
- PR preview environments   
- Redis / Postgres PVC persistence (ephemeral is fine for demos)

---

## Related docs

- [README.md](README.md)  
- [docs/atlas-yaml.md](docs/atlas-yaml.md)  
- [docs/github-app.md](docs/github-app.md)  
- [docs/cloudflare.md](docs/cloudflare.md)  
