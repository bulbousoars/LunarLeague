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

- **Sleeper (default):** NFL + NBA players, schedules, weekly stats, injuries, trending. MLB is not on Sleeper; the API worker delegates MLB player sync (and MLB stats when polling) to the **MLB Stats API** provider while `DATA_PROVIDER=sleeper`.
- **SportsData.io:** Optional paid key. NFL, NBA, and MLB on one key when your subscription includes those feeds (`DATA_PROVIDER=sportsdataio` + `SPORTSDATAIO_API_KEY`).
- **MLB Stats API only:** `DATA_PROVIDER=mlbstatsapi` — useful for baseball-only installs; NFL/NBA are not populated.

## Jobs / scale

- Worker uses in-process tickers today. **River** (already in `go.mod`) remains the upgrade path for durable, multi-replica jobs.

## Related ops notes

- Homelab redeploy checklist (VM 218): `docs/superpowers/plans/2026-05-05-lunarleague-openbao-deploy-rebuild.md`

## Multi-sport notes

- **NBA live stats** with Sleeper require `games.week` to match Sleeper’s `stats/nba/regular/{season}/{week}` week numbers. Populate `games` via the worker’s schedule sync (Sleeper schedule) before relying on live polling.
