"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import type { League, Sport } from "@/lib/types";
import { useQuery } from "@tanstack/react-query";
import { safeRedirectPath } from "@/lib/safe-redirect";

export default function NewLeaguePage() {
  const router = useRouter();
  const sports = useQuery({
    queryKey: ["sports"],
    queryFn: () => api<{ sports: Sport[] }>("/v1/sports"),
  });

  const [step, setStep] = useState(1);
  const [form, setForm] = useState({
    name: "",
    sport_code: "nfl",
    season: new Date().getFullYear(),
    league_format: "redraft" as "redraft" | "keeper",
    draft_format: "snake" as "snake" | "auction",
    team_count: 12,
    schedule_type: "h2h_points" as
      | "h2h_points"
      | "theme_ball",
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const created = await api<League>("/v1/leagues", {
        method: "POST",
        body: JSON.stringify(form),
      });
      const next =
        form.schedule_type === "theme_ball"
          ? `/leagues/${created.id}/themes`
          : `/leagues/${created.id}/setup`;
      router.replace(next);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        const next = safeRedirectPath("/leagues/new");
        router.replace(`/sign-in?next=${encodeURIComponent(next)}`);
        return;
      }
      setError(e instanceof ApiError ? e.message : "Failed to create league");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="container max-w-2xl py-10">
      <h1 className="text-2xl font-bold">Create a league</h1>
      <p className="mt-1 text-sm text-muted">
        Step {step} of 3 - takes about a minute.
      </p>

      <div className="mt-6 card space-y-4">
        {step === 1 && (
          <>
            <div>
              <label className="label">League name</label>
              <input
                className="input"
                value={form.name}
                onChange={(e) =>
                  setForm((f) => ({ ...f, name: e.target.value }))
                }
                placeholder="Bro League 2026"
                autoFocus
              />
            </div>
            <div>
              <label className="label">Sport</label>
              <select
                className="input"
                value={form.sport_code}
                onChange={(e) =>
                  setForm((f) => ({ ...f, sport_code: e.target.value }))
                }
              >
                {sports.data?.sports
                  .filter((s) => s.code === "nfl") // MVP: NFL only
                  .map((s) => (
                    <option key={s.id} value={s.code}>
                      {s.name}
                    </option>
                  )) ?? <option value="nfl">Football (NFL)</option>}
              </select>
              <p className="mt-1 text-xs text-muted">
                NBA and MLB coming next phase. Re-run setup once they ship.
              </p>
            </div>
            <div>
              <label className="label">Season</label>
              <input
                type="number"
                className="input"
                value={form.season}
                min={2024}
                max={2100}
                onChange={(e) =>
                  setForm((f) => ({ ...f, season: Number(e.target.value) }))
                }
              />
            </div>
            <div className="flex justify-end">
              <button
                className="btn-primary"
                disabled={!form.name}
                onClick={() => setStep(2)}
              >
                Continue
              </button>
            </div>
          </>
        )}

        {step === 2 && (
          <>
            <div>
              <label className="label">League format</label>
              <div className="grid grid-cols-2 gap-2">
                <FormatChoice
                  label="Redraft"
                  desc="Fresh start each year. Simplest."
                  selected={form.league_format === "redraft"}
                  onClick={() =>
                    setForm((f) => ({ ...f, league_format: "redraft" }))
                  }
                />
                <FormatChoice
                  label="Keeper"
                  desc="Carry N players year over year."
                  selected={form.league_format === "keeper"}
                  onClick={() =>
                    setForm((f) => ({ ...f, league_format: "keeper" }))
                  }
                />
              </div>
            </div>
            <div>
              <label className="label">Draft format</label>
              <div className="grid grid-cols-2 gap-2">
                <FormatChoice
                  label="Snake"
                  desc="Reverse pick order each round."
                  selected={form.draft_format === "snake"}
                  onClick={() =>
                    setForm((f) => ({ ...f, draft_format: "snake" }))
                  }
                />
                <FormatChoice
                  label="Auction"
                  desc="Budget-based bidding for every player."
                  selected={form.draft_format === "auction"}
                  onClick={() =>
                    setForm((f) => ({ ...f, draft_format: "auction" }))
                  }
                />
              </div>
            </div>
            <div>
              <label className="label">Scoring mode</label>
              <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-2">
                <FormatChoice
                  label="Head-to-head points"
                  desc="Classic weekly matchups."
                  selected={form.schedule_type === "h2h_points"}
                  onClick={() =>
                    setForm((f) => ({ ...f, schedule_type: "h2h_points" }))
                  }
                />
                <FormatChoice
                  label="Theme Ball"
                  desc="H2H points plus optional roster-identity modifiers."
                  selected={form.schedule_type === "theme_ball"}
                  onClick={() =>
                    setForm((f) => ({ ...f, schedule_type: "theme_ball" }))
                  }
                />
              </div>
            </div>
            <div>
              <label className="label">Team count</label>
              <select
                className="input"
                value={form.team_count}
                onChange={(e) =>
                  setForm((f) => ({ ...f, team_count: Number(e.target.value) }))
                }
              >
                {[4, 6, 8, 10, 12, 14, 16].map((n) => (
                  <option key={n} value={n}>
                    {n} teams
                  </option>
                ))}
              </select>
            </div>
            <div className="flex justify-between">
              <button className="btn-outline" onClick={() => setStep(1)}>
                Back
              </button>
              <button className="btn-primary" onClick={() => setStep(3)}>
                Review
              </button>
            </div>
          </>
        )}

        {step === 3 && (
          <>
            <div>
              <h2 className="text-lg font-semibold">Looks good?</h2>
              <p className="mt-1 text-sm text-muted">
                You&apos;ll get an email with setup and invite links. You can
                also invite friends from the league dashboard.
              </p>
            </div>
            <dl className="grid grid-cols-2 gap-2 text-sm">
              <Row label="Name" value={form.name} />
              <Row label="Sport" value={form.sport_code.toUpperCase()} />
              <Row label="Season" value={String(form.season)} />
              <Row label="Format" value={form.league_format} />
              <Row label="Draft" value={form.draft_format} />
              <Row
                label="Scoring"
                value={
                  form.schedule_type === "theme_ball"
                    ? "Theme Ball"
                    : "H2H points"
                }
              />
              <Row label="Teams" value={String(form.team_count)} />
            </dl>
            {error && <div className="text-sm text-red-400">{error}</div>}
            <div className="flex justify-between">
              <button className="btn-outline" onClick={() => setStep(2)}>
                Back
              </button>
              <button
                className="btn-primary"
                onClick={submit}
                disabled={submitting}
              >
                {submitting ? "Creating..." : "Create league"}
              </button>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

function FormatChoice({
  label,
  desc,
  selected,
  onClick,
}: {
  label: string;
  desc: string;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-md border p-3 text-left transition ${
        selected
          ? "border-accent bg-accent/10"
          : "border-border hover:border-accent/40"
      }`}
    >
      <div className="text-sm font-semibold">{label}</div>
      <div className="mt-1 text-xs text-muted">{desc}</div>
    </button>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <>
      <dt className="text-muted">{label}</dt>
      <dd className="text-fg capitalize">{value}</dd>
    </>
  );
}
