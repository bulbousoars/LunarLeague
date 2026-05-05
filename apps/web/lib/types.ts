// Shared TypeScript shapes for what the Go API returns. Hand-written for now;
// can be regenerated from openapi later.

export type User = {
  id: string;
  email: string;
  display_name: string;
  avatar_url?: string;
  timezone: string;
  is_admin: boolean;
};

export type Sport = {
  id: number;
  code: string;
  name: string;
  season_type: string;
};

export type League = {
  id: string;
  sport_id: number;
  sport_code: string;
  name: string;
  slug: string;
  season: number;
  league_format: "redraft" | "keeper" | "dynasty";
  draft_format: "snake" | "auction";
  team_count: number;
  invite_code?: string;
  status:
    | "setup"
    | "drafting"
    | "in_season"
    | "playoffs"
    | "complete";
  created_by: string;
};

export type Team = {
  id: string;
  league_id: string;
  owner_id: string | null;
  name: string;
  abbreviation: string;
  logo_url?: string | null;
  motto?: string | null;
  waiver_position?: number | null;
  waiver_budget?: number | null;
  auction_budget?: number | null;
  record_wins: number;
  record_losses: number;
  record_ties: number;
  points_for: string;
  points_against: string;
};

export type Player = {
  id: string;
  full_name: string;
  position: string | null;
  eligible_positions: string[];
  nfl_team: string | null;
  status: string | null;
  injury_status: string | null;
  headshot_url: string | null;
};

export type RosterEntry = {
  id: string;
  player_id: string;
  player_name: string;
  position: string | null;
  nfl_team: string | null;
  slot: string;
  acquired_via: string;
  acquired_at: string;
  keeper_round_cost?: number | null;
};

export type Settings = {
  roster_slots: Record<string, number>;
  waiver_type: "faab" | "rolling" | "reverse_standings";
  waiver_budget: number;
  waiver_run_dow: number;
  waiver_run_hour: number;
  trade_deadline_week: number | null;
  playoff_start_week: number;
  playoff_team_count: number;
  keeper_count: number;
  auction_budget?: number | null;
  schedule_type: "h2h_points" | "h2h_categories" | "rotisserie";
  public_visible: boolean;
};

export type DraftPick = {
  id: string;
  pick_no: number;
  round: number;
  team_id: string;
  player_id: string | null;
  price: number | null;
  is_keeper: boolean;
  is_autopick: boolean;
  picked_at: string | null;
};

export type OnTheClock = {
  team_id: string;
  pick_no: number;
  round: number;
  deadline: string; // ISO
};

export type Draft = {
  id: string;
  league_id: string;
  type: "snake" | "auction";
  status: "pending" | "in_progress" | "paused" | "complete";
  rounds: number;
  pick_seconds: number;
  nomination_seconds: number;
  bidding_seconds: number;
  starts_at?: string | null;
  started_at?: string | null;
  completed_at?: string | null;
  draft_order: string[];
  config: Record<string, unknown>;
  picks: DraftPick[];
  on_the_clock?: OnTheClock | null;
};

export type Matchup = {
  id: string;
  week: number;
  season: number;
  home_team_id: string;
  away_team_id: string;
  home_score: string;
  away_score: string;
  home_projected?: string | null;
  away_projected?: string | null;
  is_playoff: boolean;
  is_consolation: boolean;
  is_final: boolean;
};

export type Standing = {
  team_id: string;
  name: string;
  abbreviation: string;
  wins: number;
  losses: number;
  ties: number;
  points_for: string;
  points_against: string;
};

export type ChatMessage = {
  id: string;
  channel: string;
  user_id: string | null;
  display_name?: string | null;
  body: string;
  refs: Record<string, unknown>;
  created_at: string;
};

export type Notification = {
  id: string;
  type: string;
  title: string;
  body?: string;
  deep_link?: string;
  league_id?: string;
  read_at?: string | null;
  created_at: string;
};
