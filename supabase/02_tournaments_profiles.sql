-- ============================================================================
-- KHEL ARENA — 02: Tournaments & Player Profiles
-- Run after schema.sql + functions.sql.
--
-- Adds:
--   · tournaments + race-proof team registration (same advisory-lock +
--     count-check discipline as create_booking — a tournament can never
--     accept more teams than max_teams, even under concurrent submits)
--   · profile extensions: futsal position, jersey number, preferred foot
--   · profile_highlights — player highlight reels (YouTube/TikTok/etc.)
--
-- Note: team_standings view from schema.sql is retained in the database for
-- future seasons, but is no longer surfaced in the UI.
-- ============================================================================

-- ----------------------------------------------------------------------------
-- PROFILE EXTENSIONS
-- Position uses futsal vocabulary: Goleiro (keeper), Fixo (last man),
-- Ala (winger), Pivô (target), Universal (plays everywhere).
-- ----------------------------------------------------------------------------
alter table public.profiles
  add column if not exists jersey_number int check (jersey_number between 0 and 99),
  add column if not exists preferred_foot text check (preferred_foot in ('left','right','both'));

alter table public.profiles
  drop constraint if exists profiles_position_check;
alter table public.profiles
  add constraint profiles_position_check
  check (position is null or position in ('Goleiro','Fixo','Ala','Pivô','Universal'));

create table public.profile_highlights (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references public.profiles (id) on delete cascade,
  title       text not null check (char_length(title) between 2 and 80),
  url         text not null check (url ~* '^https?://'),
  created_at  timestamptz not null default now()
);

create index profile_highlights_user_idx on public.profile_highlights (user_id, created_at desc);

alter table public.profile_highlights enable row level security;
create policy "read highlights"   on public.profile_highlights for select using (true);
create policy "add own highlight" on public.profile_highlights
  for insert with check (auth.uid() = user_id);
create policy "remove own highlight" on public.profile_highlights
  for delete using (auth.uid() = user_id);

-- Avatar storage: create a public bucket named `avatars` in Supabase Storage,
-- with a policy allowing authenticated users to write under their own
-- `{user_id}/` prefix. uploadAvatar() in src/actions/profile.ts targets it.

-- ----------------------------------------------------------------------------
-- TOURNAMENTS
-- ----------------------------------------------------------------------------
create type tournament_format as enum ('knockout', 'league', 'group_knockout');
create type tournament_status as enum ('open', 'full', 'ongoing', 'completed', 'cancelled');

-- Helper for the prize-split constraint (subqueries are not allowed
-- directly inside CHECK constraints).
create or replace function public.int_array_sum(arr int[])
returns int immutable language sql as $$
  select coalesce(sum(s), 0)::int from unnest(arr) s;
$$;

create table public.tournaments (
  id              uuid primary key default gen_random_uuid(),
  organizer_id    uuid not null references public.profiles (id),
  arena_id        uuid references public.arenas (id),
  name            text not null check (char_length(name) between 4 and 80),
  slug            text unique not null,
  format          tournament_format not null default 'knockout',
  side_count      int not null default 5 check (side_count between 4 and 7),
  squad_cap       int not null default 10 check (squad_cap between 5 and 15),
  max_teams       int not null check (max_teams between 4 and 32),
  entry_fee_npr   int not null default 0 check (entry_fee_npr >= 0),
  prize_pool_npr  int not null default 0 check (prize_pool_npr >= 0),
  prize_split     int[] not null default '{60,30,10}',  -- % for 1st/2nd/3rd
  skill           skill_tier not null default 'casual',
  description     text check (char_length(description) <= 500),
  rules           text check (char_length(rules) <= 2000),
  starts_on       date not null,
  register_by     date not null,
  status          tournament_status not null default 'open',
  created_at      timestamptz not null default now(),

  constraint registration_before_kickoff check (register_by <= starts_on),
  constraint prize_split_sums_to_100 check (
    public.int_array_sum(prize_split) = 100
  )
);

create index tournaments_feed_idx
  on public.tournaments (status, starts_on)
  where status in ('open', 'full', 'ongoing');

create table public.tournament_teams (
  tournament_id  uuid not null references public.tournaments (id) on delete cascade,
  team_id        uuid not null references public.teams (id) on delete cascade,
  registered_by  uuid not null references public.profiles (id),
  paid           boolean not null default false,
  registered_at  timestamptz not null default now(),
  primary key (tournament_id, team_id)
);

alter table public.tournaments       enable row level security;
alter table public.tournament_teams  enable row level security;

create policy "read tournaments" on public.tournaments for select using (true);
create policy "create own tournament" on public.tournaments
  for insert with check (auth.uid() = organizer_id);
create policy "organizer updates tournament" on public.tournaments
  for update using (auth.uid() = organizer_id);
create policy "read registrations" on public.tournament_teams for select using (true);
-- Registrations are inserted only via register_team_for_tournament() below.

-- ----------------------------------------------------------------------------
-- ★ register_team_for_tournament — race-proof capacity.
-- Same defence discipline as create_booking: an advisory lock serialises
-- concurrent registrations for one tournament, the count is checked inside
-- the lock, and the status flips to 'full' atomically on the final spot.
-- ----------------------------------------------------------------------------
create or replace function public.register_team_for_tournament(
  p_tournament_id uuid,
  p_team_id       uuid
) returns public.tournament_teams
language plpgsql security definer set search_path = public as $$
declare
  v_user        uuid := auth.uid();
  v_tournament  public.tournaments%rowtype;
  v_count       int;
  v_registration public.tournament_teams%rowtype;
begin
  if v_user is null then
    raise exception 'AUTH_REQUIRED' using errcode = '28000';
  end if;

  -- Only the team's captain may register it.
  if not exists (
    select 1 from public.teams t
    where t.id = p_team_id and t.captain_id = v_user
  ) then
    raise exception 'NOT_CAPTAIN: only the team captain can register the team';
  end if;

  perform pg_advisory_xact_lock(hashtextextended(p_tournament_id::text, 7));

  select * into v_tournament from public.tournaments where id = p_tournament_id;
  if not found then
    raise exception 'TOURNAMENT_NOT_FOUND';
  end if;
  if v_tournament.status <> 'open' then
    raise exception 'REGISTRATION_CLOSED: this tournament is %', v_tournament.status;
  end if;
  if current_date > v_tournament.register_by then
    raise exception 'DEADLINE_PASSED: registration closed on %', v_tournament.register_by;
  end if;

  select count(*) into v_count
  from public.tournament_teams where tournament_id = p_tournament_id;

  if v_count >= v_tournament.max_teams then
    update public.tournaments set status = 'full' where id = p_tournament_id;
    raise exception 'TOURNAMENT_FULL: all % spots are taken', v_tournament.max_teams;
  end if;

  insert into public.tournament_teams (tournament_id, team_id, registered_by)
  values (p_tournament_id, p_team_id, v_user)
  returning * into v_registration;

  if v_count + 1 = v_tournament.max_teams then
    update public.tournaments set status = 'full' where id = p_tournament_id;
  end if;

  return v_registration;

exception
  when unique_violation then
    raise exception 'ALREADY_REGISTERED: this team is already in the tournament';
end;
$$;

grant execute on function public.register_team_for_tournament to authenticated;

-- Registered-team counts for the listing, without N+1 queries.
create or replace view public.tournament_board as
select
  t.*,
  coalesce(r.team_count, 0)::int as team_count,
  a.name as arena_name,
  a.area as arena_area,
  p.username as organizer_username
from public.tournaments t
left join (
  select tournament_id, count(*) as team_count
  from public.tournament_teams group by tournament_id
) r on r.tournament_id = t.id
left join public.arenas a on a.id = t.arena_id
left join public.profiles p on p.id = t.organizer_id;
