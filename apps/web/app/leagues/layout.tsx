"use client";

import { useAuth } from "@/lib/auth-context";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

/** Requires auth before the create-league wizard (POST /v1/leagues is authenticated). */
export default function LeaguesLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, loading } = useAuth();

  useEffect(() => {
    if (loading) return;
    if (pathname === "/leagues/new" && !user) {
      router.replace(`/sign-in?next=${encodeURIComponent("/leagues/new")}`);
    }
  }, [loading, user, pathname, router]);

  if (pathname === "/leagues/new" && (loading || !user)) {
    return (
      <main className="container max-w-2xl py-10">
        <div className="skeleton mb-4 h-8 w-64 rounded" />
        <div className="card space-y-4">
          <div className="skeleton h-10 w-full rounded" />
          <div className="skeleton h-10 w-full rounded" />
        </div>
      </main>
    );
  }

  return children;
}
