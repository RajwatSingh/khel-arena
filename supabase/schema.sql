-- ============================================================================
-- KHEL ARENA — Futsal & Sports Arena Booking Platform (Nepal)
-- Supabase / PostgreSQL schema
--
-- Design decisions:
--   1. Bookings store a `tstzrange` and are guarded by a GiST EXCLUDE
--      constraint. Double-booking the same court/hour is therefore
--      IMPOSSIBLE at the database level, regardless of application bugs,
--      retries, or concurrent requests.
--   2. TimeSlots are *derived*, not stored. Materialising every hour of
--      every court forever creates millions of dead rows. Instead,
--      `get_availability_grid()` projects the daily grid from arena
--      operating hours + pricing_rules + existing bookings. The grid the
--      frontend renders IS the TimeSlots entity — computed, always fresh.
--   3. Pricing is rule-based (day-of-week + hour window) so arenas can
--      model peak / off-peak / weekend pricing without code changes.
-- ============================================================================

create extension if not exists pgcrypto;
create extension if not exists btree_gist;   -- required for the EXCLUDE constraint

-- ----------------------------------------------------------------------------
-- ENUMS
-- ----------------------------------------------------------------------------
create type booking_status     as enum ('pending', 'confirmed', 'completed', 'cancelled', 'no_show');
create type payment_provider   as enum ('esewa', 'khalti', 'cash');
create type payment_status     as enum ('initiated', 'verified', 'failed', 'refunded');
create type sport_type         as enum ('futsal', 'basketball', 'badminton', 'cricket_net', 'tennis');
create type matchmaking_status as enum ('open', 'filled', 'expired', 'cancelled');
create type skill_tier         as enum ('casual', 'intermediate', 'competitive', 'semi_pro');

-- ----------------------------------------------------------------------------
-- USERS (extends Supabase auth.users)
-- ----------------------------------------------------------------------------
create table public.profiles (
  id              uuid primary key references auth.users (id) on delete cascade,
  username        text unique not null check (username ~ '^[a-z0-9_]{3,24}$'),
  full_name       text not null,
  avatar_url      text,
  phone           text check (phone ~ '^(98|97)\d{8}$'),   -- Nepali mobile format
  city            text default 'Kathmandu',
  position        text,                                    -- 'Pivot', 'Winger', 'Goalkeeper'...
  skill           skill_tier default 'casual',
  bio             text check (char_length(bio) <= 280),
  matches_played  int  not null default 0,
  matches_won     int  not null default 0,
  community_score int  not null default 0,                 -- reputation: shows up, plays fair
  created_at      timestamptz not null default now()
);

-- ----------------------------------------------------------------------------
-- ARENAS & COURTS
-- An arena (e.g. "Dhuku Futsal, Jhamsikhel") owns one or more courts.
-- ----------------------------------------------------------------------------
create table public.arenas (
  id           uuid primary key default gen_random_uuid(),
  owner_id     uuid not null references public.profiles (id),
  name         text not null,
  slug         text unique not null,
  area         text not null,                              -- 'Jhamsikhel', 'Baluwatar'...
  city         text not null default 'Kathmandu',
  lat          double precision,
  lng          double precision,
  description  text,
  cover_url    text,
  amenities    text[] not null default '{}',               -- {'parking','showers','floodlights'}
  opens_at     time not null default '06:00',
  closes_at    time not null default '22:00',
  rating       numeric(2,1) check (rating between 0 and 5),
  is_active    boolean not null default true,
  created_at   timestamptz not null default now()
);

create table public.courts (
  id           uuid primary key default gen_random_uuid(),
  arena_id     uuid not null references public.arenas (id) on delete cascade,
  label        text not null,                               -- 'Court A', 'Turf 2'
  sport        sport_type not null default 'futsal',
  surface      text not null default 'artificial_turf',
  side_count   int  not null default 5 check (side_count between 3 and 11),
  base_price   int  not null check (base_price > 0),        -- NPR per hour, fallback price
  is_active    boolean not null default true,
  unique (arena_id, label)
);

-- ----------------------------------------------------------------------------
-- PRICING RULES — peak / off-peak windows
-- The highest-priority matching rule wins; base_price is the fallback.
-- ----------------------------------------------------------------------------
create table public.pricing_rules (
  id          uuid primary key default gen_random_uuid(),
  court_id    uuid not null references public.courts (id) on delete cascade,
  label       text not null,                                -- 'Evening Peak', 'Saturday Premium'
  days        int[] not null,                               -- ISO dow: 1=Mon … 7=Sun
  start_hour  int  not null check (start_hour between 0 and 23),
  end_hour    int  not null check (end_hour between 1 and 24 and end_hour > start_hour),
  price_npr   int  not null check (price_npr > 0),
  is_peak     boolean not null default false,
  priority    int  not null default 1                       -- higher wins on overlap
);

-- ----------------------------------------------------------------------------
-- BOOKINGS — the race-proof core
-- ----------------------------------------------------------------------------
create table public.bookings (
  id            uuid primary key default gen_random_uuid(),
  court_id      uuid not null references public.courts (id),
  user_id       uuid not null references public.profiles (id),
  team_id       uuid,                                        -- FK added after teams table
  slot          tstzrange not null,                          -- [start, end)
  price_npr     int  not null check (price_npr >= 0),
  is_peak       boolean not null default false,
  status        booking_status not null default 'pending',
  open_to_join  boolean not null default false,              -- mirrors matchmaking state
  note          text,
  created_at    timestamptz not null default now(),

  constraint slot_is_bounded   check (not lower_inf(slot) and not upper_inf(slot)),
  constraint slot_min_duration check (upper(slot) - lower(slot) >= interval '30 minutes'),

  -- ★ THE GUARANTEE: two live bookings can never overlap on the same court.
  --   Cancelled bookings are excluded so a freed slot is instantly rebookable.
  constraint no_double_booking exclude using gist (
    court_id with =,
    slot     with &&
  ) where (status <> 'cancelled')
);

create index bookings_court_day_idx on public.bookings using gist (court_id, slot);
create index bookings_user_idx      on public.bookings (user_id, created_at desc);

-- ----------------------------------------------------------------------------
-- PAYMENTS — clean hooks for eSewa / Khalti verification flows
-- ----------------------------------------------------------------------------
create table public.payments (
  id               uuid primary key default gen_random_uuid(),
  booking_id       uuid not null references public.bookings (id) on delete cascade,
  provider         payment_provider not null,
  amount_npr       int not null check (amount_npr > 0),
  status           payment_status not null default 'initiated',
  transaction_uuid text unique not null,                     -- ours; sent to the gateway
  provider_ref     text,                                     -- gateway txn id (pidx / refId)
  raw_response     jsonb,
  created_at       timestamptz not null default now(),
  verified_at      timestamptz
);

-- ----------------------------------------------------------------------------
-- TEAMS & MEMBERSHIP
-- ----------------------------------------------------------------------------
create table public.teams (
  id          uuid primary key default gen_random_uuid(),
  name        text unique not null,
  tag         text unique not null check (tag ~ '^[A-Z0-9]{2,5}$'),  -- 'KTM', 'YETI'
  crest_url   text,
  captain_id  uuid not null references public.profiles (id),
  home_arena  uuid references public.arenas (id),
  created_at  timestamptz not null default now()
);

create table public.team_members (
  team_id    uuid not null references public.teams (id) on delete cascade,
  user_id    uuid not null references public.profiles (id) on delete cascade,
  role       text not null default 'player' check (role in ('captain','player')),
  joined_at  timestamptz not null default now(),
  primary key (team_id, user_id)
);

alter table public.bookings
  add constraint bookings_team_fk foreign key (team_id) references public.teams (id);

-- ----------------------------------------------------------------------------
-- MATCHES & LEADERBOARD
-- A match is a recorded result between two teams (usually tied to a booking).
-- ----------------------------------------------------------------------------
create table public.matches (
  id          uuid primary key default gen_random_uuid(),
  booking_id  uuid references public.bookings (id),
  home_team   uuid not null references public.teams (id),
  away_team   uuid not null references public.teams (id),
  home_score  int not null default 0 check (home_score >= 0),
  away_score  int not null default 0 check (away_score >= 0),
  played_at   timestamptz not null default now(),
  verified    boolean not null default false,                -- both captains confirmed
  check (home_team <> away_team)
);

-- Standings: 3 pts win / 1 pt draw, goal difference as tiebreaker.
create or replace view public.team_standings as
with results as (
  select home_team as team_id,
         case when home_score > away_score then 1 else 0 end as won,
         case when home_score = away_score then 1 else 0 end as drawn,
         case when home_score < away_score then 1 else 0 end as lost,
         home_score as gf, away_score as ga
  from public.matches where verified
  union all
  select away_team,
         case when away_score > home_score then 1 else 0 end,
         case when away_score = home_score then 1 else 0 end,
         case when away_score < home_score then 1 else 0 end,
         away_score, home_score
  from public.matches where verified
)
select
  t.id as team_id,
  t.name,
  t.tag,
  t.crest_url,
  count(r.*)::int                        as played,
  coalesce(sum(r.won), 0)::int           as won,
  coalesce(sum(r.drawn), 0)::int         as drawn,
  coalesce(sum(r.lost), 0)::int          as lost,
  coalesce(sum(r.gf), 0)::int            as goals_for,
  coalesce(sum(r.ga), 0)::int            as goals_against,
  coalesce(sum(r.gf) - sum(r.ga), 0)::int as goal_diff,
  coalesce(sum(r.won) * 3 + sum(r.drawn), 0)::int as points,
  rank() over (order by coalesce(sum(r.won) * 3 + sum(r.drawn), 0) desc,
               coalesce(sum(r.gf) - sum(r.ga), 0) desc) as rank
from public.teams t
left join results r on r.team_id = t.id
group by t.id;

-- ----------------------------------------------------------------------------
-- COMMUNITY MATCHMAKING — the Find-a-Player board
-- A post either opens an existing booking to the public ("we have the court,
-- need 2 more") or calls for a pickup game with no booking yet.
-- ----------------------------------------------------------------------------
create table public.matchmaking_posts (
  id              uuid primary key default gen_random_uuid(),
  author_id       uuid not null references public.profiles (id),
  booking_id      uuid references public.bookings (id) on delete cascade,  -- null = open call
  arena_id        uuid references public.arenas (id),
  title           text not null check (char_length(title) <= 120),
  needed_players  int not null check (needed_players between 1 and 10),
  filled_players  int not null default 0 check (filled_players >= 0),
  skill           skill_tier not null default 'casual',
  starts_at       timestamptz not null,
  status          matchmaking_status not null default 'open',
  created_at      timestamptz not null default now(),
  constraint not_overfilled check (filled_players <= needed_players)
);

create table public.matchmaking_responses (
  post_id     uuid not null references public.matchmaking_posts (id) on delete cascade,
  user_id     uuid not null references public.profiles (id) on delete cascade,
  message     text check (char_length(message) <= 200),
  accepted    boolean not null default false,
  created_at  timestamptz not null default now(),
  primary key (post_id, user_id)
);

create index matchmaking_feed_idx
  on public.matchmaking_posts (status, starts_at)
  where status = 'open';

-- ----------------------------------------------------------------------------
-- ROW LEVEL SECURITY (MVP policies)
-- ----------------------------------------------------------------------------
alter table public.profiles           enable row level security;
alter table public.arenas             enable row level security;
alter table public.courts             enable row level security;
alter table public.pricing_rules      enable row level security;
alter table public.bookings           enable row level security;
alter table public.payments           enable row level security;
alter table public.teams              enable row level security;
alter table public.team_members       enable row level security;
alter table public.matches            enable row level security;
alter table public.matchmaking_posts  enable row level security;
alter table public.matchmaking_responses enable row level security;

-- Public catalogue data is readable by everyone.
create policy "read arenas"        on public.arenas            for select using (true);
create policy "read courts"        on public.courts            for select using (true);
create policy "read pricing"       on public.pricing_rules     for select using (true);
create policy "read profiles"      on public.profiles          for select using (true);
create policy "read teams"         on public.teams             for select using (true);
create policy "read members"       on public.team_members      for select using (true);
create policy "read matches"       on public.matches           for select using (true);
create policy "read matchmaking"   on public.matchmaking_posts for select using (true);

-- Users manage their own rows.
create policy "update own profile" on public.profiles
  for update using (auth.uid() = id);

create policy "read own bookings" on public.bookings
  for select using (auth.uid() = user_id or open_to_join);

-- Bookings are NEVER inserted directly by clients — only through the
-- create_booking() function (SECURITY DEFINER), so no insert policy exists.

create policy "author manages post" on public.matchmaking_posts
  for update using (auth.uid() = author_id);

create policy "respond to posts" on public.matchmaking_responses
  for insert with check (auth.uid() = user_id);
create policy "read responses" on public.matchmaking_responses
  for select using (true);
