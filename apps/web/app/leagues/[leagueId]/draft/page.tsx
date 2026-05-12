"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useEffect, useState, useMemo, useRef } from "react";
import { api, ApiError, wsURL } from "@/lib/api";
import type { Draft, DraftPick, League, OnTheClock, Player, Team } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";

export default function DraftRoomPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const { user } = useAuth();
  const qc = useQueryClient();

  const draft = useQuery({
    queryKey: ["draft", leagueId],
    queryFn: () => api<Draft>(`/v1/leagues/${leagueId}/draft`),
    retry: false,
  });

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });

  const league = useQuery({
    queryKey: ["league", leagueId],
    queryFn: () => api<League>(`/v1/leagues/${leagueId}`),
  });

  const sport = (league.data?.sport_code ?? "nfl").toLowerCase();

  const players = useQuery({
    queryKey: ["players", "browse", sport, leagueId],
    queryFn: () =>
      api<{ players: Player[] }>(
        `/v1/players?sport=${encodeURIComponent(sport)}&limit=200`,
      ),
    enabled: league.isSuccess,
    retry: 2,
  });

  // Subscribe to draft events
  const wsRef = useRef<WebSocket | null>(null);
  useEffect(() => {
    if (!draft.data?.id) return;
    let ws: WebSocket;
    try {
      ws = new WebSocket(wsURL(`/ws/draft/${draft.data.id}`));
    } catch {
      return;
    }
    wsRef.current = ws;
    ws.onmessage = () => qc.invalidateQueries({ queryKey: ["draft", leagueId] });
    return () => ws.close();
  }, [draft.data?.id, leagueId, qc, draft.data]);

  const start = useMutation({
    mutationFn: () =>
      api(`/v1/leagues/${leagueId}/draft/start`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["draft", leagueId] }),
  });
  const pause = useMutation({
    mutationFn: () =>
      api(`/v1/leagues/${leagueId}/draft/pause`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["draft", leagueId] }),
  });
  const resume = useMutation({
    mutationFn: () =>
      api(`/v1/leagues/${leagueId}/draft/resume`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["draft", leagueId] }),
  });
  const create = useMutation({
    mutationFn: (type: "snake" | "auction") =>
      api(`/v1/leagues/${leagueId}/draft`, {
        method: "POST",
        body: JSON.stringify({ type, rounds: 16, pick_seconds: 90 }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["draft", leagueId] }),
  });
  const pick = useMutation({
    mutationFn: (playerId: string) =>
      api(`/v1/leagues/${leagueId}/draft/pick`, {
        method: "POST",
        body: JSON.stringify({ player_id: playerId }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["draft", leagueId] }),
  });

  if (draft.isLoading) {
    return (
      <main className="container py-8">
        <div className="skeleton h-32 rounded" />
      </main>
    );
  }

  if (draft.error instanceof ApiError && draft.error.status === 404) {
    return (
      <main className="container max-w-xl py-8">
        <div className="card">
          <h1 className="text-xl font-semibold">Set up your draft</h1>
          <p className="mt-1 text-sm text-muted">
            Pick a draft type to get a draft room created. You can edit the
            order, timer, and start time before going live.
          </p>
          <div className="mt-4 flex gap-2">
            <button
              className="btn-primary flex-1"
              onClick={() => create.mutate("snake")}
              disabled={create.isPending}
            >
              Snake draft
            </button>
            <button
              className="btn-outline flex-1"
              onClick={() => create.mutate("auction")}
              disabled={create.isPending}
            >
              Auction draft
            </button>
          </div>
        </div>
      </main>
    );
  }

  if (!draft.data) return null;

  const d = draft.data;
  const picks = d.picks ?? [];
  const myTeamId = teams.data?.teams.find((t) => t.owner_id === user?.id)?.id;
  const onClock = d.on_the_clock;
  const myTurn = onClock?.team_id === myTeamId;

  const drafted = new Set(
    picks.filter((p) => p.player_id).map((p) => p.player_id as string),
  );
  const available = players.data?.players.filter((p) => !drafted.has(p.id)) ?? [];

  return (
    <main className="container py-8">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-2xl font-bold">Draft room</h1>
          <p className="text-sm text-muted">
            {(d.type ?? "snake").toUpperCase()} &middot; Round{" "}
            {(onClock?.round ?? 0) || "—"} of {d.rounds}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {d.status === "pending" && (
            <button
              className="btn-primary"
              onClick={() => start.mutate()}
              disabled={start.isPending}
            >
              Start draft
            </button>
          )}
          {d.status === "in_progress" && (
            <button className="btn-outline" onClick={() => pause.mutate()}>
              Pause
            </button>
          )}
          {d.status === "paused" && (
            <button className="btn-primary" onClick={() => resume.mutate()}>
              Resume
            </button>
          )}
        </div>
      </div>

      {onClock && (
        <OnTheClockBanner
          onClock={onClock}
          teamName={teams.data?.teams.find((t) => t.id === onClock.team_id)?.name}
          myTurn={myTurn}
        />
      )}

      <div className="mt-6 grid gap-6 lg:grid-cols-3">
        <section className="lg:col-span-2">
          <h2 className="mb-2 text-sm font-semibold uppercase text-muted">
            Available players
          </h2>
          <div className="card max-h-[60vh] overflow-y-auto p-0">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-bg/95 text-xs uppercase text-muted">
                <tr>
                  <th className="px-3 py-2 text-left">Player</th>
                  <th className="px-3 py-2 text-left">Pos</th>
                  <th className="px-3 py-2 text-left">Team</th>
                  <th className="px-3 py-2"></th>
                </tr>
              </thead>
              <tbody>
                {players.isLoading && (
                  <tr>
                    <td
                      colSpan={4}
                      className="px-3 py-6 text-center text-muted"
                    >
                      Loading player list…
                    </td>
                  </tr>
                )}
                {players.isError && (
                  <tr>
                    <td
                      colSpan={4}
                      className="px-3 py-6 text-center text-red-300"
                    >
                      {players.error instanceof ApiError
                        ? `Could not load players (${players.error.status}).`
                        : "Could not load players."}
                    </td>
                  </tr>
                )}
                {players.isSuccess &&
                  available.length === 0 &&
                  !players.isLoading && (
                    <tr>
                      <td
                        colSpan={4}
                        className="px-3 py-6 text-center text-muted"
                      >
                        No players available for {sport}. Sync the player
                        universe (worker / seed) or check API logs.
                      </td>
                    </tr>
                  )}
                {available.slice(0, 100).map((p) => (
                  <tr key={p.id} className="border-t border-border">
                    <td className="px-3 py-2">{p.full_name}</td>
                    <td className="px-3 py-2 text-muted">{p.position ?? "—"}</td>
                    <td className="px-3 py-2 text-muted">{p.nfl_team ?? "—"}</td>
                    <td className="px-3 py-2 text-right">
                      <button
                        disabled={!myTurn || pick.isPending}
                        onClick={() => pick.mutate(p.id)}
                        className="btn-primary text-xs disabled:opacity-50"
                      >
                        Draft
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section>
          <h2 className="mb-2 text-sm font-semibold uppercase text-muted">
            Recent picks
          </h2>
          <div className="card max-h-[60vh] overflow-y-auto">
            <RecentPicks picks={picks} teams={teams.data?.teams ?? []} />
          </div>
        </section>
      </div>
    </main>
  );
}

function OnTheClockBanner({
  onClock,
  teamName,
  myTurn,
}: {
  onClock: OnTheClock;
  teamName?: string;
  myTurn: boolean;
}) {
  const [secs, setSecs] = useState(0);
  useEffect(() => {
    const tick = () => {
      const end = new Date(onClock.deadline).getTime();
      const left = Number.isFinite(end)
        ? Math.max(0, Math.floor((end - Date.now()) / 1000))
        : 0;
      setSecs(left);
    };
    tick();
    const i = setInterval(tick, 250);
    return () => clearInterval(i);
  }, [onClock.deadline]);
  return (
    <div
      className={`card flex items-center justify-between ${
        myTurn ? "border-accent bg-accent/10" : ""
      }`}
    >
      <div>
        <div className="text-xs uppercase tracking-wide text-muted">
          On the clock
        </div>
        <div className="text-lg font-semibold">{teamName ?? onClock.team_id}</div>
        <div className="text-xs text-muted">Pick {onClock.pick_no}</div>
      </div>
      <div className={`font-mono text-3xl ${secs < 10 ? "text-red-400" : ""}`}>
        {String(Math.floor(secs / 60)).padStart(2, "0")}:
        {String(secs % 60).padStart(2, "0")}
      </div>
    </div>
  );
}

function RecentPicks({
  picks,
  teams,
}: {
  picks: DraftPick[];
  teams: Team[];
}) {
  const safePicks = picks ?? [];
  const teamMap = useMemo(
    () => Object.fromEntries(teams.map((t) => [t.id, t])),
    [teams],
  );
  const made = safePicks.filter((p) => p.player_id).reverse().slice(0, 25);
  const nTeams = teams.length;
  const slotInRound = (pickNo: number) =>
    nTeams > 0 ? ((pickNo - 1) % nTeams) + 1 : "—";
  return (
    <ul className="space-y-2 text-sm">
      {made.map((p) => (
        <li key={p.id} className="flex items-center justify-between">
          <div>
            <div className="text-xs text-muted">
              R{p.round}.{slotInRound(p.pick_no)}
            </div>
            <div>{teamMap[p.team_id]?.abbreviation ?? "—"}</div>
          </div>
          <div className="text-right text-xs text-muted">
            {p.player_id?.slice(0, 8)}
          </div>
        </li>
      ))}
      {made.length === 0 && (
        <li className="text-center text-xs text-muted">No picks yet</li>
      )}
    </ul>
  );
}
