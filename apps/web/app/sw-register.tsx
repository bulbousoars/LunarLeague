"use client";

import { useEffect } from "react";

// Register the service worker once on first mount. Idempotent.
export function ServiceWorkerRegistration() {
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!("serviceWorker" in navigator)) return;
    navigator.serviceWorker
      .register("/sw.js", { scope: "/" })
      .catch(() => {
        // ignore — pwa is progressive
      });
  }, []);
  return null;
}
