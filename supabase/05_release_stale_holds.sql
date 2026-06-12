-- ============================================================================
-- KHEL ARENA — 05: Release stale booking holds
-- Run after schema.sql + functions.sql + 02 + 03 + 04.
--
-- Problem: create_booking() inserts bookings as 'pending'. The availability
-- grid and the no_double_booking EXCLUDE constraint both treat every non-
-- cancelled booking as "taken", so a player who starts a booking and never
-- finishes payment would hold that slot FOREVER.
--
-- Fix (defence in depth):
--   1. booking_hold_window() — single source of truth for how long an unpaid
--      hold survives (15 minutes).
--   2. get_availability_grid() — an expired, unpaid 'pending' hold no longer
--      counts as booked, so the slot frees up the instant the window lapses,
--      even before any cleanup runs.
--   3. create_booking() — releases overlapping expired holds (inside the
--      advisory lock) before inserting, so the EXCLUDE constraint cannot block
--      a legitimate rebooking of a freed slot.
--   4. release_stale_holds() + an optional pg_cron job — actually flip expired
--      holds to 'cancelled' so /my-bookings and the data stay accurate.
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 1. The hold window — change it here and every path below follows.
-- ----------------------------------------------------------------------------
create or replace function public.booking_hold_window()
returns interval
immutable language sql as $$
  select interval '15 minutes';
$$;

-- ----------------------------------------------------------------------------
-- 2. get_availability_grid — exclude expired, unpaid pending holds.
--    A slot is taken when a confirmed/completed booking covers it, OR a
--    'pending' hold covers it that is still within the hold window.
-- ----------------------------------------------------------------------------
create or replace function public.get_availability_grid(
  p_court_id uuid,
  p_date     date            -- interpreted in Asia/Kathmandu
) returns table (
  starts_at  timestamptz,
  ends_at    timestamptz,
  price_npr  int,
  is_peak    boolean,
  is_booked  boolean,
  is_past    boolean
)
language sql stable as $$
  with arena as (
    select a.opens_at, a.closes_at
    from public.courts c join public.arenas a on a.id = c.arena_id
    where c.id = p_court_id
  ),
  hours as (
    select
      ((p_date::text || ' ' || h || ':00')::timestamp at time zone 'Asia/Kathmandu') as s,
      ((p_date::text || ' ' || h || ':00')::timestamp at time zone 'Asia/Kathmandu')
        + interval '1 hour' as e
    from arena,
         generate_series(extract(hour from opens_at)::int,
                         extract(hour from closes_at)::int - 1) as h
  )
  select
    h.s,
    h.e,
    p.price_npr,
    p.is_peak,
    exists (
      select 1 from public.bookings b
      where b.court_id = p_court_id
        and b.slot && tstzrange(h.s, h.e, '[)')
        and b.status not in ('cancelled', 'no_show')
        -- A pending hold only blocks while it is still fresh.
        and (b.status <> 'pending'
             or b.created_at >= now() - public.booking_hold_window())
    ) as is_booked,
    h.s < now() as is_past
  from hours h
  cross join lateral public.resolve_slot_price(p_court_id, h.s) p
  order by h.s;
$$;

-- ----------------------------------------------------------------------------
-- 3. create_booking — same race-proof body, with one addition: release any
--    expired, unpaid holds overlapping this slot before the overlap check and
--    insert. Runs under the advisory lock, so it is serialised per court+start.
-- ----------------------------------------------------------------------------
create or replace function public.create_booking(
  p_court_id uuid,
  p_starts   timestamptz,
  p_ends     timestamptz,
  p_team_id  uuid default null,
  p_note     text default null
) returns public.bookings
language plpgsql security definer set search_path = public as $$
declare
  v_user    uuid := auth.uid();
  v_slot    tstzrange;
  v_price   int;
  v_peak    boolean;
  v_arena   public.arenas%rowtype;
  v_booking public.bookings%rowtype;
begin
  if v_user is null then
    raise exception 'AUTH_REQUIRED' using errcode = '28000';
  end if;
  if p_starts >= p_ends then
    raise exception 'INVALID_RANGE: start must precede end';
  end if;
  if p_starts < now() then
    raise exception 'SLOT_IN_PAST: cannot book a slot that has already started';
  end if;

  v_slot := tstzrange(p_starts, p_ends, '[)');

  -- Validate against arena operating hours.
  select a.* into v_arena
  from public.arenas a
  join public.courts c on c.arena_id = a.id
  where c.id = p_court_id and c.is_active and a.is_active;

  if not found then
    raise exception 'COURT_NOT_FOUND';
  end if;

  if (p_starts at time zone 'Asia/Kathmandu')::time < v_arena.opens_at
     or (p_ends at time zone 'Asia/Kathmandu')::time > v_arena.closes_at then
    raise exception 'OUTSIDE_OPERATING_HOURS: % is open % – %',
      v_arena.name, v_arena.opens_at, v_arena.closes_at;
  end if;

  -- Layer 1: serialise contenders for this court+start instant.
  perform pg_advisory_xact_lock(
    hashtextextended(p_court_id::text || extract(epoch from p_starts)::text, 42)
  );

  -- Free any expired, unpaid holds overlapping this slot so the overlap check
  -- and the EXCLUDE constraint below see only live bookings.
  update public.bookings b
     set status = 'cancelled', open_to_join = false
   where b.court_id = p_court_id
     and b.status = 'pending'
     and b.slot && v_slot
     and b.created_at < now() - public.booking_hold_window()
     and not exists (
       select 1 from public.payments p
       where p.booking_id = b.id and p.status = 'verified'
     );

  -- Layer 2: friendly pre-check now that we hold the lock.
  if exists (
    select 1 from public.bookings b
    where b.court_id = p_court_id
      and b.status <> 'cancelled'
      and b.slot && v_slot
  ) then
    raise exception 'SLOT_TAKEN: this court is already booked for the selected time'
      using errcode = 'P0001';
  end if;

  select r.price_npr, r.is_peak into v_price, v_peak
  from public.resolve_slot_price(p_court_id, p_starts) r;

  -- Price scales with duration (rules are per-hour).
  v_price := v_price * ceil(extract(epoch from (p_ends - p_starts)) / 3600.0)::int;

  -- Layer 3: the EXCLUDE constraint guards this insert unconditionally.
  insert into public.bookings (court_id, user_id, team_id, slot, price_npr, is_peak, status, note)
  values (p_court_id, v_user, p_team_id, v_slot, v_price, v_peak, 'pending', p_note)
  returning * into v_booking;

  return v_booking;

exception
  when exclusion_violation then
    raise exception 'SLOT_TAKEN: this court is already booked for the selected time'
      using errcode = 'P0001';
end;
$$;

-- ----------------------------------------------------------------------------
-- 4. release_stale_holds — the janitor. Cancels every expired, unpaid hold,
--    fails its dangling 'initiated' payment, and takes down any community post
--    tied to it. Returns how many holds were released.
-- ----------------------------------------------------------------------------
create or replace function public.release_stale_holds()
returns integer
language plpgsql security definer set search_path = public as $$
declare
  v_ids uuid[];
begin
  with released as (
    update public.bookings b
       set status = 'cancelled', open_to_join = false
     where b.status = 'pending'
       and b.created_at < now() - public.booking_hold_window()
       and not exists (
         select 1 from public.payments p
         where p.booking_id = b.id and p.status = 'verified'
       )
    returning b.id
  )
  select array_agg(id) into v_ids from released;

  if v_ids is null then
    return 0;
  end if;

  -- Mark the abandoned payment intents failed (only the not-yet-resolved ones).
  update public.payments
     set status = 'failed'
   where booking_id = any(v_ids)
     and status = 'initiated';

  -- Pull any community posts that were riding on these holds.
  update public.matchmaking_posts
     set status = 'cancelled'
   where booking_id = any(v_ids)
     and status <> 'cancelled';

  return array_length(v_ids, 1);
end;
$$;

-- Cron (or an admin client) drives the janitor; ordinary users never call it.
grant execute on function public.release_stale_holds() to service_role;
grant execute on function public.booking_hold_window() to anon, authenticated, service_role;

-- ----------------------------------------------------------------------------
-- 5. Schedule the janitor every minute — ONLY if pg_cron is installed.
--    Enable it first in the Supabase dashboard (Database → Extensions →
--    pg_cron). Without it, availability is still correct (steps 2 + 3 handle
--    that); holds just linger as 'pending' in the owner's list until rebooked.
-- ----------------------------------------------------------------------------
do $$
begin
  if exists (select 1 from pg_extension where extname = 'pg_cron') then
    if exists (select 1 from cron.job where jobname = 'release-stale-booking-holds') then
      perform cron.unschedule('release-stale-booking-holds');
    end if;
    perform cron.schedule(
      'release-stale-booking-holds',
      '* * * * *',
      'select public.release_stale_holds();'
    );
  end if;
end $$;
