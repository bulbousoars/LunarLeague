"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import type { Standing, Team, Matchup } from "@/lib/types";

export default function LeagueOverview() {
  const { leagueId } = useParams<{ leagueId: string }>();

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });
  const standings = useQuery({
    queryKey: ["standings", leagueId],
    queryFn: () =>
      api<{ standings: Standing[] }>(`/v1/leagues/${leagueId}/standings`),
  });
  const matchups = useQuery({
    queryKey: ["scoreboard", leagueId, "current"],
    queryFn: () =>
      api<{ matchups: Matchup[] }>(`/v1/leagues/${leagueId}/scoreboard?week=1`),
  });

  return (
    <main className="container py-8">
      <div className="grid gap-6 lg:grid-cols-3">
        <section className="lg:col-span-2">
          <h2 className="mb-3 text-lg font-semibold">This week</h2>
          {matchups.data && matchups.data.matchups.length === 0 ? (
            <div className="card text-sm text-muted">
              No matchups generated yet. Commissioners can generate the
              schedule from{" "}
              <Link
                href={`/leagues/${leagueId}/settings`}
                className="text-accent underline"
              >
                Settings
              </Link>
              .
            </div>
          ) : (
            <div className="space-y-2">
              {matchups.data?.matchups.map((m) => (
                <ScoreboardRow key={m.id} m={m} teams={teams.data?.teams} />
              ))}
            </div>
          )}

          <h2 className="mb-3 mt-8 text-lg font-semibold">Teams</h2>
          <div className="grid grid-cols-2 gap-2 md:grid-cols-3">
            {teams.data?.teams.map((t) => (
              <div key={t.id} className="card text-sm">
                <div className="flex items-center justify-between">
                  <div className="font-medium">{t.name}</div>
                  <div className="rounded border border-border px-1.5 py-0.5 text-[10px] text-muted">
                    {t.abbreviation}
                  </div>
                </div>
                <div className="mt-1 text-xs text-muted">
                  {t.record_wins}-{t.record_losses}
                  {t.record_ties > 0 ? `-${t.record_ties}` : ""}
                </div>
              </div>
            ))}
          </div>
        </section>

        <aside>
          <h2 className="mb-3 text-lg font-semibold">Standings</h2>
          <div className="card overflow-hidden p-0">
            <table className="w-full text-sm">
              <thead className="bg-bg/50 text-xs uppercase text-muted">
                <tr>
                  <th className="px-3 py-2 text-left">Team</th>
                  <th className="px-3 py-2 text-right">W-L</th>
                  <th className="px-3 py-2 text-right">PF</th>
                </tr>
              </thead>
              <tbody>
                {standings.data?.standings.map((s) => (
                  <tr key={s.team_id} className="border-t border-border">
                    <td className="px-3 py-2 font-medium">{s.name}</td>
                    <td className="px-3 py-2 text-right">
                      {s.wins}-{s.losses}
                    </td>
                    <td className="px-3 py-2 text-right">{s.points_for}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </aside>
      </div>
    </main>
  );
}

function ScoreboardRow({
  m,
  teams,
}: {
  m: Matchup;
  teams?: Team[];
}) {
  const home = teams?.find((t) => t.id === m.home_team_id);
  const away = teams?.find((t) => t.id === m.away_team_id);
  return (
    <div className="card flex items-center justify-between">
      <div>
        <div className="text-sm font-medium">{away?.name ?? "—"}</div>
        <div className="mt-1 text-xs text-muted">@</div>
      </div>
      <div className="flex items-center gap-4 text-right">
        <div>
          <div className="text-2xl font-bold tabular-nums">
            {m.away_score}
          </div>
          <div className="text-xs text-muted">{away?.abbreviation}</div>
        </div>
        <div className="text-muted">-</div>
        <div>
          <div className="text-2xl font-bold tabular-nums">
            {m.home_score}
          </div>
          <div className="text-xs text-muted">{home?.abbreviation}</div>
        </div>
      </div>
      <div className="text-right">
        <div className="text-sm font-medium">{home?.name ?? "—"}</div>
        <div className="mt-1 text-xs text-muted">
          {m.is_final ? "FINAL" : `Week ${m.week}`}
        </div>
      </div>
    </div>
  );
}
