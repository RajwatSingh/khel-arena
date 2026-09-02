# Khel Arena

Futsal and sports arena booking for Kathmandu. Go service over stock
PostgreSQL, with a SvelteKit frontend in `web/`.

This is a rewrite. The previous version was a Next.js app talking to Supabase
from server actions; it is in the git history, not in this tree.

## Why it looks like this

**Bookings cannot overlap, and application code cannot opt out of that.** A
`tstzrange` column plus a GiST `EXCLUDE` constraint makes two live bookings on
one court physically impossible — under concurrency, under retries, under a
buggy deploy, or at a `psql` prompt. Everything above that constraint exists to
produce a good error message, not to provide the guarantee.

**One connection pool, not one HTTP round-trip per query.** The old stack
reached the database through PostgREST and re-verified the session against
Supabase Auth on every server action, so a page that showed five things cost
five or more network round-trips to a remote service. Here, `pgx` holds pooled
connections and caches prepared statements, and a signed access token is
verified in-process without touching the database.

**Reads are joined, not looped.** "My bookings" is one query with the court and
arena joined in. The availability grid is two queries and a projection in Go,
where the old version ran a correlated price lookup per hour inside SQL.

**Business rules live in Go, in one place.** Supabase pushed authorization into
RLS policies and writes into `SECURITY DEFINER` functions because untrusted
browsers talked straight to the database. A Go service is a trusted client, so
the rules are ordinary, readable, testable code. The database keeps the
invariants only it can enforce.

## Layout

```
cmd/api/              The HTTP service: config, wiring, graceful shutdown
cmd/migrate/          Applies migrations and exits
internal/
  domain/             Entities, value objects, rules. No database, no HTTP.
  postgres/           Every line of SQL, and the pgx repositories that run it
    migrations/       Numbered, checksummed, embedded in the binary
  service/            Use cases: booking, auth, profile, the background janitor
  api/                HTTP transport: routing, middleware, DTOs, error mapping
  platform/
    config/           Environment loading, validated eagerly at startup
    token/            Argon2id password hashing, JWT and refresh tokens
web/                  SvelteKit frontend, plain JavaScript — see web/README.md
```

Dependencies point inward. `domain` imports nothing of ours; `service` depends
on `domain` and on interfaces it declares itself; only `postgres` knows pgx
exists. `api` depends on `service` and `domain`, and on `postgres` for exactly
one type — `SessionContext`, which the service layer's own signatures require.

## The API

Seventy-six endpoints.
Errors are one envelope (`{"error": {code, message, fields}}`) mapped from
`domain.Code` in a single place, so no handler picks a status itself.

| Method | Path | |
|---|---|---|
| GET | `/healthz` | Liveness — the process, nothing else |
| GET | `/readyz` | Readiness — 503 while the database is unreachable |
| POST | `/v1/auth/register` | Returns a session, 201 |
| POST | `/v1/auth/login` | Returns a session |
| POST | `/v1/auth/refresh` | Rotates the token pair |
| POST | `/v1/auth/logout` | 204, idempotent |
| POST | `/v1/auth/password/forgot` | Always 202, never returns the token |
| POST | `/v1/auth/password/reset` | 204, burns the token |
| GET | `/v1/arenas` | The venue index |
| GET | `/v1/arenas/{slug}` | One venue, its courts and pricing rules |
| GET | `/v1/areas` | Neighbourhoods with something bookable |
| GET | `/v1/ledger` | Every court in the city for one day |
| GET | `/v1/courts/{courtID}/availability` | `?date=YYYY-MM-DD` |
| GET | `/v1/payments/providers` | Gateways this deployment is configured for |
| GET | `/v1/payments/{provider}/callback` | Where a gateway returns the payer |
| POST | `/v1/bookings/{bookingID}/checkout` | Authenticated, starts a payment |
| GET | `/v1/bookings/{bookingID}/payment` | Authenticated, latest attempt |
| — | `/v1/owner/*` | 11 routes: venues, courts, pricing, reconciliation |
| PUT | `/v1/owner/arenas/{id}` | Replaces — see below |
| — | `/v1/teams/*` | 10 routes: squads, rosters, invite codes |
| — | `/v1/calls/*` | 9 routes: the pickup-game board |
| — | `/v1/tournaments/*` | 7 routes: brackets and entries |
| GET | `/v1/standings` | The table — verified results only |
| — | `/v1/matches/*` | Report a result, confirm it, withdraw it |
| — | `/v1/arenas/{id}/reviews`, `/photos` | Reviews and galleries |
| — | `/v1/me/highlights` | Clips on your own player card |
| GET | `/v1/me` | Authenticated |
| POST | `/v1/auth/password/change` | Authenticated |
| POST | `/v1/bookings` | Authenticated, takes a hold, 201 |
| GET | `/v1/bookings` | Authenticated, `?limit=`, newest first |
| DELETE | `/v1/bookings/{bookingID}` | Authenticated, 204 |

Access tokens travel in `Authorization: Bearer`; the refresh token travels
only in an httpOnly cookie, so a script that can read `localStorage` cannot
read it. The booking price and the booking's owner are both resolved
server-side and never taken from a request body.

`/v1/auth/login` and `/v1/auth/password/forgot` are rate limited -- five
attempts, then one every twelve seconds, per client address. It is an
in-process token bucket, so it does not survive running more than one
instance; that is the point at which it wants a shared counter rather than
this.

## Payments

**A redirect back from a gateway is not evidence of payment.** It arrives in
the player's browser, over a URL the player can edit. Every adapter treats it
as nothing more than a hint about *which* transaction to ask about, then asks
the gateway directly, server to server, over a connection the player is not
part of. `Verified` is set from that answer and from nothing else.

The second guard is the amount: `domain.Payment.Verify` compares what the
gateway says was paid against what the booking owed, so a correctly signed
confirmation for NPR 1 against an NPR 1,800 court is refused and recorded as
failed. Between the two, an adapter would have to be extraordinarily wrong
before a free booking came out of it.

Settlement writes the payment and confirms the booking in one transaction. A
payment recorded as verified while its booking stays pending is a player who
has been charged and whose hour the janitor releases fifteen minutes later; a
booking confirmed against a payment we failed to record is an hour given away.
No ordering of two separate writes avoids both, so they are one write.

Providers are configured, not compiled in: a gateway whose credentials are
absent is not offered, and `/v1/payments/providers` reports only what will
actually work. Cash is always present and never verifies anything -- settling
at the arena is confirmed by the venue, which is owner-facing work that does
not exist yet.

## Running it

Requires Go 1.24+ and PostgreSQL 14+ (for `pgcrypto`, `btree_gist`, `citext` —
the first migration creates them).

```sh
cp .env.example .env          # then set JWT_SECRET
make db-up                    # local Postgres in Docker, both databases
make migrate
make check                    # tidy + vet + tests under -race
make run                      # the API on :8080
```

With the API up, `make web` serves the frontend on :5173 and proxies `/v1` to
it, so both sides share an origin and the refresh cookie works as configured.
A quick check that it is alive:

```sh
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
make smoke              # walk every endpoint against the running service
```

`scripts/smoke.sh` signs up, signs in, reads the availability grid, takes a
hold, fails to double-book it, lists it and cancels it — asserting a status on
each step and exiting non-zero if any of them is wrong, so it works as a deploy
gate as well as a thing to run by hand. It finds a court through
`DATABASE_URL`, or takes one as `COURT_ID`, and skips the booking half with a
note if it has neither.

`make help` lists the rest.

## Tests

Unit tests need nothing and cover the rules worth being sure about: slot
overlap, price resolution across timezones and rule priorities, hold expiry,
payment verification, and token handling.

Integration tests need a database and skip without `TEST_DATABASE_URL`, so
`go test ./...` stays useful on a machine with no Postgres. They cover what
only a real database can demonstrate — most importantly
`TestConcurrentBookingsCannotDoubleBook`, where twenty goroutines contend for
one court-hour and exactly one may win.

```sh
make test              # unit only
make test-integration  # everything
make test-race         # everything, under the race detector
make web-test          # the frontend's unit tests
```

The frontend has its own suite under vitest, covering `lib/api/client.js` --
the transport, where the behaviour worth pinning is invisible by hand: an
expired access token being refreshed and the call replayed once, four
simultaneous 401s sharing a single refresh rather than racing to rotate the
token four times, and a server-side call refusing to run without the `fetch`
that `load()` provides. It stubs fetch, so it needs nothing running.

Whether the server actually agrees with any of that is settled by
`scripts/smoke.sh` against a live `cmd/api` -- the same split as the Go suite,
where unit tests need no database and the integration ones do.

## Migrations

Numbered `NNNN_name.sql` under `internal/postgres/migrations`, embedded into
the binary with `go:embed`. Each runs in its own transaction; a session
advisory lock keeps two instances from racing at startup.

Applied migrations are checksummed. Editing one that has already run is a
deployment error — the database cannot be brought to the state the code now
expects — so it is reported rather than silently skipped. Write a new
migration instead.

## Notes on the schema

A few deliberate departures from what it replaces:

- **Credentials and the player card are one row.** The old split between
  Supabase's `auth.users` and a `profiles` table meant a join on nearly every
  read, and an account could exist with no profile.
- **`hold_expires_at` is a column,** not `created_at` plus an interval computed
  by a SQL function. Expiry is now indexable and per-booking.
- **The exclusion constraint and the availability query agree.** They
  previously did not: the constraint ignored only `cancelled` while the grid
  ignored `cancelled` and `no_show`, so a no-show slot rendered as free and
  then threw a constraint violation on booking. Both now go through
  `booking_blocks_slot`.
- **`bookings.open_to_join` is gone.** It mirrored whether a matchmaking post
  existed — two sources of truth for one fact, which had already drifted. It is
  derived now.
- **Counters are maintained by trigger and guarded by `CHECK`.** Tournament
  capacity and matchmaking fill are enforced by the database, so the row lock
  taken by the counter update settles concurrent registrations for free.

## Authorization

Past sign-in, permission is a property of the thing, not of the account. The
one exception is registering a venue, which needs an `arena_owner` account;
after that, owning *that arena* is what lets you edit it, captaining *that
team* is what lets you change the squad, and authoring *that call* is what
lets you accept somebody into the game.

Two rules hold throughout:

**Owner-scoped writes carry their predicate in the SQL.** Every statement in
`arena_admin.go` and the cash reconciliation in `payment.go` joins through to
`arenas.owner_id` in its own WHERE clause. The service checks ownership too,
for the sake of a readable error -- but that check is not what enforces it. A
read-then-write leaves a window, and these are the writes where the window
means editing somebody else's venue.

**Updates are PUT, not PATCH.** The four update endpoints -- a venue, a court,
a team, a call -- each write every field they own, so a body that leaves one
out blanks it. That is a replace, and calling it PATCH would be a trap for any
client sending less than the whole resource. The edit forms are pre-filled from
current values for exactly this reason.

**A review has to be earned.** You can rate an arena once you have played
there -- a paid booking on one of its courts whose hour has passed. An arena's
rating is a number the listing shows and a booking decision turns on, and the
booking history is right there to check it against. The cost is deliberate: a
venue's first review can only come from its first paying customer, afterwards.

**A result counts only when both captains agree it.** One captain files a
score, the other confirms; `matches.reported_by` records who filed it so the
reporter cannot wave through their own. The standings view reads `verified`
and nothing else, so that flag is the whole value of the table.

**Refusals do not confirm existence.** Editing an arena you do not own, paying
for a booking that is not yours, or managing a call you did not write all come
back as 404 rather than 403. A distinct "forbidden" would tell a stranger the
thing is there.

The authorization rules themselves live in the domain -- `CanAddMember`,
`CanRemoveMember`, `CanTransferCaptaincy`, `CanBeJoinedBy`,
`CanAcceptResponse`, `CanAcceptRegistration` -- and the services call them
rather than restating them. A second opinion written at the service layer is
one that can drift from the first.

## Status

Done: schema and migrations; the domain layer; repositories for bookings,
availability, users, sessions and arenas; the booking, auth, profile and arena
services; the janitor; rate limiting; and the HTTP API over all of them, with
`cmd/api` wiring it together.

**The frontend reads from the service.** `web/src/lib/api/index.js` points at
`client.js`, and every page is served from Postgres:

```
/tonight, /arenas       book a court by the hour
/games                  the call sheet -- games short of players
/teams                  squads, rosters, invite codes
/tournaments            brackets and entries
/standings              the table -- agreed results only
/manage                 the back office: venues, courts, rates, gallery, the till
```

Reviews and photo galleries sit on the arena page; results are reported and
confirmed from a team's page.

Every table in the schema has a repository, a service and endpoints over it,
and nothing in `internal/domain` is unreachable from the API.

What is left is smaller than what is here:

1. **A player page.** `/v1/players/{id}/highlights` answers and
   `POST /v1/me/highlights` writes, but there is no profile screen to put a
   reel on -- the player card exists in the database and nowhere in the
   interface.
2. **Photo uploads.** Galleries take a URL you already host. Somewhere to put
   the file is a storage decision (object store, signed uploads, a size cap)
   rather than a missing endpoint.
3. **Disputes.** A captain can withdraw an unagreed result, which is enough
   for a typo. Two captains who genuinely disagree about a score have no
   route but to talk to each other.

Three things to know before this serves real traffic:

- The rate limiter is per-process and wants a shared counter behind more than
  one instance.
- The API has to sit on the same origin as the interface (a reverse proxy in
  production, the vite proxy in development) for the refresh cookie to travel,
  and for `APP_URL` to address both.
- **The gateway adapters have been exercised against eSewa's sandbox and
  against stubs, not against a completed live payment.** The refusal paths are
  verified -- a forged `COMPLETE` redirect for an unpaid transaction is
  correctly rejected -- but the success path has only ever been driven by a
  stub. Run one real transaction end to end through each provider's sandbox
  before taking money.
