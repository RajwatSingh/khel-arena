-- ============================================================================
-- 0012 — A player may review an arena more than once.
--
-- Each visit can earn its own review, so the one-per-player rule goes. Reviews
-- are also final once posted: there is no edit, only delete. With nothing able
-- to change a row, `updated_at` and its trigger would only ever echo
-- `created_at`, so they go too.
--
-- The rating trigger from 0008 is unchanged: it averages every review, so a
-- player who reviews three times counts three times.
-- ============================================================================

alter table arena_reviews drop constraint arena_reviews_arena_id_user_id_key;

drop trigger arena_reviews_set_updated_at on arena_reviews;
alter table arena_reviews drop column updated_at;

-- "Your reviews of this arena" was served by the unique constraint's index.
create index arena_reviews_author_idx on arena_reviews (arena_id, user_id, created_at desc);
