"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { useState, useMemo, useEffect } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { Team, RosterEntry } from "@/lib/types";

export default function NewTradePage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const router = useRouter();
  const { user } = useAuth();

  const teams = useQuery({
    queryKey: ["teams", leagueId],
    queryFn: () => api<{ teams: Team[] }>(`/v1/leagues/${leagueId}/teams`),
  });
  const myTeam = useMemo(
    () => teams.data?.teams.find((t) => t.owner_id === user?.id),
    [teams.data, user?.id],
  );
  const others = (teams.data?.teams ?? []).filter((t) => t.id !== myTeam?.id);

  const [otherTeamId, setOtherTeamId] = useState<string>("");
  const [give, setGive] = useState<string[]>([]);
  const [get, setGet] = useState<string[]>([]);
  const [note, setNote] = useState("");

  // Default selection
  useEffect(() => {
    if (!otherTeamId && others[0]) setOtherTeamId(others[0].id);
  }, [otherTeamId, others]);

  const myRoster = useQuery({
    queryKey: ["roster", myTeam?.id],
    queryFn: () =>
      api<{ roster: RosterEntry[] }>(
        `/v1/leagues/${leagueId}/teams/${myTeam!.id}/roster`,
      ),
    enabled: !!myTeam,
  });
  const otherRoster = useQuery({
    queryKey: ["roster", otherTeamId],
    queryFn: () =>
      api<{ roster: RosterEntry[] }>(
        `/v1/leagues/${leagueId}/teams/${otherTeamId}/roster`,
      ),
    enabled: !!otherTeamId,
  });

  const propose = useMutation({
    mutationFn: () => {
      const assets = [
        ...give.map((pid) => ({
          from_team_id: myTeam!.id,
          to_team_id: otherTeamId,
          asset_type: "player",
          player_id: pid,
        })),
        ...get.map((pid) => ({
          from_team_id: otherTeamId,
          to_team_id: myTeam!.id,
          asset_type: "player",
          player_id: pid,
        })),
      ];
      return api(`/v1/leagues/${leagueId}/trades`, {
        method: "POST",
        body: JSON.stringify({
          proposer_team_id: myTeam!.id,
          note: note || null,
          assets,
        }),
      });
    },
    onSuccess: () => router.replace(`/leagues/${leagueId}/trades`),
  });

  if (!myTeam) {
    return (
      <main className="container py-8">
        <div className="card text-sm text-muted">
          You need to claim a team to propose trades.
        </div>
      </main>
    );
  }

  return (
    <main className="container max-w-3xl py-8">
      <h1 className="mb-1 text-2xl font-bold">Propose a trade</h1>
      <p className="mb-6 text-sm text-muted">
        Pick players each side gives. Multi-asset trades and FAAB swaps will
        come from a deeper editor next phase.
      </p>

      <div className="card space-y-4">
        <div>
          <label className="label">Trade with</label>
          <select
            className="input"
            value={otherTeamId}
            onChange={(e) => setOtherTeamId(e.target.value)}
          >
            {others.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <h2 className="label">You give</h2>
            <PlayerPicker
              entries={myRoster.data?.roster ?? []}
              selected={give}
              onChange={setGive}
            />
          </div>
          <div>
            <h2 className="label">You get</h2>
            <PlayerPicker
              entries={otherRoster.data?.roster ?? []}
              selected={get}
              onChange={setGet}
            />
          </div>
        </div>

        <div>
          <label className="label">Note (optional)</label>
          <textarea
            className="input"
            rows={2}
            value={note}
            onChange={(e) => setNote(e.target.value)}
          />
        </div>

        <div className="flex justify-end">
          <button
            className="btn-primary"
            onClick={() => propose.mutate()}
            disabled={propose.isPending || (!give.length && !get.length)}
          >
            {propose.isPending ? "Sending..." : "Send proposal"}
          </button>
        </div>
      </div>
    </main>
  );
}

function PlayerPicker({
  entries,
  selected,
  onChange,
}: {
  entries: RosterEntry[];
  selected: string[];
  onChange: (s: string[]) => void;
}) {
  return (
    <ul className="card max-h-72 overflow-y-auto p-0 text-sm">
      {entries.map((e) => {
        const checked = selected.includes(e.player_id);
        return (
          <li
            key={e.id}
            className="flex items-center justify-between border-b border-border px-3 py-1.5 last:border-0"
          >
            <label className="flex items-center gap-2 truncate">
              <input
                type="checkbox"
                checked={checked}
                onChange={(ev) => {
                  if (ev.target.checked) onChange([...selected, e.player_id]);
                  else onChange(selected.filter((x) => x !== e.player_id));
                }}
              />
              <span className="truncate">{e.player_name}</span>
            </label>
            <span className="text-xs text-muted">
              {e.position ?? "—"} · {e.nfl_team ?? "—"}
            </span>
          </li>
        );
      })}
      {entries.length === 0 && (
        <li className="px-3 py-6 text-center text-xs text-muted">No players</li>
      )}
    </ul>
  );
}
