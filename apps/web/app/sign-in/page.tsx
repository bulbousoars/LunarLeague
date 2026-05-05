"use client";

import Link from "next/link";
import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { safeRedirectPath } from "@/lib/safe-redirect";

const AUTH_NEXT_KEY = "lunarleague_auth_next";

export default function SignInPage() {
  return (
    <Suspense fallback={<SignInShell />}>
      <SignInInner />
    </Suspense>
  );
}

function SignInInner() {
  const searchParams = useSearchParams();
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const next = safeRedirectPath(searchParams.get("next"));
    if (typeof window === "undefined") return;
    if (next) sessionStorage.setItem(AUTH_NEXT_KEY, next);
    else sessionStorage.removeItem(AUTH_NEXT_KEY);
  }, [searchParams]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api("/v1/auth/magic-link", {
        method: "POST",
        body: JSON.stringify({ email }),
      });
      setSent(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="container max-w-md py-20">
      <Link href="/" className="text-2xl font-bold">
        Lunar League
      </Link>
      <div className="mt-12">
        {sent ? (
          <div className="card text-center">
            <h1 className="text-xl font-semibold">Check your inbox</h1>
            <p className="mt-2 text-sm text-muted">
              We sent a sign-in link to <strong>{email}</strong>. The link
              expires in 15 minutes.
            </p>
            <p className="mt-6 text-xs text-muted">
              Self-hosted dev: open{" "}
              <a
                href="http://localhost:8025"
                className="text-accent underline"
              >
                Mailhog
              </a>{" "}
              (or your SMTP inbox) to read the message.
            </p>
          </div>
        ) : (
          <form onSubmit={onSubmit} className="card space-y-4">
            <div>
              <h1 className="text-xl font-semibold">Sign in</h1>
              <p className="mt-1 text-sm text-muted">
                We&apos;ll email you a one-time link. No passwords.
              </p>
            </div>
            <div>
              <label className="label" htmlFor="email">
                Email
              </label>
              <input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="input"
                placeholder="you@example.com"
                autoComplete="email"
              />
            </div>
            {error && <div className="text-sm text-red-400">{error}</div>}
            <button
              type="submit"
              disabled={submitting || !email}
              className="btn-primary w-full"
            >
              {submitting ? "Sending..." : "Email me a link"}
            </button>
          </form>
        )}
      </div>
    </main>
  );
}

function SignInShell() {
  return (
    <main className="container max-w-md py-20">
      <Link href="/" className="text-2xl font-bold">
        Lunar League
      </Link>
      <div className="mt-12 card space-y-4">
        <div className="skeleton h-8 w-48 rounded" />
        <div className="skeleton h-10 w-full rounded" />
      </div>
    </main>
  );
}
