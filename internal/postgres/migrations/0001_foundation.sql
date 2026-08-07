-- ============================================================================
-- 0001 — Extensions, enums and shared helpers.
--
-- This schema targets stock PostgreSQL. There are no row-level security
-- policies and no SECURITY DEFINER functions: the Go service is the only
-- client, it connects as a trusted role, and it owns every authorization
-- decision. The database's job is to enforce the invariants that only it
-- can enforce -- referential integrity, domain constraints, and the
-- non-overlap guarantee on bookings.
-- ============================================================================

create extension if not exists pgcrypto;   -- gen_random_uuid()
create extension if not exists btree_gist; -- required by the bookings EXCLUDE
create extension if not exists citext;     -- case-insensitive email

-- ----------------------------------------------------------------------------
-- Enums
-- ----------------------------------------------------------------------------
create type account_type       as enum ('player', 'arena_owner');
create type booking_status     as enum ('pending', 'confirmed', 'completed', 'cancelled', 'no_show');
create type payment_provider   as enum ('esewa', 'khalti', 'cash');
create type payment_status     as enum ('initiated', 'verified', 'failed', 'refunded');
create type sport_type         as enum ('futsal', 'basketball', 'badminton', 'cricket_net', 'tennis');
create type matchmaking_status as enum ('open', 'filled', 'expired', 'cancelled');
create type skill_tier         as enum ('casual', 'intermediate', 'competitive', 'semi_pro');
create type futsal_position    as enum ('Goleiro', 'Fixo', 'Ala', 'Pivo', 'Universal');
create type preferred_foot     as enum ('left', 'right', 'both');
create type team_role          as enum ('captain', 'player');
create type tournament_format  as enum ('knockout', 'league', 'group_knockout');
create type tournament_status  as enum ('open', 'full', 'ongoing', 'completed', 'cancelled');

-- ----------------------------------------------------------------------------
-- Helpers
-- ----------------------------------------------------------------------------

-- Immutable so it can be used inside a CHECK constraint (prize splits).
create function int_array_sum(arr int[]) returns int
language sql immutable strict parallel safe as $$
  select coalesce(sum(v), 0)::int from unnest(arr) as v;
$$;

create function set_updated_at() returns trigger
language plpgsql as $$
begin
  new.updated_at := now();
  return new;
end;
$$;
