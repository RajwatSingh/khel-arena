-- Courts carry a human name for their format.
--
-- `side_count` says how many a side, which is the number the booking rules
-- care about, but it is not what a venue calls the pitch. A futsal court is
-- "5-a-side", and that much derives cleanly -- but a basketball court is
-- "Full court" and a badminton one is "Singles or doubles", and no integer
-- produces those. The interface shows this label next to the court name, so
-- it is stored rather than guessed.
--
-- Nullable: existing courts have no label and the API derives one from
-- side_count until an owner sets it, so this migration needs no backfill and
-- breaks nothing that is already running.
alter table courts add column format text
  check (format is null or char_length(format) between 1 and 40);

comment on column courts.format is
  'Human name for the court format ("5-a-side", "Full court"). Null means derive from side_count.';
