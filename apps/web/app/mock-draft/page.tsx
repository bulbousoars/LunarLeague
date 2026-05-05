"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Player } from "@/lib/types";

// Client-side mock draft. No persistence — pure practice tool. AI bots use
// "next best available" with a small randomization within position to mimic
// realistic boards.

type Slot = { round: number; pick: number; teamIdx: number };

export default function MockDraftPage() {
  const [teams, setTeams] = useState(12);
  const [position, setPosition] = useState(6);
  const [rounds, setRounds] = useState(15);
  const [started, setStarted] = useState(false);
  const [picks, setPicks] = useState<Record<number, string>>({}); // pickNo -> playerId

  const players = useQuery({
    queryKey: ["players", "nfl-mock"],
    queryFn: () => api<{ players: Player[] }>(`/v1/players?sport=nfl&limit=400`),
  });

  const order = useMemo(() => makeSnakeOrder(teams, rounds), [teams, rounds]);

  const drafted = new Set(Object.values(picks));
  const available = (players.data?.players ?? []).filter((p) => !drafted.has(p.id));

  const myPickNos = useMemo(
    () =>
      order.filter((s) => s.teamIdx === position - 1).map((s) => s.pick),
    [order, position],
  );

  const nextPickNo =
    Object.keys(picks).length === 0 ? 1 : Math.max(...Object.keys(picks).map(Number)) + 1;
  const nextSlot = order.find((s) => s.pick === nextPickNo);
  const myTurn = nextSlot?.teamIdx === position - 1;

  // Bot picks
  useEffect(() => {
    if (!started || !nextSlot) return;
    if (myTurn) return;
    const id = setTimeout(() => {
      const pick = pickForBot(available);
      if (pick) setPicks((p) => ({ ...p, [nextPickNo]: pick.id }));
    }, 700);
    return () => clearTimeout(id);
  }, [started, nextSlot, myTurn, available, nextPickNo]);

  function userPick(playerId: string) {
    setPicks((p) => ({ ...p, [nextPickNo]: playerId }));
  }

  function reset() {
    setStarted(false);
    setPicks({});
  }

  if (!started) {
    return (
      <main className="container max-w-md py-12">
        <Link href="/dashboard" className="text-sm text-muted">
          ← Dashboard
        </Link>
        <h1 className="mt-4 text-2xl font-bold">Mock draft</h1>
        <p className="mt-1 text-sm text-muted">
          Practice your draft strategy. Bots auto-pick. No clock.
        </p>
        <div className="card mt-6 space-y-3">
          <div>
            <label className="label">Teams</label>
            <select
              className="input"
              value={teams}
              onChange={(e) => setTeams(Number(e.target.value))}
            >
              {[8, 10, 12, 14].map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="label">Your draft position</label>
            <input
              type="number"
              className="input"
              value={position}
              min={1}
              max={teams}
              onChange={(e) => setPosition(Number(e.target.value))}
            />
          </div>
          <div>
            <label className="label">Rounds</label>
            <input
              type="number"
              className="input"
              value={rounds}
              min={1}
              max={20}
              onChange={(e) => setRounds(Number(e.target.value))}
            />
          </div>
          <button className="btn-primary w-full" onClick={() => setStarted(true)}>
            Start mock
          </button>
        </div>
      </main>
    );
  }

  if (!nextSlot) {
    return (
      <main className="container max-w-2xl py-12">
        <h1 className="text-2xl font-bold">Mock complete</h1>
        <p className="mt-1 text-sm text-muted">
          You drafted {myPickNos.length} players. Review your team below.
        </p>
        <div className="card mt-6">
          <h2 className="text-sm font-semibold uppercase text-muted">
            Your team
          </h2>
          <ul className="mt-2 space-y-1 text-sm">
            {myPickNos.map((no) => {
              const pid = picks[no];
              const p = players.data?.players.find((x) => x.id === pid);
              return (
                <li key={no} className="flex justify-between">
                  <span>
                    Pick {no}: {p?.full_name ?? "—"}
                  </span>
                  <span className="text-muted">
                    {p?.position ?? "?"} · {p?.nfl_team ?? "?"}
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
        <button className="btn-outline mt-4" onClick={reset}>
          Run another
        </button>
      </main>
    );
  }

  return (
    <main className="container py-8">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Mock draft</h1>
        <button className="btn-outline text-xs" onClick={reset}>
          Reset
        </button>
      </div>
      <div
        className={`card mb-4 ${myTurn ? "border-accent bg-accent/10" : ""}`}
      >
        <div className="text-xs uppercase text-muted">On the clock</div>
        <div className="text-lg font-semibold">
          {myTurn ? "You" : `Bot ${nextSlot.teamIdx + 1}`} · Pick{" "}
          {nextPickNo} · Round {nextSlot.round}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <section className="lg:col-span-2">
          <h2 className="mb-2 text-sm font-semibold uppercase text-muted">
            Best available
          </h2>
          <div className="card max-h-[60vh] overflow-y-auto p-0">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-bg/95 text-xs uppercase text-muted">
                <tr>
                  <th className="px-3 py-2 text-left">Player</th>
                  <th className="px-3 py-2 text-left">Pos</th>
                  <th className="px-3 py-2 text-left">Team</th>
                  <th className="px-3 py-2"></th>
                </tr>
              </thead>
              <tbody>
                {available.slice(0, 60).map((p) => (
                  <tr key={p.id} className="border-t border-border">
                    <td className="px-3 py-2">{p.full_name}</td>
                    <td className="px-3 py-2 text-muted">
                      {p.position ?? "—"}
                    </td>
                    <td className="px-3 py-2 text-muted">
                      {p.nfl_team ?? "—"}
                    </td>
                    <td className="px-3 py-2 text-right">
                      <button
                        disabled={!myTurn}
                        onClick={() => userPick(p.id)}
                        className="btn-primary text-xs disabled:opacity-50"
                      >
                        Draft
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
            Recent
          </h2>
          <div className="card max-h-[60vh] overflow-y-auto">
            <ul className="space-y-1 text-sm">
              {Object.entries(picks)
                .sort((a, b) => Number(b[0]) - Number(a[0]))
                .slice(0, 20)
                .map(([no, pid]) => {
                  const p = players.data?.players.find((x) => x.id === pid);
                  const slot = order.find((s) => s.pick === Number(no));
                  return (
                    <li key={no} className="flex justify-between">
                      <span>
                        <span className="text-xs text-muted">{no}</span>{" "}
                        {p?.full_name ?? pid.slice(0, 8)}
                      </span>
                      <span className="text-xs text-muted">
                        {slot?.teamIdx === position - 1
                          ? "You"
                          : `Bot ${(slot?.teamIdx ?? 0) + 1}`}
                      </span>
                    </li>
                  );
                })}
            </ul>
          </div>
        </section>
      </div>
    </main>
  );
}

function makeSnakeOrder(teams: number, rounds: number): Slot[] {
  const out: Slot[] = [];
  let pickNo = 1;
  for (let r = 1; r <= rounds; r++) {
    const order = Array.from({ length: teams }, (_, i) => i);
    if (r % 2 === 0) order.reverse();
    for (const t of order) {
      out.push({ round: r, pick: pickNo, teamIdx: t });
      pickNo++;
    }
  }
  return out;
}

function pickForBot(available: Player[]): Player | null {
  if (available.length === 0) return null;
  // small randomness over the top 10 by position-weighted preference.
  const pool = available.slice(0, 10);
  return pool[Math.floor(Math.random() * pool.length)];
}
