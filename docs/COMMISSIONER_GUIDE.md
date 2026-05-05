# Commissioner's guide

Welcome, commissioner. This is the operations manual for running a league on Lunar League.

## Setting up a league

1. Sign in (magic link).
2. **New league** -> name, sport, season, format.
3. Confirm step 3 -> you land on the setup page with the **invite link**.
4. Copy the invite link, blast it in your group chat.
5. Friends sign in, claim a team, set their team name and abbreviation.

## Before the draft

- **Settings** (`/leagues/<id>/settings`)
  - Roster slots: matters for waivers + lineup validation.
  - Waiver type: FAAB (default), Rolling, Reverse standings.
  - Trade deadline: set the week after which trades freeze.
  - Playoff weeks + count.
- **Scoring** (`/leagues/<id>/scoring`)
  - Tweak per-stat values. Default is half-PPR.
- **Schedule**: click "Generate" in settings once all teams are claimed.
- **Keepers** (keeper leagues): each manager designates kept players + their round cost. The commissioner can edit this on anyone's behalf.

## Running the draft

1. Open `/leagues/<id>/draft`.
2. Pick **Snake** or **Auction**. The draft is created in `pending`.
3. (Optional) reorder the draft order. Random by default.
4. Click **Start draft** when everyone's online. State machine moves to `in_progress`.
5. Each pick:
   - **Snake**: timer per pick. Top of the queue auto-picks if the clock hits zero.
   - **Auction**: nominator opens at $1+. Each bid resets the bid timer. Highest bid after silence wins.
6. **Pause** if someone needs a bathroom break. **Resume** when ready.
7. Draft finishes when all picks fill (or all rosters do, in auction). League moves to `in_season`.

## In-season responsibilities

- **Waivers**: process automatically on the league's schedule (default Wed 3am ET). Commissioner can manually re-run or override.
- **Trades**: review proposed trades. League can require votes (Phase 4 polish). Commissioner has final execute / veto authority.
- **Lineups**: lock at first NFL kickoff Sunday morning. Commissioners can unlock retroactively (audited).

## Troubleshooting

- **A team won't claim**: have them sign in fresh, hit the invite link again.
- **Stats stopped updating**: check `docker compose logs worker`. The Sleeper API has occasional outages; it retries.
- **Email not arriving**: check `SMTP_*` envs and look at your relay's outbound logs. In dev, mail goes to Mailhog at :8025.

## Recovery

- Postgres is the source of truth. Restore from `pg_dump` and you're back.
- Redis is ephemeral (live draft state, hot caches, pub/sub). Clearing it is safe between games.
