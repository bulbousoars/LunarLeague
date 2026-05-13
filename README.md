# Lunar League

Open-source, self-hosted, Docker Compose deployable, multi-tenant fantasy sports platform.

NFL first. NBA and MLB next. **Right now the supported stack is the free Sleeper API** (NFL/NBA) plus the MLB Stats API bridge for baseball when `DATA_PROVIDER=sleeper`. A paid **SportsData.io** provider is scaffolded for later (sub-minute live stats, projections); we will add a straightforward way for **site admins / commissioners** to supply or rotate that key after the integration is finished.

## Why

Yahoo and ESPN are free, but they own your league, your data, and your attention. Sleeper is excellent but closed. **Lunar League** is the answer for friends who want the real fantasy experience on their own infrastructure.

## Status

Early development. See [docs/ROADMAP.md](docs/ROADMAP.md) for the roadmap.

| Phase | Status |
| --- | --- |
| 0 — Bootstrap (auth, league directory, docker stack) | in progress |
| 1 — Off-season (player sync, settings, mock draft) | scaffolded |
| 2 — Draft room (snake + auction, keepers) | scaffolded |
| 3 — Regular season (scoring, waivers, trades, chat) | scaffolded |
| 4 — Polish (PWA, push, digests) | scaffolded |
| 5 — Multi-sport (NBA, MLB) | scaffolded |

## Quick start

Requires Docker and Docker Compose v2.

```bash
git clone https://github.com/bulbousoars/LunarLeague.git
cd LunarLeague
cp deploy/.env.example deploy/.env
make dev
```

Use the **`LunarLeague`** directory as the repo root (ignore any legacy `commish` folder from older layouts).

<details>
<summary>Already cloned?</summary>

```bash
cp deploy/.env.example deploy/.env
make dev
```

</details>

This brings up:

- Postgres 16
- Redis 7
- Mailhog (catches outgoing email) at <http://localhost:8025>
- Adminer at <http://localhost:8080>
- API at <http://localhost:8000>
- Web at <http://localhost:3000>

For production deployment behind Caddy with TLS see [docs/SELF_HOSTING.md](docs/SELF_HOSTING.md): copy `deploy/.env.production.example` to `deploy/.env`, fill secrets + SMTP, then from repo root run **`make prod-up`** (or use `deploy/scripts/prod-deploy.sh` on the server). **Homelab auto-update:** periodic pull + rebuild on the VM — [docs/DEPLOY_VM_PERIODIC.md](docs/DEPLOY_VM_PERIODIC.md) (no secrets in GitHub). **Optional CI/CD:** GitHub Actions **Deploy production** (`.github/workflows/deploy.yml`) on push to **`main`** or manual; see [docs/DEPLOY_GITHUB_DUGGANCO.md](docs/DEPLOY_GITHUB_DUGGANCO.md) if you use cloud runners + SSH.

Maintainers using **OpenBao SSH CA + wrapped broker** from Windows to reach the homelab: [docs/AGENT_HOMELAB_SSH.md](docs/AGENT_HOMELAB_SSH.md).

## Architecture

- **Backend**: Go 1.23, chi, sqlc + pgx, river for jobs
- **Frontend**: Next.js 15 (App Router), Tailwind v4, shadcn/ui, TanStack Query
- **Storage**: Postgres 16, Redis 7
- **Realtime**: WebSockets fanned out via Redis pub/sub
- **Auth**: Magic link email today; OIDC for Authentik / Keycloak is [planned Phase 4](docs/ROADMAP.md) (not in the codebase yet)

## Repository layout

```
LunarLeague/
  apps/
    api/    Go backend (single binary, server / worker mode)
    web/    Next.js frontend
  deploy/   Docker Compose stacks + Caddy config
  docs/     Self-hosting and commissioner guides
```

## License

MIT. See [LICENSE](LICENSE).
