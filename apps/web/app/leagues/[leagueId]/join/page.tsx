"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import type { Team } from "@/lib/types";
import { useQuery } from "@tanstack/react-query";

export default function JoinLeaguePage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const params = useSearchParams();
  const router = useRouter();
  const code = params.get("code") ?? "";
  const [joining, setJoining] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
    retry: false,
  });

  useEffect(() => {
    if (!code) return;
    setJoining(true);
    api(`/v1/leagues/${leagueId}/join`, {
      method: "POST",
      body: JSON.stringify({ invite_code: code }),
    })
      .then(() => {
        teams.refetch();
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "join failed"))
      .finally(() => setJoining(false));
  }, [code, leagueId, teams]);

  async function claim(teamId: string) {
    try {
      await api(`/v1/leagues/${leagueId}/teams/${teamId}/claim`, {
        method: "POST",
      });
      router.replace(`/leagues/${leagueId}`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "claim failed");
    }
  }

  return (
    <main className="container max-w-2xl py-10">
      <div className="card">
        <h1 className="text-2xl font-bold">Pick your team</h1>
        <p className="mt-1 text-sm text-muted">
          {joining ? "Joining league..." : "You're in. Claim a team to start managing your roster."}
        </p>
        {error && <div className="mt-3 text-sm text-red-400">{error}</div>}
        <ul className="mt-6 space-y-2">
          {teams.data?.teams.map((t) => (
            <li
              key={t.id}
              className="flex items-center justify-between rounded border border-border px-3 py-2"
            >
              <span>{t.name}</span>
              {t.owner_id ? (
                <span className="text-xs text-muted">claimed</span>
              ) : (
                <button onClick={() => claim(t.id)} className="btn-primary text-xs">
                  Claim
                </button>
              )}
            </li>
          ))}
        </ul>
      </div>
    </main>
  );
}
