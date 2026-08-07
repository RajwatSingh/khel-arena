# Khel Arena Backend — Technical Documentation

A complete explanation of what the backend does, how it is put together, and
why each decision was made the way it was.

This document assumes you can read Go and SQL but not that you already know
this codebase. Where a decision might look arbitrary, the reasoning is spelled
out - those explanations are the point of the document, not padding.

---

## Table of contents

1. [What this system is](#1-what-this-system-is)
2. [What we replaced, and why](#2-what-we-replaced-and-why)
3. [Architecture](#3-architecture)
4. [A booking, end to end](#4-a-booking-end-to-end)
5. [The database](#5-the-database)
   - 5.1 [The double-booking guarantee](#51-the-double-booking-guarantee)
   - 5.2 [Booking lifecycle and holds](#52-booking-lifecycle-and-holds)
   - 5.3 [Counters kept honest by triggers](#53-counters-kept-honest-by-triggers)
   - 5.4 [Table reference](#54-table-reference)
6. [The domain layer](#6-the-domain-layer)
   - 6.1 [The error model](#61-the-error-model)
   - 6.2 [Time, slots and wall clocks](#62-time-slots-and-wall-clocks)
   - 6.3 [Pricing](#63-pricing)
   - 6.4 [Booking rules](#64-booking-rules)
   - 6.5 [Payment rules](#65-payment-rules)
7. [The storage layer](#7-the-storage-layer)
8. [The service layer](#8-the-service-layer)
   - 8.1 [Authentication](#81-authentication)
   - 8.2 [The janitor](#82-the-janitor)
9. [Migrations](#9-migrations)
10. [Configuration](#10-configuration)
11. [Testing](#11-testing)
12. [What is not built yet](#12-what-is-not-built-yet)
13. [File map](#13-file-map)

---

## 1. What this system is

Khel Arena lets people in Kathmandu book futsal courts. Around that core sit
teams, tournaments, a find-a-player board, arena reviews, and payments through
Nepal's two dominant gateways, eSewa and Khalti.

The backend is a Go service over stock PostgreSQL. It currently builds one
binary, `cmd/migrate`, and a set of packages that a future HTTP server will
call. There is no HTTP layer yet - that is deliberate and is the next phase.

**The one invariant that matters more than any other:** two people must never
end up holding the same court at the same time. A futsal arena that
double-books turns two teams away at the gate. Much of the design below exists
to make that outcome impossible rather than merely unlikely.

---

## 2. What we replaced, and why

The previous version was a Next.js application whose server actions talked to
Supabase. It worked, and parts of it were genuinely well designed - the
exclusion-constraint approach to bookings came from it and survives here almost
unchanged. But it had three structural problems.

### Every query was a network round-trip to a remote service

Supabase exposes Postgres through PostgREST, an HTTP layer. A server action
that read five things made five HTTP requests to a remote host. Worse, nearly
every action began with `supabase.auth.getUser()`, which is itself a network
call to Supabase's auth service. A page showing a booking list cost an auth
round-trip plus one request per query, each with TLS and HTTP overhead, before
any actual work happened.

Here, `pgx` holds a pool of open TCP connections to Postgres and caches
prepared statements on each one. A query is one round-trip on an already-open
socket. Authentication reads a signed token in-process and touches no database
at all.

### Reads fetched rows and then fetched more rows

The old "my bookings" path loaded bookings, then for each booking resolved its
court, then that court's arena. That is the N+1 pattern, and over HTTP it is
especially expensive.

Here, `ListForUser` is a single query with the joins in it. The availability
grid is two queries plus a projection in memory.

### Business rules were scattered across two languages and a policy engine

Because untrusted browsers could reach the database directly, Supabase required
Row Level Security policies to control access, and writes had to go through
`SECURITY DEFINER` functions to bypass those same policies safely. The result:
booking logic lived in PL/pgSQL, authorization lived in RLS policies, and
validation lived in TypeScript. Understanding one rule meant reading three
places, and none of it could be unit-tested without a live database.

A Go service is a *trusted* client. It connects as a privileged role and is the
only thing that talks to the database. So RLS and `SECURITY DEFINER` are not
needed — authorization is ordinary Go code, readable and testable.

**What we kept:** the database still enforces what only it can. Referential
integrity, domain constraints, and above all the non-overlap guarantee on
bookings stay in SQL, because no amount of careful application code can provide
them under concurrency.

---

## 3. Architecture

Four layers, with dependencies pointing strictly inward.

```
        ┌─────────────────────────────────────────────┐
        │  cmd/           binaries, wiring            │
        └──────────────────────┬──────────────────────┘
                               │
        ┌──────────────────────▼──────────────────────┐
        │  internal/service    use cases              │
        │  "register a user", "take a hold"           │
        └───────────┬─────────────────────┬───────────┘
                    │                     │
    ┌───────────────▼──────────┐   ┌──────▼───────────────────┐
    │  internal/postgres       │   │  internal/platform       │
    │  SQL + pgx repositories  │   │  config, tokens, crypto  │
    └───────────────┬──────────┘   └──────┬───────────────────┘
                    │                     │
        ┌───────────▼─────────────────────▼───────────┐
        │  internal/domain                            │
        │  entities, value objects, rules             │
        │  imports nothing of ours                    │
        └─────────────────────────────────────────────┘
```

### The dependency rule

`domain` imports no other package in this project. It has no idea a database or
an HTTP server exists. That constraint is what makes the rules in it readable
and testable: `go test ./internal/domain` runs in five milliseconds and needs
nothing installed.

`postgres` is the only package that imports `pgx`. If the storage engine ever
changed, the blast radius is one directory.

`service` depends on `domain` for rules and on **interfaces it declares
itself** for storage. This last point is worth dwelling on, because it inverts
the obvious arrangement.

### Interfaces are declared by the consumer

In `internal/service/booking.go`:

```go
type BookingStore interface {
    LoadCourtContext(ctx context.Context, courtID uuid.UUID) (postgres.CourtContext, error)
    BookedRanges(ctx context.Context, courtID uuid.UUID, window domain.Slot) ([]domain.Slot, error)
    CreateHold(ctx context.Context, b domain.Booking) (domain.Booking, error)
    ByID(ctx context.Context, id uuid.UUID) (domain.Booking, error)
    Cancel(ctx context.Context, bookingID, userID uuid.UUID) error
    ListForUser(ctx context.Context, userID uuid.UUID, limit int) ([]domain.BookingDetail, error)
    ReleaseStaleHolds(ctx context.Context) (int, error)
}
```

`BookingRepo` in the `postgres` package satisfies this without ever mentioning
it. Go's structural typing means the implementation does not import the
interface.

The benefit is that the interface describes what *this use case needs*, not
everything the repository can do. A test can supply a stub with six methods
instead of a database. And when you read `BookingService`, its dependencies are
listed right there in the file.

---

## 4. A booking, end to end

Before the layer-by-layer detail, here is the whole flow for the most important
operation in the system. Follow this once and the rest of the document is
filling in detail.

A player wants Court A at Dhuku Futsal, 18:00–19:00 next Friday.

### Step 1 — The service validates what it can without the database

`BookingService.Create` (`internal/service/booking.go`):

```go
slot, err := domain.NewSlot(in.Start, in.End)   // shape of the request
if slot.IsPast(now) { ... }                     // not in the past
if in.UserID == uuid.Nil { ... }                // signed in
```

`domain.NewSlot` normalises both instants to UTC and checks the range is
positive, at least 30 minutes, and at most 4 hours. These checks cost nothing
and run first, so a malformed request never reaches the locking path below and
never occupies a database connection.

### Step 2 — Load everything about the court in one round-trip

```go
court, err := s.store.LoadCourtContext(ctx, in.CourtID)
```

This needs three facts: the court (for its base price), the arena (for opening
hours), and the pricing rules. Fetching them one at a time would be three
sequential round-trips. Instead `LoadCourtContext` uses `pgx.Batch` to send two
statements — one joined query for court+arena, one for the rules — in a single
network exchange.

### Step 3 — Check the arena is actually open

```go
if !slot.WithinOperatingHours(court.OpensAt, court.ClosesAt, s.loc) {
    return domain.Invalid("slot", "%s is open from %s to %s.", ...)
}
```

The comparison happens in Kathmandu wall-clock time, not UTC. Section 6.2
explains why this matters more than it first appears.

### Step 4 — Resolve the price on the server

```go
price := domain.ResolvePrice(court.Court.BasePriceNPR, court.PricingRules, slot, s.loc)
```

The client never supplies a price. Whatever the browser displayed is advisory;
this figure is the one that gets written. A client that sends `price: 1` is
simply ignored, because there is no field for it in `CreateBookingInput`.

### Step 5 — Build the hold

```go
hold, err := domain.NewHold(courtID, userID, slot, price, teamID, note, now, s.holdWindow)
```

This produces an unsaved `Booking` with `Status: pending` and
`HoldExpiresAt: now + 15 minutes`. The booking is a *hold* — it blocks the slot
while the player pays, and lapses if they do not.

### Step 6 — Write it, with four layers of protection

`BookingRepo.CreateHold` (`internal/postgres/booking.go`) opens a transaction
and does four things in order:

**6a. Take an advisory lock keyed on this exact court and start time.**

```go
lockKey := b.CourtID.String() + "@" + b.Slot.Start.UTC().Format(time.RFC3339Nano)
tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 42))`, lockKey)
```

Two people going for the same slot now *queue* instead of racing. The second
one waits for the first to commit, then discovers the slot is taken and gets a
clean explanation. Without this, both would proceed and one would hit a raw
constraint violation — correct, but with a worse error.

The lock is transaction-scoped (`_xact_`), so it releases automatically on
commit or rollback. There is no unlock call to forget.

**6b. Release expired holds that overlap this slot.**

```sql
update bookings b
   set status = 'cancelled', hold_expires_at = null
 where b.court_id = $1
   and b.status = 'pending'
   and b.slot && $2
   and b.hold_expires_at <= now()
   and not exists (
     select 1 from payments p where p.booking_id = b.id and p.status = 'verified'
   )
```

Somebody may have started a checkout twenty minutes ago and wandered off. That
lapsed hold must not block a real booking. The `not exists` clause is the
safety catch: a hold with a verified payment behind it is somebody's game and
is never released, however old it looks.

**6c. Check for a conflict, so the error is human.**

```sql
select exists (
  select 1 from bookings b
   where b.court_id = $1 and b.slot && $2
     and booking_blocks_slot(b.status, b.hold_expires_at)
)
```

If this finds something, the player is told *"Someone took this slot moments
before you. Please pick another time."* rather than seeing a constraint name.

**6d. Insert — and let the constraint have the final word.**

The `no_double_booking` exclusion constraint guards this insert
unconditionally. If steps 6a–6c are all somehow wrong, the insert still fails.
That is the point of it; see the next section.

### Step 7 — The booking comes back

A `domain.Booking` with an ID, `pending` status, the server-resolved price, and
a 15-minute expiry. The player now has a quarter of an hour to pay.

---

## 5. The database

Eighteen tables plus a migration ledger, one view, six functions of ours, and
twelve triggers. Everything is stock PostgreSQL 14+; the only extensions are
`pgcrypto` (for `gen_random_uuid()`), `btree_gist` (required by the booking
constraint) and `citext` (case-insensitive email).

### 5.1 The double-booking guarantee

This is the most important paragraph in the document.

```sql
create table bookings (
  ...
  slot tstzrange not null,          -- [start, end)
  status booking_status not null default 'pending',

  constraint no_double_booking exclude using gist (
    court_id with =,
    slot     with &&
  ) where (status not in ('cancelled', 'no_show'))
);
```

**What an EXCLUDE constraint does.** A `UNIQUE` constraint says "no two rows
may have equal values here." An `EXCLUDE` constraint generalises that to any
operator. This one says: *no two rows may have an equal `court_id` **and**
overlapping `slot` ranges.* The `&&` operator is range overlap.

**Why this is different from checking in application code.** Any
check-then-insert has a window between the check and the insert. Two
transactions can both check (both see the slot free), then both insert. No
amount of careful Go closes that window — only the database, holding the index
during the write, can. The constraint is enforced inside the index update
itself, so there is no window at all.

This holds against:

- concurrent requests from different servers
- a retried request
- a future code path that forgets to check
- someone at a `psql` prompt
- a bug in `CreateHold` itself

**`tstzrange` is half-open: `[start, end)`.** The start instant is included,
the end instant is not. This is what makes 18:00–19:00 and 19:00–20:00
*adjacent* rather than overlapping. An arena that could not sell consecutive
hours to different teams would lose half its revenue, so this convention is
load-bearing. `domain.Slot.Overlaps` uses the same rule in Go:

```go
func (s Slot) Overlaps(other Slot) bool {
    return s.Start.Before(other.End) && other.Start.Before(s.End)
}
```

If those two definitions ever diverged, slots would render as free and then
refuse to book. Tests assert both.

**The `where` clause.** Cancelled and no-show bookings are excluded from the
constraint, so cancelling a booking frees its slot for instant rebooking.

**A bug this fixed.** In the schema we replaced, the constraint excluded only
`cancelled`, while the availability query excluded `cancelled` *and*
`no_show`. Those disagreed: a no-show slot rendered as free in the UI and then
threw a constraint violation when someone tried to book it. Here both go
through one function:

```sql
create function booking_blocks_slot(p_status booking_status, p_hold_expires_at timestamptz)
returns boolean
language sql stable parallel safe as $$
  select case
    when p_status in ('cancelled', 'no_show') then false
    when p_status = 'pending' then p_hold_expires_at > now()
    else true
  end;
$$;
```

and `domain.Booking.BlocksSlot` states the identical rule in Go for the code
paths that reason about it in memory.

### 5.2 Booking lifecycle and holds

```
                  create
                    │
                    ▼
            ┌───────────────┐   payment verified   ┌───────────┐
            │    pending    │─────────────────────►│ confirmed │
            │ (holds slot   │                      │ (holds    │
            │  until expiry)│                      │  forever) │
            └───────┬───────┘                      └─────┬─────┘
                    │                                    │
    hold expires,   │                                    │ game played
    or user cancels │                                    ▼
                    ▼                              ┌───────────┐
            ┌───────────────┐                      │ completed │
            │   cancelled   │◄─────────────────────┤           │
            │ (frees slot)  │      user cancels    └───────────┘
            └───────────────┘
                                    nobody turned up
                                            │
                                            ▼
                                     ┌────────────┐
                                     │  no_show   │
                                     │(frees slot)│
                                     └────────────┘
```

**Why holds exist.** A booking is created *before* payment, because the player
needs the slot reserved while they complete a gateway checkout. But an unpaid
reservation cannot hold a court forever, or an abandoned checkout removes that
hour from sale permanently.

**How expiry works.** `hold_expires_at` is a real column, set to
`now() + BOOKING_HOLD_WINDOW` (15 minutes by default) at creation and cleared
to `NULL` when the booking is confirmed. A `CHECK` constraint enforces that a
pending booking always has one:

```sql
constraint pending_holds_expire check (status <> 'pending' or hold_expires_at is not null)
```

**This is a change from the old schema**, which derived expiry as
`created_at + booking_hold_window()`, a constant returned by a SQL function.
Making it a column means expiry is indexable (the janitor has an index on
exactly the rows it needs), visible when you look at a row, and settable
per-booking — an arena could give a cash reservation a longer fuse than a
gateway checkout without a code change.

**Expiry takes effect immediately, not when a cleanup job runs.** Because
`booking_blocks_slot` compares against `now()`, a hold stops blocking its slot
the instant its window lapses. Availability is correct with no background job
running at all. The janitor (section 8.2) exists only to make the *stored*
state catch up — so a player's own list stops showing a booking they never
completed, and abandoned payment intents do not sit `initiated` forever.

That separation matters: correctness does not depend on a cron job. The old
system's stale-hold cleanup depended on the `pg_cron` extension being enabled
in a dashboard, which meant correctness depended on a deployment step nobody
could see from the code.

### 5.3 Counters kept honest by triggers

Three places store a count that could be derived by a query. Each is maintained
by a trigger and guarded by a `CHECK`, which turns the counter into a
concurrency-safe capacity limit for free.

**Tournament capacity.** `tournaments.team_count` with:

```sql
constraint within_capacity check (team_count <= max_teams)
```

and a trigger on `tournament_teams`:

```sql
update tournaments t
   set team_count = t.team_count + delta,
       status = case
                  when t.status not in ('open', 'full')    then t.status
                  when t.team_count + delta >= t.max_teams then 'full'::tournament_status
                  else                                          'open'::tournament_status
                end
 where t.id = target;
```

Why this is safe under concurrency: the `UPDATE` takes a row lock on the
tournament. Reading `t.team_count` *inside* the update re-reads it under that
lock, so two captains registering the last slot at the same instant are
serialised by the database. One succeeds; the other's increment pushes
`team_count` past `max_teams` and the `CHECK` rejects it.

This replaces the old system's advisory lock plus manual count. It is simpler,
and unlike a function-based approach it holds for *any* write path — including
a future admin tool nobody has written yet.

Note the `status` expression preserves `cancelled`, `ongoing` and `completed`
rather than flipping them back to `open`. Only the `open ⇄ full` pair is driven
by occupancy.

**Matchmaking fill.** `matchmaking_posts.filled_players` with
`check (filled_players <= needed_players)` and a trigger on
`matchmaking_responses` that moves it by a delta:

```sql
delta := case tg_op
           when 'INSERT' then (new.accepted)::int
           when 'DELETE' then -((old.accepted)::int)
           else               (new.accepted)::int - (old.accepted)::int
         end;
```

**This one had a real bug during development, worth recording.** The first
version recomputed the count with `select count(*) ... where accepted`. That
looks obviously correct and is not: the `COUNT` runs on its own snapshot, so
two authors accepting a player simultaneously would both count one existing
acceptance and both write `filled_players = 1`, losing an acceptance. The delta
form reads `p.filled_players` inside the `UPDATE`, under the row lock, which
serialises them properly.

**Arena rating.** `arenas.rating` and `arenas.review_count`, recomputed by a
trigger on `arena_reviews`. This one *is* a genuine recompute (an average
cannot be maintained by delta without also storing the sum), but it is cheap
and correct because the whole aggregate is written in one statement.

The reason to denormalise at all: every arena listing shows a rating. Computing
an average per arena while listing twenty arenas is exactly the per-row
subquery pattern this rewrite exists to eliminate.

### 5.4 Table reference

#### Identity

**`users`** — one row per account. Credentials and the public player card
together.

The old system split this across Supabase's `auth.users` and a `profiles`
table, joined on nearly every read, and it was possible for an account to exist
with no profile row. Merging them removes a join from most queries and makes
that inconsistency unrepresentable.

Credentials: `email` (citext, so case-insensitive-unique), `password_hash`,
`email_verified_at`. Player card: `username`, `full_name`, `account_type`
(`player` or `arena_owner`), `avatar_url`, `phone` (validated as a Nepali
mobile), `city`, `position`, `jersey_number`, `preferred_foot`, `skill`, `bio`.
Reputation: `matches_played`, `matches_won`, `community_score`, with
`check (matches_won <= matches_played)`.

`password_hash` is never in the column list the repository selects, except in
the single method that authenticates a login. It cannot leak through a handler
by accident because it is never loaded into the struct handlers see.

**`refresh_tokens`** — one row per live session. Stores `token_hash` (SHA-256
of the token, never the token), `expires_at`, `revoked_at`, and `replaced_by`,
which links a rotated token to its successor. See section 8.1.

**`verification_tokens`** — single-use tokens for password reset and email
verification, discriminated by a `purpose` enum. A partial unique index

```sql
create unique index verification_tokens_live_idx
  on verification_tokens (user_id, purpose) where consumed_at is null;
```

allows at most one live token per user per purpose, so requesting a second
reset link invalidates the first.

#### Venues

**`arenas`** — a venue. `owner_id`, `slug` (URL identifier), `area`, `city`,
optional `lat`/`lng`, `amenities` (a text array), `opens_at`/`closes_at` as
`time` columns holding Kathmandu wall-clock hours, and the denormalised
`rating`/`review_count`.

**`courts`** — a playable surface. `label` unique within its arena, `sport`,
`surface`, `side_count`, and `base_price` — the hourly fallback when no pricing
rule matches.

**`pricing_rules`** — peak/off-peak windows. `days` is an `int[]` of ISO
weekdays (1 = Monday … 7 = Sunday), `start_hour`/`end_hour` a half-open range,
`price_npr`, `is_peak`, and `priority` for resolving overlaps.

#### Bookings and money

**`bookings`** — covered above.

**`payments`** — one row per attempt to settle a booking. `transaction_uuid` is
ours and is what we hand the gateway; `provider_ref` is the gateway's own
identifier. Three uniqueness rules matter:

```sql
transaction_uuid text not null unique
create unique index payments_provider_ref_idx on payments (provider, provider_ref)
  where provider_ref is not null
create unique index payments_one_verified_per_booking on payments (booking_id)
  where status = 'verified'
```

The first two make a replayed gateway callback unable to create a second
payment. The third makes it impossible to collect twice for one booking — a
database-level guarantee, not a hope about callback handling.

There is also `check ((status = 'verified') = (verified_at is not null))`,
which keeps the timestamp and the status from disagreeing.

#### Community

**`teams`** — `name`, `tag` (2–5 uppercase alphanumerics, e.g. `KTM`),
`captain_id`, `join_code` for invite links (rotatable, so a leaked code can be
retired without disbanding the team).

**`team_members`** — roster, keyed `(team_id, user_id)`, with a partial unique
index enforcing exactly one captain per team.

**`matches`** — a recorded result. `verified` means both captains confirmed;
only verified matches count toward standings.

**`team_standings`** — a view: three points a win, one a draw, goal difference
as tiebreaker, ranked.

**`tournaments`** / **`tournament_teams`** — brackets and registrations. Prize
splits are an `int[]` of percentages with
`check (int_array_sum(prize_split) = 100)`, which needs `int_array_sum` to be
`IMMUTABLE` so it can appear in a constraint.

**`matchmaking_posts`** / **`matchmaking_responses`** — the find-a-player
board. A post either opens an existing booking (`booking_id` set) or calls for
a pickup game with no court yet (`booking_id` null). A partial unique index
gives each booking at most one post.

**A column that was deliberately deleted:** the old schema had
`bookings.open_to_join`, a boolean mirroring whether a matchmaking post
existed. Two sources of truth for one fact drift the moment any write path
updates one and not the other — and one had: cancelling a booking updated the
post, but a post expiring never cleared the flag. It is derived now, in the
`ListForUser` query:

```sql
(mp.id is not null and mp.status = 'open') as open_to_join
```

**`arena_reviews`**, **`arena_photos`**, **`profile_highlights`** — one review
per player per arena (enforced by a unique constraint), gallery images, and
player highlight-reel links.

---

## 6. The domain layer

`internal/domain` holds the vocabulary of the business. No database, no HTTP,
no logger. Roughly 60% of its statements are covered by tests that run in
milliseconds.

### 6.1 The error model

Every error crossing a layer boundary is a `*domain.Error`:

```go
type Error struct {
    Code    Code       // what the caller should do about it
    Message string     // written for a human, safe to send over the wire
    Field   string     // which input was wrong, for validation failures
    Fields  []*Error   // all of them, when a form was wrong in several places
    cause   error      // for the log, never for the client
}
```

**Codes classify by remedy, not by origin:** `invalid`, `unauthenticated`,
`forbidden`, `not_found`, `conflict`, `rate_limited`, `unavailable`,
`internal`. The future HTTP layer maps these to status codes in one place; no
handler needs to know which business rule produced one.

**Messages are written for the person who will read them.** Not
`"SLOT_TAKEN"` but `"Someone took this slot moments before you. Please pick
another time."` That copy lives next to the rule that produces it, so it stays
accurate when the rule changes.

**Unclassified errors are treated as defects, which is the safe default:**

```go
func UserMessage(err error) string {
    var e *Error
    if errors.As(err, &e) && e.Code != CodeInternal {
        return e.Message
    }
    return "Something went wrong on our end. Please try again."
}
```

A raw driver error — say `pq: password authentication failed for user "khel"` —
collapses to the generic message, because nobody decided it was safe to show. A
test asserts this specifically. The detail still reaches the log through the
wrapped cause.

**Validation accumulates.** Filling in a signup form and being told about one
mistake at a time is miserable, so `Validation` collects every failure:

```go
v := &Validation{}
v.Check(nameLen >= 2, "name", "Give the team a name.")
v.Check(teamTagPattern.MatchString(t.Tag), "tag", "The tag should be 2 to 5 letters or numbers, like KTM.")
return v.Err()
```

`Err()` returns `nil`, or a single field error, or a combined error whose
`FieldErrors()` exposes each one so a client can attach messages to the right
controls.

### 6.2 Time, slots and wall clocks

This section describes the subtlest part of the system.

**Kathmandu is UTC+05:45.** Not a whole number of hours. Any code that reasons
about local time by adding hours to UTC is wrong here in a way it would not be
in most timezones — which makes this an excellent place for latent bugs.

**Two kinds of time, kept distinct by the type system:**

`domain.Slot` holds two absolute instants, always normalised to UTC. This is
when a booking actually happens.

`domain.DayTime` is a wall-clock time of day with no date — `{Hour: 6}` means
"six in the morning", whatever that is in UTC on a given day. Arena opening
hours are `DayTime`, matching the Postgres `time` column they come from.

Hour 24 is legal in `DayTime` and means end-of-day, so an arena can close at
midnight without the closing time sorting before the opening time.

**Why opening hours must be compared in local time.** An arena open
"06:00–22:00" means those hours on the clock on the wall. At UTC+05:45, a 06:00
Kathmandu start is **00:15 UTC the same day**. Comparing that instant's UTC
hour (0) against an opening hour of 6 would reject the first slot of every day.
There is a test that asserts exactly this, including the inverse — that reading
the same slot in UTC *does* reject it, confirming the zone is what decides:

```go
func TestOperatingHoursUseLocalWallClock(t *testing.T) { ... }
```

**The same applies to pricing rules.** A "Saturday Premium" rule is about
Saturday in Kathmandu. At 00:30 Saturday local time it is still 18:45 *Friday*
in UTC — so a rule evaluated in the wrong zone applies on the wrong day
entirely. Tested.

**ISO weekday conversion.** Postgres stores rule days as ISO numbers (Monday =
1 … Sunday = 7). Go's `time.Weekday` counts Sunday = 0. Getting this wrong
shifts every pricing rule by one day, which is the kind of bug that produces
puzzled support emails rather than crashes. The conversion lives in exactly one
place — `ISOWeekday` and `WeekdayFromISO` — and is round-trip tested.

**Slot rules:** minimum 30 minutes, maximum 4 hours, and a one-minute
`PastSlotGrace` so that a player clicking the 18:00 slot at 17:59:59.8 is not
rejected for a request that was in flight across the boundary.

**Billable hours round up:** `Hours()` returns 2 for a 90-minute booking,
because pricing rules are quoted per hour.

### 6.3 Pricing

Pure functions, exhaustively tested, no database involvement.

```go
func ResolvePrice(base int, rules []PricingRule, slot Slot, loc *time.Location) Price
```

**The algorithm:**

1. Find every rule covering the slot's **start** hour, in the arena's local zone.
2. Pick a winner.
3. Multiply that rate by the billable hours.

**Why the start hour decides the whole booking.** An 18:00–20:00 booking that
begins in the evening peak is charged at the peak rate throughout, rather than
being split across rate boundaries. That is how arenas actually quote a court,
and splitting would produce prices nobody could verify by hand.

**Tie-breaking is deterministic**, which matters more than which rule wins:

```go
func beats(candidate, incumbent PricingRule) bool {
    if candidate.Priority != incumbent.Priority {
        return candidate.Priority > incumbent.Priority   // 1. higher priority
    }
    if candidate.span() != incumbent.span() {
        return candidate.span() < incumbent.span()        // 2. narrower window
    }
    return candidate.PriceNPR > incumbent.PriceNPR        // 3. higher price
}
```

Without rules 2 and 3, two rules at equal priority would resolve by whichever
row the database returned first — meaning the price of a slot could change
between two identical requests. Tested by resolving the same pair of rules in
both orders and asserting the same answer.

The narrower window winning a tie is the intuitive outcome: a two-hour "Friday
Prime" rule beats an all-day "Weekday Evening" rule laid over it.

### 6.4 Booking rules

Authorization decisions are methods on the entity, so the rule and its
explanation live together:

```go
func (b Booking) CanBeCancelledBy(userID uuid.UUID, now time.Time) error {
    if !b.IsHeldBy(userID) {
        return NotFound("That booking doesn't exist.")
    }
    ...
}
```

**Note that a stranger gets `not_found`, not `forbidden`.** Telling someone
"you are not allowed to cancel this booking" confirms the booking exists.
Returning "that booking doesn't exist" reveals nothing. This is applied
consistently and is asserted by a test whose name says why.

`domain.NewHold` is the constructor. It takes `now` and `holdWindow` as
*arguments* rather than reading the clock, so time is an input to the rule and
every time-dependent behaviour is testable without waiting.

### 6.5 Payment rules

`Payment.Verify` is the most security-sensitive function in the codebase,
because a gateway callback is reachable by anyone on the internet.

```go
func (p *Payment) Verify(result GatewayResult, now time.Time) (confirmBooking bool, err error)
```

It applies four rules in order:

**1. A settled payment ignores the callback.** Gateways retry, and a player can
refresh the success page. Neither may change a payment that already resolved,
and neither may move `verified_at`.

**2. A callback for a different transaction is refused.** Applying it would
settle the wrong booking.

**3. An unverified result marks the payment failed** — and keeps the raw
response and gateway reference as evidence.

**4. The amount must match.** This is the one that matters:

```go
if result.AmountNPR != p.AmountNPR {
    p.Status = PaymentFailed
    return false, Conflict("The amount paid (NPR %d) doesn't match this booking (NPR %d)...", ...)
}
```

Without this check, someone pays NPR 1 for an NPR 2,000 court and hands us a
**genuine, correctly signed** gateway confirmation. Signature verification does
not help — the signature is real. Only comparing what was paid against what was
owed catches it. There is a test named
`TestPaymentVerifyRejectsAnUnderpayment` whose comment says exactly this.

The amount itself comes from the booking, never the client:

```go
func NewPaymentIntent(b Booking, provider PaymentProvider) (Payment, error) {
    ...
    AmountNPR: b.PriceNPR,
}
```

---

## 7. The storage layer

`internal/postgres` holds every line of SQL in the service.

### Connection pooling

`Connect` configures a `pgxpool` and pins two session parameters:

```go
poolCfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
poolCfg.ConnConfig.RuntimeParams["application_name"] = "khel-arena"
```

Pinning UTC means a server-local timezone setting can never silently
reinterpret a timestamp. Kathmandu conversion is explicit at the few places
that need it.

**A bug found while writing the tests:** the config originally copied
`MaxConnLifetime` straight from config, and a zero value made pgx expire every
connection instantly, failing every acquire with a confusing message. Zero now
means "no opinion — use the driver default":

```go
if cfg.MaxConnLifetime > 0 {
    poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
}
```

### Transactions

```go
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error
```

Commits if `fn` returns nil, rolls back otherwise, and rolls back on panic
before letting it unwind. Every multi-statement invariant goes through it, so
no caller has to remember the rollback path.

One subtlety: the rollback uses `context.WithoutCancel(ctx)`. If `fn` failed
*because* the context was cancelled, a rollback on the cancelled context would
never reach the server.

### Error translation

Repositories convert Postgres SQLSTATE codes into domain errors, so nothing
above this layer switches on `23505`:

```go
if isUniqueViolation(err) {
    switch pgErrConstraint(err) {
    case "users_email_key":    return domain.Conflict("An account with that email already exists.")
    case "users_username_key": return domain.Conflict("That username is taken.")
    }
}
```

Naming the constraint is what lets one generic violation become two useful
messages.

### The `DB` interface

```go
type DB interface {
    Exec(...) (pgconn.CommandTag, error)
    Query(...) (pgx.Rows, error)
    QueryRow(...) pgx.Row
}
```

Both `*pgxpool.Pool` and `pgx.Tx` satisfy this, so a repository method can run
either standalone or inside a caller's transaction without knowing which.
`BookingRepo.Confirm` takes a `DB` for exactly this reason — confirming a
booking must happen in the same transaction as verifying its payment.

### Query discipline

**Column lists are always explicit.** No `select *` anywhere. Adding a column
cannot silently change what a scan expects, and `password_hash` cannot appear
in a result set by accident.

**Checks live in the `WHERE` clause where possible.** `Cancel` does not load
the booking, check ownership, then update — that leaves a window where the
booking can change between check and write. Instead:

```sql
update bookings set status = 'cancelled', hold_expires_at = null
 where id = $1 and user_id = $2 and status in ('pending', 'confirmed')
```

If zero rows change, *then* it reads the row to work out which condition failed
and produce a specific message — without revealing the existence of a booking
belonging to someone else.

### Repositories that exist

- **`BookingRepo`** — `CreateHold`, `ByID`, `Cancel`, `Confirm`,
  `ListForUser`, `ReleaseStaleHolds`, plus `LoadCourtContext` and
  `BookedRanges` for availability.
- **`UserRepo`** — `Create`, `ByID`, `ByUsername`, `CredentialsByEmail`,
  `UpdatePassword`, `UpdateProfile`, `MarkEmailVerified`.
- **`SessionRepo`** — refresh token storage and rotation, verification tokens.

`UpdateProfile` deserves a note. Partial updates use `coalesce($n, column)` so
a nil argument leaves the column alone, keeping it one statement rather than
SQL assembled per request. But `coalesce` cannot express "set this to null",
and clearing your playing position is something a player must be able to do. So
the three nullable enum fields carry an explicit "was this mentioned" flag:

```sql
position = case when $8::boolean then $9::futsal_position else position end
```

---

## 8. The service layer

Use cases. Each one reads as the sequence of decisions it actually makes.

`BookingService` is covered in section 4. Two things remain.

### 8.1 Authentication

We own identity end to end. There is no external auth provider.

#### Password hashing — Argon2id

```go
const (
    argonMemoryKiB  uint32 = 64 * 1024   // 64 MiB
    argonIterations uint32 = 3
    argonKeyLength  uint32 = 32
    argonSaltLength        = 16
)
var argonParallelism = uint8(min(runtime.NumCPU(), 4))
```

**Why Argon2id rather than bcrypt.** It is memory-hard. An attacker with GPUs
cannot cheaply trade silicon for memory bandwidth, which is exactly the
advantage they have against bcrypt. These are the RFC 9106 second recommended
parameters — about 50ms per hash, which is negligible for one login and ruinous
for an offline crack.

**Hashes are self-describing.** The stored form is a PHC string:

```
$argon2id$v=19$m=65536,t=3,p=4$<base64 salt>$<base64 hash>
```

Because the parameters travel with the hash, they can be raised later without
invalidating anyone's password. `NeedsRehash` reports which stored hashes are
behind, and `Login` upgrades them — a successful login is the only moment the
plaintext exists to re-hash with:

```go
if token.NeedsRehash(creds.PasswordHash) {
    if rehashed, err := token.HashPassword(password); err == nil {
        _ = s.users.UpdatePassword(ctx, user.ID, rehashed)  // best effort
    }
}
```

Failing to upgrade must never fail the login, hence the discarded error.

**Comparison is constant-time** (`subtle.ConstantTimeCompare`). A byte-by-byte
comparison that returns early leaks, through timing, how much of a guess was
right.

**Password rules are length-only:** minimum 10 characters, maximum 256.
Composition rules ("one uppercase, one symbol") push people toward predictable
substitutions like `Password1!` and are deliberately absent. The maximum exists
because Argon2 hashes the entire input — an unbounded password is an invitation
to spend 64 MiB of server memory per request on someone else's behalf.

#### Login does not leak which emails are registered

```go
creds, err := s.users.CredentialsByEmail(ctx, email)
switch {
case errors.Is(err, domain.ErrNotFound):
    _ = token.VerifyPassword(password, token.DummyHash)   // spend the same time
    return Session{}, domain.Unauthenticated(rejection)
```

Two defences, both necessary:

1. **The same message** for a wrong password and an unknown account. A
   different message is a free account-enumeration oracle.
2. **The same work.** Returning early without hashing would answer the same
   question through response time — an unknown email replies in 2ms, a real one
   in 50ms. `DummyHash` is a real Argon2id hash of a value nobody knows,
   computed once at startup, purely to make the timing match.

#### Access tokens — JWT, HS256, pinned

Claims carry only the user ID and account type. Anything that can change under
a live session — a username, a ban — is read from the database where it
matters, not trusted from a token that may be 15 minutes stale.

```go
jwt.ParseWithClaims(tokenString, &claims,
    func(t *jwt.Token) (any, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("%w: unexpected signing method %v", ErrTokenInvalid, t.Header["alg"])
        }
        return i.secret, nil
    },
    jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
    jwt.WithIssuer(i.issuer),
    jwt.WithExpirationRequired(),
)
```

**The signing method is pinned twice.** Accepting whatever the token's own
header names is the classic JWT forgery: a token declaring `"alg": "none"`
verifies against any key. There is a test that constructs exactly such a token
and asserts it is rejected. The issuer is also checked, so a token minted by a
sibling service sharing the secret cannot authenticate here.

Access tokens live 15 minutes by default, which is what makes it acceptable to
verify them without a database round-trip.

#### Refresh tokens — opaque, rotating, theft-detecting

Refresh tokens are **not** JWTs. A JWT is valid until it expires and cannot be
withdrawn; a refresh token lives 30 days, so it must be revocable.

They are 32 random bytes, base64url-encoded. Only a SHA-256 digest is stored:

```go
func HashRefreshToken(plaintext string) []byte {
    sum := sha256.Sum256([]byte(plaintext))
    return sum[:]
}
```

Plain SHA-256 is correct here, unlike for passwords: the token already has 256
bits of entropy, so there is nothing to guess. Slow hashing would only cost the
server on every refresh.

**Rotation is single-use.** Every refresh issues a new token and marks the old
one revoked, pointing `replaced_by` at its successor — both in one transaction,
so there is never an instant where both work or neither does.

**Presenting an already-rotated token means theft.** Two parties hold that
token, and there is no way to tell the thief from the victim. So the entire
chain is burned:

```go
if revokedAt != nil {
    // Reuse of a rotated token. Burn every session this user holds.
    update refresh_tokens set revoked_at = now() where user_id = $1 and revoked_at is null
    return domain.Unauthenticated("Your session has expired. Please sign in again.")
}
```

Making the victim log in again is much better than leaving the thief's session
alive.

The lookup uses `SELECT ... FOR UPDATE`, so two simultaneous refreshes with the
same token cannot both succeed.

#### Password reset

`BeginPasswordReset` returns a token only if the address is registered, and
reports success either way — otherwise the endpoint becomes an account
enumerator. The caller sends the email if there is a token and says the same
thing to the user regardless.

Reset links last one hour, because a link sitting in an inbox is a bearer
credential.

`UpdatePassword` revokes every session in the same transaction as it sets the
new hash. Someone resetting a password is often locking an intruder out;
leaving that intruder's sessions alive would defeat the point.

### 8.2 The janitor

A background goroutine that sweeps every minute:

```go
func (j *Janitor) Run(ctx context.Context) {
    ticker := time.NewTicker(j.interval)
    defer ticker.Stop()
    j.sweep(ctx)   // once at startup
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:   j.sweep(ctx)
        }
    }
}
```

It releases expired unpaid holds (marking them cancelled, failing their
abandoned payment intents, closing their community posts) and deletes refresh
tokens nobody can use.

**It sweeps once at startup**, so a restart after downtime does not wait a full
interval to clear what accumulated.

**Each pass is bounded** by a 30-second timeout, so a slow query cannot stall
every later tick. A failed sweep is logged and the loop continues — nothing
downstream depends on any single pass succeeding, because availability is
already correct without it.

**Revoked tokens are retained for seven days** rather than deleted immediately,
so that theft detection still has something to match when a stolen token is
presented shortly after rotation.

This replaces the old system's `pg_cron` job, moving a correctness-adjacent
concern from "an extension somebody enabled in a dashboard" to "part of the
service". If the service runs, the sweeping runs.

---

## 9. Migrations

Numbered `NNNN_name.sql` under `internal/postgres/migrations`, embedded into
the binary with `go:embed`. The binary carries its own schema; there is no
directory to ship alongside it.

**Each migration runs in its own transaction**, so a failure leaves the
database at the last good version rather than half-applied.

**A session advisory lock serialises concurrent migrators.** Two instances
starting simultaneously will not both apply migration 5; the loser waits, then
finds nothing to do.

**Applied migrations are checksummed.** Editing one that has already run is a
deployment error — the database cannot be brought to the state the code now
expects — so it is reported loudly rather than skipped silently:

```
migration 0006_tournaments was modified after it was applied
(recorded 489925236558, embedded 3cc82e137b96); write a new migration instead
```

This was verified during development by deliberately editing an applied
migration and confirming the refusal.

The eight migrations: `0001_foundation` (extensions, enums, helpers),
`0002_users`, `0003_arenas`, `0004_bookings`, `0005_teams`,
`0006_tournaments`, `0007_matchmaking`, `0008_arena_reviews`.

---

## 10. Configuration

Everything comes from the environment, read once at startup and validated
eagerly. A misconfigured service should fail to boot with a clear message, not
fail on the first request that happens to touch the missing setting.

`config.Load` reports **every** problem at once rather than making the operator
rerun to discover the next one:

```
invalid configuration:
  - DATABASE_URL is required (postgres://user:pass@host:5432/khel_arena)
  - JWT_SECRET must be at least 32 bytes, got 12
```

Settings: `APP_ENV`, `HTTP_ADDR`, `DATABASE_URL` and pool sizing, `JWT_SECRET`
(minimum 32 bytes), `JWT_ISSUER`, `ACCESS_TOKEN_TTL` (15m),
`REFRESH_TOKEN_TTL` (720h), `BOOKING_HOLD_WINDOW` (15m), and `ARENA_TIMEZONE`
(`Asia/Kathmandu`), which is validated by actually loading the zone.

See `.env.example` for the annotated list.

---

## 11. Testing

Two tiers, split by whether they need a database.

### Unit tests — fast, no dependencies

`internal/domain` (57% coverage) and `internal/platform/token` (88%). These run
in about a second with nothing installed.

What they cover is chosen by consequence, not by chasing a coverage number:
slot overlap including the adjacency boundary, price resolution across
timezones and rule priorities and tie-breaks, hold expiry at the exact
boundary, payment verification including the underpayment attack, JWT forgery
including `alg: none`, password hashing and rehash detection, and the guarantee
that internal errors do not leak detail.

### Integration tests — against a real Postgres

`internal/postgres` (40%). These **skip** when `TEST_DATABASE_URL` is unset, so
`go test ./...` stays useful on a machine with no database.

There is no fake database, on purpose. What is under test *is* the database's
behaviour — an exclusion constraint, an advisory lock, and what happens when
transactions collide. A stub would only prove the stub agrees with itself.

**The test that justifies the architecture:**

```go
func TestConcurrentBookingsCannotDoubleBook(t *testing.T) {
    const contenders = 20
    // ... 20 goroutines held at a sync.WaitGroup gate, released at once,
    //     all attempting the same court and the same hour
    if wins != 1 { t.Errorf("%d bookings succeeded, want exactly 1", wins) }
    if n := f.countLiveBookings(t, slot); n != 1 {
        t.Errorf("the database holds %d live bookings for this slot, want exactly 1", n)
    }
}
```

Twenty goroutines released simultaneously; exactly one wins, nineteen are told
the slot is taken, and the database is then queried directly to confirm it
holds exactly one live booking. It passes under `-race`.

Others cover overlap in all four geometries (identical, starts-inside,
ends-inside, containing), adjacency being allowed, cancellation freeing a slot,
ownership enforcement returning `not_found` to strangers, expired holds
releasing, **paid** holds never releasing, and the janitor failing abandoned
payment intents.

Each test seeds its own arena, court and players with a unique suffix and
cleans up after itself, so tests do not collide.

### Running them

```sh
make test               # unit only, no database
make test-integration   # everything
make test-race          # everything, under the race detector
make check              # what CI runs: tidy + vet + race
```

CI runs Postgres 17 as a service container and executes the full suite with
`-race`.

---

## 12. What is not built yet

Being explicit, so nothing here reads as more finished than it is.

**The HTTP API.** No `cmd/api`, no routing, no handlers, no middleware. The
services are ready to be called; what is missing is transport — routing, JSON
encoding, an auth middleware reading the `Authorization` header, and the
mapping from `domain.Code` to HTTP status codes. That mapping is the reason
`Code` exists.

**Repositories for teams, tournaments, matchmaking and arena management.**
Their tables exist and are tested at the SQL level; their domain types and
rules exist and compile. What is missing is the storage code between them.

**Payment gateway adapters.** `domain.Payment` has the security-critical logic
— including the amount check that stops a signed NPR 1 payment settling an NPR
2,000 booking — but the eSewa HMAC signing and status-check calls, and the
Khalti equivalents, are not ported to Go. There is also no `PaymentRepo` yet.

These callbacks are attacker-reachable, so they deserve focused attention
rather than being folded into general API work.

**Email delivery.** `BeginPasswordReset` returns a token for the caller to
send; nothing sends it yet.

**Rate limiting.** `CodeRateLimited` exists in the error model; nothing
produces it. Login and password reset are the endpoints that will need it.

---

## 13. File map

```
cmd/
  migrate/main.go              Applies pending migrations and exits

internal/domain/               No external dependencies. Pure rules.
  errors.go                    Error codes, messages, validation accumulator
  slot.go                      Slot, DayTime, operating-hours logic
  pricing.go                   PricingRule, ResolvePrice, ISO weekday conversion
  availability.go              BuildGrid — projects the bookable-hours matrix
  booking.go                   Booking, BookingDetail, NewHold, BlocksSlot
  payment.go                   Payment, Verify, NewPaymentIntent
  user.go                      User, Credentials, Registration, ProfileUpdate
  arena.go                     Arena, Court, Review, Photo, Slugify
  team.go                      Team, Member, Match, Standing
  tournament.go                Tournament, prize splits, registration window
  matchmaking.go               Call, Response
  enums.go                     Every enum, each self-validating
  *_test.go                    ~1,400 lines of tests, no database required

internal/postgres/             The only package that imports pgx
  db.go                        Pool, InTx, SQLSTATE classification
  migrate.go                   Embedded, checksummed migration runner
  booking.go                   CreateHold and the rest of the booking queries
  availability.go              LoadCourtContext (batched), BookedRanges
  user.go                      Account queries
  session.go                   Refresh and verification tokens
  migrations/*.sql             0001 … 0008
  *_test.go                    Integration tests; skip without TEST_DATABASE_URL

internal/service/
  booking.go                   Create, Availability, Cancel, ListMine
  auth.go                      Register, Login, Refresh, Logout, password reset
  janitor.go                   Background sweeper

internal/platform/
  config/config.go             Environment loading with eager validation
  token/password.go            Argon2id hashing, timing-safe comparison
  token/jwt.go                 Access tokens, refresh token generation
```

---

## Appendix: quick reference

**Where is the double-booking guarantee?**
`internal/postgres/migrations/0004_bookings.sql`, the `no_double_booking`
constraint. Everything else about booking is there to produce a good error
message.

**Where does a price get decided?**
`domain.ResolvePrice`, called from `BookingService.Create`. Never from a client.

**Where is a booking's slot decided as taken or free?**
`booking_blocks_slot` in SQL, `Booking.BlocksSlot` in Go. They state the same
rule and must stay in agreement.

**Why do strangers get "not found" instead of "forbidden"?**
Because "forbidden" confirms the thing exists.

**Why is `hold_expires_at` a column instead of a computed value?**
So expiry is indexable, visible, and settable per booking.

**Why is there no RLS?**
Because no untrusted client talks to this database. The Go service is the only
client, and it makes the authorization decisions in readable code.
