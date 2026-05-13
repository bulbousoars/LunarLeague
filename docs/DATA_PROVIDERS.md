# Sports data providers

Lunar League runs on a pluggable `DataProvider` interface. Pick one (or write your own) by setting the `DATA_PROVIDER` env var.

## What we run today

**Default and recommended:** `DATA_PROVIDER=sleeper`. That path is what we develop and test against: NFL/NBA from Sleeper, MLB player sync and stats from the **MLB Stats API** helper (no extra key). Treat this as the **bare minimum Sleeper footprint** until premium feeds are fully integrated.

## Sleeper (default — free)

```bash
DATA_PROVIDER=sleeper
```

Uses the public, unauthenticated [Sleeper API](https://docs.sleeper.com/) at `https://api.sleeper.app/v1/`.

| Capability | Status |
| --- | --- |
| NFL player universe | yes |
| NBA player universe | yes |
| MLB player universe | not supported |
| Weekly stats | yes (post-game) |
| Live in-game stats | partial — Sleeper updates fast, but isn't a true sub-minute push feed |
| Injuries | yes |
| Schedule | yes |
| Trending adds/drops | yes |
| Auth | none (free) |
| Rate limits | ~1000 req/min |

## SportsData.io (paid — deferred)

**Not production-ready.** Code lives under `apps/api/internal/provider/sportsdataio` as a starting point; wiring, coverage, and tests are incomplete. Prefer **`DATA_PROVIDER=sleeper`** until this is explicitly marked done in [ROADMAP.md](ROADMAP.md).

When we ship it, keys will start as **deployment env** (`SPORTSDATAIO_API_KEY`) for operators; a follow-up is **UI for admins/commissioners** to set or rotate a league-scoped or instance-scoped key without editing server files (see roadmap).

## MLB Stats API (free, MLB only)

```bash
DATA_PROVIDER=mlbstatsapi
```

The official MLB Stats API at `https://statsapi.mlb.com/api/v1/`. Free, no auth required. Rate limits are forgiving.

| Capability | Status |
| --- | --- |
| Player universe | yes |
| Schedule | yes |
| Live stats | scaffolded (game-by-game data is available; categories vs. points is a config decision) |
| Injuries | not yet wired |

## Writing your own

`provider.DataProvider` is a small interface in [`apps/api/internal/provider/provider.go`](../apps/api/internal/provider/provider.go). Implement it, register the constructor in `cmd/server/serve.go`, and you're done. We'd love a PR.
