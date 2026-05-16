"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import type {
  League,
  ThemeCatalogEntry,
  ThemeModifiersConfig,
  ThemeModifiersResponse,
  ThemeVote,
} from "@/lib/types";

export default function ThemeBallPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const qc = useQueryClient();

  const league = useQuery({
    queryKey: ["league", leagueId],
    queryFn: () => api<League>(`/v1/leagues/${leagueId}`),
  });

  const catalog = useQuery({
    queryKey: ["theme-catalog"],
    queryFn: () =>
      api<{ themes: ThemeCatalogEntry[] }>("/v1/meta/theme-catalog"),
  });

  const mods = useQuery({
    queryKey: ["theme-modifiers", leagueId],
    queryFn: () =>
      api<ThemeModifiersResponse>(`/v1/leagues/${leagueId}/theme-modifiers`),
    enabled: league.isSuccess,
  });

  const votes = useQuery({
    queryKey: ["theme-votes", leagueId],
    queryFn: () =>
      api<{
        votes: ThemeVote[];
        eligible_owners: number;
        need_yes_votes: number;
      }>(`/v1/leagues/${leagueId}/theme-votes`),
    enabled: league.isSuccess,
  });

  const [draft, setDraft] = useState<ThemeModifiersConfig | null>(null);
  useEffect(() => {
    if (mods.data?.modifiers) setDraft(mods.data.modifiers);
  }, [mods.data?.modifiers]);

  const save = useMutation({
    mutationFn: (modifiers: ThemeModifiersConfig) =>
      api(`/v1/leagues/${leagueId}/theme-modifiers`, {
        method: "PATCH",
        body: JSON.stringify({ modifiers }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["theme-modifiers", leagueId] });
    },
  });

  const [voteSlug, setVoteSlug] = useState("");
  const [voteAction, setVoteAction] = useState<"enable" | "disable">("enable");

  const openVote = useMutation({
    mutationFn: () =>
      api(`/v1/leagues/${leagueId}/theme-votes`, {
        method: "POST",
        body: JSON.stringify({ slug: voteSlug, action: voteAction }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["theme-votes", leagueId] });
    },
  });

  const castBallot = useMutation({
    mutationFn: ({ voteId, yes }: { voteId: string; yes: boolean }) =>
      api(`/v1/leagues/${leagueId}/theme-votes/${voteId}/ballot`, {
        method: "POST",
        body: JSON.stringify({ yes }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["theme-votes", leagueId] });
    },
  });

  if (mods.isError && mods.error instanceof ApiError && mods.error.status === 400) {
    return (
      <main className="container max-w-2xl py-10">
        <p className="text-muted">This league is not Theme Ball.</p>
        <Link
          href={`/leagues/${leagueId}/settings`}
          className="btn-outline mt-4 inline-block"
        >
          Settings
        </Link>
      </main>
    );
  }

  if (!draft || !catalog.data) {
    return (
      <main className="container max-w-3xl py-10">
        <div className="skeleton h-64 rounded" aria-hidden />
      </main>
    );
  }

  const openVoteRow = votes.data?.votes.find((v) => v.status === "open");
  const isThemeBall = mods.data?.schedule_type === "theme_ball";

  return (
    <main className="container max-w-3xl py-10">
      <HeaderBlock leagueId={leagueId} votes={votes.data} />

      {!isThemeBall && (
        <p className="mb-4 rounded border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
          Schedule type is not Theme Ball yet. Set{" "}
          <code className="font-mono">theme_ball</code> in league settings.
        </p>
      )}

      <section className="card mb-6 space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-base font-semibold">
            Modifiers{" "}
            <span className="text-muted">({mods.data?.enabled_count ?? 0} on)</span>
          </h2>
          <button
            type="button"
            className="btn-primary text-sm"
            disabled={save.isPending || !isThemeBall}
            onClick={() => save.mutate(draft)}
          >
            {save.isPending ? "Saving…" : "Save toggles"}
          </button>
        </div>
        <ul className="max-h-[32rem] space-y-2 overflow-y-auto pr-1">
          {catalog.data.themes.map((t) => {
            const entry = draft[t.slug] ?? { enabled: false };
            return (
              <li
                key={t.slug}
                className="flex items-start gap-3 rounded-md border border-border px-3 py-2"
              >
                <input
                  type="checkbox"
                  className="mt-1 rounded border-border"
                  checked={entry.enabled}
                  disabled={!t.available}
                  onChange={(e) =>
                    setDraft((d) => ({
                      ...d,
                      [t.slug]: { ...entry, enabled: e.target.checked },
                    }))
                  }
                  aria-label={`Enable ${t.name}`}
                />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium">
                    {t.name}
                    {!t.available && (
                      <span className="ml-2 text-xs text-muted">(coming soon)</span>
                    )}
                  </div>
                  <p className="text-xs text-muted">{t.description}</p>
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      <section className="card space-y-4">
        <h2 className="text-base font-semibold">Owner vote</h2>
        {openVoteRow ? (
          <OpenVotePanel
            vote={openVoteRow}
            onBallot={(yes) =>
              castBallot.mutate({ voteId: openVoteRow.id, yes })
            }
            ballotPending={castBallot.isPending}
          />
        ) : (
          <>
            <p className="text-sm text-muted">
              Propose enabling or disabling one theme. Voting stays open 7 days.
            </p>
            <div className="flex flex-wrap items-end gap-2">
              <div>
                <label className="label">Theme</label>
                <select
                  className="input min-w-[12rem]"
                  value={voteSlug}
                  onChange={(e) => setVoteSlug(e.target.value)}
                >
                  <option value="">Select…</option>
                  {catalog.data.themes
                    .filter((t) => t.available)
                    .map((t) => (
                      <option key={t.slug} value={t.slug}>
                        {t.name}
                      </option>
                    ))}
                </select>
              </div>
              <div>
                <label className="label">Action</label>
                <select
                  className="input"
                  value={voteAction}
                  onChange={(e) =>
                    setVoteAction(e.target.value as "enable" | "disable")
                  }
                >
                  <option value="enable">Enable</option>
                  <option value="disable">Disable</option>
                </select>
              </div>
              <button
                type="button"
                className="btn-outline"
                disabled={!voteSlug || openVote.isPending}
                onClick={() => openVote.mutate()}
              >
                Start vote
              </button>
            </div>
            {openVote.isError && (
              <p className="text-xs text-red-300">Could not open vote.</p>
            )}
          </>
        )}
      </section>
    </main>
  );
}

function HeaderBlock({
  leagueId,
  votes,
}: {
  leagueId: string;
  votes?: { need_yes_votes: number; eligible_owners: number };
}) {
  return (
    <div className="mb-6">
      <Link
        href={`/leagues/${leagueId}/setup`}
        className="text-sm text-muted hover:text-fg"
      >
        ← Setup
      </Link>
      <h1 className="mt-2 text-2xl font-bold">Theme Ball rules</h1>
      <p className="mt-1 text-sm text-muted">
        Toggle modifiers before the draft. During the season, owners can vote to
        change a rule — needs a majority of claimed franchises (
        {votes?.need_yes_votes ?? "—"} yes
        {votes?.eligible_owners != null ? ` of ${votes.eligible_owners}` : ""}).
      </p>
    </div>
  );
}

function OpenVotePanel({
  vote,
  onBallot,
  ballotPending,
}: {
  vote: ThemeVote;
  onBallot: (yes: boolean) => void;
  ballotPending: boolean;
}) {
  return (
    <div className="rounded-md border border-accent/40 bg-accent/5 p-3 text-sm">
      <p>
        Open vote: <strong>{vote.action}</strong>{" "}
        <code className="font-mono text-xs">{vote.slug}</code>
      </p>
      <p className="mt-1 text-muted">
        Yes {vote.yes_count} / need {vote.need_yes_votes} · closes{" "}
        {new Date(vote.closes_at).toLocaleString()}
      </p>
      <div className="mt-3 flex gap-2">
        <button
          type="button"
          className="btn-primary text-xs"
          disabled={ballotPending}
          onClick={() => onBallot(true)}
        >
          Vote yes
        </button>
        <button
          type="button"
          className="btn-outline text-xs"
          disabled={ballotPending}
          onClick={() => onBallot(false)}
        >
          Vote no
        </button>
        {vote.my_ballot != null && (
          <span className="self-center text-xs text-muted">
            You voted {vote.my_ballot ? "yes" : "no"}
          </span>
        )}
      </div>
    </div>
  );
}
