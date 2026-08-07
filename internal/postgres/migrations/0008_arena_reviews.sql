-- ============================================================================
-- 0008 — Arena reviews and photo galleries.
--
-- One review per player per arena. The arena's `rating` and `review_count`
-- columns (declared in 0003) are maintained here by trigger, so listing a
-- page of arenas with their ratings is a single scan rather than an average
-- recomputed per arena.
-- ============================================================================

create table arena_reviews (
  id         uuid primary key default gen_random_uuid(),
  arena_id   uuid not null references arenas (id) on delete cascade,
  user_id    uuid not null references users (id)  on delete cascade,
  rating     int  not null check (rating between 1 and 5),
  comment    text check (char_length(comment) <= 500),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),

  unique (arena_id, user_id)
);

create trigger arena_reviews_set_updated_at
  before update on arena_reviews
  for each row execute function set_updated_at();

create index arena_reviews_arena_idx on arena_reviews (arena_id, created_at desc);

-- ----------------------------------------------------------------------------
create function sync_arena_rating() returns trigger
language plpgsql as $$
declare
  target uuid := coalesce(new.arena_id, old.arena_id);
begin
  update arenas a
     set rating       = agg.avg_rating,
         review_count = agg.n
    from (
      select round(avg(rating)::numeric, 1) as avg_rating,
             count(*)::int                  as n
      from arena_reviews
      where arena_id = target
    ) agg
   where a.id = target;

  return null;
end;
$$;

create trigger arena_reviews_sync_rating
  after insert or update of rating or delete on arena_reviews
  for each row execute function sync_arena_rating();

-- ----------------------------------------------------------------------------
create table arena_photos (
  id         uuid primary key default gen_random_uuid(),
  arena_id   uuid not null references arenas (id) on delete cascade,
  url        text not null,
  caption    text check (char_length(caption) <= 120),
  sort_order int not null default 0,
  created_at timestamptz not null default now()
);

create index arena_photos_arena_idx on arena_photos (arena_id, sort_order, created_at desc);

-- ----------------------------------------------------------------------------
-- Player highlight reels (YouTube/TikTok links on the player card).
-- ----------------------------------------------------------------------------
create table profile_highlights (
  id         uuid primary key default gen_random_uuid(),
  user_id    uuid not null references users (id) on delete cascade,
  title      text not null check (char_length(title) between 2 and 80),
  url        text not null check (url ~* '^https?://'),
  created_at timestamptz not null default now()
);

create index profile_highlights_user_idx on profile_highlights (user_id, created_at desc);
