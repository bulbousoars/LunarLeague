# Sports data providers

Lunar League runs on a pluggable `DataProvider` interface. Pick one (or write your own) by setting the `DATA_PROVIDER` env var.

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

## SportsData.io (paid — premium)

```bash
DATA_PROVIDER=sportsdataio
SPORTSDATAIO_API_KEY=your-key
```

For commissioners who want sub-minute live scoring during NFL games. The Fantasy Sports tier is ~$19/mo as of 2026 and includes real-time stats, projections, news, and DFS slates. (Implementation lives at `apps/api/internal/provider/sportsdataio` — currently scaffolded; see issue tracker for status.)

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
