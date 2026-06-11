-- ============================================================================
-- 03_teams.sql — RLS policies for teams + team_members, plus join_code column
-- Run after schema.sql has created the tables.
-- ============================================================================

-- Add join_code column for invite links
alter table public.teams
  add column if not exists join_code text unique;

-- Backfill existing teams with a random join code
update public.teams
  set join_code = upper(substring(md5(random()::text) from 1 for 8))
  where join_code is null;

-- Make join_code not-null going forward
alter table public.teams
  alter column join_code set not null,
  alter column join_code set default upper(substring(md5(random()::text) from 1 for 8));

-- ── Team policies ───────────────────────────────────────────────────────────

-- Insert: authenticated users can create teams they captain
create policy "insert team"
  on public.teams for insert
  to authenticated
  with check (captain_id = auth.uid());

-- Update: only the captain can update their team
create policy "update own team"
  on public.teams for update
  to authenticated
  using (captain_id = auth.uid())
  with check (captain_id = auth.uid());

-- ── Team member policies ────────────────────────────────────────────────────

-- Insert: captain can add members to their team
create policy "insert member"
  on public.team_members for insert
  to authenticated
  with check (
    exists (
      select 1 from public.teams
      where teams.id = team_id
        and teams.captain_id = auth.uid()
    )
    -- or the user is inserting themselves (for join-by-code flow)
    or user_id = auth.uid()
  );

-- Delete: captain can remove anyone, or a player can remove themselves
create policy "delete member"
  on public.team_members for delete
  to authenticated
  using (
    user_id = auth.uid()
    or exists (
      select 1 from public.teams
      where teams.id = team_id
        and teams.captain_id = auth.uid()
    )
  );
