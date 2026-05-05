"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";

type Rules = Record<string, number>;

const GROUPS: { label: string; keys: { key: string; label: string }[] }[] = [
  {
    label: "Passing",
    keys: [
      { key: "pass_yd", label: "Yards" },
      { key: "pass_td", label: "TD" },
      { key: "pass_int", label: "INT" },
      { key: "pass_2pt", label: "2PT" },
    ],
  },
  {
    label: "Rushing",
    keys: [
      { key: "rush_yd", label: "Yards" },
      { key: "rush_td", label: "TD" },
      { key: "rush_2pt", label: "2PT" },
    ],
  },
  {
    label: "Receiving",
    keys: [
      { key: "rec", label: "Reception (PPR)" },
      { key: "rec_yd", label: "Yards" },
      { key: "rec_td", label: "TD" },
      { key: "rec_2pt", label: "2PT" },
    ],
  },
  {
    label: "Misc",
    keys: [{ key: "fum_lost", label: "Fumble lost" }],
  },
  {
    label: "Defense / ST",
    keys: [
      { key: "def_int", label: "INT" },
      { key: "def_fr", label: "FR" },
      { key: "def_sack", label: "Sack" },
      { key: "def_td", label: "TD" },
      { key: "def_safe", label: "Safety" },
      { key: "st_td", label: "ST TD" },
    ],
  },
  {
    label: "Kicker",
    keys: [
      { key: "fgm_0_19", label: "FG 0-19" },
      { key: "fgm_20_29", label: "FG 20-29" },
      { key: "fgm_30_39", label: "FG 30-39" },
      { key: "fgm_40_49", label: "FG 40-49" },
      { key: "fgm_50p", label: "FG 50+" },
      { key: "fgmiss", label: "FG miss" },
      { key: "xpm", label: "XP" },
      { key: "xpmiss", label: "XP miss" },
    ],
  },
  {
    label: "Bonuses",
    keys: [
      { key: "bonus_pass_yd_300", label: "300+ pass yd" },
      { key: "bonus_rush_yd_100", label: "100+ rush yd" },
      { key: "bonus_rec_yd_100", label: "100+ rec yd" },
    ],
  },
];

export default function ScoringPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const qc = useQueryClient();

  const rulesQ = useQuery({
    queryKey: ["scoring", leagueId],
    queryFn: () => api<{ rules: Rules }>(`/v1/leagues/${leagueId}/scoring`),
  });

  const [rules, setRules] = useState<Rules>({});
  useEffect(() => {
    if (rulesQ.data) setRules(rulesQ.data.rules);
  }, [rulesQ.data]);

  const save = useMutation({
    mutationFn: (r: Rules) =>
      api(`/v1/leagues/${leagueId}/scoring`, {
        method: "PATCH",
        body: JSON.stringify({ rules: r }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["scoring", leagueId] }),
  });

  return (
    <main className="container max-w-4xl py-8">
      <h1 className="mb-4 text-2xl font-bold">Scoring rules</h1>
      <p className="mb-6 text-sm text-muted">
        Points per stat. The engine recomputes scores on read, so changes here
        retroactively update past matchups - use with caution mid-season.
      </p>

      <div className="grid gap-4 md:grid-cols-2">
        {GROUPS.map((g) => (
          <section key={g.label} className="card">
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-accent">
              {g.label}
            </h2>
            <div className="grid grid-cols-2 gap-3">
              {g.keys.map((k) => (
                <div key={k.key}>
                  <label className="label">{k.label}</label>
                  <input
                    type="number"
                    step="0.01"
                    className="input"
                    value={rules[k.key] ?? 0}
                    onChange={(e) =>
                      setRules((r) => ({
                        ...r,
                        [k.key]: Number(e.target.value),
                      }))
                    }
                  />
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>

      <div className="mt-6 flex items-center justify-between">
        <p className="text-xs text-muted">
          Tip: set <code className="rounded bg-card px-1">rec</code> to{" "}
          <code className="rounded bg-card px-1">1</code> for full PPR,{" "}
          <code className="rounded bg-card px-1">0.5</code> for half,{" "}
          <code className="rounded bg-card px-1">0</code> for standard.
        </p>
        <button
          className="btn-primary"
          onClick={() => save.mutate(rules)}
          disabled={save.isPending}
        >
          {save.isPending ? "Saving..." : "Save"}
        </button>
      </div>
    </main>
  );
}
