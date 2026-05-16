"use client";

import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";
import type { League } from "@/lib/types";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

const BASE_TABS = [
  { href: "", label: "Overview" },
  { href: "/team", label: "My Team" },
  { href: "/scoreboard", label: "Scoreboard" },
  { href: "/draft", label: "Draft" },
  { href: "/players", label: "Players" },
  { href: "/waivers", label: "Waivers" },
  { href: "/trades", label: "Trades" },
  { href: "/chat", label: "Chat" },
  { href: "/settings", label: "Settings" },
] as const;

const THEME_BALL_TAB = { href: "/themes", label: "Themes" } as const;

export default function LeagueLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const params = useParams<{ leagueId: string }>();
  const pathname = usePathname();
  const router = useRouter();
  const { user, loading } = useAuth();

  useEffect(() => {
    if (!loading && !user) router.replace("/sign-in");
  }, [loading, user, router]);

  const league = useQuery({
    queryKey: ["league", params.leagueId],
    queryFn: () => api<League>(`/v1/leagues/${params.leagueId}`),
    enabled: !!user,
  });

  if (loading || !user) return null;

  const base = `/leagues/${params.leagueId}`;
  const tabs =
    league.data?.schedule_type === "theme_ball"
      ? [
          ...BASE_TABS.slice(0, 8),
          THEME_BALL_TAB,
          ...BASE_TABS.slice(8),
        ]
      : [...BASE_TABS];

  return (
    <div className="min-h-screen">
      <header className="border-b border-border bg-card/40 backdrop-blur">
        <div className="container">
          <div className="flex h-14 items-center justify-between">
            <div className="flex items-center gap-4">
              <Link
                href="/dashboard"
                className="text-sm text-muted hover:text-fg"
              >
                ← All leagues
              </Link>
              <h1 className="text-lg font-semibold">
                {league.data?.name ?? <span className="skeleton inline-block h-5 w-32 rounded" />}
              </h1>
              {league.data && (
                <span className="hidden rounded-full border border-border px-2 py-0.5 text-xs uppercase text-muted md:inline">
                  {league.data.status.replace("_", " ")}
                </span>
              )}
            </div>
            {league.data?.invite_code && (
              <button
                onClick={() => {
                  const url = `${window.location.origin}${base}/join?code=${league.data?.invite_code}`;
                  navigator.clipboard.writeText(url);
                }}
                className="btn-outline text-xs"
                title="Copy invite link"
              >
                Invite friends
              </button>
            )}
          </div>
          <nav className="-mb-px flex gap-1 overflow-x-auto">
            {tabs.map((t) => {
              const href = `${base}${t.href}`;
              const active =
                pathname === href ||
                (t.href !== "" && pathname.startsWith(href));
              return (
                <Link
                  key={t.href}
                  href={href}
                  className={`whitespace-nowrap border-b-2 px-3 py-2 text-sm transition ${
                    active
                      ? "border-accent text-fg"
                      : "border-transparent text-muted hover:text-fg"
                  }`}
                >
                  {t.label}
                </Link>
              );
            })}
          </nav>
        </div>
      </header>
      {children}
    </div>
  );
}
