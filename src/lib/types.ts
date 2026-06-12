// ============================================================================
// Domain types — mirrors supabase/schema.sql
// ============================================================================

export type BookingStatus = "pending" | "confirmed" | "completed" | "cancelled" | "no_show";
export type SkillTier = "casual" | "intermediate" | "competitive" | "semi_pro";
export type MatchmakingStatus = "open" | "filled" | "expired" | "cancelled";
export type PaymentProvider = "esewa" | "khalti" | "cash";

export type FutsalPosition = "Goleiro" | "Fixo" | "Ala" | "Pivô" | "Universal";

export type AccountType = "player" | "futsal_owner";

export interface Profile {
  id: string;
  username: string;
  full_name: string;
  account_type: AccountType;
  avatar_url: string | null;
  city: string;
  position: FutsalPosition | null;
  jersey_number: number | null;
  preferred_foot: "left" | "right" | "both" | null;
  bio: string | null;
  skill: SkillTier;
  matches_played: number;
  matches_won: number;
  community_score: number;
}

export interface ProfileHighlight {
  id: string;
  user_id: string;
  title: string;
  url: string;
  created_at: string;
}

export interface Arena {
  id: string;
  owner_id: string;
  name: string;
  slug: string;
  area: string;
  city: string;
  description: string | null;
  amenities: string[];
  opens_at: string; // "06:00:00"
  closes_at: string;
  rating: number | null;
  cover_url: string | null;
  is_active: boolean;
}

export interface Court {
  id: string;
  arena_id: string;
  label: string;
  sport: string;
  side_count: number;
  base_price: number;
}

/** One cell of the booking matrix — returned by get_availability_grid(). */
export interface GridSlot {
  starts_at: string; // ISO timestamptz
  ends_at: string;
  price_npr: number;
  is_peak: boolean;
  is_booked: boolean;
  is_past: boolean;
}

export interface Booking {
  id: string;
  court_id: string;
  user_id: string;
  team_id: string | null;
  slot: string; // tstzrange text form
  price_npr: number;
  is_peak: boolean;
  status: BookingStatus;
  open_to_join: boolean;
  created_at: string;
}

export interface MatchmakingPost {
  id: string;
  author_id: string;
  booking_id: string | null;
  arena_id: string | null;
  title: string;
  needed_players: number;
  filled_players: number;
  skill: SkillTier;
  starts_at: string;
  status: MatchmakingStatus;
  // hydrated joins for the feed
  author?: Pick<Profile, "username" | "full_name" | "avatar_url" | "community_score">;
  arena?: Pick<Arena, "name" | "area">;
}

/** A player's request to join a call — pending (accepted=false) or accepted. */
export interface MatchmakingResponse {
  user_id: string;
  message: string | null;
  accepted: boolean;
  created_at: string;
  responder?: Pick<Profile, "username" | "full_name" | "avatar_url" | "community_score"> | null;
}

/** One of the author's own calls, with its join requests — for the manage view. */
export interface MatchmakingCall extends MatchmakingPost {
  responses: MatchmakingResponse[];
}

export type TournamentFormat = "knockout" | "league" | "group_knockout";
export type TournamentStatus = "open" | "full" | "ongoing" | "completed" | "cancelled";

/** Row from the tournament_board view — listing-ready with counts joined. */
export interface Tournament {
  id: string;
  organizer_id: string;
  arena_id: string | null;
  name: string;
  slug: string;
  format: TournamentFormat;
  side_count: number;
  squad_cap: number;
  max_teams: number;
  entry_fee_npr: number;
  prize_pool_npr: number;
  prize_split: number[];
  skill: SkillTier;
  description: string | null;
  rules: string | null;
  starts_on: string;   // "YYYY-MM-DD"
  register_by: string;
  status: TournamentStatus;
  team_count: number;
  arena_name: string | null;
  arena_area: string | null;
  organizer_username: string | null;
}

/** Enriched booking with court/arena context — used by the My Bookings page. */
export interface MyBooking extends Booking {
  court_label: string;
  arena_name: string;
  arena_area: string;
  starts_at: string;   // parsed from tstzrange slot
  ends_at: string;
}

export interface Team {
  id: string;
  name: string;
  tag: string;          // 2-5 uppercase alphanumeric
  crest_url: string | null;
  captain_id: string;
  home_arena: string | null;
  join_code: string;
  created_at: string;
  member_count: number;
}

export interface TeamMember {
  team_id: string;
  user_id: string;
  role: "captain" | "player";
  joined_at: string;
  // joined profile fields
  username: string;
  full_name: string;
  avatar_url: string | null;
}

/** Discriminated result type used by every server action. */
export type ActionResult<T> =
  | { ok: true; data: T }
  | { ok: false; error: string; code?: string };
