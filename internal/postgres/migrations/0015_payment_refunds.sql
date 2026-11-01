-- ============================================================================
-- 0015 — Money that arrived for a booking we couldn't honour.
--
-- When eSewa or Khalti confirms a payment, the money has moved. That fact and
-- the gateway's own response are always recorded, even when the booking can
-- no longer take the payment: it was cancelled or lapsed while the player was
-- on the gateway, or it had already been paid for another way. Such a payment
-- is kept as verified and flagged for the venue to refund by hand.
--
-- refunded_at is the venue saying they have sent it back. The payment stays
-- `verified` (the gateway's word on what happened does not change); the refund
-- is a separate fact about what the venue did next.
-- ============================================================================

alter table payments
  add column refund_due_at timestamptz,
  add column refund_reason text check (char_length(refund_reason) <= 200),
  add column refunded_at   timestamptz,

  add constraint refund_only_on_verified
    check (refund_due_at is null or status = 'verified'),
  add constraint refunded_only_if_due
    check (refunded_at is null or refund_due_at is not null);

-- Still one *kept* payment per booking: the money is collected once. A second
-- verified payment for the same booking is allowed only as one to give back.
drop index payments_one_verified_per_booking;
create unique index payments_one_verified_per_booking on payments (booking_id)
  where status = 'verified' and refund_due_at is null;

-- The venue's list of refunds still to make.
create index payments_refunds_owed_idx on payments (refund_due_at)
  where refund_due_at is not null and refunded_at is null;
