"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api";
import type { Player } from "@/lib/types";

export default function PlayersPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");

  const players = useQuery({
    queryKey: ["players", "search", search, position, leagueId],
    queryFn: () =>
      api<{ players: Player[] }>(
        `/v1/players?sport=nfl&limit=200&q=${encodeURIComponent(search)}&position=${position}`,
      ),
  });

  return (
    <main className="container py-8">
      <h1 className="mb-4 text-2xl font-bold">Players</h1>
      <div className="mb-4 flex gap-2">
        <input
          className="input flex-1"
          placeholder="Search by name..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="input max-w-xs"
          value={position}
          onChange={(e) => setPosition(e.target.value)}
        >
          <option value="">All positions</option>
          {["QB", "RB", "WR", "TE", "K", "DEF"].map((p) => (
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
            {players.isLoading && (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-muted">
                  Loading...
                </td>
              </tr>
            )}
            {players.data?.players.map((p) => (
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
            {players.data?.players.length === 0 && (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-muted">
                  No players match. Run{" "}
                  <code className="rounded bg-card px-1">make seed</code> to
                  seed sports, then the worker syncs the player universe within
                  a minute.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </main>
  );
}
