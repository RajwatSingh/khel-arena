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

Fifteen endpoints over the two services that have repositories behind them.
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
| GET | `/v1/courts/{courtID}/availability` | `?date=YYYY-MM-DD` |
| GET | `/v1/me` | Authenticated |
| POST | `/v1/auth/password/change` | Authenticated |
| POST | `/v1/bookings` | Authenticated, takes a hold, 201 |
| GET | `/v1/bookings` | Authenticated, `?limit=`, newest first |
| DELETE | `/v1/bookings/{bookingID}` | Authenticated, 204 |

Access tokens travel in `Authorization: Bearer`; the refresh token travels
only in an httpOnly cookie, so a script that can read `localStorage` cannot
read it. The booking price and the booking's owner are both resolved
server-side and never taken from a request body.

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

## Status

Done: schema and migrations; the domain layer; repositories for bookings,
availability, users and sessions; the booking, auth and profile services; the
janitor; and the HTTP API over all of them, with `cmd/api` wiring it together.

The frontend still reads from the mock in `web/src/lib/api/mock.js`. It cannot
be switched over yet, because seven of the fourteen calls it makes have no
endpoint behind them: `listArenas`, `getArena`, `listAreas` and `cityLedger`
need arena and court repositories, and `payBooking` needs payments. The three
flows the API does cover — sign in, availability, book and cancel — can be
pointed at `client.js` today.

Not yet written, in the order they unblock the frontend:

1. **Arena and court repositories,** then `GET /v1/arenas`, `/v1/arenas/{slug}`
   and `/v1/areas`. This is what four of the seven missing calls need, and it
   is the largest single step toward retiring the mock.
2. **Payments** — a `PaymentRepo` and the eSewa/Khalti adapters. The last thing
   between an unpaid hold and a confirmed booking. The provider callbacks are
   attacker-reachable and want a focused pass of their own, with signature
   verification done properly rather than folded into general API work.
3. **Email delivery,** to make password reset usable. Until then the reset
   token is logged outside production and nowhere else.
4. **Rate limiting** on `/v1/auth/login` and `/v1/auth/password/forgot` — the
   online guessing target and the inbox sprayer. `domain.CodeRateLimited`
   exists with nothing producing it.
5. **Repositories and endpoints for teams, tournaments and matchmaking**, whose
   tables and domain types are in place but which nothing stores yet.
6. **Arena management** — owner-facing writes, and the first place
   `domain.AccountArenaOwner` and `domain.CodeForbidden` do real authorization
   work rather than existing as enum values.

One smaller gap worth naming: `domain.GridSlot` does not carry the label of the
pricing rule that won, so the availability response reports `"rule": "Peak
rate"` or `"Base rate"` where the frontend expects the rule's own name
("Evening Peak"). Fixing it properly means carrying the winning rule out of
`domain.BuildGrid`.
