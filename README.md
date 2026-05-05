# Lunar League

Open-source, self-hosted, Docker Compose deployable, multi-tenant fantasy sports platform.

NFL first. NBA and MLB next. Built around a pluggable sports-data provider model so you can run on free Sleeper API data or drop in a paid SportsData.io key for premium real-time stats.

## Why

Yahoo and ESPN are free, but they own your league, your data, and your attention. Sleeper is excellent but closed. **Lunar League** is the answer for friends who want the real fantasy experience on their own infrastructure.

## Status

Early development. See [the plan](.cursor/plans) for the roadmap.

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

For production deployment behind Caddy with TLS see [docs/SELF_HOSTING.md](docs/SELF_HOSTING.md). Optional: GitHub Actions **Deploy production** (`.github/workflows/deploy.yml`) — run it manually under **Actions** after configuring `LUNARLEAGUE_*` secrets; add a `push` trigger there if you want deploy-on-push.

## Architecture

- **Backend**: Go 1.23, chi, sqlc + pgx, river for jobs
- **Frontend**: Next.js 15 (App Router), Tailwind v4, shadcn/ui, TanStack Query
- **Storage**: Postgres 16, Redis 7
- **Realtime**: WebSockets fanned out via Redis pub/sub
- **Auth**: Magic link email by default; OIDC for Authentik / Keycloak users

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
