-- ============================================================================
-- 0013 — User score, booking bans, and why a booking was cancelled.
--
-- Every player starts at 100. Cancelling a confirmed booking less than five
-- hours before kick-off costs points (see domain.LateCancelPenalty); at 10 or
-- below the player can't book anything for a month. When that month is up the
-- score reads as 50 again. That reset is applied where the score is read, not
-- by a job, so there is no window where a lifted ban still shows.
--
-- A cancellation now carries a reason, and when it happened, so a venue can
-- see why an hour came back on the board.
-- ============================================================================

alter table users
  add column user_score int not null default 100
    check (user_score between 0 and 100),
  add column booking_banned_until timestamptz;

alter table bookings
  add column cancel_reason text check (char_length(cancel_reason) <= 300),
  add column cancelled_at  timestamptz;
