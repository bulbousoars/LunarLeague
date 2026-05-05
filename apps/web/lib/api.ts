// Thin fetch wrapper for the Lunar League Go API. Includes credentials so the
// session cookie travels on every request; the API also accepts Bearer tokens
// if you'd rather hold the session in localStorage.

const API_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

export async function api<T = unknown>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const url = path.startsWith("http") ? path : `${API_URL}${path}`;
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  // Reading from localStorage if running in browser.
  if (typeof window !== "undefined") {
    const tok = window.localStorage.getItem("lunarleague_token");
    if (tok && !headers.has("Authorization")) {
      headers.set("Authorization", `Bearer ${tok}`);
    }
  }
  const res = await fetch(url, {
    ...init,
    headers,
    credentials: "include",
    cache: "no-store",
  });
  const text = await res.text();
  let json: unknown;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    json = text;
  }
  if (!res.ok) {
    const msg =
      typeof json === "object" && json && "error" in json
        ? String((json as { error: string }).error)
        : `HTTP ${res.status}`;
    throw new ApiError(res.status, msg);
  }
  return json as T;
}

export const wsURL = (path: string) => {
  const base = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8000";
  return `${base}${path}`;
};
