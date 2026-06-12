// ============================================================================
// Demo dataset — lets the full experience run before Supabase is provisioned.
// Mirrors the shapes returned by the real RPCs exactly, so swapping to live
// data changes zero component code. Deterministic "randomness" keeps the
// matrix stable across renders.
// ============================================================================

import type {
  Arena,
  ArenaPhoto,
  ArenaProfile,
  ArenaReview,
  Court,
  GridSlot,
  MatchmakingPost,
  MyBooking,
  Profile,
  ProfileHighlight,
  Team,
  TeamMember,
  Tournament,
} from "@/lib/types";

export const DEMO_COURTS: (Court & { arenaName: string; arenaArea: string; arenaSlug: string })[] = [
  {
    id: "11111111-1111-4111-8111-111111111111",
    arena_id: "a1",
    label: "Court A",
    sport: "futsal",
    side_count: 5,
    base_price: 1200,
    arenaName: "Dhuku Futsal",
    arenaArea: "Jhamsikhel",
    arenaSlug: "dhuku-futsal",
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
    arenaSlug: "hattiban-arena",
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
    arenaSlug: "baluwatar-turf",
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
    description:
      "Friendly but quick 5-a-side — rolling subs, first to 10 wins the set. Bibs and ball provided, just bring water.",
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
  account_type: "player",
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

export const DEMO_TEAMS: Team[] = [
  {
    id: "team-1111",
    name: "Jhamsikhel FC",
    tag: "JFC",
    crest_url: null,
    captain_id: "demo-user",
    home_arena: "a1",
    join_code: "JFC-X7K2",
    created_at: new Date().toISOString(),
    member_count: 4,
  },
  {
    id: "team-2222",
    name: "Boudha Bulls",
    tag: "BULLS",
    crest_url: null,
    captain_id: "u2",
    home_arena: null,
    join_code: "BUL-M3P9",
    created_at: new Date().toISOString(),
    member_count: 3,
  },
];

export const DEMO_MEMBERS: Record<string, TeamMember[]> = {
  "team-1111": [
    { team_id: "team-1111", user_id: "demo-user", role: "captain", joined_at: new Date().toISOString(), username: "sajan_ktm", full_name: "Sajan Maharjan", avatar_url: null },
    { team_id: "team-1111", user_id: "u2", role: "player", joined_at: new Date().toISOString(), username: "yeti_anish", full_name: "Anish Gurung", avatar_url: null },
    { team_id: "team-1111", user_id: "u3", role: "player", joined_at: new Date().toISOString(), username: "prerana.s", full_name: "Prerana Shrestha", avatar_url: null },
    { team_id: "team-1111", user_id: "u4", role: "player", joined_at: new Date().toISOString(), username: "ramesh_10", full_name: "Ramesh Tamang", avatar_url: null },
  ],
  "team-2222": [
    { team_id: "team-2222", user_id: "u2", role: "captain", joined_at: new Date().toISOString(), username: "yeti_anish", full_name: "Anish Gurung", avatar_url: null },
    { team_id: "team-2222", user_id: "u5", role: "player", joined_at: new Date().toISOString(), username: "kiran.b", full_name: "Kiran Basnet", avatar_url: null },
    { team_id: "team-2222", user_id: "u6", role: "player", joined_at: new Date().toISOString(), username: "suman_gk", full_name: "Suman Rai", avatar_url: null },
  ],
};

export const DEMO_MY_BOOKINGS: MyBooking[] = [
  {
    id: "bk-1111", court_id: DEMO_COURTS[0].id, user_id: "demo-user", team_id: null,
    slot: `[${at(2, 19)},${at(2, 20)})`, price_npr: 1800, is_peak: true,
    status: "confirmed", open_to_join: false, created_at: new Date().toISOString(),
    court_label: DEMO_COURTS[0].label, arena_name: DEMO_COURTS[0].arenaName, arena_area: DEMO_COURTS[0].arenaArea,
    starts_at: at(2, 19), ends_at: at(2, 20),
  },
  {
    id: "bk-2222", court_id: DEMO_COURTS[1].id, user_id: "demo-user", team_id: null,
    slot: `[${at(5, 17)},${at(5, 19)})`, price_npr: 4500, is_peak: true,
    status: "pending", open_to_join: false, created_at: new Date().toISOString(),
    court_label: DEMO_COURTS[1].label, arena_name: DEMO_COURTS[1].arenaName, arena_area: DEMO_COURTS[1].arenaArea,
    starts_at: at(5, 17), ends_at: at(5, 19),
  },
  {
    id: "bk-3333", court_id: DEMO_COURTS[0].id, user_id: "demo-user", team_id: "team-1111",
    slot: `[${at(-3, 18)},${at(-3, 19)})`, price_npr: 1800, is_peak: true,
    status: "completed", open_to_join: true, created_at: new Date(Date.now() - 3 * 86400000).toISOString(),
    court_label: DEMO_COURTS[0].label, arena_name: DEMO_COURTS[0].arenaName, arena_area: DEMO_COURTS[0].arenaArea,
    starts_at: at(-3, 18), ends_at: at(-3, 19),
  },
  {
    id: "bk-4444", court_id: DEMO_COURTS[2].id, user_id: "demo-user", team_id: null,
    slot: `[${at(-7, 10)},${at(-7, 11)})`, price_npr: 2000, is_peak: false,
    status: "cancelled", open_to_join: false, created_at: new Date(Date.now() - 7 * 86400000).toISOString(),
    court_label: DEMO_COURTS[2].label, arena_name: DEMO_COURTS[2].arenaName, arena_area: DEMO_COURTS[2].arenaArea,
    starts_at: at(-7, 10), ends_at: at(-7, 11),
  },
];

export const DEMO_ARENAS: Arena[] = [
  {
    id: "a1", owner_id: "o1", name: "Dhuku Futsal", slug: "dhuku-futsal",
    area: "Jhamsikhel", city: "Lalitpur",
    description:
      "Jhamsikhel's neighbourhood pitch — fresh turf laid in 2024, proper floodlights, and a chiya stand that stays open until the last whistle.",
    amenities: ["Floodlights", "Changing room", "Parking", "Chiya stand", "Bibs & balls"],
    opens_at: "06:00:00", closes_at: "22:00:00",
    rating: 4.6, cover_url: null, is_active: true,
  },
  {
    id: "a2", owner_id: "o2", name: "Hattiban Arena", slug: "hattiban-arena",
    area: "Hattiban", city: "Lalitpur",
    description:
      "The valley's biggest 7-a-side turf, ringed by pine forest. Tournament-grade pitch with seating for fifty.",
    amenities: ["Floodlights", "Spectator stand", "Showers", "Cafeteria", "First aid"],
    opens_at: "06:00:00", closes_at: "21:00:00",
    rating: 4.2, cover_url: null, is_active: true,
  },
  {
    id: "a3", owner_id: "o3", name: "Baluwatar Turf", slug: "baluwatar-turf",
    area: "Baluwatar", city: "Kathmandu",
    description:
      "Rooftop turf in the heart of town. Quick to reach, quicker to fill — book the sunrise slots before the regulars do.",
    amenities: ["Rooftop view", "Floodlights", "Drinking water", "Lockers"],
    opens_at: "06:00:00", closes_at: "22:00:00",
    rating: 3.9, cover_url: null, is_active: true,
  },
];

/** Inline SVG placeholder so demo galleries render without external images. */
function demoPhotoUrl(label: string, hue: number): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(${hue},32%,24%)"/><stop offset="1" stop-color="hsl(${hue},38%,12%)"/></linearGradient></defs><rect width="800" height="600" fill="url(#g)"/><circle cx="400" cy="300" r="90" fill="none" stroke="rgba(255,255,255,0.25)" stroke-width="3"/><line x1="400" y1="0" x2="400" y2="600" stroke="rgba(255,255,255,0.25)" stroke-width="3"/><text x="400" y="560" text-anchor="middle" font-family="monospace" font-size="24" fill="rgba(255,255,255,0.55)">${label}</text></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

export const DEMO_ARENA_PHOTOS: Record<string, ArenaPhoto[]> = {
  a1: [
    { id: "ph1", arena_id: "a1", url: demoPhotoUrl("DHUKU · MAIN COURT", 140), caption: "Court A under the new floodlights", created_at: new Date(Date.now() - 5 * 86400000).toISOString() },
    { id: "ph2", arena_id: "a1", url: demoPhotoUrl("DHUKU · TURF CLOSE-UP", 95), caption: "Fresh turf, laid March 2024", created_at: new Date(Date.now() - 12 * 86400000).toISOString() },
    { id: "ph3", arena_id: "a1", url: demoPhotoUrl("DHUKU · CHIYA STAND", 30), caption: null, created_at: new Date(Date.now() - 30 * 86400000).toISOString() },
  ],
  a2: [
    { id: "ph4", arena_id: "a2", url: demoPhotoUrl("HATTIBAN · FULL PITCH", 160), caption: "7-a-side pitch from the stand", created_at: new Date(Date.now() - 3 * 86400000).toISOString() },
    { id: "ph5", arena_id: "a2", url: demoPhotoUrl("HATTIBAN · PINES", 120), caption: "Pine forest behind the far goal", created_at: new Date(Date.now() - 20 * 86400000).toISOString() },
  ],
  a3: [
    { id: "ph6", arena_id: "a3", url: demoPhotoUrl("BALUWATAR · ROOFTOP", 200), caption: "Sunrise slot, city behind", created_at: new Date(Date.now() - 8 * 86400000).toISOString() },
  ],
};

export const DEMO_ARENA_REVIEWS: Record<string, ArenaReview[]> = {
  a1: [
    { id: "rv1", arena_id: "a1", user_id: "u2", rating: 5, comment: "Best turf in Jhamsikhel right now. Lights are bright enough for late games and the bounce is true.", created_at: new Date(Date.now() - 2 * 86400000).toISOString(), author: { username: "yeti_anish", full_name: "Anish Gurung", avatar_url: null } },
    { id: "rv2", arena_id: "a1", user_id: "u3", rating: 4, comment: "Great pitch, parking gets tight after 6.", created_at: new Date(Date.now() - 9 * 86400000).toISOString(), author: { username: "prerana.s", full_name: "Prerana Shrestha", avatar_url: null } },
    { id: "rv3", arena_id: "a1", user_id: "u4", rating: 5, comment: null, created_at: new Date(Date.now() - 15 * 86400000).toISOString(), author: { username: "ramesh_10", full_name: "Ramesh Tamang", avatar_url: null } },
  ],
  a2: [
    { id: "rv4", arena_id: "a2", user_id: "u5", rating: 4, comment: "Worth the drive for the 7-a-side pitch. Showers actually have hot water.", created_at: new Date(Date.now() - 4 * 86400000).toISOString(), author: { username: "kiran.b", full_name: "Kiran Basnet", avatar_url: null } },
    { id: "rv5", arena_id: "a2", user_id: "u6", rating: 4, comment: "Tournament weekends get crowded, weekday mornings are perfect.", created_at: new Date(Date.now() - 11 * 86400000).toISOString(), author: { username: "suman_gk", full_name: "Suman Rai", avatar_url: null } },
  ],
  a3: [
    { id: "rv6", arena_id: "a3", user_id: "u2", rating: 4, comment: "Rooftop view is unbeatable at sunrise.", created_at: new Date(Date.now() - 6 * 86400000).toISOString(), author: { username: "yeti_anish", full_name: "Anish Gurung", avatar_url: null } },
  ],
};

export function demoArenaProfile(slug: string): ArenaProfile | null {
  const arena = DEMO_ARENAS.find((a) => a.slug === slug);
  if (!arena) return null;
  const reviews = DEMO_ARENA_REVIEWS[arena.id] ?? [];
  return {
    arena,
    courts: DEMO_COURTS.filter((c) => c.arena_id === arena.id),
    photos: DEMO_ARENA_PHOTOS[arena.id] ?? [],
    reviews,
    myReview: null,
    reviewCount: reviews.length,
  };
}

export const isDemoMode = () => !process.env.NEXT_PUBLIC_SUPABASE_URL;
