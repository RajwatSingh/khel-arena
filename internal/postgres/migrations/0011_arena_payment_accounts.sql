-- ============================================================================
-- 0011 — Per-arena payment accounts.
--
-- The gateway credentials used to be one set per deployment (ESEWA_SECRET_KEY
-- and friends in the environment), so every online payment settled into a
-- single merchant account. That is wrong the moment Khel Arena lists a venue
-- it does not own: the money for a court belongs to whoever runs that court.
--
-- Each row here is one venue's own merchant account with one provider. The
-- gateway adapter is built from these values per booking, not from a global
-- registry.
--
-- secret_key is stored as ciphertext (AES-256-GCM, key from PAYMENT_ENC_KEY).
-- A database dump must not hand out every venue's merchant secret. The column
-- is bytea and never leaves the server in the clear — the owner-facing API
-- returns only a four-character hint.
-- ============================================================================

create table arena_payment_accounts (
  id            uuid primary key default gen_random_uuid(),
  arena_id      uuid not null references arenas (id) on delete cascade,

  -- 'esewa' | 'khalti'. Cash needs no credentials and is not stored here.
  provider      text not null check (provider in ('esewa', 'khalti')),

  secret_key    bytea not null,
  -- eSewa's merchant/product code. Unused by Khalti, left ''.
  merchant_code text  not null default '',

  -- false → the provider's sandbox host; true → its production host. Named
  -- rather than derived so a test account is visible as one in the row.
  live          boolean not null default false,
  -- An owner can hold a configured account but stop taking that provider
  -- without deleting the credentials.
  enabled       boolean not null default true,

  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now(),

  -- One account per provider per venue. Switching keys is an update, not a
  -- second row.
  unique (arena_id, provider)
);

create trigger arena_payment_accounts_set_updated_at
  before update on arena_payment_accounts
  for each row execute function set_updated_at();

-- The hot read is "which providers can this arena take, right now" — every
-- checkout and every arena page asks it.
create index arena_payment_accounts_live_idx
  on arena_payment_accounts (arena_id)
  where enabled;
