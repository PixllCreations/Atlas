# GitHub App for Atlas

Atlas can use a **GitHub App** so users connect GitHub once and get automatic push webhooks — no per-repo webhook setup.

## Register the app (one time)

1. Open [GitHub → Developer settings → GitHub Apps → New](https://github.com/settings/apps/new).
2. Configure:

| Field | Value |
|-------|--------|
| GitHub App name | e.g. `edward-scott-atlas` |
| Homepage URL | Your console URL (private is fine) |
| **Callback URL** | `https://hooks.edwardscott.dev/auth/github/callback` |
| **Setup URL** (Post installation) | `https://hooks.edwardscott.dev/auth/github/callback` |
| **Redirect on update** | ✅ checked |
| Webhook URL | `https://hooks.edwardscott.dev/webhooks/github` |
| Webhook secret | Same value as `ATLAS_WEBHOOK_SECRET` |
| Permissions | **Contents**: Read-only, **Metadata**: Read-only |
| Subscribe to events | **Push** |

> **Important:** After install, GitHub redirects to the **Setup URL**, not the Callback URL. Without a Setup URL, GitHub drops you on `github.com/settings/installations/...` and Atlas never records the connection.

3. Create the app, then note the **App ID**.
4. Generate a **private key** and download the `.pem` file.
5. Add to `.env`:

```bash
ATLAS_GITHUB_APP_ID=123456
ATLAS_GITHUB_APP_SLUG=edward-scott-atlas
# Host path (bind-mounted by docker-compose)
ATLAS_GITHUB_APP_PRIVATE_KEY_HOST=./edward-scott-atlas.private-key.pem
```

6. Restart Atlas: `make up`

## User flow

1. Open a project → **Connect GitHub**.
2. Install the app on your account or org and select repositories.
3. GitHub should redirect back to Atlas. If you already installed and land on the installation settings page, either click **Save** (with Redirect on update enabled) or use **Sync existing install** in Atlas.
4. Pick a repo and branch → **Link repository**.
5. Pushes to that branch trigger builds automatically.

## Cloudflare Tunnel routes

| Public hostname | Service |
|-----------------|---------|
| `hooks.edwardscott.dev` | `http://api:8080` |

Paths used:

- `POST /webhooks/github` — push events from GitHub
- `GET /auth/github/install` — start install
- `GET /auth/github/callback` — GitHub redirects here after install (Setup URL)

## Legacy mode

If GitHub App env vars are unset, Atlas falls back to manual repo URLs and per-repo webhooks (see `docs/cloudflare.md`).

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Lands on `github.com/settings/installations/...` | Set **Setup URL** to the callback URL; enable **Redirect on update** |
| Connect GitHub 404 | Set `ATLAS_GITHUB_APP_ID`, `ATLAS_GITHUB_APP_SLUG`, and private key |
| Already installed but Atlas shows disconnected | Click **Sync existing install** |
| Repo picker empty | Grant the app access to the repository, then refresh |
| Push ignored | Branch must match linked branch; check app has repo access |
| Private clone fails | Contents permission must be Read on the GitHub App |
