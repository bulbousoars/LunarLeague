"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import type { League } from "@/lib/types";

export default function DashboardPage() {
  const leagues = useQuery({
    queryKey: ["leagues"],
    queryFn: () => api<{ leagues: League[] }>("/v1/leagues"),
  });

  return (
    <main className="container py-10">
      <div className="mb-6 flex items-end justify-between">
        <div>
          <h1 className="text-3xl font-bold">Your leagues</h1>
          <p className="mt-1 text-sm text-muted">
            Pick up where you left off, or start something new.
          </p>
        </div>
        <Link href="/leagues/new" className="btn-primary">
          New league
        </Link>
      </div>

      {leagues.isLoading && <div className="skeleton h-24 rounded" />}
      {leagues.data && leagues.data.leagues.length === 0 && (
        <div className="card text-center">
          <h2 className="text-lg font-semibold">No leagues yet</h2>
          <p className="mt-1 text-sm text-muted">
            Be the commissioner. Spin one up in 30 seconds.
          </p>
          <Link href="/leagues/new" className="btn-primary mt-4 inline-flex">
            Create your first league
          </Link>
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {leagues.data?.leagues.map((l) => (
          <Link
            key={l.id}
            href={`/leagues/${l.id}`}
            className="card-hover block"
          >
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold">{l.name}</h3>
              <span className="rounded-full border border-border px-2 py-0.5 text-xs uppercase tracking-wide text-muted">
                {l.sport_code}
              </span>
            </div>
            <div className="mt-3 grid grid-cols-3 gap-2 text-xs text-muted">
              <Stat label="Format" value={l.league_format} />
              <Stat label="Draft" value={l.draft_format} />
              <Stat label="Teams" value={String(l.team_count)} />
            </div>
            <div className="mt-3 inline-flex items-center gap-2 text-xs">
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  l.status === "in_season"
                    ? "bg-emerald-400"
                    : l.status === "drafting"
                      ? "bg-amber-400"
                      : "bg-slate-500"
                }`}
              />
              <span className="text-muted capitalize">
                {l.status.replace("_", " ")}
              </span>
            </div>
          </Link>
        ))}
      </div>
    </main>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[10px] uppercase tracking-wide text-muted/70">
        {label}
      </div>
      <div className="mt-0.5 capitalize text-fg">{value}</div>
    </div>
  );
}
