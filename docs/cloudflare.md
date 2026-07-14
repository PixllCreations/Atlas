# Cloudflare Tunnel + DNS for Atlas

Atlas publishes apps at `https://<project>.edwardscott.dev` and receives GitHub webhooks at `https://hooks.edwardscott.dev/webhooks/github`. Your portfolio stays on GitHub Pages at `edwardscott.dev` / `www`.

## Prerequisites

- Domain `edwardscott.dev` uses **Cloudflare nameservers** (set in Porkbun → transfer DNS to Cloudflare, or use Cloudflare as registrar).
- Docker, k3d, kubectl installed on the Atlas host.
- `make cluster` has been run at least once (k3d Traefik on host `:80`).

## 1. Create the tunnel (one time)

1. Open [Cloudflare Zero Trust](https://one.dash.cloudflare.com/) → **Networks** → **Tunnels**.
2. **Create a tunnel** → name it `atlas` → choose **Docker**.
3. Copy the **tunnel token** into `.env`:

```bash
CLOUDFLARE_TUNNEL_TOKEN=eyJhIjoi...
```

4. Do **not** run the printed `docker run` command — Atlas starts `cloudflared` via Compose.

## 2. Public hostnames (one time)

In the tunnel → **Public Hostname** → add two routes:

| Public hostname | Path | Service type | URL |
|-----------------|------|--------------|-----|
| `hooks.edwardscott.dev` | `*` | HTTP | `http://api:8080` |
| `*.edwardscott.dev` | `*` | HTTP | `http://host.docker.internal:80` |

Put **`hooks` above the wildcard**. Tunnel ingress is first-match; if the wildcard wins, webhook traffic hits Traefik and returns 404.

Cloudflare creates the DNS records in your zone automatically (CNAME to the tunnel).

**Leave existing GitHub Pages records alone** for apex and `www`:

```text
edwardscott.dev      → GitHub Pages (A/CNAME you already have)
www.edwardscott.dev  → GitHub Pages
*.edwardscott.dev    → tunnel (new — Atlas apps)
hooks.edwardscott.dev → tunnel (new — webhooks)
```

Wildcard `*.edwardscott.dev` does not override apex or `www`; those stay separate records.

## 3. SSL

In Cloudflare → **SSL/TLS** → set mode to **Full** (origin is HTTP on `:80` / `:8080`; Cloudflare terminates HTTPS for visitors).

## 4. Start Atlas

```bash
cp .env.example .env
# edit: CLOUDFLARE_TUNNEL_TOKEN, ATLAS_WEBHOOK_SECRET (openssl rand -hex 32)

make up
```

Console (private): `http://localhost:8080` or Tailscale.  
Public app: deploy project `portfolio` → `https://portfolio.edwardscott.dev`.  
GitHub webhook: `https://hooks.edwardscott.dev/webhooks/github` with secret `ATLAS_WEBHOOK_SECRET`.

For automatic webhooks (no manual repo setup), configure the GitHub App — see [github-app.md](./github-app.md).

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Tunnel container exits | Check token in `.env`; verify Zero Trust tunnel is healthy |
| `502` on `*.edwardscott.dev` | k3d not running — `make cluster` then `make up` |
| Webhook `401` | GitHub secret must match `ATLAS_WEBHOOK_SECRET` |
| App Ingress missing | `ATLAS_INGRESS_DOMAIN=edwardscott.dev` in `.env` |
