"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { api, ApiError } from "@/lib/api";
import type {
  League,
  PlayerStatsWindowsResponse,
  PlayersListResponse,
} from "@/lib/types";
import {
  formatHeightInches,
  statCell,
} from "@/lib/player-stats";
import { positionsForSport } from "@/lib/sport-ui";
import { useAuth } from "@/lib/auth-context";

const PAGE_SIZE = 100;
/** Season summed for YTD + per-week avg columns (`player_stats`, week &gt; 0). */
const AGGREGATE_STATS_SEASON = 2025;

/** Sticky first column — must use opaque bg + border-separate on table for clean scroll. */
const stickyPlayerTh =
  "sticky left-0 z-30 min-w-[10.5rem] max-w-[13rem] whitespace-normal bg-card " +
  "border-r border-border shadow-[6px_0_14px_-6px_rgba(0,0,0,0.55)]";
const stickyPlayerTd =
  "sticky left-0 z-20 min-w-[10.5rem] max-w-[13rem] whitespace-normal bg-card " +
  "border-r border-border shadow-[6px_0_14px_-6px_rgba(0,0,0,0.55)] align-middle";
const bioTh =
  "px-2.5 py-2.5 text-left align-bottom text-[11px] font-semibold uppercase tracking-wide text-muted";
const bioTd =
  "px-2.5 py-2.5 align-middle text-[13px] tabular-nums text-muted/95 leading-snug";
const bioTextTd =
  "px-2.5 py-2.5 align-middle text-[13px] leading-snug text-muted/95 max-w-[9rem] truncate";
const statGroupHeader =
  "border-l border-border/90 bg-bg/40 px-2 py-2 text-center text-[12px] font-semibold normal-case tracking-normal text-fg";
const statKeyTh =
  "border-l border-border/80 bg-bg/35 px-2 py-1.5 text-right font-mono text-[11px] font-medium normal-case tracking-normal text-muted";
const statBandTdClass = (band: "wk" | "ytd" | "avg") => {
  const bandBg =
    band === "wk"
      ? "bg-sky-500/[0.07]"
      : band === "ytd"
        ? "bg-violet-500/[0.07]"
        : "bg-amber-500/[0.07]";
  return `border-l border-border/80 px-2 py-1.5 text-right font-mono text-[12px] tabular-nums text-muted/95 min-w-[3.25rem] ${bandBg}`;
};

export default function PlayersPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");
  const [page, setPage] = useState(1);
  const [onlyWithTeam, setOnlyWithTeam] = useState(true);
  const [includeStats, setIncludeStats] = useState(true);
  /** Optional override for which `player_stats` row joins as “latest week”. */
  const [statsSeasonIn, setStatsSeasonIn] = useState("");
  const [statsWeekIn, setStatsWeekIn] = useState("");
  const { user, loading: authLoading } = useAuth();
  const qc = useQueryClient();

  const league = useQuery({
    queryKey: ["league", leagueId],
    queryFn: () => api<League>(`/v1/leagues/${leagueId}`),
  });

  const sport = (league.data?.sport_code ?? "nfl").toLowerCase();

  const offset = (page - 1) * PAGE_SIZE;

  const playersUrl = useMemo(() => {
    const q = new URLSearchParams();
    q.set("sport", sport);
    q.set("limit", String(PAGE_SIZE));
    q.set("offset", String(offset));
    if (search.trim()) q.set("q", search.trim());
    if (position) q.set("position", position);
    if (onlyWithTeam) q.set("has_team", "1");
    if (includeStats) {
      q.set("include_stats", "1");
      q.set("aggregate_season", String(AGGREGATE_STATS_SEASON));
      const ss = statsSeasonIn.trim();
      const sw = statsWeekIn.trim();
      if (ss !== "" && sw !== "") {
        q.set("season", ss);
        q.set("week", sw);
      }
    }
    return `/v1/players?${q.toString()}`;
  }, [
    sport,
    offset,
    search,
    position,
    onlyWithTeam,
    includeStats,
    statsSeasonIn,
    statsWeekIn,
  ]);

  const players = useQuery({
    queryKey: ["players", "browse", playersUrl, leagueId],
    queryFn: () => api<PlayersListResponse>(playersUrl),
    enabled: league.isSuccess,
    retry: 2,
  });

  const statsWindows = useQuery({
    queryKey: ["players", "stats-windows", sport],
    queryFn: () =>
      api<PlayerStatsWindowsResponse>(
        `/v1/players/stats-windows?sport=${encodeURIComponent(sport)}`,
      ),
    enabled: league.isSuccess && includeStats,
    staleTime: 60_000,
  });

  const weekOptions = useMemo(() => {
    const by = statsWindows.data?.weeks_by_season;
    if (!by || !statsSeasonIn) return [];
    return by[statsSeasonIn] ?? [];
  }, [statsSeasonIn, statsWindows.data]);

  useEffect(() => {
    if (!statsWindows.isSuccess || !statsWindows.data) return;
    const seasons = statsWindows.data.seasons;
    if (!statsSeasonIn) return;
    const y = Number(statsSeasonIn);
    if (!Number.isFinite(y) || !seasons.includes(y)) {
      setStatsSeasonIn("");
      setStatsWeekIn("");
    }
  }, [statsSeasonIn, statsWindows.isSuccess, statsWindows.data]);

  useEffect(() => {
    if (!statsSeasonIn || !statsWindows.data) return;
    if (!weekOptions.length) {
      if (statsWeekIn !== "") setStatsWeekIn("");
      return;
    }
    const wn = Number(statsWeekIn);
    if (
      statsWeekIn === "" ||
      !Number.isFinite(wn) ||
      !weekOptions.includes(wn)
    ) {
      setStatsWeekIn(String(weekOptions[0]));
    }
  }, [statsSeasonIn, statsWeekIn, weekOptions, statsWindows.data]);

  const total = players.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const statKeys = players.data?.stat_columns ?? [];
  const aggSeasonShown =
    players.data?.aggregate_season ?? AGGREGATE_STATS_SEASON;
  const statColGroup =
    includeStats && statKeys.length > 0 ? statKeys.length * 3 : 0;
  /** Identity + bio: name, #, pos, elig, team, status, injury, age, ht, wt, exp, college (+ GP when stats on). */
  const baseColCount = 12 + (includeStats ? 1 : 0);
  const tableColSpan = baseColCount + statColGroup;
  const tableMinWidth = 520 + baseColCount * 44 + statColGroup * 52;

  const posOptions = positionsForSport(sport);

  const seedSports = useMutation({
    mutationFn: () =>
      api<{ ok: boolean }>(`/v1/leagues/${leagueId}/seed-sports`, {
        method: "POST",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: ["players", "browse"],
      });
      void qc.invalidateQueries({
        queryKey: ["players", "stats-windows"],
      });
    },
  });

  const syncPlayers = useMutation({
    mutationFn: () =>
      api<{ ok: boolean; sport_code: string; count: number }>(
        `/v1/leagues/${leagueId}/sync-players`,
        { method: "POST" },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: ["players", "browse"],
      });
      void qc.invalidateQueries({
        queryKey: ["players", "stats-windows"],
      });
    },
  });

  return (
    <main className="container py-8">
      <h1 className="mb-4 text-2xl font-bold">Players</h1>

      {league.isLoading && (
        <p className="mb-4 text-sm text-muted">Loading league…</p>
      )}
      {league.isError && (
        <p className="mb-4 rounded border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-200">
          Could not load this league.{" "}
          {league.error instanceof ApiError
            ? `(${league.error.status})`
            : ""}{" "}
          Try returning to the dashboard and opening the league again.
        </p>
      )}

      <div className="mb-4 flex flex-col gap-3 md:flex-row md:flex-wrap md:items-end">
        <input
          className="input min-w-[200px] flex-1"
          placeholder="Search by name..."
          value={search}
          disabled={!league.isSuccess}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
        />
        <select
          className="input max-w-xs"
          value={position}
          disabled={!league.isSuccess}
          onChange={(e) => {
            setPosition(e.target.value);
            setPage(1);
          }}
        >
          <option value="">All positions</option>
          {posOptions.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))}
        </select>
        <label className="flex cursor-pointer items-center gap-2 text-sm text-muted">
          <input
            type="checkbox"
            className="rounded border-border"
            checked={onlyWithTeam}
            disabled={!league.isSuccess}
            onChange={(e) => {
              setOnlyWithTeam(e.target.checked);
              setPage(1);
            }}
          />
          Pro / club team only (hide FA & blank)
        </label>
        <label className="flex cursor-pointer items-center gap-2 text-sm text-muted">
          <input
            type="checkbox"
            className="rounded border-border"
            checked={includeStats}
            disabled={!league.isSuccess}
            onChange={(e) => {
              setIncludeStats(e.target.checked);
              setPage(1);
            }}
          />
          Load stat columns (latest week + season totals + per-week avg)
        </label>
        {includeStats && (
          <div className="flex w-full basis-full flex-wrap items-center gap-2 text-sm text-muted">
            <span className="whitespace-nowrap">Stat window</span>
            <select
              className="input w-[7.5rem] font-mono"
              aria-label="Stats season"
              value={statsSeasonIn}
              disabled={!league.isSuccess || statsWindows.isLoading}
              onChange={(e) => {
                const v = e.target.value;
                setStatsSeasonIn(v);
                setPage(1);
                if (!v) {
                  setStatsWeekIn("");
                  return;
                }
                const weeks = statsWindows.data?.weeks_by_season[v] ?? [];
                setStatsWeekIn(weeks[0] != null ? String(weeks[0]) : "");
              }}
            >
              <option value="">Latest in DB</option>
              {(statsWindows.data?.seasons ?? []).map((s) => (
                <option key={s} value={String(s)}>
                  {s}
                </option>
              ))}
            </select>
            <select
              className="input w-[7.5rem] font-mono"
              aria-label="Stats week"
              value={statsWeekIn}
              disabled={
                !league.isSuccess ||
                !statsSeasonIn ||
                statsWindows.isLoading ||
                weekOptions.length === 0
              }
              onChange={(e) => {
                setStatsWeekIn(e.target.value);
                setPage(1);
              }}
            >
              {!statsSeasonIn || weekOptions.length === 0 ? (
                <option value="">
                  {!statsSeasonIn
                    ? "—"
                    : statsWindows.isLoading
                      ? "…"
                      : "No rows"}
                </option>
              ) : null}
              {weekOptions.map((w) => (
                <option key={w} value={String(w)}>
                  Wk {w}
                </option>
              ))}
            </select>
            {statsWindows.isError && (
              <span className="text-xs text-amber-300/90">
                Could not load stat windows (dropdowns empty).
              </span>
            )}
            <button
              type="button"
              className="btn text-xs"
              disabled={!league.isSuccess}
              onClick={() => {
                setStatsSeasonIn("");
                setStatsWeekIn("");
                setPage(1);
              }}
            >
              Reset to latest
            </button>
          </div>
        )}
      </div>

      {players.isSuccess && (
        <p className="mb-2 text-sm text-muted">
          Showing{" "}
          <strong className="text-fg">
            {total === 0 ? 0 : offset + 1}–{Math.min(offset + PAGE_SIZE, total)}
          </strong>{" "}
          of <strong className="text-fg">{total}</strong> matching players
          {includeStats && statKeys.length > 0 && (
            <span className="ml-1">
              · per-stat columns: latest week (season{" "}
              <span className="font-mono text-fg">
                {players.data?.current_stats_season ?? "—"}
              </span>
              , week{" "}
              <span className="font-mono text-fg">
                {players.data?.current_stats_week ?? "—"}
              </span>
              ),{" "}
              <span className="font-mono text-fg">{aggSeasonShown}</span> season
              totals, and per-week averages (mean over weeks with a{" "}
              <code className="rounded bg-card px-1">player_stats</code> row,
              week &gt; 0)
            </span>
          )}
        </p>
      )}

      <div className="card overflow-hidden p-0">
        <div className="overflow-x-auto overscroll-x-contain">
          <table
            className="w-full border-separate border-spacing-0 text-[13px] leading-snug"
            style={{ minWidth: Math.max(960, tableMinWidth) }}
          >
            <thead className="bg-bg/60 text-muted backdrop-blur-sm [&_th]:align-middle">
              {includeStats && statKeys.length > 0 ? (
                <>
                  <tr>
                    <th rowSpan={2} className={`${stickyPlayerTh} ${bioTh}`}>
                      Player
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      #
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Pos
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Elig
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Team
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Status
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Inj
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Age
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Ht
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Wt
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      Exp
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      College
                    </th>
                    <th rowSpan={2} className={`${bioTh} bg-bg/60`}>
                      GP
                    </th>
                    <th
                      colSpan={statKeys.length}
                      className={`${statGroupHeader} border-l-sky-500/25`}
                    >
                      Latest week
                    </th>
                    <th
                      colSpan={statKeys.length}
                      className={`${statGroupHeader} border-l-violet-500/25`}
                    >
                      {aggSeasonShown} season
                    </th>
                    <th
                      colSpan={statKeys.length}
                      className={`${statGroupHeader} border-l-amber-500/25`}
                    >
                      {aggSeasonShown} / wk avg
                    </th>
                  </tr>
                  <tr>
                    {statKeys.map((k) => (
                      <th
                        key={`h-w-${k}`}
                        title={k}
                        className={`${statKeyTh} min-w-[3.25rem] border-l-sky-500/20 bg-sky-500/[0.08]`}
                      >
                        {k}
                      </th>
                    ))}
                    {statKeys.map((k) => (
                      <th
                        key={`h-s-${k}`}
                        title={k}
                        className={`${statKeyTh} min-w-[3.25rem] border-l-violet-500/20 bg-violet-500/[0.08]`}
                      >
                        {k}
                      </th>
                    ))}
                    {statKeys.map((k) => (
                      <th
                        key={`h-a-${k}`}
                        title={k}
                        className={`${statKeyTh} min-w-[3.25rem] border-l-amber-500/20 bg-amber-500/[0.08]`}
                      >
                        {k}
                      </th>
                    ))}
                  </tr>
                </>
              ) : (
                <tr>
                  <th className={`${stickyPlayerTh} ${bioTh}`}>Player</th>
                  <th className={`${bioTh} bg-bg/60`}>#</th>
                  <th className={`${bioTh} bg-bg/60`}>Pos</th>
                  <th className={`${bioTh} bg-bg/60`}>Elig</th>
                  <th className={`${bioTh} bg-bg/60`}>Team</th>
                  <th className={`${bioTh} bg-bg/60`}>Status</th>
                  <th className={`${bioTh} bg-bg/60`}>Inj</th>
                  <th className={`${bioTh} bg-bg/60`}>Age</th>
                  <th className={`${bioTh} bg-bg/60`}>Ht</th>
                  <th className={`${bioTh} bg-bg/60`}>Wt</th>
                  <th className={`${bioTh} bg-bg/60`}>Exp</th>
                  <th className={`${bioTh} bg-bg/60`}>College</th>
                  {includeStats && (
                    <th className={`${bioTh} bg-bg/60`}>GP</th>
                  )}
                </tr>
              )}
            </thead>
            <tbody>
              {(league.isLoading || (league.isSuccess && players.isLoading)) && (
                <tr>
                  <td
                    colSpan={tableColSpan}
                    className="px-3 py-6 text-center text-muted"
                  >
                    Loading players…
                  </td>
                </tr>
              )}
              {players.isError && (
                <tr>
                  <td
                    colSpan={tableColSpan}
                    className="px-3 py-6 text-center text-red-300"
                  >
                    {players.error instanceof ApiError
                      ? `Could not load players (${players.error.status}). ${players.error.message}`
                      : "Could not load players. Check that the API is running and you are signed in."}
                  </td>
                </tr>
              )}
              {players.isSuccess &&
                players.data?.players.map((p) => {
                  const wk = p.weekly_stats as Record<string, unknown> | null;
                  const ytd = p.season_totals;
                  const avg = p.season_weekly_avg;
                  return (
                    <tr
                      key={p.id}
                      className="border-t border-border/90 transition-colors hover:bg-bg/[0.06]"
                    >
                      <td
                        className={`${stickyPlayerTd} bg-card py-2.5 pl-3 pr-2 font-medium text-fg`}
                      >
                        {p.full_name?.trim() || "—"}
                      </td>
                      <td className={`${bioTd} bg-card text-left`}>
                        {p.jersey_number != null ? p.jersey_number : "—"}
                      </td>
                      <td className={`${bioTd} bg-card text-left font-medium text-fg/90`}>
                        {p.position ?? "—"}
                      </td>
                      <td
                        className={`${bioTextTd} bg-card max-w-[8.5rem]`}
                        title={
                          p.eligible_positions?.length
                            ? p.eligible_positions.join(", ")
                            : undefined
                        }
                      >
                        {p.eligible_positions?.length
                          ? p.eligible_positions.join(", ")
                          : "—"}
                      </td>
                      <td className={`${bioTd} bg-card text-left font-medium tracking-wide`}>
                        {p.nfl_team ?? "—"}
                      </td>
                      <td className={`${bioTd} bg-card text-left`}>
                        {p.status?.trim() || "—"}
                      </td>
                      <td className={`${bioTd} bg-card text-left`}>
                        {p.injury_status?.trim() ? (
                          <span className="inline-block max-w-[6.5rem] truncate rounded-md bg-red-500/15 px-2 py-0.5 text-[12px] font-medium text-red-200/95">
                            {p.injury_status}
                          </span>
                        ) : (
                          <span className="text-muted/60">—</span>
                        )}
                      </td>
                      <td className={`${bioTd} bg-card text-right`}>
                        {p.age != null && p.age > 0 ? p.age : "—"}
                      </td>
                      <td className={`${bioTd} bg-card text-left font-mono text-[12px]`}>
                        {formatHeightInches(p.height_inches)}
                      </td>
                      <td className={`${bioTd} bg-card text-right`}>
                        {p.weight_lbs != null && p.weight_lbs > 0
                          ? p.weight_lbs
                          : "—"}
                      </td>
                      <td className={`${bioTd} bg-card text-right`}>
                        {p.years_exp != null && p.years_exp >= 0 ? p.years_exp : "—"}
                      </td>
                      <td
                        className={`${bioTextTd} bg-card max-w-[10rem]`}
                        title={p.college?.trim() || undefined}
                      >
                        {p.college?.trim() || "—"}
                      </td>
                      {includeStats && (
                        <td className={`${bioTd} bg-card text-right font-mono text-[12px]`}>
                          {p.season_weeks != null && p.season_weeks > 0
                            ? p.season_weeks
                            : "—"}
                        </td>
                      )}
                      {includeStats && statKeys.length > 0
                        ? statKeys.map((k) => (
                            <td
                              key={`${p.id}-w-${k}`}
                              title={`${k} (latest week)`}
                              className={statBandTdClass("wk")}
                            >
                              {statCell(wk, k)}
                            </td>
                          ))
                        : null}
                      {includeStats && statKeys.length > 0
                        ? statKeys.map((k) => (
                            <td
                              key={`${p.id}-s-${k}`}
                              title={`${k} (season total)`}
                              className={statBandTdClass("ytd")}
                            >
                              {statCell(
                                ytd as Record<string, unknown> | null | undefined,
                                k,
                              )}
                            </td>
                          ))
                        : null}
                      {includeStats && statKeys.length > 0
                        ? statKeys.map((k) => (
                            <td
                              key={`${p.id}-a-${k}`}
                              title={`${k} (per-week avg)`}
                              className={statBandTdClass("avg")}
                            >
                              {statCell(
                                avg as Record<string, unknown> | null | undefined,
                                k,
                              )}
                            </td>
                          ))
                        : null}
                    </tr>
                  );
                })}
              {players.isSuccess && players.data?.players.length === 0 && (
                <tr>
                  <td
                    colSpan={tableColSpan}
                    className="px-3 py-6 text-center text-muted"
                  >
                    <p>
                      No players match these filters for{" "}
                      <span className="font-mono text-fg">{sport}</span>.
                    </p>
                    <p className="mt-2 text-xs">
                      Try turning off &quot;Pro / club team only&quot; to include
                      free agents, or clear search. The worker runs a daily{" "}
                      <code className="rounded bg-card px-1">player-sync</code> job;
                      commissioners can sync from the tools below.
                    </p>
                    {!authLoading && user && (
                      <div className="mt-4 flex flex-col items-center gap-2">
                        <div className="flex flex-wrap justify-center gap-2">
                          <button
                            type="button"
                            className="btn-primary text-sm"
                            disabled={seedSports.isPending}
                            onClick={() => seedSports.mutate()}
                          >
                            {seedSports.isPending
                              ? "Seeding sports…"
                              : "Seed sports (NFL / NBA / MLB)"}
                          </button>
                          <button
                            type="button"
                            className="btn-primary text-sm"
                            disabled={
                              syncPlayers.isPending || seedSports.isPending
                            }
                            onClick={() => syncPlayers.mutate()}
                          >
                            {syncPlayers.isPending
                              ? `Syncing ${sport.toUpperCase()} players…`
                              : `Sync ${sport.toUpperCase()} players`}
                          </button>
                        </div>
                        {syncPlayers.isSuccess && (
                          <p className="text-xs text-emerald-400/90">
                            Loaded {syncPlayers.data?.count ?? 0} players for{" "}
                            <span className="font-mono">
                              {syncPlayers.data?.sport_code ?? sport}
                            </span>
                            .
                          </p>
                        )}
                      </div>
                    )}
                    {!authLoading && !user && (
                      <p className="mt-3 text-xs text-muted">
                        Sign in to run the sports seed from the browser.
                      </p>
                    )}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {players.isSuccess && total > PAGE_SIZE && (
        <nav
          className="mt-4 flex flex-wrap items-center justify-center gap-2"
          aria-label="Pagination"
        >
          <button
            type="button"
            className="btn text-sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            Previous
          </button>
          <span className="px-2 text-sm text-muted">
            Page <span className="font-mono text-fg">{page}</span> of{" "}
            <span className="font-mono text-fg">{totalPages}</span>
          </span>
          <button
            type="button"
            className="btn text-sm"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          >
            Next
          </button>
        </nav>
      )}

      {players.isSuccess && total > 0 && total <= PAGE_SIZE && page === 1 && (
        <p className="mt-3 text-center text-xs text-muted">
          All {total} matching players are on this page.
        </p>
      )}
    </main>
  );
}
