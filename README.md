# Khel Arena — Backend

Futsal and sports arena booking for Kathmandu. Go service over stock
PostgreSQL.

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
cmd/migrate/          Applies migrations and exits
internal/
  domain/             Entities, value objects, rules. No database, no HTTP.
  postgres/           Every line of SQL, and the pgx repositories that run it
    migrations/       Numbered, checksummed, embedded in the binary
  service/            Use cases: booking, auth, the background janitor
  platform/
    config/           Environment loading, validated eagerly at startup
    token/            Argon2id password hashing, JWT and refresh tokens
```

Dependencies point inward. `domain` imports nothing of ours; `service` depends
on `domain` and on interfaces it declares itself; only `postgres` knows pgx
exists.

## Running it

Requires Go 1.24+ and PostgreSQL 14+ (for `pgcrypto`, `btree_gist`, `citext` —
the first migration creates them).

```sh
cp .env.example .env          # then set JWT_SECRET
make db-up                    # local Postgres in Docker, both databases
make migrate
make check                    # tidy + vet + tests under -race
```

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
```

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
availability, users and sessions; the booking and auth services; the janitor.

Not yet written: the HTTP API, and repositories for teams, tournaments,
matchmaking and arena management. The tables, domain types and rules for those
are in place — what is missing is the storage and transport code over them.
