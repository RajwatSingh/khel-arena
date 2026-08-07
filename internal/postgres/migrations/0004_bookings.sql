-- ============================================================================
-- 0004 — Bookings and payments.
--
-- The single most important constraint in this database is `no_double_booking`.
-- Two live bookings can never overlap on the same court, whatever the service
-- layer does -- concurrent requests, retries, a buggy deploy, or someone at a
-- psql prompt. Application code cannot opt out of it.
--
-- Two deliberate changes from the schema this replaces:
--
--   1. `hold_expires_at` is a real column rather than `created_at` plus an
--      interval computed by a SQL function. Expiry is now readable, indexable
--      and per-booking (a cash reservation can be given a longer fuse than a
--      gateway checkout) instead of one global constant baked into a function.
--
--   2. The EXCLUDE predicate and the availability query agree on what "taken"
--      means. Previously the constraint ignored only 'cancelled' while the
--      grid ignored 'cancelled' and 'no_show', so a no-show slot rendered as
--      free but threw a constraint violation on booking. Both now use the
--      same rule, expressed once in `booking_blocks_slot`.
-- ============================================================================

create table bookings (
  id         uuid primary key default gen_random_uuid(),
  court_id   uuid not null references courts (id) on delete restrict,
  user_id    uuid not null references users (id)  on delete restrict,
  team_id    uuid,                                 -- FK added in 0005
  slot       tstzrange not null,                   -- [start, end)
  price_npr  int not null check (price_npr >= 0),  -- resolved server-side
  is_peak    boolean not null default false,
  status     booking_status not null default 'pending',
  note       text check (char_length(note) <= 280),

  -- When an unpaid 'pending' hold stops blocking the slot. Null once the
  -- booking is confirmed: a paid booking never expires.
  hold_expires_at timestamptz,

  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),

  constraint slot_is_bounded    check (not lower_inf(slot) and not upper_inf(slot)),
  constraint slot_min_duration  check (upper(slot) - lower(slot) >= interval '30 minutes'),
  constraint slot_max_duration  check (upper(slot) - lower(slot) <= interval '4 hours'),
  constraint pending_holds_expire check (status <> 'pending' or hold_expires_at is not null),

  -- The guarantee. A cancelled or no-show booking releases its slot for
  -- immediate rebooking; anything else holds it.
  constraint no_double_booking exclude using gist (
    court_id with =,
    slot     with &&
  ) where (status not in ('cancelled', 'no_show'))
);

create trigger bookings_set_updated_at
  before update on bookings
  for each row execute function set_updated_at();

-- Availability lookups: one court, one day.
create index bookings_court_slot_idx on bookings using gist (court_id, slot)
  where status not in ('cancelled', 'no_show');

-- "My bookings", newest first.
create index bookings_user_idx on bookings (user_id, created_at desc);

-- The janitor's working set: expiring holds only.
create index bookings_stale_hold_idx on bookings (hold_expires_at)
  where status = 'pending';

-- ----------------------------------------------------------------------------
-- Whether a booking row currently blocks its slot.
--
-- Stable, not immutable: it reads now(). Used by the availability query and
-- by the pre-insert overlap check so the two can never drift apart.
-- ----------------------------------------------------------------------------
create function booking_blocks_slot(p_status booking_status, p_hold_expires_at timestamptz)
returns boolean
language sql stable parallel safe as $$
  select case
    when p_status in ('cancelled', 'no_show') then false
    when p_status = 'pending' then p_hold_expires_at > now()
    else true
  end;
$$;

-- ----------------------------------------------------------------------------
-- Payments.
--
-- `transaction_uuid` is ours and is sent to the gateway; `provider_ref` is
-- whatever the gateway calls its own transaction (eSewa ref_id, Khalti pidx).
-- Both are unique so a callback replay cannot credit a booking twice.
-- ----------------------------------------------------------------------------
create table payments (
  id               uuid primary key default gen_random_uuid(),
  booking_id       uuid not null references bookings (id) on delete cascade,
  provider         payment_provider not null,
  amount_npr       int not null check (amount_npr > 0),
  status           payment_status not null default 'initiated',
  transaction_uuid text not null unique,
  provider_ref     text,
  raw_response     jsonb,
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now(),
  verified_at      timestamptz,

  constraint verified_payments_have_a_timestamp
    check ((status = 'verified') = (verified_at is not null))
);

create trigger payments_set_updated_at
  before update on payments
  for each row execute function set_updated_at();

create index payments_booking_idx on payments (booking_id, created_at desc);

-- A gateway reference, once known, identifies exactly one payment.
create unique index payments_provider_ref_idx on payments (provider, provider_ref)
  where provider_ref is not null;

-- At most one verified payment per booking: the money is collected once.
create unique index payments_one_verified_per_booking on payments (booking_id)
  where status = 'verified';
