-- ============================================================================
-- KHEL ARENA — Database functions
-- These are the only write-paths for bookings. Clients call them via
-- supabase.rpc(); they run as SECURITY DEFINER so the table itself stays
-- locked down behind RLS.
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Price resolution: highest-priority pricing rule covering the slot's start
-- hour on that day-of-week; falls back to the court's base price.
-- ----------------------------------------------------------------------------
create or replace function public.resolve_slot_price(
  p_court_id uuid,
  p_starts   timestamptz
) returns table (price_npr int, is_peak boolean)
language sql stable as $$
  select
    coalesce(r.price_npr, c.base_price)        as price_npr,
    coalesce(r.is_peak, false)                 as is_peak
  from public.courts c
  left join lateral (
    select pr.price_npr, pr.is_peak
    from public.pricing_rules pr
    where pr.court_id = c.id
      and extract(isodow from p_starts at time zone 'Asia/Kathmandu')::int = any (pr.days)
      and extract(hour   from p_starts at time zone 'Asia/Kathmandu')::int >= pr.start_hour
      and extract(hour   from p_starts at time zone 'Asia/Kathmandu')::int <  pr.end_hour
    order by pr.priority desc
    limit 1
  ) r on true
  where c.id = p_court_id;
$$;

-- ----------------------------------------------------------------------------
-- ★ create_booking — transaction-safe, race-proof.
--
-- Defence in depth, three layers:
--   1. pg_advisory_xact_lock serialises concurrent attempts on the same
--      court+hour, so the second request *waits* instead of failing.
--   2. An explicit overlap check returns a clean, user-friendly error.
--   3. The no_double_booking EXCLUDE constraint is the final, unbreakable
--      backstop — even a code path that skips this function cannot
--      double-book.
--
-- Price is computed server-side from pricing_rules. The client's displayed
-- price is advisory only; the database is the source of truth.
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
-- ★ toggle_matchmaking_slot — open/close a booking to the community.
-- Opening creates (or reopens) the linked matchmaking post atomically;
-- closing marks it filled. Only the booking owner may toggle.
-- ----------------------------------------------------------------------------
create or replace function public.toggle_matchmaking_slot(
  p_booking_id     uuid,
  p_open           boolean,
  p_needed_players int  default 2,
  p_title          text default null,
  p_skill          skill_tier default 'casual'
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
      (author_id, booking_id, arena_id, title, needed_players, skill, starts_at, status)
    values
      (v_user, p_booking_id, v_arena,
       coalesce(p_title, 'Players needed — join this booking'),
       p_needed_players, p_skill, lower(v_booking.slot), 'open')
    on conflict do nothing;

    -- Reopen if a post already existed for this booking.
    update public.matchmaking_posts
       set status = 'open',
           needed_players = p_needed_players,
           skill = p_skill,
           title = coalesce(p_title, title)
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

-- Booking-linked posts need a uniqueness anchor for the reopen logic above.
create unique index if not exists matchmaking_one_post_per_booking
  on public.matchmaking_posts (booking_id) where booking_id is not null;

-- ----------------------------------------------------------------------------
-- ★ get_availability_grid — the virtual TimeSlots entity.
-- Projects every 1-hour slot for a court on a given Kathmandu date, with
-- live availability + resolved pricing. One round-trip renders the matrix.
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
        and b.status <> 'cancelled'
        and b.slot && tstzrange(h.s, h.e, '[)')
    ) as is_booked,
    h.s < now() as is_past
  from hours h
  cross join lateral public.resolve_slot_price(p_court_id, h.s) p
  order by h.s;
$$;

grant execute on function public.create_booking          to authenticated;
grant execute on function public.toggle_matchmaking_slot to authenticated;
grant execute on function public.get_availability_grid   to anon, authenticated;
grant execute on function public.resolve_slot_price      to anon, authenticated;
