import type { Player } from "@/lib/types";
import { statNumeric } from "@/lib/player-stats";

export type SortDir = "asc" | "desc";

export type SortRule = {
  id: string;
  dir: SortDir;
};

/** Column ids for primary header clicks and advanced sort. */
export const PLAYER_SORT_IDS = {
  full_name: "full_name",
  jersey_number: "jersey_number",
  position: "position",
  eligible: "eligible",
  nfl_team: "nfl_team",
  status: "status",
  injury: "injury",
  age: "age",
  height_inches: "height_inches",
  weight_lbs: "weight_lbs",
  years_exp: "years_exp",
  college: "college",
  gp: "gp",
} as const;

export function statSortId(
  band: "wk" | "ytd" | "avg",
  key: string,
): string {
  return `stat:${band}:${key}`;
}

function str(v: string | null | undefined): string {
  return (v ?? "").trim().toLowerCase();
}

export function sortValueForColumn(
  p: Player,
  colId: string,
  includeStats: boolean,
): string | number {
  switch (colId) {
    case PLAYER_SORT_IDS.full_name:
      return str(p.full_name);
    case PLAYER_SORT_IDS.jersey_number:
      return p.jersey_number ?? -1;
    case PLAYER_SORT_IDS.position:
      return str(p.position);
    case PLAYER_SORT_IDS.eligible:
      return (p.eligible_positions ?? []).join(",").toLowerCase();
    case PLAYER_SORT_IDS.nfl_team:
      return str(p.nfl_team);
    case PLAYER_SORT_IDS.status:
      return str(p.status);
    case PLAYER_SORT_IDS.injury:
      return str(p.injury_status);
    case PLAYER_SORT_IDS.age:
      return p.age ?? -1;
    case PLAYER_SORT_IDS.height_inches:
      return p.height_inches ?? -1;
    case PLAYER_SORT_IDS.weight_lbs:
      return p.weight_lbs ?? -1;
    case PLAYER_SORT_IDS.years_exp:
      return p.years_exp ?? -1;
    case PLAYER_SORT_IDS.college:
      return str(p.college);
    case PLAYER_SORT_IDS.gp:
      return includeStats ? (p.season_weeks ?? -1) : -1;
    default: {
      if (!colId.startsWith("stat:")) return "";
      const rest = colId.slice("stat:".length);
      const idx = rest.indexOf(":");
      if (idx < 0) return "";
      const band = rest.slice(0, idx) as "wk" | "ytd" | "avg";
      const key = rest.slice(idx + 1);
      const wk = p.weekly_stats as Record<string, unknown> | null | undefined;
      const ytd = p.season_totals as Record<string, unknown> | null | undefined;
      const avg = p.season_weekly_avg as Record<string, unknown> | null | undefined;
      let n: number | null = null;
      if (band === "wk") n = statNumeric(wk, key);
      else if (band === "ytd") n = statNumeric(ytd, key);
      else n = statNumeric(avg, key);
      return n ?? Number.NaN;
    }
  }
}

function comparePrimitive(
  a: string | number,
  b: string | number,
  dir: SortDir,
): number {
  let cmp = 0;
  if (typeof a === "number" && typeof b === "number") {
    if (!Number.isFinite(a) && !Number.isFinite(b)) cmp = 0;
    else if (!Number.isFinite(a)) cmp = 1;
    else if (!Number.isFinite(b)) cmp = -1;
    else cmp = a === b ? 0 : a < b ? -1 : 1;
  } else {
    cmp = String(a).localeCompare(String(b), undefined, { sensitivity: "base" });
  }
  return dir === "desc" ? -cmp : cmp;
}

export function sortPlayers(
  rows: Player[],
  primary: SortRule | null,
  advanced: SortRule[],
  includeStats: boolean,
): Player[] {
  const rules: SortRule[] =
    advanced.length > 0
      ? [...advanced]
      : primary
        ? [primary]
        : [];
  if (!rules.length) return rows;

  const copy = rows.slice();
  copy.sort((a, b) => {
    for (const r of rules) {
      const va = sortValueForColumn(a, r.id, includeStats);
      const vb = sortValueForColumn(b, r.id, includeStats);
      let cmp = 0;
      if (typeof va === "number" && typeof vb === "number") {
        const aFin = Number.isFinite(va);
        const bFin = Number.isFinite(vb);
        if (!aFin && !bFin) cmp = 0;
        else if (!aFin) cmp = 1;
        else if (!bFin) cmp = -1;
        else cmp = va === vb ? 0 : va < vb ? -1 : 1;
        if (r.dir === "desc") cmp = -cmp;
      } else {
        cmp = comparePrimitive(va, vb, r.dir);
      }
      if (cmp !== 0) return cmp;
    }
    return str(a.full_name).localeCompare(str(b.full_name));
  });
  return copy;
}

export const SORT_COLUMN_OPTIONS: { id: string; label: string }[] = [
  { id: PLAYER_SORT_IDS.full_name, label: "Player" },
  { id: PLAYER_SORT_IDS.jersey_number, label: "#" },
  { id: PLAYER_SORT_IDS.position, label: "Pos" },
  { id: PLAYER_SORT_IDS.eligible, label: "Elig" },
  { id: PLAYER_SORT_IDS.nfl_team, label: "Team" },
  { id: PLAYER_SORT_IDS.status, label: "Status" },
  { id: PLAYER_SORT_IDS.injury, label: "Injury" },
  { id: PLAYER_SORT_IDS.age, label: "Age" },
  { id: PLAYER_SORT_IDS.height_inches, label: "Ht (in)" },
  { id: PLAYER_SORT_IDS.weight_lbs, label: "Wt" },
  { id: PLAYER_SORT_IDS.years_exp, label: "Exp" },
  { id: PLAYER_SORT_IDS.college, label: "College" },
  { id: PLAYER_SORT_IDS.gp, label: "GP" },
];

import { statColumnLabels } from "@/lib/stat-labels";

export function statColumnOptions(statKeys: string[]): { id: string; label: string }[] {
  const out: { id: string; label: string }[] = [...SORT_COLUMN_OPTIONS];
  for (const k of statKeys) {
    const lab = statColumnLabels(k);
    out.push(
      { id: statSortId("wk", k), label: `Wk · ${lab.short}` },
      { id: statSortId("ytd", k), label: `YTD · ${lab.short}` },
      { id: statSortId("avg", k), label: `Avg · ${lab.short}` },
    );
  }
  return out;
}
