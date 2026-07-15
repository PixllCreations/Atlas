# `atlas.yaml`

Every repository Atlas deploys must include an `atlas.yaml` at the **repository root**. Config is the source of truth for the application port and managed dependencies.

## Schema (version 1)

```yaml
version: 1

app:
  port: 8080

dependencies:
  redis:
    type: redis
```

| Field | Required | Notes |
|-------|----------|-------|
| `version` | yes | Must be `1` |
| `app.port` | yes | Container listen port, 1–65535. Injected as `PORT`. |
| `dependencies` | no | Map of dependency name → config |
| `dependencies.<name>.type` | yes | Currently only `redis` is provisioned; `postgres` / `nats` are recognized and rejected as unsupported |

### Names

Dependency keys become Kubernetes resource names (`Deployment` / `Service`). Use lowercase DNS labels (`redis`, `cache`). The name `app` is reserved for the primary workload.

Only **one** Redis dependency is allowed per project in the current design.

## What Atlas creates

For a project named `we-know-ball` with the example above:

| Resource | Location |
|----------|----------|
| Namespace | `atlas-we-know-ball` |
| `Deployment/app`, `Service/app`, `Ingress/app` | project namespace |
| `Deployment/redis`, `Service/redis` (ClusterIP `:6379`) | project namespace |

Application environment:

```env
PORT=8080
REDIS_URL=redis://redis:6379
```

Ingress host: `we-know-ball.<ATLAS_INGRESS_DOMAIN>` (e.g. `we-know-ball.edwardscott.dev`).

Kaniko build Jobs still run in `ATLAS_K8S_NAMESPACE` (system namespace).

## Application contract

- Root `Dockerfile` is always the primary workload.
- Listen on `app.port` (or honor `PORT`).
- Prefer `REDIS_URL` for Redis clients when a redis dependency is declared.

Absence of `atlas.yaml` fails the deploy with an explicit error.
