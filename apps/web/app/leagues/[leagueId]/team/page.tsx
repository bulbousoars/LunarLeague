"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import type { RosterEntry, Team } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { useMemo } from "react";

export default function MyTeamPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const { user } = useAuth();

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });

  const myTeam = useMemo(
    () => teams.data?.teams.find((t) => t.owner_id === user?.id),
    [teams.data, user?.id],
  );

  const roster = useQuery({
    queryKey: ["roster", myTeam?.id],
    queryFn: () =>
      api<{ roster: RosterEntry[] }>(
        `/v1/leagues/${leagueId}/teams/${myTeam!.id}/roster`,
      ),
    enabled: !!myTeam,
  });

  if (!myTeam) {
    return (
      <main className="container py-8">
        <div className="card">
          <h2 className="text-lg font-semibold">No team yet</h2>
          <p className="mt-1 text-sm text-muted">
            Claim a team from the Overview page to manage a roster.
          </p>
        </div>
      </main>
    );
  }

  const starters = (roster.data?.roster ?? []).filter(
    (e) => e.slot !== "BN" && e.slot !== "IR",
  );
  const bench = (roster.data?.roster ?? []).filter((e) => e.slot === "BN");
  const ir = (roster.data?.roster ?? []).filter((e) => e.slot === "IR");

  return (
    <main className="container py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{myTeam.name}</h1>
          <p className="text-sm text-muted">
            {myTeam.record_wins}-{myTeam.record_losses}
            {myTeam.record_ties > 0 ? `-${myTeam.record_ties}` : ""} &middot;{" "}
            {myTeam.points_for} PF
          </p>
        </div>
      </div>

      <RosterSection title="Starters" entries={starters} />
      <RosterSection title="Bench" entries={bench} className="mt-4" />
      {ir.length > 0 && (
        <RosterSection title="IR" entries={ir} className="mt-4" />
      )}
    </main>
  );
}

function RosterSection({
  title,
  entries,
  className,
}: {
  title: string;
  entries: RosterEntry[];
  className?: string;
}) {
  return (
    <section className={className}>
      <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted">
        {title}
      </h2>
      <div className="card overflow-hidden p-0">
        <table className="w-full text-sm">
          <thead className="bg-bg/50 text-xs uppercase text-muted">
            <tr>
              <th className="px-3 py-2 text-left">Slot</th>
              <th className="px-3 py-2 text-left">Player</th>
              <th className="px-3 py-2 text-left">Pos</th>
              <th className="px-3 py-2 text-left">Team</th>
              <th className="px-3 py-2 text-right">Acquired</th>
            </tr>
          </thead>
          <tbody>
            {entries.length === 0 && (
              <tr>
                <td colSpan={5} className="px-3 py-6 text-center text-muted">
                  Empty
                </td>
              </tr>
            )}
            {entries.map((e) => (
              <tr key={e.id} className="border-t border-border">
                <td className="px-3 py-2 font-medium">{e.slot}</td>
                <td className="px-3 py-2">{e.player_name}</td>
                <td className="px-3 py-2 text-muted">{e.position ?? "—"}</td>
                <td className="px-3 py-2 text-muted">{e.nfl_team ?? "—"}</td>
                <td className="px-3 py-2 text-right text-xs text-muted">
                  {e.acquired_via}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
