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
cp .env.production.example .env   # production VPS (HTTPS + real SMTP); see comments inside
# Or: cp .env.example .env            # local/dev-oriented defaults (Mailhog when using make dev)
$EDITOR .env
docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env up -d --build
```

That brings up:

- `postgres` (Postgres 16 with a named volume)
- `redis` (Redis 7 with AOF persistence)
- `api` (Go HTTP + WebSocket server on :8000)
- `worker` (Go background worker for sync, waivers, digests)
- `web` (Next.js standalone server on :3000)
- `caddy` (TLS on :80 / :443 when using [`docker-compose.caddy.yml`](../deploy/docker-compose.caddy.yml))

The `api` and `web` services still publish their ports on the host for debugging; in production you can firewall those and rely on Caddy only.

From the **repository root** you can also run `make prod-up` (same compose merge + `deploy/.env`).

### Repeatable deploy on the VPS

After DNS points at the server and `.env` is filled, use [`deploy/scripts/prod-deploy.sh`](../deploy/scripts/prod-deploy.sh) from the repo root:

```bash
chmod +x deploy/scripts/prod-deploy.sh
./deploy/scripts/prod-deploy.sh          # git pull + rebuild
MIGRATE=1 ./deploy/scripts/prod-deploy.sh   # also run migrations
```

### TLS without editing base compose

[`deploy/docker-compose.caddy.yml`](../deploy/docker-compose.caddy.yml) layers Caddy on top of [`deploy/docker-compose.yml`](../deploy/docker-compose.yml). Set **`CADDY_DOMAIN`** and **`CADDY_EMAIL`** in `.env` for Let's Encrypt.

The bundled [`Caddyfile`](../deploy/Caddyfile) routes `/v1/*`, `/healthz`, and `/ws/*` to the API, and everything else to Next.js.

## First-run checklist

1. `docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env run --rm api migrate up`
2. `docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env run --rm api seed` (inserts NFL/NBA/MLB rows in the `sports` table)
3. Open `https://your-domain/` and sign in with magic link.
4. Create your league. The first user becomes the commissioner.

## Operations

| Task | Command |
| --- | --- |
| Tail logs | `docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env logs -f api worker web caddy` |
| Re-sync players | `docker compose ... restart worker` (next tick syncs) |
| Backup DB | `docker compose ... exec postgres pg_dump -U $POSTGRES_USER $POSTGRES_DB > backup.sql` |
| Update | `git pull && docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env up -d --build` (from `deploy/`) |

### Automated deploy from GitHub Actions

If this repo is on GitHub, workflow **Deploy production** (`.github/workflows/deploy.yml`) SSHes into your VPS when you run it from **Actions → Deploy production → Run workflow**. Add repository secrets `LUNARLEAGUE_SSH_HOST`, `LUNARLEAGUE_SSH_USER`, `LUNARLEAGUE_SSH_KEY`, and `LUNARLEAGUE_DEPLOY_DIR` (absolute path on the server to **`LunarLeague/deploy`** — the folder that contains `docker-compose.yml`, optional `docker-compose.caddy.yml`, and `.env`). The workflow merges `docker-compose.caddy.yml` automatically when that file exists. To deploy on every push to `main`, add a `push` trigger back to that workflow after secrets are configured.

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
