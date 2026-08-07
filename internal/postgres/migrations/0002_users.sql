-- ============================================================================
-- 0002 — Users and authentication.
--
-- The old schema split identity across Supabase's `auth.users` and a public
-- `profiles` table joined on every read. We own identity now, so credentials
-- and the public player card live in one row: one lookup, no join, no chance
-- of a profile going missing for a user that exists.
--
-- Secret columns (password_hash) are never selected by the repository layer,
-- which always names its columns explicitly.
-- ============================================================================

create table users (
  id                uuid primary key default gen_random_uuid(),

  -- Credentials
  email             citext not null unique check (email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'),
  password_hash     text   not null,
  email_verified_at timestamptz,

  -- Public player card
  username          text not null unique check (username ~ '^[a-z0-9_]{3,24}$'),
  full_name         text not null check (char_length(full_name) between 1 and 80),
  account_type      account_type not null default 'player',
  avatar_url        text,
  phone             text check (phone ~ '^(98|97)[0-9]{8}$'),   -- Nepali mobile
  city              text not null default 'Kathmandu',
  position          futsal_position,
  jersey_number     int check (jersey_number between 0 and 99),
  preferred_foot    preferred_foot,
  skill             skill_tier not null default 'casual',
  bio               text check (char_length(bio) <= 280),

  -- Reputation counters, maintained by the service layer
  matches_played    int not null default 0 check (matches_played  >= 0),
  matches_won       int not null default 0 check (matches_won     >= 0),
  community_score   int not null default 0,

  created_at        timestamptz not null default now(),
  updated_at        timestamptz not null default now(),

  constraint wins_cannot_exceed_matches check (matches_won <= matches_played)
);

create trigger users_set_updated_at
  before update on users
  for each row execute function set_updated_at();

-- ----------------------------------------------------------------------------
-- Refresh tokens.
--
-- Only a SHA-256 digest of the token is stored, so a database leak does not
-- hand out live sessions. Rotation is enforced by `replaced_by`: presenting a
-- token that has already been rotated is evidence of theft, and lets the
-- service revoke the whole chain.
-- ----------------------------------------------------------------------------
create table refresh_tokens (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references users (id) on delete cascade,
  token_hash  bytea not null unique,
  issued_at   timestamptz not null default now(),
  expires_at  timestamptz not null,
  revoked_at  timestamptz,
  replaced_by uuid references refresh_tokens (id),
  user_agent  text,
  ip          inet,

  constraint refresh_token_outlives_issue check (expires_at > issued_at)
);

create index refresh_tokens_user_idx on refresh_tokens (user_id)
  where revoked_at is null;

-- Sweep target for expired rows.
create index refresh_tokens_expiry_idx on refresh_tokens (expires_at);

-- ----------------------------------------------------------------------------
-- Single-use tokens for password reset and email verification.
--
-- One table, discriminated by purpose: both flows need exactly the same
-- columns and the same "hash it, expire it, burn it on use" discipline.
-- ----------------------------------------------------------------------------
create type verification_purpose as enum ('password_reset', 'email_verification');

create table verification_tokens (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references users (id) on delete cascade,
  purpose     verification_purpose not null,
  token_hash  bytea not null unique,
  expires_at  timestamptz not null,
  consumed_at timestamptz,
  created_at  timestamptz not null default now()
);

-- At most one live token per user per purpose: issuing a new reset link
-- invalidates the previous one rather than leaving several usable.
create unique index verification_tokens_live_idx
  on verification_tokens (user_id, purpose)
  where consumed_at is null;
