# Theme Ball — game type design spec

**Status:** Approved (2026-05-15)  
**Sport:** NFL only (v1)  
**Date:** 2026-05-15

## Summary

**Theme Ball** is a fourth `schedule_type` alongside `h2h_points`, `h2h_categories`, and `rotisserie`. Gameplay is standard head-to-head points with the same roster slots and base `scoring_rules` JSON — plus up to **30 optional theme modifiers** that apply multipliers or small bonuses to fantasy scoring based on roster identity, real NFL results, and weekly context.

Commissioners enable/disable each theme **before the draft** (setup UI: one control per theme). During the season, the **commissioner may toggle any theme immediately**, and **franchise owners may force a change by vote** (strict majority of eligible owners).

**Removed from earlier brainstorm (will not ship):**

- IR-slot compliance tied to NFL team concentration (“Giants stack” penalty).
- “No Cowboys” / anti-franchise gag rules.

---

## Franchise stack (revised)

**Slug:** `franchise_stack_win`

**Trigger:** For a given fantasy team and NFL week, find NFL franchises (`players.nfl_team`) with **≥3 starters** (players in non-`BN` lineup slots for that week).

**Bonus:** For each such franchise, if that NFL team **won its real game** that week (join `games` / schedule sync on `nfl_team` + week), apply a multiplier (default **×1.06**, commissioner-tunable) to **only those starters’** stat lines before base scoring. Multiple qualifying franchises on one roster are evaluated independently.

**No penalty** for not stacking; no IR requirement.

**Data:** `roster_slots` assignments per week, `players.nfl_team`, `games` outcomes (home/away, scores, status `final`).

---

## Game type registration

| Field | Value |
|-------|--------|
| `league_settings.schedule_type` | `'theme_ball'` |
| Matchup structure | Same as `h2h_points` |
| Base scoring | Existing `scoring_rules` JSON |
| Theme config | New `theme_modifiers` JSONB on `league_settings` (or dedicated table) |

Creating a league with `schedule_type = theme_ball` seeds default NFL `scoring_rules` and an empty-or-preset `theme_modifiers` document. Web copy: *“Theme Ball — H2H points with optional roster-identity modifiers.”*

---

## Theme modifier catalog (30)

Each entry has: **slug**, **name**, **default_enabled** (all `false` for v1), **comparator** (how league ranks teams), **effect**. Strength defaults are suggestions; commissioner can tune in advanced settings later.

Comparative themes (#1–5, 11–13, 19–20): rank all teams weekly after lineup lock; **#1** gets bonus, **last** gets inverse (or only top/bottom — document per row).

| # | Slug | Name | Effect (defaults) |
|---|------|------|-------------------|
| 1 | `heaviest_team` | Heaviest | #1 avg starter weight: `rush_td` ×1.08; last ×0.92 |
| 2 | `tallest_team` | Tallest | #1 avg height: `rec_td` ×0.95; last ×1.05 |
| 3 | `widest_spread` | Widest | Largest max−min weight on roster: `fum_lost` ×1.15; narrowest ×0.90 |
| 4 | `lightest_skill` | Lightest skill | RB/WR/TE avg weight #1: `rec_yd` ×1.05; heaviest skill ×1.05 `rush_yd` |
| 5 | `tower_te` | Tower TE | #1 TE room avg height: TE `rec_td` ×1.10; last TE height ×0.95 |
| 6 | `franchise_stack_win` | Franchise stack | ≥3 starters same NFL team + NFL win → those starters ×1.06 (see above) |
| 7 | `bird_caucus` | Bird caucus | Most starters on PHI/ATL/BAL/ARI/SEA: `def_td`, `int` ×1.15 |
| 8 | `underdog_market` | Small market | Most players from configured small-market teams: `rec` ×1.03 |
| 9 | `division_grudge` | Division grudge | Starter vs division opponent that week: that player’s stats ×1.04 |
| 10 | `oldest_lineup` | Oldest | #1 avg age: DST/`int` ×1.20; youngest `rush_yd` ×1.05 |
| 11 | `rookie_factory` | Rookie factory | Most `years_exp≤1`: `rec_td` ×1.12, `fum_lost` ×1.15 |
| 12 | `veteran_floor` | Veteran floor | Highest avg `years_exp`: each `rec` +0.5 PPR-style flat |
| 13 | `jersey_chaos` | Jersey chaos | Highest avg jersey #: `pass_yd` ×0.97; lowest `rush_td` ×1.05 |
| 14 | `prime_87` | Prime 87 | Rostered player wearing #87 that week: their `rec_td` ×1.25 |
| 15 | `sec_speed` | SEC speed | Most SEC colleges: `rush_yd` ×1.05 |
| 16 | `big_ten_grit` | Big Ten grit | Most Big Ten colleges: `rec` ×1.05 |
| 17 | `ivy_accountant` | Ivy accountant | Most Ivy: `rec` PPR −0.25, `pass_td` +1 flat per TD |
| 18 | `long_names` | Long names | Longest avg last name length: `pass_yd` ×1.03; shortest `rec_yd` ×1.03 |
| 19 | `alliteration` | Alliteration | ≥3 starters same first/last initial: weekly +5 or −5 coin flip |
| 20 | `rb_hoarder` | RB hoarder | Most RBs rostered: `rush_td` ×1.08, RB `rec` ×0.95 |
| 21 | `zero_rb` | Zero-RB | Fewest RBs: WR `rec_td` ×1.06; most RBs `rush_td` ×0.97 |
| 22 | `kicker_chaos` | Kicker chaos | ≥2 K rostered: all non-K stats ×0.98; 0 K: `def_st_td` +2 flat |
| 23 | `bench_mob` | Bench mob | Deepest bench: if bench total &lt;10 pts, −1; if bench beats opponent bench, +2 |
| 24 | `questionable_gambit` | Questionable | Most Q tags among starters: if they play, ×1.06; benched Q who scored 10+ FP, −1 |
| 25 | `iron_man` | Iron man | Fewest injury tags on starters: `fum_lost` ×0.90 |
| 26 | `bye_survivor` | Bye survivor | Most starters on bye: each non-bye starter ×1.04 |
| 27 | `primetime_mayor` | Primetime | Most FP from 7pm+ kickoffs: `rec` +0.5 each; early-only `rush_yd` ×1.03 |
| 28 | `weather_goblin` | Weather goblin | **v2** — most outdoor cold snaps; `rush_td` ×1.05 (needs weather feed) |
| 29 | `wheel_of_stat` | Wheel of stat | Commissioner/random weekly stat ×1.15 league-wide; prior week winner ×0.90 on that stat |
| 30 | `motto_or_tax` | Motto | Empty team motto: random stat ×0.98 until set; optional league vote for +1 |

**Note:** #28 ships disabled until weather data exists; still appears in UI as “coming soon” toggle (locked off).

---

## Configuration model

### Storage

Add to `league_settings`:

```json
{
  "theme_modifiers": {
    "heaviest_team": { "enabled": true, "strength": 1.0 },
    "franchise_stack_win": { "enabled": true, "multiplier": 1.06, "min_starters": 3 },
    "...": { "enabled": false }
  }
}
```

- `enabled` — boolean per slug.
- Optional `strength` (0.5–2.0) scales deviation from 1.0 for comparative themes.
- Per-theme overrides (e.g. `min_starters` for franchise stack).

Migration: new JSONB column `theme_modifiers` default `'{}'`, backfill null → all disabled for non–theme_ball leagues.

### When modifiers apply

| Phase | Behavior |
|-------|----------|
| Pre-draft (`status = setup`) | Commissioner + co-commissioners edit toggles freely; UI shows all 30 as **switches** (not mutually exclusive — any combination). |
| Drafting / in season | Toggles change only via **commissioner action** or **passed vote** (below). |
| Scoring | After lineup lock for week W: compute theme ranks → apply per-player multipliers → then `Compute(base_rules, adjusted_stats)`. |

**Recommendation (from prior discussion):** Comparative themes (#1–5, 10–13, 19–21) **recompute weekly**; identity themes (#6–9, 14–18, 22–30) also weekly where they depend on starters/injuries/schedule.

---

## Governance: commissioner vs owner vote

### Commissioner

- Any time: set `enabled` true/false on any theme (immediate for **next** scoring run).
- Logged in `theme_modifier_audit` (who, when, old/new).
- Cannot block a **passed** owner vote without a separate “commissioner veto” setting (default: **no veto**).

### Owner vote

**Eligibility:** One ballot per **claimed** `teams.owner_id` in the league.

**Proposal types:**

1. Enable theme slug X  
2. Disable theme slug X  

**Process:**

1. Any owner opens a proposal (7-day window or until quorum).
2. Owners vote Yes / No (no abstain in v1 — non-vote = No).
3. **Passes if:** `yes_votes > floor(eligible_owners / 2)` — strict majority of **all** eligible owners, not only voters.
4. On pass: config updates at **next lineup lock** (or immediately if no games locked that week).

**Tables (v1):**

- `theme_votes` — id, league_id, slug, action (`enable`|`disable`), opened_by, opens_at, closes_at, status (`open`|`passed`|`failed`|`cancelled`)
- `theme_vote_ballots` — vote_id, team_id, user_id, yes boolean, cast_at

Commissioner may **cancel** an open vote before close.

### Pre-draft UI

- League setup / draft lobby: grid of 30 themes with **toggle switches**, short description tooltip, link to full rules doc.
- “Enable all” / “Disable all” for sandbox leagues.
- Snapshot saved when draft starts (`theme_modifiers_snapshot` on draft record optional).

---

## Scoring pipeline (implementation sketch)

```
for each starter player P in week W:
  stats = normalized box score
  mult = 1.0
  for each enabled theme T:
    mult *= themeMultiplier(T, franchise, league, W, P, stats)
  adjusted = scaleStats(stats, mult)  // or per-stat map
  fp += Compute(base_rules, adjusted)
```

- Store breakdown in `matchup_lineup_scores` (new JSONB `theme_breakdown`) for transparency UI.
- Matchup page: “Your themes this week” card + per-player chips (“Franchise stack ×1.06”).

---

## API / UI (phased)

### Phase A — Data + scoring

- Migration: `theme_modifiers`, vote tables, `schedule_type` check includes `theme_ball`.
- `internal/scoring/themes/` package: registry of 30 slugs → calculator funcs.
- Wire into weekly scoring job (same path as existing matchup points).

### Phase B — Setup + commissioner

- `GET/PATCH /v1/leagues/{id}/theme-modifiers` (commish/admin; PATCH blocked in-season except audit path).
- `POST /v1/leagues/{id}/theme-votes` + ballot endpoints.

### Phase C — Web

- Create league: schedule type **Theme Ball**.
- Setup page: 30 toggles.
- Season: vote UI + audit log.

---

## Out of scope (v1)

- NBA/MLB themes.
- Paid SportsData.io-only fields.
- Commissioner veto (unless added later).
- Weighted voting (co-owners, fractional teams).

---

## Open questions

1. **Vote threshold:** Confirm strict majority of *all* owners (non-vote = no) vs majority of *ballots cast*.
2. **#28 Weather:** Ship as disabled placeholder or omit from the 30 until data exists?
3. **Coin-flip theme (#19):** Acceptable RNG in production, or replace with deterministic tie-break?

---

## Approval

Reply **approved** (with any edits to open questions) to proceed to an implementation plan (`writing-plans` / `docs/superpowers/plans/...`).
