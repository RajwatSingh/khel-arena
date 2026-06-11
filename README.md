# Khel Arena — Futsal & Sports Arena Booking · Kathmandu

A premium, community-driven booking platform for Nepal's futsal scene.
Light editorial interface (Fraunces / Archivo / IBM Plex Mono on a warm
porcelain canvas with espresso type and deep-gold accents) over a race-proof
Postgres booking engine with a complete eSewa/Khalti payment loop.

The design speaks futsal: the hero opens *standing on the court* — halfway
line, center circle and penalty arcs chalk themselves in as hairlines — a
pitch-divider motif separates sections, and players pick their position
(Goleiro / Fixo / Ala / Pivô / Universal) by **tapping the zone on a
top-down court diagram** rather than choosing from a dropdown.

Beyond booking: host or enter **tournaments** (format, side count, prize
purse with a live split preview, capacity-gauged registration) and build an
**editorial player card** — avatar, position, jersey, preferred foot, bio,
and highlight reels — that updates live as you edit.

## Routes

| Route | Purpose |
| --- | --- |
| `/` | Cinematic landing + doorways into the product |
| `/book` | The booking matrix — live availability, dynamic pricing, payment handoff |
| `/book/confirmation` | Gateway return destination (success / failure verdict) |
| `/tournaments` | Enter a cup or host your own — format, purse, deadline |
| `/community` | Matchmaking board — open calls for players |
| `/profile` | Customize your editorial player card |
| `/api/payments/esewa/callback` | eSewa return — signature + status-API verification |
| `/api/payments/khalti/callback` | Khalti return — pidx lookup verification |

## Run it now (demo mode)

```bash
npm install
npm run dev        # → http://localhost:3000
```

With no Supabase env vars set, the app runs the **full experience on bundled
demo data** — the matrix, dynamic pricing, the simulated payment loop, matchmaking
board, and standings all work. Component code is identical in both modes.

## Go live

1. Create a Supabase project, then run in the SQL editor, in order:
   - `supabase/schema.sql`
   - `supabase/functions.sql`
   - `supabase/02_tournaments_profiles.sql`
   Then create a public Storage bucket named `avatars` (see the comment in
   the SQL file) for profile-photo uploads.
2. Copy `.env.example` → `.env.local` and fill in your keys.
3. Restart `npm run dev` — the page now reads/writes the real database.

## Architecture

```
src/
├── app/                    Next.js App Router (server-first)
├── components/
│   ├── Nav.tsx             Shared sticky navigation
│   ├── HeroSection.tsx     Cinematic landing — orchestrated reveal
│   ├── BookingMatrix.tsx   The signature slot grid + selection dock
│   ├── CommunityHub.tsx    Matchmaking board + season standings
│   ├── PitchLines.tsx      Futsal court markings as design elements
│   ├── TournamentBoard.tsx Listing + "host a tournament" form
│   ├── FutsalPitchPicker.tsx  Pick your position by tapping the court
│   ├── ProfileStudio.tsx   Player card + customization studio
│   ├── BookClient.tsx      /book composition — booking → payment handoff
│   ├── CommunityClient.tsx /community composition
│   ├── TournamentsClient.tsx  /tournaments composition
│   └── ProfileClient.tsx   /profile composition (live ↔ demo routing)
├── actions/                Server actions (validation + error translation)
├── stores/                 Zustand — ephemeral selection state only
└── lib/
    ├── payments/           eSewa ePay v2 + Khalti ePayment hooks
    ├── supabase/           SSR + browser clients
    ├── demo.ts             Bundled dataset mirroring real RPC shapes
    └── types.ts            Shared domain types
supabase/
├── schema.sql              Tables, enums, RLS, standings view
└── functions.sql           create_booking, toggle_matchmaking_slot,
                            get_availability_grid, resolve_slot_price
```

## Futsal design language

The interface speaks futsal, not generic SaaS. Court markings (halfway line,
center circle, penalty arcs) are drawn in hairlines as the hero backdrop and
chalk themselves in on load like a groundskeeper at dawn (`PitchLines.tsx`).
The `PitchDivider` reuses the halfway-line + center-circle motif as a section
rule. Positions use authentic futsal vocabulary — **Goleiro, Fixo, Ala, Pivô,
Universal** — and you set yours by **tapping where you play on a top-down
court** (`FutsalPitchPicker.tsx`). Prize purses, jersey numbers, and team
capacity all render in the same display-gold / mono-data editorial system.

## Tournaments

`/tournaments` lists open competitions and hosts a creation form: format
(knockout / league / groups+KO), side count, max teams, entry fee, **prize
pool with a live per-place split preview**, skill tier, and deadlines.
Registration runs through `register_team_for_tournament()` — the same
advisory-lock + capacity-check discipline as bookings, so a tournament can
**never** accept more teams than `max_teams` even under concurrent submits,
and flips to `full` atomically on the final spot.

## Profiles

`/profile` is a two-pane studio: the editorial player card on the left
updates live as you edit it on the right. Avatar uploads go to a Supabase
Storage `avatars` bucket (under each user's own prefix); position, jersey
number, preferred foot, skill, and bio are RLS-guarded profile updates; and
highlight reels are link-based (`profile_highlights`) with source detection
for YouTube / TikTok / Instagram / Drive.

> The `team_standings` view from `schema.sql` is retained in the database for
> future seasons but is no longer surfaced in the UI, per the latest design.

## Why double-booking is impossible

Three independent layers, strongest last:

1. **Advisory lock** — `pg_advisory_xact_lock` on a hash of court + start
   time serialises concurrent attempts, so the loser waits rather than errors.
2. **Pre-check** — an overlap query inside the lock returns a clean,
   human-readable `SLOT_TAKEN` message.
3. **`EXCLUDE USING gist (court_id WITH =, slot WITH &&)`** — a database
   constraint. Even a buggy code path that bypasses `create_booking()`
   physically cannot insert an overlapping live booking. Cancelled bookings
   are excluded, so freed slots are instantly rebookable.

Pricing is also resolved **server-side** inside the transaction
(`resolve_slot_price`), so a stale or tampered client price can never be
charged.

Tournament registration reuses the same discipline:
`register_team_for_tournament()` takes an advisory lock, checks the team
count inside it, and flips the tournament to `full` on the final spot — so a
cup can never accept more teams than `max_teams`, even under a stampede of
concurrent sign-ups.

## Why TimeSlots are virtual

Materialising every hour × court × day creates unbounded dead rows.
`get_availability_grid(court, date)` projects the daily grid from arena
operating hours, pricing rules, and live bookings in one round trip — the
grid the frontend renders *is* the TimeSlots entity, always fresh, never
stale.

## Payments

Fully wired, end to end:

1. **Confirm & pay** on `/book` calls `createBooking()` (slot held under the
   race-proof transaction) then `payForBooking()` (server action), which
   creates a `payments` intent row and returns a gateway instruction.
2. The browser enters the gateway — eSewa via signed HMAC-SHA256 form POST,
   Khalti via redirect to its `payment_url`.
3. The gateway returns to `/api/payments/{provider}/callback`, which verifies
   **server-to-server** (eSewa status API / Khalti pidx lookup, with amount
   cross-check) — redirect parameters are never trusted. Only then does
   `payments.status → 'verified'` and `bookings.status → 'confirmed'`.
4. The player lands on `/book/confirmation` with the verdict and receipt ref.

Callbacks are idempotent (replays find the row already verified) and run on
the service-role client, since the gateway redirect carries no user session.
In demo mode the whole loop is simulated locally, ending on the same
confirmation page.

## Next milestones

- Court onboarding dashboard for arena owners (replace the MVP court list)
- Supabase Realtime channel on `bookings` for live grid invalidation
- Match result submission + dual-captain verification flow
- Player profile pages (editorial cards are designed in the token system)
