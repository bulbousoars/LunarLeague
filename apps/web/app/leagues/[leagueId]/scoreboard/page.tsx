"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import type { Matchup, Team } from "@/lib/types";
import { wsURL } from "@/lib/api";

export default function ScoreboardPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const [week, setWeek] = useState(1);

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });
  const matchups = useQuery({
    queryKey: ["scoreboard", leagueId, week],
    queryFn: () =>
      api<{ matchups: Matchup[] }>(
        `/v1/leagues/${leagueId}/scoreboard?week=${week}`,
      ),
    refetchInterval: 30_000,
  });

  // WS for live updates
  useEffect(() => {
    const ws = new WebSocket(wsURL(`/ws/league/${leagueId}`));
    ws.onmessage = () => matchups.refetch();
    return () => ws.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [leagueId]);

  const teamById = (id: string) => teams.data?.teams.find((t) => t.id === id);

  return (
    <main className="container py-8">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Scoreboard</h1>
        <div className="flex items-center gap-2">
          <button
            className="btn-outline"
            onClick={() => setWeek((w) => Math.max(1, w - 1))}
          >
            ←
          </button>
          <span className="px-2 text-sm">Week {week}</span>
          <button
            className="btn-outline"
            onClick={() => setWeek((w) => Math.min(18, w + 1))}
          >
            →
          </button>
        </div>
      </div>

      <div className="space-y-2">
        {matchups.data?.matchups.length === 0 && (
          <div className="card text-sm text-muted">
            No matchups scheduled for week {week}.
          </div>
        )}
        {matchups.data?.matchups.map((m) => {
          const home = teamById(m.home_team_id);
          const away = teamById(m.away_team_id);
          return (
            <div key={m.id} className="card">
              <div className="grid grid-cols-3 items-center gap-2">
                <div className="text-right">
                  <div className="text-sm font-medium">{away?.name ?? "—"}</div>
                  <div className="text-xs text-muted">
                    {away?.record_wins ?? 0}-{away?.record_losses ?? 0}
                  </div>
                </div>
                <div className="text-center">
                  <div className="font-mono text-2xl font-bold tabular-nums">
                    {m.away_score}
                    <span className="px-3 text-muted">-</span>
                    {m.home_score}
                  </div>
                  <div className="mt-1 text-xs text-muted">
                    {m.is_final ? "FINAL" : "Live"}
                  </div>
                </div>
                <div>
                  <div className="text-sm font-medium">{home?.name ?? "—"}</div>
                  <div className="text-xs text-muted">
                    {home?.record_wins ?? 0}-{home?.record_losses ?? 0}
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </main>
  );
}
