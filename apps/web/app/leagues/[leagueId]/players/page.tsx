"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useEffect, useMemo, useState, type ReactNode } from "react";
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
import { statColumnLabels } from "@/lib/stat-labels";
import { positionsForSport } from "@/lib/sport-ui";
import { useAuth } from "@/lib/auth-context";
import {
  PLAYER_SORT_IDS,
  sortPlayers,
  statColumnOptions,
  statSortId,
  type SortRule,
} from "@/lib/players-table-sort";

const PAGE_SIZE = 100;
/** Season summed for YTD + per-week avg columns (`player_stats`, week &gt; 0). */
const AGGREGATE_STATS_SEASON = 2025;

/** Sticky first column — must use opaque bg + border-separate on table for clean scroll. */
const stickyPlayerTh =
  "sticky left-0 z-30 min-w-[12rem] max-w-[15rem] whitespace-normal bg-card " +
  "border-r border-border shadow-[6px_0_14px_-6px_rgba(0,0,0,0.55)]";
const stickyPlayerTd =
  "sticky left-0 z-20 min-w-[12rem] max-w-[15rem] whitespace-normal bg-card " +
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
  return `border-l border-border/80 px-2 py-1.5 text-right font-mono text-[12px] tabular-nums text-muted/95 min-w-[3.75rem] ${bandBg}`;
};

function cyclePrimary(current: SortRule | null, id: string): SortRule | null {
  if (!current || current.id !== id) return { id, dir: "asc" };
  if (current.dir === "asc") return { id, dir: "desc" };
  return null;
}

function sortMark(
  primary: SortRule | null,
  advancedActive: boolean,
  id: string,
): string {
  if (advancedActive) return "";
  if (!primary || primary.id !== id) return "";
  return primary.dir === "asc" ? "↑" : "↓";
}

function SortThBio(props: {
  sortId: string;
  children: ReactNode;
  className: string;
  primary: SortRule | null;
  advancedActive: boolean;
  onCycle: () => void;
  rowSpan?: number;
  colSpan?: number;
}) {
  const mark = sortMark(props.primary, props.advancedActive, props.sortId);
  return (
    <th
      rowSpan={props.rowSpan}
      colSpan={props.colSpan}
      className={props.className}
    >
      <button
        type="button"
        title="Sort: ascending → descending → off"
        className="flex w-full min-w-0 items-center justify-start gap-1 text-left font-inherit"
        onClick={props.onCycle}
      >
        <span className="min-w-0 flex-1">{props.children}</span>
        {mark ? (
          <span
            className="shrink-0 font-mono text-[10px] text-muted"
            aria-hidden
          >
            {mark}
          </span>
        ) : null}
      </button>
    </th>
  );
}

function SortThStat(props: {
  sortId: string;
  label: string;
  title: string;
  className: string;
  primary: SortRule | null;
  advancedActive: boolean;
  onCycle: () => void;
}) {
  const mark = sortMark(props.primary, props.advancedActive, props.sortId);
  return (
    <th title={props.title} className={props.className}>
      <button
        type="button"
        title="Sort: ascending → descending → off"
        className="flex w-full min-w-0 items-end justify-end gap-1 text-right font-inherit"
        onClick={props.onCycle}
      >
        <span className="min-w-0 break-all text-right">{props.label}</span>
        {mark ? (
          <span
            className="shrink-0 font-mono text-[10px] text-muted"
            aria-hidden
          >
            {mark}
          </span>
        ) : null}
      </button>
    </th>
  );
}

export default function PlayersPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");
  const [page, setPage] = useState(1);
  const [onlyWithTeam, setOnlyWithTeam] = useState(true);
  const [includeStats, setIncludeStats] = useState(true);
  /** Stat window: default to aggregate season (2025); week optional — API picks latest week for that season. */
  const [statsSeasonIn, setStatsSeasonIn] = useState(
    String(AGGREGATE_STATS_SEASON),
  );
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
      if (ss !== "") {
        q.set("season", ss);
        if (sw !== "") q.set("week", sw);
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
    const raw = by[statsSeasonIn] ?? [];
    return [...raw].sort((a, b) => b - a);
  }, [statsSeasonIn, statsWindows.data]);

  useEffect(() => {
    setStatsSeasonIn(String(AGGREGATE_STATS_SEASON));
    setStatsWeekIn("");
  }, [sport, leagueId]);

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
  const tableMinWidth = 560 + baseColCount * 48 + statColGroup * 58;

  const posOptions = positionsForSport(sport);

  const [primarySort, setPrimarySort] = useState<SortRule | null>(null);
  const [advancedSortRules, setAdvancedSortRules] = useState<SortRule[]>([]);
  const advancedActive = advancedSortRules.length > 0;

  const sortColumnOptions = useMemo(
    () => statColumnOptions(includeStats ? statKeys : []),
    [includeStats, statKeys],
  );

  const sortedPlayers = useMemo(() => {
    const raw = players.data?.players ?? [];
    return sortPlayers(raw, primarySort, advancedSortRules, includeStats);
  }, [players.data?.players, primarySort, advancedSortRules, includeStats]);

  const cycleColumnSort = (id: string) => {
    setAdvancedSortRules([]);
    setPrimarySort((prev) => cyclePrimary(prev, id));
  };

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
    <main className="w-full max-w-full px-3 py-8 sm:px-5 lg:px-8 xl:px-12 2xl:px-16">
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
              const on = e.target.checked;
              setIncludeStats(on);
              setPage(1);
              if (on) {
                setStatsSeasonIn(String(AGGREGATE_STATS_SEASON));
                setStatsWeekIn("");
              }
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
                const sorted = [...weeks].sort((a, b) => b - a);
                setStatsWeekIn(sorted[0] != null ? String(sorted[0]) : "");
              }}
            >
              <option value="">
                Aggregate season only (no fixed week)
              </option>
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
                setStatsSeasonIn(String(AGGREGATE_STATS_SEASON));
                setStatsWeekIn("");
                setPage(1);
              }}
            >
              Reset to {AGGREGATE_STATS_SEASON}
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
              week &gt; 0). The stat window defaults to season{" "}
              <span className="font-mono text-fg">{AGGREGATE_STATS_SEASON}</span>{" "}
              (latest week for that season when you don&apos;t pick a week). Weekly
              stats never borrow a different season, so they stay aligned with YTD.
              If YTD or averages are blank for{" "}
              <span className="font-mono text-fg">{aggSeasonShown}</span>, the DB
              may not have weekly stat rows for that season yet (API workers sync
              live and final games and run a periodic backfill after deploy).
            </span>
          )}
        </p>
      )}

      {players.isSuccess && advancedActive && (
        <p className="mb-2 text-xs text-muted">
          Advanced multi-sort is active ({advancedSortRules.length} level
          {advancedSortRules.length === 1 ? "" : "s"}). Column header arrows are
          hidden until you clear advanced rules or click a header (which resets
          advanced sort).
        </p>
      )}

      <details className="mb-5 w-full rounded-lg border border-border bg-card/80 p-4 text-sm shadow-sm backdrop-blur-sm">
        <summary className="cursor-pointer select-none text-base font-semibold text-fg">
          Advanced sorting
        </summary>
        <div className="mt-4 space-y-3 text-muted">
          <p className="max-w-5xl text-xs leading-relaxed text-muted">
            Add multiple sort levels in order (e.g. YTD passing yards, then age).
            Uses the same column keys as the table. Applies only to the current
            page ({PAGE_SIZE} rows per request).
          </p>
          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            {advancedSortRules.map((rule, idx) => (
              <div
                key={idx}
                className="flex min-w-0 flex-wrap items-center gap-2 rounded-md border border-border/60 bg-bg/40 p-2.5"
              >
                <span className="w-5 shrink-0 text-center text-[11px] font-mono text-muted">
                  {idx + 1}
                </span>
                <select
                  className="input min-w-0 flex-1 basis-[8rem] py-1.5 text-xs"
                  aria-label={`Sort column level ${idx + 1}`}
                  value={rule.id}
                  onChange={(e) => {
                    const id = e.target.value;
                    setAdvancedSortRules((prev) => {
                      const next = prev.slice();
                      next[idx] = { ...next[idx], id };
                      return next;
                    });
                  }}
                >
                  {sortColumnOptions.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.label}
                    </option>
                  ))}
                </select>
                <select
                  className="input w-[5.5rem] shrink-0 py-1.5 text-xs font-mono"
                  aria-label={`Sort direction level ${idx + 1}`}
                  value={rule.dir}
                  onChange={(e) => {
                    const dir = e.target.value as SortRule["dir"];
                    setAdvancedSortRules((prev) => {
                      const next = prev.slice();
                      next[idx] = { ...next[idx], dir };
                      return next;
                    });
                  }}
                >
                  <option value="asc">Asc</option>
                  <option value="desc">Desc</option>
                </select>
                <button
                  type="button"
                  className="btn shrink-0 px-2 py-1 text-xs"
                  title="Remove this sort level"
                  onClick={() => {
                    setAdvancedSortRules((prev) =>
                      prev.filter((_, i) => i !== idx),
                    );
                  }}
                >
                  Remove
                </button>
              </div>
            ))}
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className="btn text-xs"
              onClick={() => {
                setAdvancedSortRules((prev) => [
                  ...prev,
                  {
                    id: PLAYER_SORT_IDS.full_name,
                    dir: "asc",
                  },
                ]);
              }}
            >
              Add sort level
            </button>
            <button
              type="button"
              className="btn text-xs"
              disabled={advancedSortRules.length === 0}
              onClick={() => setAdvancedSortRules([])}
            >
              Clear all
            </button>
          </div>
        </div>
      </details>

      <div className="card w-full min-w-0 overflow-hidden border-border/90 p-0 shadow-md">
        <div className="overflow-x-auto overscroll-x-contain">
          <table
            className="w-full border-separate border-spacing-0 text-[13px] leading-snug"
            style={{ minWidth: Math.max(1040, tableMinWidth) }}
          >
            <thead className="bg-bg/60 text-muted backdrop-blur-sm [&_th]:align-middle">
              {includeStats && statKeys.length > 0 ? (
                <>
                  <tr>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.full_name}
                      className={`${stickyPlayerTh} ${bioTh}`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.full_name)}
                    >
                      Player
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.jersey_number}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() =>
                        cycleColumnSort(PLAYER_SORT_IDS.jersey_number)
                      }
                    >
                      #
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.position}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.position)}
                    >
                      Pos
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.eligible}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.eligible)}
                    >
                      Elig
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.nfl_team}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.nfl_team)}
                    >
                      Team
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.status}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.status)}
                    >
                      Status
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.injury}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.injury)}
                    >
                      Inj
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.age}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.age)}
                    >
                      Age
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.height_inches}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() =>
                        cycleColumnSort(PLAYER_SORT_IDS.height_inches)
                      }
                    >
                      Ht
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.weight_lbs}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.weight_lbs)}
                    >
                      Wt
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.years_exp}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.years_exp)}
                    >
                      Exp
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.college}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.college)}
                    >
                      College
                    </SortThBio>
                    <SortThBio
                      rowSpan={2}
                      sortId={PLAYER_SORT_IDS.gp}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.gp)}
                    >
                      GP
                    </SortThBio>
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
                    {statKeys.map((k) => {
                      const { short: sk, long: lk } = statColumnLabels(k);
                      return (
                      <SortThStat
                        key={`h-w-${k}`}
                        sortId={statSortId("wk", k)}
                        label={sk}
                        title={lk}
                        className={`${statKeyTh} min-w-[3.75rem] border-l-sky-500/20 bg-sky-500/[0.08]`}
                        primary={primarySort}
                        advancedActive={advancedActive}
                        onCycle={() => cycleColumnSort(statSortId("wk", k))}
                      />
                      );
                    })}
                    {statKeys.map((k) => {
                      const { short: sk, long: lk } = statColumnLabels(k);
                      return (
                      <SortThStat
                        key={`h-s-${k}`}
                        sortId={statSortId("ytd", k)}
                        label={sk}
                        title={lk}
                        className={`${statKeyTh} min-w-[3.75rem] border-l-violet-500/20 bg-violet-500/[0.08]`}
                        primary={primarySort}
                        advancedActive={advancedActive}
                        onCycle={() => cycleColumnSort(statSortId("ytd", k))}
                      />
                      );
                    })}
                    {statKeys.map((k) => {
                      const { short: sk, long: lk } = statColumnLabels(k);
                      return (
                      <SortThStat
                        key={`h-a-${k}`}
                        sortId={statSortId("avg", k)}
                        label={sk}
                        title={lk}
                        className={`${statKeyTh} min-w-[3.75rem] border-l-amber-500/20 bg-amber-500/[0.08]`}
                        primary={primarySort}
                        advancedActive={advancedActive}
                        onCycle={() => cycleColumnSort(statSortId("avg", k))}
                      />
                      );
                    })}
                  </tr>
                </>
              ) : (
                <tr>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.full_name}
                    className={`${stickyPlayerTh} ${bioTh}`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.full_name)}
                  >
                    Player
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.jersey_number}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() =>
                      cycleColumnSort(PLAYER_SORT_IDS.jersey_number)
                    }
                  >
                    #
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.position}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.position)}
                  >
                    Pos
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.eligible}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.eligible)}
                  >
                    Elig
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.nfl_team}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.nfl_team)}
                  >
                    Team
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.status}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.status)}
                  >
                    Status
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.injury}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.injury)}
                  >
                    Inj
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.age}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.age)}
                  >
                    Age
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.height_inches}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() =>
                      cycleColumnSort(PLAYER_SORT_IDS.height_inches)
                    }
                  >
                    Ht
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.weight_lbs}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.weight_lbs)}
                  >
                    Wt
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.years_exp}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.years_exp)}
                  >
                    Exp
                  </SortThBio>
                  <SortThBio
                    sortId={PLAYER_SORT_IDS.college}
                    className={`${bioTh} bg-bg/60`}
                    primary={primarySort}
                    advancedActive={advancedActive}
                    onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.college)}
                  >
                    College
                  </SortThBio>
                  {includeStats && (
                    <SortThBio
                      sortId={PLAYER_SORT_IDS.gp}
                      className={`${bioTh} bg-bg/60`}
                      primary={primarySort}
                      advancedActive={advancedActive}
                      onCycle={() => cycleColumnSort(PLAYER_SORT_IDS.gp)}
                    >
                      GP
                    </SortThBio>
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
                sortedPlayers.map((p) => {
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
                              title={`${statColumnLabels(k).long} (latest week)`}
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
                              title={`${statColumnLabels(k).long} (season total)`}
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
                              title={`${statColumnLabels(k).long} (per-week avg)`}
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
