"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { League, Player } from "@/lib/types";
import { positionsForSport } from "@/lib/sport-ui";
import { useAuth } from "@/lib/auth-context";

export default function PlayersPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");
  const { user, loading: authLoading } = useAuth();
  const qc = useQueryClient();

  const league = useQuery({
    queryKey: ["league", leagueId],
    queryFn: () => api<League>(`/v1/leagues/${leagueId}`),
  });

  const sport = (league.data?.sport_code ?? "nfl").toLowerCase();

  const players = useQuery({
    queryKey: ["players", "browse", sport, search, position, leagueId],
    queryFn: () =>
      api<{ players: Player[] }>(
        `/v1/players?sport=${encodeURIComponent(sport)}&limit=200&q=${encodeURIComponent(search)}&position=${encodeURIComponent(position)}`,
      ),
    enabled: league.isSuccess,
    retry: 2,
  });

  const posOptions = positionsForSport(sport);

  const seedSports = useMutation({
    mutationFn: () =>
      api<{ ok: boolean }>(`/v1/leagues/${leagueId}/seed-sports`, {
        method: "POST",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: ["players", "browse", sport, search, position, leagueId],
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

      <div className="mb-4 flex gap-2">
        <input
          className="input flex-1"
          placeholder="Search by name..."
          value={search}
          disabled={!league.isSuccess}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="input max-w-xs"
          value={position}
          disabled={!league.isSuccess}
          onChange={(e) => setPosition(e.target.value)}
        >
          <option value="">All positions</option>
          {posOptions.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))}
        </select>
      </div>

      <div className="card overflow-hidden p-0">
        <table className="w-full text-sm">
          <thead className="bg-bg/50 text-xs uppercase text-muted">
            <tr>
              <th className="px-3 py-2 text-left">Player</th>
              <th className="px-3 py-2 text-left">Pos</th>
              <th className="px-3 py-2 text-left">Team</th>
              <th className="px-3 py-2 text-left">Status</th>
            </tr>
          </thead>
          <tbody>
            {(league.isLoading || (league.isSuccess && players.isLoading)) && (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-muted">
                  Loading players…
                </td>
              </tr>
            )}
            {players.isError && (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-red-300">
                  {players.error instanceof ApiError
                    ? `Could not load players (${players.error.status}). ${players.error.message}`
                    : "Could not load players. Check that the API is running and you are signed in."}
                </td>
              </tr>
            )}
            {players.isSuccess &&
              players.data?.players.map((p) => (
                <tr key={p.id} className="border-t border-border">
                  <td className="px-3 py-2 font-medium">{p.full_name}</td>
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
                </tr>
              ))}
            {players.isSuccess && players.data?.players.length === 0 && (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-muted">
                  <p>
                    No players in the database for{" "}
                    <span className="font-mono text-fg">{sport}</span> yet. The
                    worker must run a{" "}
                    <code className="rounded bg-card px-1">player-sync</code> job
                    to load rosters (often within 24h, or restart the worker for
                    an immediate sync).
                  </p>
                  <p className="mt-2 text-xs">
                    First-time hosts also need sports rows (NFL/NBA/MLB). If you
                    are signed in as this league&apos;s{" "}
                    <strong>commissioner</strong> (or a site admin), use the
                    button below. Otherwise use the shell:{" "}
                    <code className="rounded bg-card px-1">
                      docker compose run --rm api seed
                    </code>{" "}
                    or <code className="rounded bg-card px-1">make seed</code> in
                    dev.
                  </p>
                  {!authLoading && user && (
                    <div className="mt-4 flex flex-col items-center gap-2">
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
                      {seedSports.isError && (
                        <p className="text-xs text-red-300">
                          {seedSports.error instanceof ApiError
                            ? seedSports.error.message
                            : "Request failed"}
                        </p>
                      )}
                      {seedSports.isSuccess && (
                        <p className="text-xs text-emerald-400/90">
                          Sports table updated. Player rows still require a
                          worker sync.
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
    </main>
  );
}
