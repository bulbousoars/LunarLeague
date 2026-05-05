# Self-hosting Lunar League

This guide gets you from "fresh Linux box" to "your buddies are signing up" in about 15 minutes.

## What you need

- A Linux server (any 1+ GB VPS works for a 12-team league: Hetzner, DigitalOcean, your homelab Proxmox VM, etc.)
- Docker 26+ and the Docker Compose v2 plugin
- A domain pointing at the server (for TLS via Caddy)
- An outbound SMTP relay you can authenticate to (Postmark, Resend, SendGrid, your provider). Magic-link emails will not deliver reliably from a self-hosted IP without a relay.
- Production `.env` must use your relay hostname (**do not** use `SMTP_HOST=mailhog` unless Mailhog runs on that compose stack — it exists only in the dev overlay). Port **587** uses STARTTLS automatically; **465** uses implicit TLS. Only use **`SMTP_ALLOW_PLAINTEXT=true`** for legacy internal relays (not on the public Internet).

## Quick start (production)

```bash
git clone https://github.com/bulbousoars/LunarLeague.git
cd LunarLeague/deploy
cp .env.example .env
$EDITOR .env   # set domain, secrets, SMTP creds
docker compose --env-file .env up -d --build
```

That brings up:

- `postgres` (Postgres 16 with a named volume)
- `redis` (Redis 7 with AOF persistence)
- `api` (Go HTTP + WebSocket server on :8000)
- `worker` (Go background worker for sync, waivers, digests)
- `web` (Next.js standalone server on :3000)

The `api` service publishes `:8000` so a reverse proxy can sit in front.

### Putting Caddy in front for TLS

Add Caddy as another service in `docker-compose.yml`:

```yaml
caddy:
  image: caddy:2-alpine
  restart: unless-stopped
  ports: ["80:80", "443:443"]
  volumes:
    - ./Caddyfile:/etc/caddy/Caddyfile
    - caddy_data:/data
    - caddy_config:/config
  environment:
    CADDY_DOMAIN: ${CADDY_DOMAIN}
    CADDY_EMAIL: ${CADDY_EMAIL}
  depends_on: [api, web]
```

The bundled [`Caddyfile`](../deploy/Caddyfile) routes `/v1/*`, `/healthz`, and `/ws/*` to the API, and everything else to Next.js.

## First-run checklist

1. `docker compose run --rm api migrate up`
2. `docker compose run --rm api seed` (inserts NFL/NBA/MLB rows in the `sports` table)
3. Open `https://your-domain/` and sign in with magic link.
4. Create your league. The first user becomes the commissioner.

## Operations

| Task | Command |
| --- | --- |
| Tail logs | `docker compose logs -f api worker web` |
| Re-sync players | `docker compose restart worker` (next tick syncs) |
| Backup DB | `docker compose exec postgres pg_dump -U $POSTGRES_USER $POSTGRES_DB > backup.sql` |
| Update | `git pull && docker compose --env-file .env up -d --build` (from your `deploy/` directory) |

### Automated deploy from GitHub Actions

If this repo is on GitHub, workflow **Deploy production** (`.github/workflows/deploy.yml`) SSHes into your VPS when you run it from **Actions → Deploy production → Run workflow**. Add repository secrets `LUNARLEAGUE_SSH_HOST`, `LUNARLEAGUE_SSH_USER`, `LUNARLEAGUE_SSH_KEY`, and `LUNARLEAGUE_DEPLOY_DIR` (absolute path on the server to the folder that contains `docker-compose.yml` and `.env`). To deploy on every push to `main`, add a `push` trigger back to that workflow after secrets are configured.

Point **`PUBLIC_WEB_URL`** and **`PUBLIC_API_URL`** in `.env` at your real HTTPS domain (e.g. `https://fantasy.yourdomain.com`) so magic-link and league emails link correctly.

## Premium data provider (SportsData.io)

The free Sleeper provider gives weekly stats post-game. If you want sub-minute live scoring during NFL game windows, switch to SportsData.io:

```bash
DATA_PROVIDER=sportsdataio
SPORTSDATAIO_API_KEY=your-key-here
```

SportsData.io's Fantasy Sports tier is ~$19/mo as of 2026. The provider abstraction in `apps/api/internal/provider/` makes the switch a one-env-var change.

## Hosting recommendations

- **Backups**: enable automated postgres dumps. The `LunarLeague/api` image has `pg_dump` as a stretch goal but for now use a host cron.
- **Monitoring**: hit `/healthz` with your monitoring tool. The endpoint returns 200 only when the API can talk to Postgres + Redis.
- **Scaling**: a single 1 vCPU / 1 GB box handles a 12-team league with a live draft easily. Scale by adding API replicas behind Caddy; the WebSocket hub will fan out via Redis pub/sub. Worker should remain single-replica until river is wired in.

## Authentik / OIDC

If you already run Authentik (or any OIDC IdP), Lunar League will plug in via OIDC in Phase 3 polish work. Until then, magic-link email is the supported flow.
