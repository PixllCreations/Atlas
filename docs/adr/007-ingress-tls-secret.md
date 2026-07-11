# ADR-007: TLS via Ingress Secret Reference

## Status

Accepted

## Context

Phase 4 adds Ingress-based external access for deployed apps. Homelab users need HTTPS, but Atlas is not yet a certificate authority or cert-manager integration layer.

Common homelab setups already have:

- A wildcard certificate for `*.homelab.local`
- A `kubernetes.io/tls` Secret in the deploy namespace
- Traefik (or another ingress controller) terminating TLS at the edge

We need a minimal way to enable HTTPS without introducing certificate issuance, renewal, or per-app cert configuration in Atlas.

## Decision

Atlas enables TLS by referencing an **existing** Kubernetes TLS Secret on each app Ingress:

- Add optional `TLSSecretName` to `runtime.IngressOptions`
- When set, populate `spec.tls` with the app host and secret name
- Wire configuration through `ATLAS_INGRESS_TLS_SECRET` → `WorkerConfig.IngressTLSSecret` → `EnsureIngress`
- When unset, Ingress remains HTTP-only (unchanged behavior)

Atlas does **not**:

- Create or upload TLS certificates
- Integrate with cert-manager or Let's Encrypt
- Manage certificate renewal

## Consequences

**Positive**

- Simple homelab path: one wildcard secret covers all apps
- Fail-soft: empty env var preserves current HTTP behavior
- No new cluster dependencies beyond what the user already installed
- TLS logic is testable without a live cluster (`ingressTLS` unit tests)

**Negative**

- Users must create and maintain the TLS Secret themselves
- Wildcard certs work well; per-app certs require manual secret management or future automation
- No automatic HTTP → HTTPS redirect yet

**Follow-ups (out of scope for this ADR)**

- cert-manager integration for automatic Let's Encrypt certs
- Per-app TLS configuration via API
- HTTP → HTTPS redirect on Ingress
