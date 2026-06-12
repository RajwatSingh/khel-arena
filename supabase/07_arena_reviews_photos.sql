-- ============================================================================
-- KHEL ARENA — 07: Arena reviews & photo galleries
-- Run after schema.sql + functions.sql + 02..06.
--
-- Adds:
--   · arena_reviews — players rate an arena 1–5 stars with an optional
--     comment. One review per player per arena (re-submitting updates it).
--     A trigger keeps arenas.rating in sync with the live average.
--   · arena_photos — owners post photos of their futsal; everyone can view.
--
-- Storage: create a PUBLIC bucket named `arena-photos` in Supabase Storage
-- (mirror of `avatars`), with a policy allowing authenticated users to write
-- under the `{arena_id}/` prefix of an arena they own:
--
--   create policy "owner uploads arena photo"
--     on storage.objects for insert to authenticated
--     with check (
--       bucket_id = 'arena-photos'
--       and exists (
--         select 1 from public.arenas a
--         where a.id::text = (storage.foldername(name))[1]
--           and a.owner_id = auth.uid()
--       )
--     );
--   create policy "owner deletes arena photo"
--     on storage.objects for delete to authenticated
--     using (
--       bucket_id = 'arena-photos'
--       and exists (
--         select 1 from public.arenas a
--         where a.id::text = (storage.foldername(name))[1]
--           and a.owner_id = auth.uid()
--       )
--     );
-- ============================================================================

-- ── Reviews ─────────────────────────────────────────────────────────────────
create table public.arena_reviews (
  id          uuid primary key default gen_random_uuid(),
  arena_id    uuid not null references public.arenas (id) on delete cascade,
  user_id     uuid not null references public.profiles (id) on delete cascade,
  rating      int  not null check (rating between 1 and 5),
  comment     text check (comment is null or char_length(comment) <= 500),
  created_at  timestamptz not null default now(),
  unique (arena_id, user_id)
);

create index arena_reviews_arena_idx on public.arena_reviews (arena_id, created_at desc);

alter table public.arena_reviews enable row level security;

create policy "read reviews"
  on public.arena_reviews for select using (true);

-- A player reviews any arena except their own.
create policy "write own review"
  on public.arena_reviews for insert
  to authenticated
  with check (
    user_id = auth.uid()
    and not exists (
      select 1 from public.arenas a
      where a.id = arena_id and a.owner_id = auth.uid()
    )
  );

create policy "update own review"
  on public.arena_reviews for update
  to authenticated
  using (user_id = auth.uid())
  with check (user_id = auth.uid());

create policy "delete own review"
  on public.arena_reviews for delete
  to authenticated
  using (user_id = auth.uid());

-- Keep arenas.rating equal to the live average. SECURITY DEFINER because the
-- reviewer is never the arena's owner, so RLS would block the arenas update.
create or replace function public.sync_arena_rating()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
declare
  target uuid := coalesce(new.arena_id, old.arena_id);
begin
  update public.arenas
     set rating = (
       select round(avg(rating)::numeric, 1)
       from public.arena_reviews
       where arena_id = target
     )
   where id = target;
  return null;
end;
$$;

create trigger arena_reviews_sync_rating
  after insert or update or delete on public.arena_reviews
  for each row execute function public.sync_arena_rating();

-- ── Photos ──────────────────────────────────────────────────────────────────
create table public.arena_photos (
  id          uuid primary key default gen_random_uuid(),
  arena_id    uuid not null references public.arenas (id) on delete cascade,
  url         text not null,
  caption     text check (caption is null or char_length(caption) <= 120),
  created_at  timestamptz not null default now()
);

create index arena_photos_arena_idx on public.arena_photos (arena_id, created_at desc);

alter table public.arena_photos enable row level security;

create policy "read arena photos"
  on public.arena_photos for select using (true);

create policy "owner adds photo"
  on public.arena_photos for insert
  to authenticated
  with check (
    exists (
      select 1 from public.arenas a
      where a.id = arena_id and a.owner_id = auth.uid()
    )
  );

create policy "owner removes photo"
  on public.arena_photos for delete
  to authenticated
  using (
    exists (
      select 1 from public.arenas a
      where a.id = arena_id and a.owner_id = auth.uid()
    )
  );
