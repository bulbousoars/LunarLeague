"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";

export default function LandingPage() {
  const { user, loading } = useAuth();
  return (
    <main className="container py-16">
      <header className="mb-16 flex items-center justify-between">
        <div className="text-2xl font-bold tracking-tight">Lunar League</div>
        {!loading && (
          <nav className="flex items-center gap-3 text-sm">
            {user ? (
              <Link href="/dashboard" className="btn-primary">
                Open dashboard
              </Link>
            ) : (
              <Link href="/sign-in" className="btn-primary">
                Sign in
              </Link>
            )}
          </nav>
        )}
      </header>

      <section className="mx-auto max-w-3xl text-center">
        <h1 className="text-balance text-5xl font-bold leading-tight md:text-6xl">
          Fantasy sports your league actually owns.
        </h1>
        <p className="mt-6 text-pretty text-lg text-muted">
          Open-source. Self-hosted. Real-time draft, full waivers, trade
          reviews, league chat — everything Yahoo does, none of the ads, all of
          your data.
        </p>
        <div className="mt-10 flex items-center justify-center gap-3">
          {user ? (
            <Link href="/dashboard" className="btn-primary">
              Go to your leagues
            </Link>
          ) : (
            <Link href="/sign-in" className="btn-primary">
              Start a league
            </Link>
          )}
          <a
            href="https://github.com/bulbousoars/LunarLeague"
            className="btn-outline"
          >
            View on GitHub
          </a>
        </div>
      </section>

      <section className="mt-24 grid gap-4 md:grid-cols-3">
        {features.map((f) => (
          <div key={f.title} className="card">
            <div className="text-xs font-semibold uppercase tracking-wide text-accent">
              {f.label}
            </div>
            <h3 className="mt-2 text-lg font-semibold">{f.title}</h3>
            <p className="mt-2 text-sm text-muted">{f.body}</p>
          </div>
        ))}
      </section>

      <footer className="mt-24 border-t border-border pt-8 text-xs text-muted">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span>Lunar League &mdash; MIT licensed</span>
          <span>
            Powered by Sleeper&apos;s public API. Drop in SportsData.io for
            premium real-time stats.
          </span>
        </div>
      </footer>
    </main>
  );
}

const features = [
  {
    label: "Drafts",
    title: "Live snake + auction rooms",
    body: "WebSocket-driven draft board, autopick queues, commissioner controls, and pre-draft keeper designation. Built for Sunday-night drafts with a beer and 10 friends.",
  },
  {
    label: "In season",
    title: "Real waivers, real trades",
    body: "FAAB or rolling waivers, scheduled processing, league-wide veto votes, commissioner overrides. Multi-team trades supported.",
  },
  {
    label: "Multi-sport",
    title: "Football today, hoops + baseball next",
    body: "Schema multi-sport from day one. NFL ships first, NBA piggybacks on Sleeper's API, MLB plugs into the official MLB Stats API.",
  },
];
