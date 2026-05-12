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
