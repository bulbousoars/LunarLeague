/**
 * Provider season year for YTD aggregates (matches API schedule.SeasonForSport).
 * NFL Jan–Feb still uses the prior season year.
 */
export function defaultAggregateSeason(sportCode: string): number {
  const sport = sportCode.toLowerCase();
  const y = new Date().getFullYear();
  const m = new Date().getMonth() + 1;
  if (sport === "nfl" && m <= 2) {
    return y - 1;
  }
  return y;
}

/** Roster-style positions shown in browse / draft filters by sport code. */
export function positionsForSport(sportCode: string): string[] {
  switch (sportCode.toLowerCase()) {
    case "nba":
      return ["PG", "SG", "SF", "PF", "C", "G", "F"];
    case "mlb":
      return ["P", "SP", "RP", "C", "1B", "2B", "3B", "SS", "OF", "DH"];
    default:
      return ["QB", "RB", "WR", "TE", "K", "DEF"];
  }
}
