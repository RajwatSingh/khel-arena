-- ============================================================================
-- 0014 — What a cancellation cost, and whether the venue gave it back.
--
-- A late cancellation takes points off the player's user score (0013). The
-- venue sees each cancellation with its reason, and may hand the points back
-- if the player asks: a genuine emergency, say. Recording the charge on the
-- booking is what makes that a one-time act with a known amount, rather than
-- the venue typing a number into somebody's score.
-- ============================================================================

alter table bookings
  add column cancel_penalty      int not null default 0 check (cancel_penalty >= 0),
  add column penalty_restored_at timestamptz,

  add constraint restored_only_if_charged
    check (penalty_restored_at is null or cancel_penalty > 0);

-- The venue's list of cancellations, newest first.
create index bookings_cancelled_idx on bookings (court_id, cancelled_at desc)
  where status = 'cancelled';
