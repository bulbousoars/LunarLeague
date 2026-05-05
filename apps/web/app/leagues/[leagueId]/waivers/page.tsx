"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { Settings, Team } from "@/lib/types";

type FA = {
  id: string;
  full_name: string;
  position: string | null;
  nfl_team: string | null;
};

type Claim = {
  id: string;
  team_id: string;
  add_player_id: string;
  drop_player_id: string | null;
  bid_amount: number | null;
  priority: number;
  status: string;
  process_at: string;
};

export default function WaiversPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const { user } = useAuth();
  const qc = useQueryClient();

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });
  const myTeam = useMemo(
    () => teams.data?.teams.find((t) => t.owner_id === user?.id),
    [teams.data, user?.id],
  );
  const settings = useQuery({
    queryKey: ["settings", leagueId],
    queryFn: () => api<Settings>(`/v1/leagues/${leagueId}/settings`),
  });
  const fa = useQuery({
    queryKey: ["free-agents", leagueId],
    queryFn: () =>
      api<{ free_agents: FA[] }>(`/v1/leagues/${leagueId}/free-agents`),
  });
  const claims = useQuery({
    queryKey: ["waivers", leagueId],
    queryFn: () =>
      api<{ claims: Claim[] }>(`/v1/leagues/${leagueId}/waivers`),
  });

  const [bid, setBid] = useState<Record<string, number>>({});

  const submit = useMutation({
    mutationFn: (input: { add: string; drop?: string; amount?: number }) =>
      api(`/v1/leagues/${leagueId}/waivers`, {
        method: "POST",
        body: JSON.stringify({
          team_id: myTeam!.id,
          add_player_id: input.add,
          drop_player_id: input.drop,
          bid_amount: input.amount,
        }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["waivers", leagueId] }),
  });

  const isFAAB = settings.data?.waiver_type === "faab";

  return (
    <main className="container py-8">
      <h1 className="mb-1 text-2xl font-bold">Waivers</h1>
      <p className="mb-6 text-sm text-muted">
        {isFAAB
          ? "Highest blind bid wins. Ties broken by waiver priority."
          : "Lowest waiver priority number wins. Winner moves to the back."}
      </p>

      <div className="grid gap-6 lg:grid-cols-3">
        <section className="lg:col-span-2">
          <h2 className="mb-2 text-sm font-semibold uppercase text-muted">
            Free agents
          </h2>
          <div className="card max-h-[55vh] overflow-y-auto p-0">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-bg/95 text-xs uppercase text-muted">
                <tr>
                  <th className="px-3 py-2 text-left">Player</th>
                  <th className="px-3 py-2 text-left">Pos</th>
                  <th className="px-3 py-2 text-left">Team</th>
                  {isFAAB && <th className="px-3 py-2">Bid</th>}
                  <th className="px-3 py-2"></th>
                </tr>
              </thead>
              <tbody>
                {fa.data?.free_agents.slice(0, 200).map((p) => (
                  <tr key={p.id} className="border-t border-border">
                    <td className="px-3 py-2">{p.full_name}</td>
                    <td className="px-3 py-2 text-muted">{p.position ?? "—"}</td>
                    <td className="px-3 py-2 text-muted">{p.nfl_team ?? "—"}</td>
                    {isFAAB && (
                      <td className="px-3 py-2">
                        <input
                          type="number"
                          className="input w-20"
                          min={0}
                          value={bid[p.id] ?? 0}
                          onChange={(e) =>
                            setBid((b) => ({
                              ...b,
                              [p.id]: Number(e.target.value),
                            }))
                          }
                        />
                      </td>
                    )}
                    <td className="px-3 py-2 text-right">
                      <button
                        disabled={!myTeam}
                        onClick={() =>
                          submit.mutate({
                            add: p.id,
                            amount: isFAAB ? bid[p.id] ?? 0 : undefined,
                          })
                        }
                        className="btn-primary text-xs disabled:opacity-50"
                      >
                        Claim
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
            Pending claims
          </h2>
          <div className="card max-h-[55vh] overflow-y-auto p-0">
            <ul className="divide-y divide-border text-sm">
              {claims.data?.claims
                .filter((c) => c.status === "pending")
                .map((c) => (
                  <li
                    key={c.id}
                    className="flex items-center justify-between px-3 py-2"
                  >
                    <div>
                      <div className="font-medium">
                        {c.add_player_id.slice(0, 8)}
                      </div>
                      <div className="text-xs text-muted">
                        Process {new Date(c.process_at).toLocaleString()}
                      </div>
                    </div>
                    <div className="text-right text-xs">
                      {c.bid_amount != null && (
                        <div>${c.bid_amount}</div>
                      )}
                      <div className="text-muted">prio {c.priority}</div>
                    </div>
                  </li>
                ))}
              {claims.data?.claims.filter((c) => c.status === "pending")
                .length === 0 && (
                <li className="px-3 py-6 text-center text-muted">No claims</li>
              )}
            </ul>
          </div>
        </section>
      </div>
    </main>
  );
}
