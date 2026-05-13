"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useMemo, useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { League, PlayersListResponse } from "@/lib/types";
import { formatProfileBrief, statCell } from "@/lib/player-stats";
import { positionsForSport } from "@/lib/sport-ui";
import { useAuth } from "@/lib/auth-context";

const PAGE_SIZE = 100;

export default function PlayersPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");
  const [page, setPage] = useState(1);
  const [onlyWithTeam, setOnlyWithTeam] = useState(true);
  const [includeStats, setIncludeStats] = useState(true);
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
      q.set("aggregate_season", String(league.data?.season ?? 2025));
    }
    return `/v1/players?${q.toString()}`;
  }, [
    sport,
    offset,
    search,
    position,
    onlyWithTeam,
    includeStats,
    league.data?.season,
  ]);

  const players = useQuery({
    queryKey: [
      "players",
      "browse",
      sport,
      search,
      position,
      leagueId,
      page,
      onlyWithTeam,
      includeStats,
    ],
    queryFn: () => api<PlayersListResponse>(playersUrl),
    enabled: league.isSuccess,
    retry: 2,
  });

  const total = players.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const statKeys = players.data?.stat_columns ?? [];
  const aggSeasonShown =
    players.data?.aggregate_season ?? league.data?.season ?? 2025;
  const statColGroup =
    includeStats && statKeys.length > 0 ? statKeys.length * 3 : 0;
  const tableColSpan = 6 + statColGroup;
  const tableMinWidth = 720 + statColGroup * 56;

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
              · per-stat columns: latest week (S
              {players.data?.current_stats_season ?? "—"} W
              {players.data?.current_stats_week ?? "—"}),{" "}
              <span className="font-mono text-fg">{aggSeasonShown}</span> season
              totals, and per-week averages (mean over weeks with a{" "}
              <code className="rounded bg-card px-1">player_stats</code> row,
              week &gt; 0)
            </span>
          )}
        </p>
      )}

      <div className="card overflow-hidden p-0">
        <div className="overflow-x-auto">
          <table
            className="w-full text-sm"
            style={{ minWidth: Math.max(960, tableMinWidth) }}
          >
            <thead className="bg-bg/50 text-xs uppercase text-muted">
              {includeStats && statKeys.length > 0 ? (
                <>
                  <tr>
                    <th rowSpan={2} className="px-3 py-2 text-left align-bottom">
                      Player
                    </th>
                    <th rowSpan={2} className="px-3 py-2 text-left align-bottom">
                      #
                    </th>
                    <th rowSpan={2} className="px-3 py-2 text-left align-bottom">
                      Pos
                    </th>
                    <th rowSpan={2} className="px-3 py-2 text-left align-bottom">
                      Team
                    </th>
                    <th rowSpan={2} className="px-3 py-2 text-left align-bottom">
                      Status
                    </th>
                    <th rowSpan={2} className="px-3 py-2 text-left align-bottom">
                      Profile
                    </th>
                    <th
                      colSpan={statKeys.length}
                      className="border-l border-border px-2 py-2 text-center text-fg normal-case"
                    >
                      Latest week
                    </th>
                    <th
                      colSpan={statKeys.length}
                      className="border-l border-border px-2 py-2 text-center text-fg normal-case"
                    >
                      {aggSeasonShown} season
                    </th>
                    <th
                      colSpan={statKeys.length}
                      className="border-l border-border px-2 py-2 text-center text-fg normal-case"
                    >
                      {aggSeasonShown} / wk avg
                    </th>
                  </tr>
                  <tr>
                    {statKeys.map((k) => (
                      <th
                        key={`h-w-${k}`}
                        className="border-l border-border px-1.5 py-1 text-left font-mono text-[10px] font-normal normal-case tracking-normal text-muted"
                      >
                        {k}
                      </th>
                    ))}
                    {statKeys.map((k) => (
                      <th
                        key={`h-s-${k}`}
                        className="border-l border-border px-1.5 py-1 text-left font-mono text-[10px] font-normal normal-case tracking-normal text-muted"
                      >
                        {k}
                      </th>
                    ))}
                    {statKeys.map((k) => (
                      <th
                        key={`h-a-${k}`}
                        className="border-l border-border px-1.5 py-1 text-left font-mono text-[10px] font-normal normal-case tracking-normal text-muted"
                      >
                        {k}
                      </th>
                    ))}
                  </tr>
                </>
              ) : (
                <tr>
                  <th className="px-3 py-2 text-left">Player</th>
                  <th className="px-3 py-2 text-left">#</th>
                  <th className="px-3 py-2 text-left">Pos</th>
                  <th className="px-3 py-2 text-left">Team</th>
                  <th className="px-3 py-2 text-left">Status</th>
                  <th className="px-3 py-2 text-left">Profile</th>
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
                    <tr key={p.id} className="border-t border-border">
                      <td className="px-3 py-2 font-medium">
                        {p.full_name?.trim() || "—"}
                      </td>
                      <td className="px-3 py-2 text-muted">
                        {p.jersey_number != null ? p.jersey_number : "—"}
                      </td>
                      <td className="px-3 py-2 text-muted">{p.position ?? "—"}</td>
                      <td className="px-3 py-2 text-muted">{p.nfl_team ?? "—"}</td>
                      <td className="px-3 py-2 text-muted">
                        {p.injury_status ? (
                          <span className="rounded bg-red-500/20 px-1.5 py-0.5 text-xs text-red-300">
                            {p.injury_status}
                          </span>
                        ) : (
                          p.status ?? "—"
                        )}
                      </td>
                      <td
                        className="max-w-[220px] truncate px-3 py-2 text-xs text-muted"
                        title={formatProfileBrief(p)}
                      >
                        {formatProfileBrief(p)}
                      </td>
                      {includeStats && statKeys.length > 0
                        ? statKeys.map((k) => (
                            <td
                              key={`${p.id}-w-${k}`}
                              className="border-l border-border px-1.5 py-2 text-right font-mono text-xs text-muted"
                            >
                              {statCell(wk, k)}
                            </td>
                          ))
                        : null}
                      {includeStats && statKeys.length > 0
                        ? statKeys.map((k) => (
                            <td
                              key={`${p.id}-s-${k}`}
                              className="border-l border-border px-1.5 py-2 text-right font-mono text-xs text-muted"
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
                              className="border-l border-border px-1.5 py-2 text-right font-mono text-xs text-muted"
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
