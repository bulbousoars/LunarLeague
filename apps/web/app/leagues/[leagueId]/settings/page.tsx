"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Settings } from "@/lib/types";
import Link from "next/link";

export default function SettingsPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const qc = useQueryClient();

  const settings = useQuery({
    queryKey: ["settings", leagueId],
    queryFn: () => api<Settings>(`/v1/leagues/${leagueId}/settings`),
  });

  const [draft, setDraft] = useState<Settings | null>(null);
  useEffect(() => {
    if (settings.data) setDraft(settings.data);
  }, [settings.data]);

  const save = useMutation({
    mutationFn: (s: Settings) =>
      api(`/v1/leagues/${leagueId}/settings`, {
        method: "PATCH",
        body: JSON.stringify(s),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["settings", leagueId] }),
  });

  const generate = useMutation({
    mutationFn: (regularWeeks: number) =>
      api(`/v1/leagues/${leagueId}/schedule/generate`, {
        method: "POST",
        body: JSON.stringify({
          season: new Date().getFullYear(),
          regular_weeks: regularWeeks,
        }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["scoreboard", leagueId] }),
  });

  if (!draft) return <main className="container py-8"><div className="skeleton h-48 rounded" /></main>;

  return (
    <main className="container max-w-3xl py-8">
      <h1 className="mb-4 text-2xl font-bold">League settings</h1>

      <section className="card mb-4 space-y-4">
        <h2 className="text-base font-semibold">Roster</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {Object.entries(draft.roster_slots).map(([slot, count]) => (
            <div key={slot}>
              <label className="label">{slot}</label>
              <input
                type="number"
                className="input"
                value={count}
                min={0}
                onChange={(e) =>
                  setDraft((d) =>
                    d
                      ? {
                          ...d,
                          roster_slots: {
                            ...d.roster_slots,
                            [slot]: Number(e.target.value),
                          },
                        }
                      : d,
                  )
                }
              />
            </div>
          ))}
        </div>
      </section>

      <section className="card mb-4 space-y-4">
        <h2 className="text-base font-semibold">Waivers</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <div>
            <label className="label">Type</label>
            <select
              className="input"
              value={draft.waiver_type}
              onChange={(e) =>
                setDraft((d) =>
                  d ? { ...d, waiver_type: e.target.value as Settings["waiver_type"] } : d,
                )
              }
            >
              <option value="faab">FAAB</option>
              <option value="rolling">Rolling</option>
              <option value="reverse_standings">Reverse standings</option>
            </select>
          </div>
          <div>
            <label className="label">Budget</label>
            <input
              type="number"
              className="input"
              value={draft.waiver_budget}
              onChange={(e) =>
                setDraft((d) =>
                  d ? { ...d, waiver_budget: Number(e.target.value) } : d,
                )
              }
            />
          </div>
          <div>
            <label className="label">Run day (0-6)</label>
            <input
              type="number"
              className="input"
              value={draft.waiver_run_dow}
              min={0}
              max={6}
              onChange={(e) =>
                setDraft((d) =>
                  d ? { ...d, waiver_run_dow: Number(e.target.value) } : d,
                )
              }
            />
          </div>
        </div>
      </section>

      <section className="card mb-4 space-y-4">
        <h2 className="text-base font-semibold">Playoffs</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <div>
            <label className="label">Start week</label>
            <input
              type="number"
              className="input"
              value={draft.playoff_start_week}
              onChange={(e) =>
                setDraft((d) =>
                  d ? { ...d, playoff_start_week: Number(e.target.value) } : d,
                )
              }
            />
          </div>
          <div>
            <label className="label">Teams</label>
            <input
              type="number"
              className="input"
              value={draft.playoff_team_count}
              onChange={(e) =>
                setDraft((d) =>
                  d ? { ...d, playoff_team_count: Number(e.target.value) } : d,
                )
              }
            />
          </div>
          <div>
            <label className="label">Trade deadline</label>
            <input
              type="number"
              className="input"
              value={draft.trade_deadline_week ?? ""}
              placeholder="Week"
              onChange={(e) =>
                setDraft((d) =>
                  d
                    ? {
                        ...d,
                        trade_deadline_week: e.target.value ? Number(e.target.value) : null,
                      }
                    : d,
                )
              }
            />
          </div>
        </div>
      </section>

      <section className="card mb-4 space-y-3">
        <h2 className="text-base font-semibold">Schedule</h2>
        <p className="text-sm text-muted">
          Generate a regular-season schedule. Run this once when your league is
          fully claimed and before the draft.
        </p>
        <div className="flex items-center gap-2">
          <button
            className="btn-outline"
            onClick={() => generate.mutate(14)}
            disabled={generate.isPending}
          >
            Generate 14-week schedule
          </button>
          {generate.isSuccess && (
            <span className="text-xs text-emerald-400">Generated</span>
          )}
        </div>
      </section>

      <div className="flex justify-between">
        <Link href={`/leagues/${leagueId}/scoring`} className="btn-outline">
          Edit scoring rules →
        </Link>
        <button
          className="btn-primary"
          onClick={() => save.mutate(draft)}
          disabled={save.isPending}
        >
          {save.isPending ? "Saving..." : "Save settings"}
        </button>
      </div>
    </main>
  );
}
