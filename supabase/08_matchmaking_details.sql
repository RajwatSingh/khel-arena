-- ============================================================================
-- KHEL ARENA — 08: Richer community calls
-- Run after schema.sql + functions.sql + 02..07. Safe to re-run.
--
-- Changes:
--   · needed_players cap raised from 10 to 15 (full 7-a-side squad + subs).
--   · matchmaking_posts.description — the author's note on how the game
--     will run (vibe, rules, what to bring, which positions are wanted).
--   · toggle_matchmaking_slot signature gains p_description.
-- ============================================================================

alter table public.matchmaking_posts
  drop constraint if exists matchmaking_posts_needed_players_check;
alter table public.matchmaking_posts
  add constraint matchmaking_posts_needed_players_check
  check (needed_players between 1 and 15);

alter table public.matchmaking_posts
  add column if not exists description text
  check (description is null or char_length(description) <= 280);

-- Wanted positions live in the description text now; drop the column if an
-- earlier draft of this migration added it.
alter table public.matchmaking_posts
  drop column if exists positions;

-- Replace (not overload) the toggle function — drop every prior signature.
drop function if exists public.toggle_matchmaking_slot(uuid, boolean, int, text, skill_tier);
drop function if exists public.toggle_matchmaking_slot(uuid, boolean, int, text, skill_tier, text, text[]);

create or replace function public.toggle_matchmaking_slot(
  p_booking_id     uuid,
  p_open           boolean,
  p_needed_players int  default 2,
  p_title          text default null,
  p_skill          skill_tier default 'casual',
  p_description    text default null
) returns public.matchmaking_posts
language plpgsql security definer set search_path = public as $$
declare
  v_user    uuid := auth.uid();
  v_booking public.bookings%rowtype;
  v_arena   uuid;
  v_post    public.matchmaking_posts%rowtype;
begin
  select b.* into v_booking
  from public.bookings b
  where b.id = p_booking_id
  for update;                              -- lock the row for the whole toggle

  if not found then
    raise exception 'BOOKING_NOT_FOUND';
  end if;
  if v_booking.user_id <> v_user then
    raise exception 'NOT_OWNER: only the booking owner can open it to the community';
  end if;
  if v_booking.status in ('cancelled', 'completed') then
    raise exception 'BOOKING_INACTIVE';
  end if;
  if p_open and lower(v_booking.slot) < now() then
    raise exception 'SLOT_IN_PAST';
  end if;

  select c.arena_id into v_arena from public.courts c where c.id = v_booking.court_id;

  update public.bookings set open_to_join = p_open where id = p_booking_id;

  if p_open then
    insert into public.matchmaking_posts
      (author_id, booking_id, arena_id, title, needed_players, skill, starts_at,
       status, description)
    values
      (v_user, p_booking_id, v_arena,
       coalesce(p_title, 'Players needed — join this booking'),
       p_needed_players, p_skill, lower(v_booking.slot), 'open',
       p_description)
    on conflict do nothing;

    -- Reopen if a post already existed for this booking.
    update public.matchmaking_posts
       set status = 'open',
           needed_players = p_needed_players,
           skill = p_skill,
           title = coalesce(p_title, title),
           description = p_description
     where booking_id = p_booking_id
    returning * into v_post;
  else
    update public.matchmaking_posts
       set status = 'filled'
     where booking_id = p_booking_id
    returning * into v_post;
  end if;

  return v_post;
end;
$$;

grant execute on function public.toggle_matchmaking_slot to authenticated;
