// ============================================================================
// Demo dataset — lets the full experience run before Supabase is provisioned.
// Mirrors the shapes returned by the real RPCs exactly, so swapping to live
// data changes zero component code. Deterministic "randomness" keeps the
// matrix stable across renders.
// ============================================================================

import type {
  Court,
  GridSlot,
  MatchmakingPost,
  Profile,
  ProfileHighlight,
  Tournament,
} from "@/lib/types";

export const DEMO_COURTS: (Court & { arenaName: string; arenaArea: string })[] = [
  {
    id: "11111111-1111-4111-8111-111111111111",
    arena_id: "a1",
    label: "Court A",
    sport: "futsal",
    side_count: 5,
    base_price: 1200,
    arenaName: "Dhuku Futsal",
    arenaArea: "Jhamsikhel",
  },
  {
    id: "22222222-2222-4222-8222-222222222222",
    arena_id: "a2",
    label: "Turf 1",
    sport: "futsal",
    side_count: 5,
    base_price: 1500,
    arenaName: "Hattiban Arena",
    arenaArea: "Lalitpur",
  },
  {
    id: "33333333-3333-4333-8333-333333333333",
    arena_id: "a3",
    label: "Court B",
    sport: "futsal",
    side_count: 7,
    base_price: 2000,
    arenaName: "Baluwatar Turf",
    arenaArea: "Kathmandu",
  },
];

/** Stable hash → the same slot is always "booked" for a given court+hour+date. */
function seeded(str: string): number {
  let h = 2166136261;
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return (h >>> 0) / 4294967295;
}

/** Mirrors get_availability_grid(): 06:00–22:00 Kathmandu, peak 17–21 + Sat. */
export function demoGrid(courtId: string, dateISO: string): GridSlot[] {
  const court = DEMO_COURTS.find((c) => c.id === courtId) ?? DEMO_COURTS[0];
  const isSaturday = new Date(`${dateISO}T12:00:00+05:45`).getDay() === 6;

  return Array.from({ length: 16 }, (_, i) => {
    const hour = 6 + i;
    const starts = new Date(`${dateISO}T${String(hour).padStart(2, "0")}:00:00+05:45`);
    const ends = new Date(starts.getTime() + 3_600_000);
    const isPeak = (hour >= 17 && hour < 21) || (isSaturday && hour >= 9);
    const price = Math.round((court.base_price * (isPeak ? 1.5 : 1)) / 50) * 50;
    return {
      starts_at: starts.toISOString(),
      ends_at: ends.toISOString(),
      price_npr: price,
      is_peak: isPeak,
      is_booked: seeded(`${courtId}-${dateISO}-${hour}`) < (isPeak ? 0.45 : 0.2),
      is_past: starts.getTime() < Date.now(),
    };
  });
}

function futureDate(daysAhead: number): string {
  const d = new Date();
  d.setDate(d.getDate() + daysAhead);
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Kathmandu" }).format(d);
}

function at(daysAhead: number, hour: number): string {
  const d = new Date();
  d.setDate(d.getDate() + daysAhead);
  const iso = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Kathmandu" }).format(d);
  return new Date(`${iso}T${String(hour).padStart(2, "0")}:00:00+05:45`).toISOString();
}

export const DEMO_POSTS: MatchmakingPost[] = [
  {
    id: "p1",
    author_id: "u1",
    booking_id: "b1",
    arena_id: "a1",
    title: "Need 2 players for tonight — court is booked, just show up",
    needed_players: 2,
    filled_players: 0,
    skill: "intermediate",
    starts_at: at(0, 19),
    status: "open",
    author: { username: "sajan_ktm", full_name: "Sajan Maharjan", avatar_url: null, community_score: 87 },
    arena: { name: "Dhuku Futsal", area: "Jhamsikhel" },
  },
  {
    id: "p2",
    author_id: "u2",
    booking_id: "b2",
    arena_id: "a2",
    title: "Keeper wanted — Saturday morning 7-a-side",
    needed_players: 1,
    filled_players: 0,
    skill: "competitive",
    starts_at: at(2, 8),
    status: "open",
    author: { username: "yeti_anish", full_name: "Anish Gurung", avatar_url: null, community_score: 142 },
    arena: { name: "Hattiban Arena", area: "Lalitpur" },
  },
  {
    id: "p3",
    author_id: "u3",
    booking_id: null,
    arena_id: "a3",
    title: "Casual pickup — anyone free tomorrow evening?",
    needed_players: 6,
    filled_players: 3,
    skill: "casual",
    starts_at: at(1, 18),
    status: "open",
    author: { username: "prerana.s", full_name: "Prerana Shrestha", avatar_url: null, community_score: 64 },
    arena: { name: "Baluwatar Turf", area: "Kathmandu" },
  },
];

export const DEMO_TOURNAMENTS: Tournament[] = [
  {
    id: "t-1111", organizer_id: "u2", arena_id: "a2",
    name: "Lalitpur Monsoon Cup", slug: "lalitpur-monsoon-cup",
    format: "group_knockout", side_count: 5, squad_cap: 10,
    max_teams: 16, entry_fee_npr: 5000, prize_pool_npr: 100000,
    prize_split: [60, 30, 10], skill: "competitive",
    description: "The valley's flagship 5-a-side. Group stage Saturday, knockouts Sunday under the floodlights.",
    rules: "FIFA futsal laws. Rolling subs. 2 × 20 min in knockouts.",
    starts_on: futureDate(18), register_by: futureDate(11), status: "open",
    team_count: 11, arena_name: "Hattiban Arena", arena_area: "Lalitpur",
    organizer_username: "yeti_anish",
  },
  {
    id: "t-2222", organizer_id: "u1", arena_id: "a1",
    name: "Jhamsikhel Midweek League", slug: "jhamsikhel-midweek-league",
    format: "league", side_count: 5, squad_cap: 8,
    max_teams: 8, entry_fee_npr: 3000, prize_pool_npr: 40000,
    prize_split: [70, 30], skill: "intermediate",
    description: "Seven match nights, every Wednesday. One champion before Dashain.",
    rules: null,
    starts_on: futureDate(9), register_by: futureDate(6), status: "open",
    team_count: 6, arena_name: "Dhuku Futsal", arena_area: "Jhamsikhel",
    organizer_username: "sajan_ktm",
  },
  {
    id: "t-3333", organizer_id: "u3", arena_id: "a3",
    name: "Baluwatar Sunrise Sevens", slug: "baluwatar-sunrise-sevens",
    format: "knockout", side_count: 7, squad_cap: 12,
    max_teams: 8, entry_fee_npr: 0, prize_pool_npr: 15000,
    prize_split: [100], skill: "casual",
    description: "Free entry, 7-a-side, one Saturday morning. Winner takes the purse and the bragging rights.",
    rules: null,
    starts_on: futureDate(4), register_by: futureDate(2), status: "full",
    team_count: 8, arena_name: "Baluwatar Turf", arena_area: "Kathmandu",
    organizer_username: "prerana.s",
  },
];

export const DEMO_PROFILE: Profile = {
  id: "demo-user",
  username: "sajan_ktm",
  full_name: "Sajan Maharjan",
  avatar_url: null,
  city: "Kathmandu",
  position: "Pivô",
  jersey_number: 9,
  preferred_foot: "right",
  bio: "Pivô with a soft first touch and a loud celebration. Jhamsikhel regular — book the 7 PM and I'll be there.",
  skill: "intermediate",
  matches_played: 48,
  matches_won: 29,
  community_score: 87,
};

export const DEMO_HIGHLIGHTS: ProfileHighlight[] = [
  {
    id: "h1", user_id: "demo-user",
    title: "Hat-trick vs Boudha Bulls",
    url: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
    created_at: new Date().toISOString(),
  },
  {
    id: "h2", user_id: "demo-user",
    title: "Toe-poke winner, Monsoon Cup semis",
    url: "https://www.tiktok.com/@sajan_ktm/video/720834",
    created_at: new Date().toISOString(),
  },
];

export const isDemoMode = () => !process.env.NEXT_PUBLIC_SUPABASE_URL;
