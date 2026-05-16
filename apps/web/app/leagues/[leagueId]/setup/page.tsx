"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import type { League, Team } from "@/lib/types";

export default function LeagueSetupPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const league = useQuery({
    queryKey: ["league", leagueId],
    queryFn: () => api<League>(`/v1/leagues/${leagueId}`),
  });
  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });

  if (!league.data) return null;

  const inviteUrl = `${typeof window !== "undefined" ? window.location.origin : ""}/leagues/${leagueId}/join?code=${league.data.invite_code}`;

  return (
    <main className="container max-w-3xl py-10">
      <div className="card">
        <div>
          <div className="text-xs uppercase tracking-wide text-accent">
            League created
          </div>
          <h1 className="mt-1 text-2xl font-bold">{league.data.name}</h1>
          <p className="mt-1 text-sm text-muted">
            Invite your friends, then head to settings to lock in scoring rules
            and the draft date.
          </p>
        </div>

        <div className="mt-6 rounded-md border border-border bg-bg/40 p-3">
          <div className="label">Invite link</div>
          <div className="flex items-center gap-2">
            <code className="flex-1 truncate rounded bg-bg px-2 py-1.5 text-sm">
              {inviteUrl}
            </code>
            <button
              onClick={() => navigator.clipboard.writeText(inviteUrl)}
              className="btn-outline text-xs"
            >
              Copy
            </button>
          </div>
        </div>

        <div className="mt-6">
          <h2 className="text-sm font-semibold">Teams</h2>
          <p className="text-xs text-muted">
            Invited friends will claim a team after they sign in.
          </p>
          <ul className="mt-2 space-y-1 text-sm">
            {teams.data?.teams.map((t) => (
              <li
                key={t.id}
                className="flex items-center justify-between rounded border border-border px-3 py-2"
              >
                <span>{t.name}</span>
                {t.owner_id ? (
                  <span className="text-xs text-emerald-400">claimed</span>
                ) : (
                  <span className="text-xs text-muted">unclaimed</span>
                )}
              </li>
            ))}
          </ul>
        </div>

        <div className="mt-6 flex flex-wrap justify-between gap-2">
          <Link href={`/leagues/${leagueId}`} className="btn-outline">
            Open league
          </Link>
          <div className="flex gap-2">
            <Link href={`/leagues/${leagueId}/themes`} className="btn-outline">
              Theme Ball rules
            </Link>
            <Link
              href={`/leagues/${leagueId}/settings`}
              className="btn-primary"
            >
              Configure settings
            </Link>
          </div>
        </div>
      </div>
    </main>
  );
}
