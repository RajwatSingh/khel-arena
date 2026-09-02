-- Records which captain filed a result.
--
-- A match is confirmed by the *other* captain: that is what "both captains
-- agreed the score" means, and it is the whole value of the `verified` flag
-- that gates the standings view. Without knowing who reported it, the person
-- who typed the score could confirm their own win, and `verified` would mean
-- nothing.
--
-- Nullable, because rows predating this column have no answer -- and a null
-- reporter is treated as "nobody may confirm this", which is the safe reading
-- rather than "anybody may".
alter table matches add column reported_by uuid references users (id) on delete set null;

comment on column matches.reported_by is
  'The captain who filed the result. Confirmation must come from the other side.';
