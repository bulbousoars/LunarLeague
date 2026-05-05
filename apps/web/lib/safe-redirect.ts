/** Same-origin path for post sign-in navigation (open redirects rejected). */
export function safeRedirectPath(raw: string | null): string {
  if (!raw) return "";
  const s = raw.trim();
  if (!s) return "";
  const lower = s.toLowerCase();
  if (lower.includes("javascript:") || lower.includes("data:")) return "";
  if (s.includes("://") || s.startsWith("//")) return "";
  if (!s.startsWith("/")) return "";
  return s;
}
