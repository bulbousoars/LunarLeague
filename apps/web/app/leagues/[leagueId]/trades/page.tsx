"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import type { Team } from "@/lib/types";
import Link from "next/link";

type Trade = {
  id: string;
  proposer_team_id: string;
  status: string;
  note: string | null;
  review_until: string | null;
  assets: {
    from_team_id: string;
    to_team_id: string;
    asset_type: string;
    player_id?: string | null;
    faab_amount?: number | null;
  }[];
  created_at: string;
};

export default function TradesPage() {
  const { leagueId } = useParams<{ leagueId: string }>();

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });
  const trades = useQuery({
    queryKey: ["trades", leagueId],
    queryFn: () =>
      api<{ trades: Trade[] }>(`/v1/leagues/${leagueId}/trades`),
  });

  const teamMap = Object.fromEntries(
    (teams.data?.teams ?? []).map((t) => [t.id, t]),
  );

  return (
    <main className="container py-8">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Trades</h1>
        <Link
          href={`/leagues/${leagueId}/trades/new`}
          className="btn-primary"
        >
          Propose trade
        </Link>
      </div>

      <div className="space-y-2">
        {trades.data?.trades.length === 0 && (
          <div className="card text-sm text-muted">No trades yet.</div>
        )}
        {trades.data?.trades.map((t) => (
          <div key={t.id} className="card">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-xs text-muted">
                  {new Date(t.created_at).toLocaleString()}
                </div>
                <div className="font-medium">
                  Proposed by{" "}
                  {teamMap[t.proposer_team_id]?.name ??
                    t.proposer_team_id.slice(0, 8)}
                </div>
              </div>
              <div>
                <span
                  className={`rounded px-2 py-0.5 text-xs uppercase ${
                    t.status === "executed"
                      ? "bg-emerald-500/20 text-emerald-300"
                      : t.status === "rejected" || t.status === "vetoed"
                        ? "bg-red-500/20 text-red-300"
                        : "bg-amber-500/20 text-amber-300"
                  }`}
                >
                  {t.status}
                </span>
              </div>
            </div>
            <ul className="mt-3 space-y-1 text-sm">
              {t.assets.map((a, i) => (
                <li key={i} className="text-muted">
                  {teamMap[a.from_team_id]?.abbreviation ?? "?"} →{" "}
                  {teamMap[a.to_team_id]?.abbreviation ?? "?"}: {a.asset_type}
                  {a.player_id && ` (${a.player_id.slice(0, 8)})`}
                  {a.faab_amount != null && ` $${a.faab_amount}`}
                </li>
              ))}
            </ul>
            {t.note && (
              <p className="mt-3 text-sm italic text-muted">&quot;{t.note}&quot;</p>
            )}
          </div>
        ))}
      </div>
    </main>
  );
}
