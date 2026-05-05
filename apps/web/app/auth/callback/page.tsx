"use client";

import { Suspense } from "react";
import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import type { User } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { safeRedirectPath } from "@/lib/safe-redirect";

const AUTH_NEXT_KEY = "lunarleague_auth_next";

type VerifyResponse = {
  session_token: string;
  user: User;
};

export default function CallbackPage() {
  return (
    <Suspense fallback={<CallbackShell />}>
      <CallbackContent />
    </Suspense>
  );
}

function CallbackContent() {
  const params = useSearchParams();
  const router = useRouter();
  const { setSessionToken } = useAuth();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const token = params.get("token");
    if (!token) {
      setError("Missing token");
      return;
    }
    api<VerifyResponse>("/v1/auth/verify", {
      method: "POST",
      body: JSON.stringify({ token }),
    })
      .then((res) => {
        setSessionToken(res.session_token, res.user);
        let dest = "/dashboard";
        if (typeof window !== "undefined") {
          const raw = sessionStorage.getItem(AUTH_NEXT_KEY);
          sessionStorage.removeItem(AUTH_NEXT_KEY);
          const next = safeRedirectPath(raw);
          if (next) dest = next;
        }
        router.replace(dest);
      })
      .catch((e) => setError(e.message ?? "Sign-in failed"));
  }, [params, router, setSessionToken]);

  return (
    <main className="container max-w-md py-20 text-center">
      {error ? (
        <div className="card">
          <h1 className="text-xl font-semibold">Sign-in failed</h1>
          <p className="mt-2 text-sm text-muted">{error}</p>
          <a href="/sign-in" className="btn-primary mt-4 inline-flex">
            Try again
          </a>
        </div>
      ) : (
        <div className="card">
          <h1 className="text-xl font-semibold">Signing you in...</h1>
          <p className="mt-2 text-sm text-muted">One moment.</p>
        </div>
      )}
    </main>
  );
}

function CallbackShell() {
  return (
    <main className="container max-w-md py-20 text-center">
      <div className="card">
        <h1 className="text-xl font-semibold">Signing you in...</h1>
        <p className="mt-2 text-sm text-muted">One moment.</p>
      </div>
    </main>
  );
}
