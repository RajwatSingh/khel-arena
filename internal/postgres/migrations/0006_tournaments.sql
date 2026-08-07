-- ============================================================================
-- 0006 — Tournaments and team registration.
--
-- Capacity is enforced by the database, not by the application. `team_count`
-- is maintained by trigger and guarded by a CHECK against `max_teams`, so the
-- row lock taken by the counter UPDATE serialises concurrent registrations for
-- free. Two captains registering the last slot at the same instant means one
-- of them gets a constraint violation -- never an over-subscribed bracket.
--
-- This replaces the advisory-lock-plus-manual-count dance the old
-- register_team_for_tournament() function performed, and unlike that function
-- it also holds for any other write path.
-- ============================================================================

create table tournaments (
  id             uuid primary key default gen_random_uuid(),
  organizer_id   uuid not null references users (id) on delete restrict,
  arena_id       uuid references arenas (id) on delete set null,
  name           text not null check (char_length(name) between 4 and 80),
  slug           text not null unique check (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  format         tournament_format not null default 'knockout',
  side_count     int not null default 5 check (side_count between 4 and 7),
  squad_cap      int not null default 10 check (squad_cap between 5 and 15),
  max_teams      int not null check (max_teams between 4 and 32),
  team_count     int not null default 0 check (team_count >= 0),
  entry_fee_npr  int not null default 0 check (entry_fee_npr  >= 0),
  prize_pool_npr int not null default 0 check (prize_pool_npr >= 0),
  prize_split    int[] not null default '{60,30,10}',   -- % to 1st/2nd/3rd
  skill          skill_tier not null default 'casual',
  description    text check (char_length(description) <= 500),
  rules          text check (char_length(rules) <= 2000),
  starts_on      date not null,
  register_by    date not null,
  status         tournament_status not null default 'open',
  created_at     timestamptz not null default now(),
  updated_at     timestamptz not null default now(),

  constraint registration_before_kickoff check (register_by <= starts_on),
  constraint prize_split_sums_to_100     check (int_array_sum(prize_split) = 100),
  constraint within_capacity             check (team_count <= max_teams)
);

create trigger tournaments_set_updated_at
  before update on tournaments
  for each row execute function set_updated_at();

create index tournaments_open_idx on tournaments (starts_on)
  where status in ('open', 'full');
create index tournaments_organizer_idx on tournaments (organizer_id);

-- ----------------------------------------------------------------------------
create table tournament_teams (
  tournament_id uuid not null references tournaments (id) on delete cascade,
  team_id       uuid not null references teams (id)       on delete cascade,
  registered_by uuid not null references users (id)       on delete restrict,
  paid          boolean not null default false,
  registered_at timestamptz not null default now(),

  primary key (tournament_id, team_id)
);

create index tournament_teams_team_idx on tournament_teams (team_id);

-- ----------------------------------------------------------------------------
-- Keep team_count honest. The UPDATE below locks the tournament row, which is
-- what makes concurrent registration safe; `within_capacity` then rejects the
-- registration that would overflow the bracket.
--
-- The tournament also flips open <-> full on its own, so no caller has to
-- remember to.
-- ----------------------------------------------------------------------------
create function sync_tournament_team_count() returns trigger
language plpgsql as $$
declare
  target uuid := coalesce(new.tournament_id, old.tournament_id);
  delta  int  := case when tg_op = 'INSERT' then 1 else -1 end;
begin
  -- One statement, so the row lock covers both the count and the status.
  -- Reading `t.team_count` inside the UPDATE re-reads it under that lock,
  -- which is what makes the increment safe against concurrent registration.
  -- A tournament that is cancelled, ongoing or completed keeps its status:
  -- only the open <-> full pair is driven by occupancy.
  update tournaments t
     set team_count = t.team_count + delta,
         status = case
                    when t.status not in ('open', 'full')      then t.status
                    when t.team_count + delta >= t.max_teams   then 'full'::tournament_status
                    else                                            'open'::tournament_status
                  end
   where t.id = target;

  return null;
end;
$$;

create trigger tournament_teams_sync_count
  after insert or delete on tournament_teams
  for each row execute function sync_tournament_team_count();
