# ADR-008: Builder Isolation via Kubernetes Jobs

## Status

Accepted

## Context

ADR-005 established host-based builds: the API process clones repos and runs `docker build` / `docker push` on the host filesystem. That works for bootstrap but has drawbacks:

- Builds share the host Docker daemon and filesystem with the API
- No resource limits or isolation between concurrent builds
- Requires `git` and `docker` on the same machine as `atlas-api`
- A malicious or broken Dockerfile runs with host-level access

Phase 4 already requires a Kubernetes cluster for deploys. The cluster can also run builds in isolated Jobs.

## Decision

When Kubernetes is available, Atlas runs each **image build** as a **Batch Job** in the system namespace (`ATLAS_K8S_NAMESPACE`):

1. **Worker (API host)** — shallow-clones the repo once to read and validate `atlas.yaml` (requires `git` in the API image)
2. **Init container (Job)** — shallow `git clone` into a shared `emptyDir` for the build context
3. **Main container** — [Kaniko](https://github.com/GoogleContainerTools/kaniko) builds the Dockerfile and pushes to the registry

Job naming: `atlas-build-<build-id>` (one Job per build attempt).

Application workloads are **not** created in the system namespace. After a successful build, Atlas applies a `DeploymentPlan` into `atlas-<project-name>` (app + managed dependencies + Ingress).

The worker:

- Creates the Job after marking the build `running`
- Waits for Job completion (same synchronous flow as today)
- On success, saves the registry-qualified image and reconciles the project namespace
- On failure, marks the build `failed`

**Fallback:** If no deployer/registry Job path is configured, keep the host-based build path (ADR-005) using the same checkout that provided `atlas.yaml`.

**Registry auth:** Kaniko reads push credentials from a `dockerconfigjson` Secret mounted at `/kaniko/.docker/config.json`. Homelab registries may also need `--insecure` / `--skip-tls-verify` flags (configured via env).

**Private repos:** Supported when the app is linked through the GitHub App (installation token clone URL).

Atlas does **not** (yet):

- Run a separate `cmd/builder` binary — the worker submits Jobs via client-go
- Watch Jobs asynchronously — worker blocks until completion
- Support BuildKit or Docker-in-Docker — Kaniko is sufficient for Dockerfile-based builds without privileged pods

## Consequences

**Positive**

- Builds are isolated from the API host
- Resource limits can be applied to Job pods
- No Docker daemon required on the API machine when Job builds are used
- Same worker lifecycle and build status model — minimal API surface change

**Negative**

- Requires Kaniko-compatible cluster (works on k3s; needs registry access from the node/pod network)
- API host clones once for `atlas.yaml` in addition to the Job clone (Kaniko architecture trade-off)
- Job + init container adds latency vs host builds
- Registry auth secret must be created manually in the system namespace

**Follow-ups (out of scope for this ADR)**

- Async Job watching with build log streaming
- Private repo credentials via Secret
- Per-build resource limits via API
- Separate builder worker deployment
