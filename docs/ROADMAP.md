# Lunar League roadmap

Canonical phase list for the product (the README status table mirrors this file).

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | Bootstrap: Docker stack, magic-link auth, league directory, multi-sport data plumbing | In progress |
| 1 | Off-season: player sync (NFL/NBA/MLB), league settings, mock draft | In progress |
| 2 | Draft room: snake + auction, keepers, live WebSocket draft | Scaffolded → hardening |
| 3 | Regular season: scoring, waivers, trades, chat | Scaffolded → hardening |
| 4 | Polish: PWA, push notifications, email digests, **OIDC (Authentik / Keycloak)** | Scaffolded |
| 5 | Multi-sport depth: NBA/MLB parity (schedules, live stats, category vs points modes) | In progress |

## Auth

- **Shipped:** Magic-link email (SMTP).
- **Planned (Phase 4):** OIDC authorization-code flow for self-hosters with an existing IdP. Not started in code; see `docs/SELF_HOSTING.md`.

## Data providers

- **Sleeper (default — current focus):** NFL + NBA players, schedules, weekly stats, injuries, trending via the public Sleeper API. MLB is not on Sleeper; while `DATA_PROVIDER=sleeper`, the worker uses the **MLB Stats API** for MLB player sync and stats. This is the **bare minimum** external footprint we maintain day to day.
- **SportsData.io:** **Deferred.** Scaffold exists (`apps/api/internal/provider/sportsdataio`); finish implementation, then document. Until then, self-hosters should stay on `DATA_PROVIDER=sleeper` (or `mlbstatsapi` for baseball-only).
- **MLB Stats API only:** `DATA_PROVIDER=mlbstatsapi` — baseball-only installs; NFL/NBA are not populated.

### Premium provider keys (planned)

1. **Phase A — operator:** keep env-based `SPORTSDATAIO_API_KEY` + `DATA_PROVIDER=sportsdataio` for people comfortable editing `.env` / secrets manager (e.g. OpenBao, Docker secrets).
2. **Phase B — product:** let **site admins** (`is_admin`) and/or **league commissioners** configure or rotate a SportsData.io (or other paid) key from the web app, stored server-side (encrypted at rest, never returned in full after save). Exact scope (global vs per-league) TBD when SDIO ships.

## Jobs / scale

- Worker uses in-process tickers today. **River** (already in `go.mod`) remains the upgrade path for durable, multi-replica jobs.

## Related ops notes

- Homelab redeploy checklist (VM 218): `docs/superpowers/plans/2026-05-05-lunarleague-openbao-deploy-rebuild.md`

## Multi-sport notes

- **NBA live stats** with Sleeper require `games.week` to match Sleeper’s `stats/nba/regular/{season}/{week}` week numbers. Populate `games` via the worker’s schedule sync (Sleeper schedule) before relying on live polling.
