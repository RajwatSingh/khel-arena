-- ============================================================================
-- 0005 — Teams, membership and recorded matches.
-- ============================================================================

create table teams (
  id         uuid primary key default gen_random_uuid(),
  name       text not null unique check (char_length(name) between 2 and 40),
  tag        text not null unique check (tag ~ '^[A-Z0-9]{2,5}$'),   -- 'KTM', 'YETI'
  crest_url  text,
  captain_id uuid not null references users (id) on delete restrict,
  home_arena uuid references arenas (id) on delete set null,

  -- Invite code for join-by-link. Rotatable by the captain, so a leaked code
  -- can be retired without disbanding the team.
  join_code  text not null unique default upper(substr(md5(gen_random_uuid()::text), 1, 8)),

  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create trigger teams_set_updated_at
  before update on teams
  for each row execute function set_updated_at();

create index teams_captain_idx on teams (captain_id);

-- ----------------------------------------------------------------------------
create table team_members (
  team_id   uuid not null references teams (id) on delete cascade,
  user_id   uuid not null references users (id) on delete cascade,
  role      team_role not null default 'player',
  joined_at timestamptz not null default now(),

  primary key (team_id, user_id)
);

create index team_members_user_idx on team_members (user_id);

-- Exactly one captain per team, enforced rather than assumed.
create unique index team_members_one_captain_idx on team_members (team_id)
  where role = 'captain';

-- Bookings may be attributed to a team (deferred from 0004).
alter table bookings
  add constraint bookings_team_fk
  foreign key (team_id) references teams (id) on delete set null;

create index bookings_team_idx on bookings (team_id) where team_id is not null;

-- ----------------------------------------------------------------------------
-- Matches — a recorded result between two teams, usually tied to a booking.
-- `verified` means both captains confirmed the score; only verified matches
-- count toward standings.
-- ----------------------------------------------------------------------------
create table matches (
  id         uuid primary key default gen_random_uuid(),
  booking_id uuid references bookings (id) on delete set null,
  home_team  uuid not null references teams (id) on delete cascade,
  away_team  uuid not null references teams (id) on delete cascade,
  home_score int not null default 0 check (home_score >= 0),
  away_score int not null default 0 check (away_score >= 0),
  played_at  timestamptz not null default now(),
  verified   boolean not null default false,
  created_at timestamptz not null default now(),

  constraint teams_must_differ check (home_team <> away_team)
);

create index matches_team_idx    on matches (home_team, played_at desc);
create index matches_away_idx    on matches (away_team, played_at desc);
create unique index matches_booking_idx on matches (booking_id)
  where booking_id is not null;

-- ----------------------------------------------------------------------------
-- Standings: 3 points for a win, 1 for a draw, goal difference as tiebreaker.
-- ----------------------------------------------------------------------------
create view team_standings as
with results as (
  select home_team as team_id,
         (home_score >  away_score)::int as won,
         (home_score =  away_score)::int as drawn,
         (home_score <  away_score)::int as lost,
         home_score as gf, away_score as ga
  from matches where verified
  union all
  select away_team,
         (away_score >  home_score)::int,
         (away_score =  home_score)::int,
         (away_score <  home_score)::int,
         away_score, home_score
  from matches where verified
)
select
  t.id   as team_id,
  t.name,
  t.tag,
  t.crest_url,
  count(r.*)::int                                  as played,
  coalesce(sum(r.won),   0)::int                   as won,
  coalesce(sum(r.drawn), 0)::int                   as drawn,
  coalesce(sum(r.lost),  0)::int                   as lost,
  coalesce(sum(r.gf),    0)::int                   as goals_for,
  coalesce(sum(r.ga),    0)::int                   as goals_against,
  coalesce(sum(r.gf) - sum(r.ga), 0)::int          as goal_diff,
  coalesce(sum(r.won) * 3 + sum(r.drawn), 0)::int  as points,
  rank() over (
    order by coalesce(sum(r.won) * 3 + sum(r.drawn), 0) desc,
             coalesce(sum(r.gf) - sum(r.ga), 0) desc
  )::int as rank
from teams t
left join results r on r.team_id = t.id
group by t.id;
