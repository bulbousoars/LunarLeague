"use client";

import { useAuth } from "@/lib/auth-context";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { user, loading, signOut } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !user) router.replace("/sign-in");
  }, [loading, user, router]);

  if (loading || !user) {
    return (
      <main className="container py-16">
        <div className="skeleton h-12 w-48 rounded" />
      </main>
    );
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-border bg-card/40 backdrop-blur">
        <div className="container flex h-14 items-center justify-between">
          <Link href="/dashboard" className="text-lg font-bold tracking-tight">
            Lunar League
          </Link>
          <nav className="flex items-center gap-2 text-sm">
            <span className="hidden text-muted md:inline">{user.email}</span>
            <button onClick={signOut} className="btn-ghost">
              Sign out
            </button>
          </nav>
        </div>
      </header>
      {children}
    </div>
  );
}
