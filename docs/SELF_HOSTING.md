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
- `caddy` (HTTP reverse proxy on **:80** when using [`docker-compose.caddy.yml`](../deploy/docker-compose.caddy.yml); put TLS at Traefik / your edge)

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

[`deploy/docker-compose.caddy.yml`](../deploy/docker-compose.caddy.yml) layers Caddy on top of [`deploy/docker-compose.yml`](../deploy/docker-compose.yml). Caddy listens on **:80** and forwards to **`api:${API_PORT}`** and **`web:${WEB_PORT}`** (defaults `8000` / `3000`). **`CADDY_DOMAIN` / `CADDY_EMAIL` are no longer required** for this layout—terminate HTTPS at Traefik (or another edge proxy) and reach this stack over HTTP.

The bundled [`Caddyfile`](../deploy/Caddyfile) routes `/v1/*`, `/healthz`, and `/ws*` to the API, and everything else to Next.js.

## First-run checklist

1. `docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env run --rm api migrate up`
2. `docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env run --rm api seed` (inserts NFL/NBA/MLB rows in the `sports` table). **Alternatively**, sign in as the league **commissioner** and use **Players → Seed sports** in the web UI (same effect). Site **admins** (`is_admin`) can also call **`POST /v1/admin/seed`**.
3. Open `https://your-domain/` and sign in with magic link.
4. Create your league. The first user becomes the commissioner.

To grant admin for the seed button (PostgreSQL):  
`docker compose ... exec postgres psql -U $POSTGRES_USER $POSTGRES_DB -c "UPDATE users SET is_admin = true WHERE email = 'you@example.com';"`

## Operations

| Task | Command |
| --- | --- |
| Tail logs | `docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env logs -f api worker web caddy` |
| Re-sync players | `docker compose ... restart worker` (next tick syncs) |
| Backup DB | `docker compose ... exec postgres pg_dump -U $POSTGRES_USER $POSTGRES_DB > backup.sql` |
| Update | `git pull && docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env up -d --build` (from `deploy/`) |

### Automated deploy from GitHub Actions

Workflow **Deploy production** (`.github/workflows/deploy.yml`) keeps production in sync with **`main`**:

- **On push** to `main` when `apps/`, `deploy/`, or the workflow file changes.
- **Manual:** Actions → **Deploy production** → **Run workflow**.

It SSHs to your host, **git fast-forwards** the repo root (parent of `deploy/`), runs **docker compose up --build**, **`api migrate up`**, then curls **`/healthz`** on the API port from `deploy/.env` (`API_PORT`, default `8000`).

**Setup:** see **[docs/DEPLOY_GITHUB_DUGGANCO.md](DEPLOY_GITHUB_DUGGANCO.md)** for secrets, optional variable **`LUNARLEAGUE_COMPOSE_OVERLAY`**, and the important note that **GitHub’s cloud runners cannot SSH to a private `192.168.x.x` host** unless you use a self-hosted runner, VPN, or similar.

**VM-only automation** (periodic `git pull` + Docker on the host, no GitHub credentials in the repo): **[docs/DEPLOY_VM_PERIODIC.md](DEPLOY_VM_PERIODIC.md)**.

Point **`PUBLIC_WEB_URL`** and **`PUBLIC_API_URL`** in `.env` at your real HTTPS domain (e.g. `https://lunarleague.dugganco.com`) so magic-link and league emails link correctly.

## Premium data provider (SportsData.io)

The free Sleeper provider covers **NFL and NBA**. **MLB** uses the official MLB Stats API automatically when `DATA_PROVIDER=sleeper` (no extra key). For all three sports on one commercial feed, use SportsData.io:

```bash
DATA_PROVIDER=sportsdataio
SPORTSDATAIO_API_KEY=your-key-here
```

SportsData.io subscriptions are per product; ensure your key includes the NFL, NBA, and/or MLB endpoints you need. The provider abstraction in `apps/api/internal/provider/` maps env → implementation.

**MLB-only** installs can set `DATA_PROVIDER=mlbstatsapi` (no Sleeper NFL/NBA universe).

## Hosting recommendations

- **Backups**: enable automated postgres dumps. The `LunarLeague/api` image has `pg_dump` as a stretch goal but for now use a host cron.
- **Monitoring**: hit `/healthz` with your monitoring tool. The handler checks Postgres (`Ping`) and Redis (`PING`) before returning 200.
- **Scaling**: a single 1 vCPU / 1 GB box handles a 12-team league with a live draft easily. Scale by adding API replicas behind Caddy; the WebSocket hub will fan out via Redis pub/sub. Worker should remain single-replica until river is wired in.

## Authentik / OIDC

OIDC (Authentik, Keycloak, etc.) is **planned Phase 4** work; see [docs/ROADMAP.md](ROADMAP.md). Magic-link email is the only supported login flow today.
