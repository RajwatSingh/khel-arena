-- ============================================================================
-- 0007 — Community matchmaking, the find-a-player board.
--
-- A post either opens an existing booking to the public ("we have the court,
-- need 2 more") or calls for a pickup game with no booking yet.
--
-- The old schema also carried `bookings.open_to_join`, a boolean mirroring
-- whether a post existed. Two sources of truth for one fact drift the moment
-- any write path forgets one of them -- and one did: cancelling a booking
-- updated the post but a post expiring never cleared the flag. The column is
-- gone; whether a booking is open to join is derived from this table.
-- ============================================================================

create table matchmaking_posts (
  id             uuid primary key default gen_random_uuid(),
  author_id      uuid not null references users (id) on delete cascade,
  booking_id     uuid references bookings (id) on delete cascade,   -- null = open call
  arena_id       uuid references arenas (id)   on delete set null,
  title          text not null check (char_length(title) between 3 and 120),
  description    text check (char_length(description) <= 280),
  needed_players int not null check (needed_players between 1 and 15),
  filled_players int not null default 0 check (filled_players >= 0),
  skill          skill_tier not null default 'casual',
  starts_at      timestamptz not null,
  status         matchmaking_status not null default 'open',
  created_at     timestamptz not null default now(),
  updated_at     timestamptz not null default now(),

  constraint not_overfilled check (filled_players <= needed_players)
);

create trigger matchmaking_posts_set_updated_at
  before update on matchmaking_posts
  for each row execute function set_updated_at();

-- A booking has at most one post, which is what makes reopening idempotent.
create unique index matchmaking_one_post_per_booking
  on matchmaking_posts (booking_id) where booking_id is not null;

-- The feed: open calls, soonest first.
create index matchmaking_feed_idx on matchmaking_posts (starts_at)
  where status = 'open';

create index matchmaking_author_idx on matchmaking_posts (author_id, created_at desc);

-- ----------------------------------------------------------------------------
-- Responses — a player asking to join. `accepted` is the author's decision.
-- ----------------------------------------------------------------------------
create table matchmaking_responses (
  post_id    uuid not null references matchmaking_posts (id) on delete cascade,
  user_id    uuid not null references users (id) on delete cascade,
  message    text check (char_length(message) <= 200),
  accepted   boolean not null default false,
  created_at timestamptz not null default now(),

  primary key (post_id, user_id)
);

create index matchmaking_responses_user_idx on matchmaking_responses (user_id);

-- ----------------------------------------------------------------------------
-- filled_players tracks the number of accepted responses -- derived, never
-- assigned by hand. `not_overfilled` then rejects the acceptance that would
-- oversubscribe the game.
--
-- The counter moves by a delta inside a single UPDATE rather than being
-- recomputed with a COUNT. A recount would read the response table on its own
-- snapshot, so two authors accepting a player at the same instant would both
-- count one and the second acceptance would be lost. Reading `p.filled_players`
-- inside the UPDATE re-reads it under that statement's row lock, which
-- serialises concurrent acceptances and lets `not_overfilled` do its job.
-- ----------------------------------------------------------------------------
create function sync_matchmaking_filled() returns trigger
language plpgsql as $$
declare
  target uuid := coalesce(new.post_id, old.post_id);
  delta  int;
begin
  delta := case tg_op
             when 'INSERT' then (new.accepted)::int
             when 'DELETE' then -((old.accepted)::int)
             else               (new.accepted)::int - (old.accepted)::int
           end;

  if delta = 0 then
    return null;
  end if;

  -- A full game closes itself; a withdrawal reopens it. A cancelled or
  -- expired call keeps its status.
  update matchmaking_posts p
     set filled_players = p.filled_players + delta,
         status = case
                    when p.status not in ('open', 'filled')             then p.status
                    when p.filled_players + delta >= p.needed_players   then 'filled'::matchmaking_status
                    else                                                     'open'::matchmaking_status
                  end
   where p.id = target;

  return null;
end;
$$;

create trigger matchmaking_responses_sync_filled
  after insert or update of accepted or delete on matchmaking_responses
  for each row execute function sync_matchmaking_filled();
