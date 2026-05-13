/** Short preview of a provider weekly stat map for table cells. */
export function summarizeWeeklyStats(
  raw: Record<string, unknown> | null | undefined,
  maxPairs = 5,
): string {
  if (!raw || typeof raw !== "object") return "—";
  const pairs = Object.entries(raw).filter(
    ([, v]) => typeof v === "number" && Number(v) !== 0,
  );
  if (!pairs.length) return "—";
  return pairs
    .slice(0, maxPairs)
    .map(([k, v]) => `${k}:${v}`)
    .join(" ");
}

export function formatStatNumber(n: number): string {
  if (!Number.isFinite(n)) return "—";
  if (Math.abs(n - Math.round(n)) < 1e-6) return String(Math.round(n));
  return n.toFixed(2);
}

/** Read a numeric stat from a provider JSON map (Sleeper keys, etc.). */
export function statCell(
  raw: Record<string, unknown> | null | undefined,
  key: string,
): string {
  if (!raw || typeof raw !== "object") return "—";
  const v = raw[key];
  if (typeof v === "number" && Number.isFinite(v)) {
    if (v === 0) return "0";
    return formatStatNumber(v);
  }
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    if (Number.isFinite(n)) return formatStatNumber(n);
  }
  return "—";
}

export function formatProfileBrief(p: {
  age?: number | null;
  height_inches?: number | null;
  weight_lbs?: number | null;
  college?: string | null;
  years_exp?: number | null;
}): string {
  const bits: string[] = [];
  if (p.age != null && p.age > 0) bits.push(`age ${p.age}`);
  if (p.height_inches != null && p.height_inches > 0) {
    const ft = Math.floor(p.height_inches / 12);
    const inch = p.height_inches % 12;
    bits.push(`${ft}'${inch}"`);
  }
  if (p.weight_lbs != null && p.weight_lbs > 0) bits.push(`${p.weight_lbs} lb`);
  if (p.years_exp != null && p.years_exp >= 0) bits.push(`${p.years_exp} y exp`);
  if (p.college && p.college.trim()) bits.push(p.college.trim());
  return bits.length ? bits.join(" · ") : "—";
}
