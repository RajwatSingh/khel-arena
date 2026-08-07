-- ============================================================================
-- 0003 — Arenas, courts and pricing rules.
--
-- An arena ("Dhuku Futsal, Jhamsikhel") owns one or more courts. Courts carry
-- a fallback hourly price; pricing_rules layer peak/off-peak windows on top.
-- ============================================================================

create table arenas (
  id          uuid primary key default gen_random_uuid(),
  owner_id    uuid not null references users (id) on delete restrict,
  name        text not null check (char_length(name) between 2 and 80),
  slug        text not null unique check (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  area        text not null,                    -- 'Jhamsikhel', 'Baluwatar'
  city        text not null default 'Kathmandu',
  lat         double precision check (lat between -90  and 90),
  lng         double precision check (lng between -180 and 180),
  description text check (char_length(description) <= 2000),
  cover_url   text,
  amenities   text[] not null default '{}',     -- {'parking','showers','floodlights'}
  phone       text check (phone ~ '^(98|97|0?1)[0-9]{7,9}$'),

  -- Operating hours, in Asia/Kathmandu wall-clock time.
  opens_at    time not null default '06:00',
  closes_at   time not null default '22:00',

  -- Denormalised review aggregate, recomputed by trigger in 0008. Kept here
  -- because every arena listing shows it and recomputing an average over all
  -- reviews per listed arena is the classic N+1 this rewrite exists to kill.
  rating       numeric(2,1) check (rating between 0 and 5),
  review_count int not null default 0 check (review_count >= 0),

  is_active   boolean not null default true,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),

  constraint arena_opens_before_closes check (opens_at < closes_at)
);

create trigger arenas_set_updated_at
  before update on arenas
  for each row execute function set_updated_at();

create index arenas_owner_idx  on arenas (owner_id);
create index arenas_active_idx on arenas (city, area) where is_active;

-- ----------------------------------------------------------------------------
create table courts (
  id         uuid primary key default gen_random_uuid(),
  arena_id   uuid not null references arenas (id) on delete cascade,
  label      text not null check (char_length(label) between 1 and 40),
  sport      sport_type not null default 'futsal',
  surface    text not null default 'artificial_turf',
  side_count int not null default 5 check (side_count between 3 and 11),
  base_price int not null check (base_price > 0),   -- NPR/hour fallback
  is_active  boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),

  unique (arena_id, label)
);

create trigger courts_set_updated_at
  before update on courts
  for each row execute function set_updated_at();

create index courts_arena_idx on courts (arena_id) where is_active;

-- ----------------------------------------------------------------------------
-- Pricing rules — peak / off-peak windows.
--
-- A rule applies when the slot's start hour falls in [start_hour, end_hour)
-- on a matching ISO day-of-week. On overlap the highest `priority` wins;
-- if nothing matches, the court's base_price applies.
-- ----------------------------------------------------------------------------
create table pricing_rules (
  id         uuid primary key default gen_random_uuid(),
  court_id   uuid not null references courts (id) on delete cascade,
  label      text not null,                     -- 'Evening Peak', 'Saturday Premium'
  days       int[] not null check (
               array_length(days, 1) between 1 and 7
               and days <@ array[1,2,3,4,5,6,7]  -- ISO dow: 1=Mon .. 7=Sun
             ),
  start_hour int not null check (start_hour between 0 and 23),
  end_hour   int not null check (end_hour between 1 and 24),
  price_npr  int not null check (price_npr > 0),
  is_peak    boolean not null default false,
  priority   int not null default 1,

  constraint pricing_window_is_forward check (end_hour > start_hour)
);

create index pricing_rules_court_idx on pricing_rules (court_id, priority desc);
