-- ============================================================================
-- KHEL ARENA — 04: Arena owner accounts
-- Run after schema.sql + functions.sql + 02 + 03.
--
-- Adds:
--   · profiles.account_type — 'player' (default) or 'futsal_owner'.
--     Chosen at sign-up; futsal owners manage an arena instead of a
--     player card on /profile.
--   · RLS policies so owners can create and manage their own arena,
--     its courts, and its pricing rules. Players are unaffected.
-- ============================================================================

alter table public.profiles
  add column if not exists account_type text not null default 'player'
  check (account_type in ('player', 'futsal_owner'));

-- ── Arenas ──────────────────────────────────────────────────────────────────
create policy "owner inserts arena"
  on public.arenas for insert
  to authenticated
  with check (
    owner_id = auth.uid()
    and exists (
      select 1 from public.profiles p
      where p.id = auth.uid() and p.account_type = 'futsal_owner'
    )
  );

create policy "owner updates arena"
  on public.arenas for update
  to authenticated
  using (owner_id = auth.uid())
  with check (owner_id = auth.uid());

-- ── Courts ──────────────────────────────────────────────────────────────────
create policy "owner inserts court"
  on public.courts for insert
  to authenticated
  with check (
    exists (
      select 1 from public.arenas a
      where a.id = arena_id and a.owner_id = auth.uid()
    )
  );

create policy "owner updates court"
  on public.courts for update
  to authenticated
  using (
    exists (
      select 1 from public.arenas a
      where a.id = arena_id and a.owner_id = auth.uid()
    )
  );

-- ── Pricing rules ────────────────────────────────────────────────────────────
create policy "owner manages pricing"
  on public.pricing_rules for all
  to authenticated
  using (
    exists (
      select 1 from public.courts c
      join public.arenas a on a.id = c.arena_id
      where c.id = court_id and a.owner_id = auth.uid()
    )
  )
  with check (
    exists (
      select 1 from public.courts c
      join public.arenas a on a.id = c.arena_id
      where c.id = court_id and a.owner_id = auth.uid()
    )
  );
