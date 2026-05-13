/**
 * Short table header text + longer hover/tooltip text for canonical stat keys
 * (snake_case from API / statsnorm).
 */

export type StatColumnLabels = { short: string; long: string };

const NFL: Record<string, StatColumnLabels> = {
  pass_yd: { short: "Pass Yds", long: "Passing Yards" },
  pass_td: { short: "Pass TD", long: "Passing Touchdowns" },
  pass_int: { short: "Pass Int", long: "Passing Interceptions" },
  pass_2pt: { short: "Pass 2Pt", long: "Passing Two-Point Conversions" },
  rush_yd: { short: "Rush Yds", long: "Rushing Yards" },
  rush_td: { short: "Rush TD", long: "Rushing Touchdowns" },
  rush_2pt: { short: "Rush 2Pt", long: "Rushing Two-Point Conversions" },
  rec: { short: "Rec", long: "Receptions" },
  rec_yd: { short: "Rec Yds", long: "Receiving Yards" },
  rec_td: { short: "Rec TD", long: "Receiving Touchdowns" },
  rec_2pt: { short: "Rec 2Pt", long: "Receiving Two-Point Conversions" },
  fum_lost: { short: "Fum Lost", long: "Fumbles Lost" },
  def_int: { short: "Def Int", long: "Defensive Interceptions" },
  def_fr: { short: "Def FR", long: "Defensive Fumble Recoveries" },
  def_sack: { short: "Def Sack", long: "Defensive Sacks" },
  def_td: { short: "Def TD", long: "Defensive Touchdowns" },
  def_safe: { short: "Def Safe", long: "Defensive Safeties" },
  def_block_kick: { short: "Def Block Kick", long: "Defensive Blocked Kick" },
  st_td: { short: "ST TD", long: "Special Teams Touchdowns" },
  fgm_0_19: { short: "FGM 0–19", long: "Field Goals Made (0–19 yd)" },
  fgm_20_29: { short: "FGM 20–29", long: "Field Goals Made (20–29 yd)" },
  fgm_30_39: { short: "FGM 30–39", long: "Field Goals Made (30–39 yd)" },
  fgm_40_49: { short: "FGM 40–49", long: "Field Goals Made (40–49 yd)" },
  fgm_50p: { short: "FGM 50+", long: "Field Goals Made (50+ yd)" },
  fgmiss: { short: "FG Miss", long: "Field Goals Missed" },
  xpm: { short: "XPM", long: "Extra Points Made" },
  xpmiss: { short: "XP Miss", long: "Extra Points Missed" },
};

const NBA: Record<string, StatColumnLabels> = {
  pts: { short: "Pts", long: "Points" },
  reb: { short: "Reb", long: "Rebounds" },
  ast: { short: "Ast", long: "Assists" },
  stl: { short: "Stl", long: "Steals" },
  blk: { short: "Blk", long: "Blocks" },
  to: { short: "TO", long: "Turnovers" },
  fg3m: { short: "3PM", long: "Three-Pointers Made" },
};

const MLB: Record<string, StatColumnLabels> = {
  hit: { short: "Hits", long: "Hits" },
  run: { short: "Runs", long: "Runs" },
  rbi: { short: "RBI", long: "Runs Batted In" },
  hr: { short: "HR", long: "Home Runs" },
  sb: { short: "SB", long: "Stolen Bases" },
  bb: { short: "BB", long: "Walks" },
  k: { short: "K", long: "Strikeouts (Batting)" },
  win: { short: "W", long: "Wins (Pitching)" },
  loss: { short: "L", long: "Losses (Pitching)" },
  qs: { short: "QS", long: "Quality Starts" },
  sv: { short: "SV", long: "Saves" },
  ip: { short: "IP", long: "Innings Pitched" },
  er: { short: "ER", long: "Earned Runs" },
  k_p: { short: "K", long: "Strikeouts (Pitching)" },
};

const ALL: Record<string, StatColumnLabels> = { ...NFL, ...NBA, ...MLB };

/** Token tweaks when there is no full-key preset (unknown / provider keys). */
const SHORT_WORD: Record<string, string> = {
  yd: "Yds",
  yds: "Yds",
  td: "TD",
  int: "Int",
  pts: "Pts",
  ast: "Ast",
  reb: "Reb",
  stl: "Stl",
  blk: "Blk",
  def: "Def",
  pass: "Pass",
  rush: "Rush",
  rec: "Rec",
  st: "ST",
  safe: "Safe",
  sack: "Sack",
  fum: "Fum",
  fgmiss: "FG Miss",
  xpmiss: "XP Miss",
  xpm: "XPM",
  fgm: "FGM",
  fg3m: "3PM",
  to: "TO",
  hr: "HR",
  rbi: "RBI",
  sb: "SB",
  bb: "BB",
  qs: "QS",
  sv: "SV",
  ip: "IP",
  er: "ER",
  fr: "FR",
  k: "K",
  win: "W",
  loss: "L",
  run: "Runs",
  hit: "Hits",
  bonus: "Bonus",
  miss: "Miss",
  "2pt": "2Pt",
};

const LONG_WORD: Record<string, string> = {
  yd: "yards",
  yds: "yards",
  td: "touchdowns",
  int: "interceptions",
  pts: "points",
  ast: "assists",
  reb: "rebounds",
  stl: "steals",
  blk: "blocks",
  def: "defensive",
  pass: "passing",
  rush: "rushing",
  rec: "receiving",
  st: "special teams",
  safe: "safeties",
  sack: "sacks",
  fum: "fumbles",
  lost: "lost",
  kick: "kick",
  block: "blocked",
  bonus: "bonus",
  miss: "missed",
  fgmiss: "field goals missed",
  xpmiss: "extra points missed",
  xpm: "extra points made",
  fgm: "field goals made",
  fg3m: "three-pointers made",
  to: "turnovers",
  hr: "home runs",
  rbi: "runs batted in",
  sb: "stolen bases",
  bb: "walks",
  k: "strikeouts",
  k_p: "pitching strikeouts",
  qs: "quality starts",
  sv: "saves",
  ip: "innings pitched",
  er: "earned runs",
  fr: "fumble recoveries",
  win: "wins",
  loss: "losses",
  run: "runs",
  hit: "hits",
  att: "attempts",
  cmp: "completions",
  tgt: "targets",
  "2pt": "two-point conversions",
};

function titleWord(w: string): string {
  const lower = w.toLowerCase();
  if (/^\d+$/.test(w)) return w;
  return w.length <= 2 ? w.toUpperCase() : lower.charAt(0).toUpperCase() + lower.slice(1);
}

function fallbackFromSnake(key: string): StatColumnLabels {
  const parts = key.split("_").filter(Boolean);
  if (!parts.length) return { short: key, long: key };
  const short = parts
    .map((p) => SHORT_WORD[p.toLowerCase()] ?? titleWord(p))
    .join(" ");
  const long = parts
    .map((p) => LONG_WORD[p.toLowerCase()] ?? titleWord(p))
    .join(" ");
  const longSentence =
    long.length > 0 ? long.charAt(0).toUpperCase() + long.slice(1) : long;
  return { short, long: longSentence };
}

function lookupPreset(key: string): StatColumnLabels | null {
  if (ALL[key]) return ALL[key];
  return null;
}

/** bonus_<base>_<threshold> e.g. bonus_pass_yd_300 */
function parseBonusKey(key: string): StatColumnLabels | null {
  if (!key.startsWith("bonus_")) return null;
  const rest = key.slice("bonus_".length);
  const m = rest.match(/_(\d+)$/);
  if (!m) return null;
  const thr = m[1];
  const baseKey = rest.slice(0, rest.length - m[0].length);
  if (!baseKey) return null;
  const base = statColumnLabels(baseKey);
  return {
    short: `${base.short} ${thr}+`,
    long: `Bonus: ${base.long} (${thr}+)`,
  };
}

/**
 * Human labels for a canonical stat key.
 * `short` fits tight column headers; `long` is for `title` / tooltips.
 */
export function statColumnLabels(key: string): StatColumnLabels {
  const preset = lookupPreset(key);
  if (preset) return preset;
  const bonus = parseBonusKey(key);
  if (bonus) return bonus;
  return fallbackFromSnake(key);
}
