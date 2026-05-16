# Theme Ball Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship Theme Ball as `schedule_type = theme_ball` with 30 toggleable modifiers, commissioner control, and owner majority votes.

**Architecture:** JSONB `theme_modifiers` on `league_settings`; catalog + config in `internal/themes`; HTTP on league service; scoring apply layer in Phase B. Votes use `theme_votes` + `theme_vote_ballots`.

**Tech Stack:** Go 1.23/chi/pgx, Next.js 15, Postgres 16.

**Spec:** `docs/superpowers/specs/2026-05-15-theme-ball-design.md` (approved)

---

## Phase A — Foundation (this session)

- [x] Migration `007_theme_ball.sql`
- [x] `internal/themes` catalog (30 slugs) + config merge/validate
- [x] League create accepts `schedule_type`; seeds `theme_modifiers`
- [x] `GET/PATCH /v1/leagues/{id}/theme-modifiers` + audit log
- [x] Vote CRUD + strict-majority tally
- [x] Web: create-league Theme Ball option + `/themes` setup page

## Phase B — Scoring engine

- [x] Weekly context builder (starters, player attrs, games)
- [x] Implement calculators per slug (v1: `franchise_stack_win`, `heaviest_team`, `tallest_team`, `prime_87`, `veteran_floor`, `bird_caucus`)
- [x] Wire into matchup scoring + `theme_breakdown` JSON (worker `matchup-scoring` job)
- [ ] Remaining 24 theme slugs

## Phase C — Polish

- [ ] In-season vote UI + commissioner audit view
- [ ] Commissioner guide + ROADMAP update

---

## Task 1: Database

**Files:** `apps/api/db/migrations/007_theme_ball.sql`

- `league_settings.theme_modifiers jsonb NOT NULL DEFAULT '{}'`
- `theme_modifier_audit`, `theme_votes`, `theme_vote_ballots` tables

## Task 2: Themes package

**Files:** `apps/api/internal/themes/catalog.go`, `config.go`, `config_test.go`

## Task 3: League API

**Files:** `apps/api/internal/league/themes.go`, modify `service.go`

## Task 4: Web

**Files:** `lib/types.ts`, `lib/theme-catalog.ts`, `app/leagues/new/page.tsx`, `app/leagues/[leagueId]/themes/page.tsx`, `setup/page.tsx`
