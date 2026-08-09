# Khel Arena — web

SvelteKit, Svelte 5, plain JavaScript. No TypeScript anywhere in this tree.

```sh
make web-install     # or: npm --prefix web install
make web             # dev server on :5173
make web-check       # svelte-check over every component
make web-build       # production build into web/build, run with `node web/build`
```

## Where the data comes from

`cmd/api` does not exist yet (see the repo README, "Status"), so every page
reads from `src/lib/api/mock.js`. That module answers the same endpoint set as
apiPlan.md §3, with the same payload shapes as §5–6: `snake_case` keys, RFC 3339
UTC instants, `HH:MM` day times, money as whole NPR integers, and the error
envelope from §4 decoded into one `ApiError` type.

Two rules the real service enforces are reproduced faithfully, because the
interface depends on them:

- **The availability grid is projected, not stored.** Hours are walked from the
  arena's opening to its closing and priced by rule — the same shape as
  `domain.BuildGrid` and `domain.ResolvePrice`, including the half-open
  `[start_hour, end_hour)` window and highest-priority-wins tie-breaking.
- **A booking starts as a hold that expires.** Fifteen minutes, matching
  `BOOKING_HOLD_WINDOW`. The countdown in the booking panel is that clock.

Occupancy is seeded from a hash of court, date and hour, so the board renders
identically on the server and in the browser and survives a reload. Your own
session and bookings live in `sessionStorage` and last as long as the tab.

**Switching to the real API** is one file: `src/lib/api/index.js` re-exports the
mock. `src/lib/api/client.js` already implements the endpoint table against
`/v1` with the §4 error decoding, and `vite.config.js` proxies `/v1` to
`$API_ORIGIN` (default `http://localhost:8080`) in development.

Try the mock account `rajwat@khelarena.np` / `kathmandu2026`, or register a new
one — the signup form renders the multi-field validation errors that
`Registration.Validate()` is built to return all at once.

## Design

Daylight, not floodlight. The ground is `#ECEFEA`, a faint green-grey — turf in
the morning, desaturated until it stops competing with text — and white cards
sit on it. Nothing is pure white or pure black. One action colour, `--pine`
(`#2C5847`), carries every button, selection and link; a sand tint marks
peak-rate hours so price never has to be a second text colour; a muted brick is
kept for errors only. All tokens are at the top of `src/app.css`.

Two faces do all the work:

- **Newsreader** — a low-contrast text serif with a real optical-size axis, set
  per step so big headings thin out and small ones stay sturdy. Headings,
  prices, the wordmark.
- **Hanken Grotesk** — tall x-height, warm, legible. Every word you actually
  read, at **17px / 1.65** with tabular numerals wherever figures line up.

The signature element is **the hour ledger** (`src/lib/components/HourLedger.svelte`):
every court down the side, the hours of one day across the top, the price of
each free hour in the cell, taken hours sunk and struck. It is the whole
argument of the product — court time is inventory that expires — and it is the
home page's own content rather than a screenshot of it.

The hero pairs it with one control: **what, where, when** and a button that
hands you the board already filtered. `/tonight` is URL-driven, so a filtered
board is shareable and the back button walks back through the searches.

Structural markers are the clock — `Right away`, `Within 15 minutes`,
`At kick-off` — not `01 / 02 / 03`. The hold window is the content.

## Routes

| Path | What it is |
|---|---|
| `/` | How many hours are left tonight, the search control, and the ledger |
| `/tonight` | The board for any of the next seven days, by sport and area |
| `/arenas` | The register of arenas |
| `/arenas/[slug]` | One arena: courts, hours, and the hold-and-pay panel |
| `/bookings` | Your hours, with the hold countdown on unpaid ones |
| `/login`, `/register` | Auth |
| `/robots.txt`, `/sitemap.xml` | Generated |
