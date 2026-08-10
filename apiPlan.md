# Assignment: Build the Khel Arena HTTP API

**Course, for the purposes of this exercise:** you, teaching yourself Go.
**Prerequisite:** none of `cmd/api` or `internal/api` exists yet. You are
writing it from nothing.

**What exists already, that you may read but must not need to change:**
- `internal/domain` — business rules and the `domain.Code` / `domain.Error`
  vocabulary (`internal/domain/errors.go`).
- `internal/service` — `AuthService` (`internal/service/auth.go`) and
  `BookingService` (`internal/service/booking.go`), the use cases your
  handlers will call.
- `internal/postgres` — repositories and `SessionContext`
  (`internal/postgres/session.go`).
- `web/` — the SvelteKit frontend, **already written against the exact JSON
  shapes below**. `web/src/lib/api/client.js` is the real fetch client
  waiting for you to give it something to talk to; `web/src/lib/api/mock.js`
  and `web/src/lib/api/fixtures.js` are a hand-built fake server that
  produces those shapes today. Treat the mock as the spec when this document
  is ambiguous — if `mock.js` returns a field, your handler must too.

**What you're writing:** everything under `cmd/api/` and `internal/api/`.
Nine files, no new dependencies — Go 1.22+'s `net/http.ServeMux` does method
and wildcard routing (`POST /v1/bookings/{id}`) natively, so `go.mod` doesn't
move.

---

## 0. Learning goals

By the end of this you should be able to explain, not just recite:

- Why Go's standard library treats a handler as `func(http.ResponseWriter, *http.Request)`, and how middleware is "just a function that returns a function of that shape."
- Why a transport layer maps an internal error type to an HTTP status in exactly one place, instead of each handler deciding.
- Why request context (`context.Context`) is the thing that carries a deadline, a request ID, and an authenticated user ID through a call stack, rather than global state.
- The difference between a domain type and a wire (JSON) type, and why a server shouldn't `json.Marshal` its internal structs directly.
- What graceful shutdown actually drains, and in what order, and why the order matters.

---

## 1. Readings — do these before / while you write the matching part

You don't need to read all of these cover to cover. Skim for the shape,
come back when you're stuck on the matching part below.

**Go fundamentals, if `internal/service` and `internal/domain` were the first
Go you've read closely:**
- [A Tour of Go](https://go.dev/tour/) — if struct embedding, interfaces, or `defer` in `internal/domain/errors.go` and `internal/service/booking.go` felt unfamiliar, do the relevant sections first.
- [Effective Go](https://go.dev/doc/effective_go) — especially the sections on interfaces, errors, and concurrency.

**`net/http` and routing:**
- [`net/http` package docs](https://pkg.go.dev/net/http) — read `Handler`, `HandlerFunc`, `ServeMux`, and `Server` at minimum.
- [Routing Enhancements for Go 1.22](https://go.dev/blog/routing-enhancements) — the blog post announcing method + wildcard patterns in `ServeMux`. This is what makes `POST /v1/bookings/{id}` work with zero dependencies.
- [MDN: HTTP response status codes](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status) — for Part A, mapping `domain.Code` to a status.

**JSON:**
- [`encoding/json` package docs](https://pkg.go.dev/encoding/json) — read `Marshal`, `NewDecoder`, `Decoder.DisallowUnknownFields`, and struct tags.
- [JSON and Go](https://go.dev/blog/json) — the older but still-correct blog post on how struct tags control marshalling.

**Context and middleware:**
- [`context` package docs](https://pkg.go.dev/context) — read `WithValue`, `WithTimeout`, and the package-level docs' warning about context keys (this is why §7 below requires an unexported key type, not a `string`).
- [Go Concurrency Patterns: Context](https://go.dev/blog/context) — the canonical explanation of *why* context exists and how it threads through a call chain.
- [Writing HTTP Middleware in Go](https://drstearns.github.io/tutorials/gomiddleware/) — a clear worked example of the "wrap a handler, return a handler" pattern you'll use for every item in Part B.

**JWTs, since `AuthService.Authenticate` verifies one and you're writing the code that calls it:**
- [Introduction to JSON Web Tokens](https://jwt.io/introduction) — just enough to understand what "verify, don't round-trip to the database" (`internal/service/auth.go:171-190`) is buying you.

**Graceful shutdown, for Part F:**
- [`http.Server.Shutdown` docs](https://pkg.go.dev/net/http#Server.Shutdown) — read the whole doc comment, not just the signature.
- [`signal.NotifyContext` docs](https://pkg.go.dev/os/signal#NotifyContext) — the mechanism `cmd/migrate/main.go` already uses for `SIGINT`/`SIGTERM`; read that file for a working example before writing your own.

**`slog`, for structured logging in Part B:**
- [`log/slog` package docs](https://pkg.go.dev/log/slog) — read `New`, `Handler`, `Logger.With`.

---

## 2. Setup and how to check your work

```
go build ./...          # does it compile
go vet ./...             # does it look sane
go test ./... -race      # do your tests pass
```

There is no scaffolding to generate — you are creating `cmd/api/main.go` and
every file under `internal/api/` yourself. Start each file with a failing
`go build` (an empty file with just `package api`) and grow it outward;
don't try to write all nine files before compiling any of them.

If you have a local Postgres running, you can smoke-test end to end with
`curl` once Part F compiles. If not, `go build`, `go vet`, and your own
`httptest`-based tests (Part A/D/E) are your only feedback loop — that's
fine, they're supposed to be enough.

---

## 3. Scope

**In scope** — the two services that already have repositories behind them:

- **`AuthService`** — all 8 methods are reachable.
- **`BookingService`** — 4 of 5 methods (`ReleaseStaleHolds` belongs to the
  janitor, not to a request — don't expose it).

**Out of scope, and why you shouldn't try to add it:**

- **Teams, tournaments, matchmaking, arena management.** The domain types
  and tables exist but `internal/postgres` has no repositories for them.
  There's nothing to call.
- **Payments.** `domain.Payment` (`internal/domain/paymentar.go`) holds the
  logic but there's no `PaymentRepo` and no eSewa/Khalti adapter yet.
- **Email delivery.** Affects what `/auth/password/forgot` can do — see
  Part D, `handlePasswordForgot`.

---

## 4. Package layout — what you're creating

```
cmd/api/main.go              Config, wiring, janitor, graceful shutdown   (Part F)

internal/api/                The transport layer. Imports service; never pgx.
  server.go                  Server struct, dependency interfaces, routes()  (Part C)
  errors.go                  domain.Code -> status, error response writing  (Part A)
  json.go                    decode/encode helpers, body size limits        (Part A)
  middleware.go              request ID, logging, recovery, CORS, auth      (Part B)
  auth.go                    handlers for /v1/auth/*                        (Part D)
  bookings.go                handlers for /v1/bookings, /v1/courts/*        (Part E)
  health.go                  /healthz, /readyz                             (Part C)
  dto.go                     wire types + domain -> wire conversion         (Part A)
```

**Package name is `api`, not `http`.** `internal/http` would shadow the
standard `net/http` package at every use site inside it — you'd be writing
`http.Get` to mean the stdlib and something else to mean your own package.
`internal/api` sidesteps that.

**Dependency direction, and don't violate it:** `internal/api` imports
`internal/service` and `internal/domain`. **It must not import
`internal/postgres`** — the transport layer doesn't get to know the storage
layer exists, same rule `internal/service` follows for `pgx`. You will hit
one annoying exception to this in Part D (`SessionContext`) — read that
callout before you work around it by importing `postgres` anyway.

---

## Part A — Foundation: `errors.go`, `json.go`, `dto.go`

Write these first and write their tests first. Every handler in Part D and E
depends on all three files compiling and behaving correctly, and bugs here
are the ones most likely to leak internal information to a client.

### `errors.go`

#### `func codeToStatus(code domain.Code) int`

Pure mapping function, no I/O. Given one of `domain.Code`'s eight values,
return the matching `http.Status*` constant.

| `domain.Code` | Status |
|---|---|
| `domain.CodeInvalid` | `http.StatusBadRequest` (400) |
| `domain.CodeUnauthenticated` | `http.StatusUnauthorized` (401) |
| `domain.CodeForbidden` | `http.StatusForbidden` (403) |
| `domain.CodeNotFound` | `http.StatusNotFound` (404) |
| `domain.CodeConflict` | `http.StatusConflict` (409) |
| `domain.CodeRateLimited` | `http.StatusTooManyRequests` (429) |
| `domain.CodeUnavailable` | `http.StatusServiceUnavailable` (503) |
| `domain.CodeInternal`, or anything not in this table | `http.StatusInternalServerError` (500) |

*Hint:* `domain.CodeOf(err)` (`internal/domain/errors.go:167`) already
collapses an unrecognized error to `CodeInternal` for you — call it before
this function rather than duplicating that fallback here.

#### `func writeError(w http.ResponseWriter, r *http.Request, err error) `

Writes the JSON error envelope and sets the status. Signature is up to you,
but it needs the response writer, the error, and (for logging) either the
request or a logger already carrying the request ID.

Body shape — match this exactly, `web/src/lib/api/client.js:36-41` decodes
exactly this shape:

```json
{
  "error": {
    "code": "invalid",
    "message": "Use at least 10 characters. Usernames need at least 3 characters.",
    "fields": [
      { "field": "password", "message": "Use at least 10 characters." },
      { "field": "username", "message": "Usernames need at least 3 characters." }
    ]
  }
}
```

Rules your implementation must hold to — these aren't style preferences,
each one closes a real information leak:

1. **The `message` field always comes from `domain.UserMessage(err)`**
   (`internal/domain/errors.go:181`), never from `err.Error()`.
   `UserMessage` already collapses `CodeInternal` and unrecognized errors to
   a generic apology — that's what stops a raw `pgx` error putting a
   connection string in a response body.
2. **`CodeInternal` is the only code you log the cause for.** Get the cause
   with `errors.Unwrap` or by type-asserting to `*domain.Error` and calling
   `.Unwrap()` — it's deliberately not part of the serialized JSON.
   Logging every other code at error level buries real defects in routine
   401 noise; log those at info/debug if you log them at all.
3. **`fields` comes from `(*domain.Error).FieldErrors()`**
   (`internal/domain/errors.go:65`), which already normalizes the
   single-field and multi-field cases into one shape. Omit the key
   entirely (not `"fields": []`) when it's empty — check how
   `encoding/json`'s `omitempty` tag behaves for a slice.

*Test this file first, before anything else.* `errors_test.go` should cover
every `Code` → status pairing, confirm `CodeInternal` and an unrecognized
`errors.New("boom")` never put their text in the response body, and check
field-error rendering for both a single-field and multi-field
`domain.Error`.

### `json.go`

#### `func decode[T any](r *http.Request) (T, error)` (or non-generic per-type variants — your call)

Reads and validates a JSON request body.

- Wrap `r.Body` in `http.MaxBytesReader(w, r.Body, 64<<10)` (64 KiB) before
  decoding — a handler that skips this can be sent an unbounded body.
- Use `json.NewDecoder(...).DisallowUnknownFields()` so a client's typo'd
  field name becomes a 400, not a silently-ignored value.
- On decode failure, return a `*domain.Error` with `domain.CodeInvalid` —
  `writeError` in Part A already knows how to turn that into a 400.

#### `func encode(w http.ResponseWriter, status int, v any)` (or similar)

Sets `Content-Type: application/json`, writes the status, marshals `v`.
Success responses are **the bare resource, no envelope** — a `bookingDTO`
serializes as `{ "id": ..., "reference": ... }`, not
`{ "data": { "id": ... } }`. The status code already tells the client
whether it's an error; a `{data, error}` wrapper buys nothing.

### `dto.go`

**Why this file has to exist at all — three concrete reasons, not
hypothetical ones:**

- `domain.User` (`internal/domain/user.go:17`) carries `Email` and
  `EmailVerifiedAt`. Fine to return from `/v1/me`; wrong the instant a user
  object shows up in someone *else's* view (a booking's owner, say). You
  need at least a `userDTO` (private view) and a smaller public shape.
  `PasswordHash` is already off `User` entirely — `Credentials` is a
  separate struct — so you don't have to worry about that one leaking, but
  notice *why* that split exists and keep the same instinct here.
- `domain.Slot` (`internal/domain/slot.go:29`) is `{Start, End}` nested.
  The frontend wants `starts_at` / `ends_at` flat on a booking — see
  `web/src/lib/api/mock.js:159-168` for the exact shape it's already
  written against.
- `domain.BookingDetail` (`internal/domain/booking.go:143`) embeds
  `Booking`. Default `encoding/json` marshalling of an embedded struct
  either flattens or nests depending on tags nobody has written yet — write
  the tags on purpose instead of finding out by accident.

Build one `fromDomain`-style constructor per DTO, all in this file, so the
domain→wire mapping is reviewable as a unit rather than smeared across
`auth.go` and `bookings.go`.

**DTOs to write**, cross-checked against what the frontend already consumes:

| DTO | Built from | Check your shape against |
|---|---|---|
| `sessionDTO` | `service.Session` | `web/src/lib/session.svelte.js:16-18` — `.user` and `.access_token` are both read off the login response |
| `userDTO` | `domain.User` | `web/src/lib/api/mock.js:234-245` — the account object shape (`full_name`, `username`, `email`, `account_type`, `skill`, `position`, `jersey_number`, `preferred_foot`) |
| `bookingDTO` | `domain.Booking` | `web/src/lib/api/mock.js:335-357` — note `starts_at`/`ends_at` flat, `hold_expires_at` nullable, `status` as a string |
| `bookingDetailDTO` | `domain.BookingDetail` | same shape as `bookingDTO` plus `court_name`/`arena_name`/`arena_slug`/`arena_area` — see the same mock.js booking object, which already includes these |
| `gridSlotDTO` | `domain.GridSlot` | `web/src/lib/api/mock.js:159-168` |
| `errorDTO` | `*domain.Error` | the error envelope in `errors.go` above |

Money: `PriceNPR` is `int` in the domain and in the schema — keep it an
`int` on the wire too, never a float. Times: `domain.Slot` already
normalizes to UTC on construction (`internal/domain/slot.go:34`), so a plain
`time.Time` field marshals correctly as RFC 3339 with no extra work — don't
hand-format it. `DayTime` (used for `opens_at`/`closes_at` if you surface
them) has a `ParseDayTime` already; round-trip through it rather than
reformatting by hand.

Availability response example, for the shape of a full endpoint payload —
compare against `web/src/lib/api/mock.js:142-172`:

```json
{
  "court_id": "…",
  "date": "2026-08-14",
  "slots": [
    { "starts_at": "2026-08-14T12:15:00Z", "ends_at": "2026-08-14T13:15:00Z",
      "price_npr": 1800, "is_peak": true, "is_booked": false,
      "is_past": false, "available": true }
  ]
}
```

`available` is **computed**, from whether the slot is neither booked nor
past — don't leave that for the client to re-derive; the rule already lives
in the domain layer's grid logic (`internal/domain/availability.go`) and
your DTO constructor should apply the same `!IsBooked && !IsPast` logic
once.

---

## Part B — Middleware

Applied outermost to innermost — get the order right, it matters:

1. **Panic recovery.** Outermost, so it catches panics thrown by everything
   inside it, including your other middleware. Recovers, logs with a stack
   trace, and writes a 500 through the *same* `writeError` from Part A —
   don't hand-roll a second error path here.
2. **Request ID.** Accepts an inbound `X-Request-Id` header or generates a
   UUID if absent. Puts it in the request context and echoes it on the
   response.
3. **Structured logging**, via `log/slog`. One line per request: method,
   path, status, duration, request ID, and user ID *when authenticated*
   (i.e., this middleware needs to run after auth, or read whatever auth
   left in the context, to get that last field).
4. **CORS.** Read allowed origins from config. You only need this if a
   browser client running on a different origin than the API will call it —
   decide that for your own setup before writing it.
5. **Timeout**, via `http.TimeoutHandler` or a per-request context deadline.
   Caps how long a slow query can pin a connection. Every layer beneath this
   already threads `ctx` through (`AuthService` and `BookingService` methods
   all take `context.Context` first) so this works end to end for free —
   you're not adding deadline-checking anywhere else.

**Then, on protected routes only:**

6. **Authentication.** Reads `Authorization: Bearer <token>`, calls
   `AuthService.Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)`
   (`internal/service/auth.go:176`), and on success puts the user ID and
   account type into the request context for handlers to read.
   `Authenticate` verifies the JWT in-process and touches no database — that's
   deliberate, it's what makes it cheap enough to sit in front of every
   protected route. Don't add a database call here; if you find yourself
   wanting one, you've misread what the method is for.

**Two things to get right structurally, not just functionally:**

- **Context keys must be an unexported struct type in package `api`, not a
  `string`.** This is directly called out in the `context` package docs —
  read why before you write `ctx.Value("userID")`. Write typed accessor
  functions (`userIDFromContext(ctx) (uuid.UUID, bool)`) rather than
  exposing the raw key.
- **Building `postgres.SessionContext`.** `Register`, `Login`, and
  `Refresh` all take one (`postgres.SessionContext{UserAgent, IP}`,
  `internal/postgres/session.go:25`). `UserAgent` is just `r.UserAgent()`.
  `IP` is the one genuinely tricky bit in this whole assignment:
  `r.RemoteAddr` behind a reverse proxy is the *proxy's* address, and the
  `X-Forwarded-For` header is client-controlled and trivially spoofable
  unless you know how many trusted hops sit in front of you. Decide, as a
  design choice you write down in a comment: are you behind a proxy or not?
  If not, use `r.RemoteAddr` and ignore `X-Forwarded-For` entirely — don't
  read a spoofable header "just in case." Note that
  `SessionContext.addr()` (`internal/postgres/session.go:30`) silently
  returns `nil` for anything that doesn't parse as an IP, so getting this
  wrong doesn't error, it just quietly writes empty session-audit rows —
  worth a comment explaining your choice so a future reader (you, in three
  months) doesn't wonder why.

**This is the one exception to "never import `postgres` from `internal/api`"**
mentioned in §4 — you have to import `postgres` for the `SessionContext`
type itself. That's a real wart in the existing design (the service layer
importing the storage layer's type is backwards), not something you're
expected to fix as part of this assignment. Just don't let it become an
excuse to import `postgres` for anything *else* in the transport layer.

---

## Part C — `server.go`, `health.go`, routing

### `server.go`

#### `type Server struct { ... }`

Holds whatever the handlers need: your `AuthAPI`/`BookingAPI` interfaces
(see Part-A-adjacent note below, and Part G on testing), a logger, and any
config the handlers read directly (CORS origins, etc). Does **not** hold a
`*pgxpool.Pool` or anything from `internal/postgres` — that's the dependency
direction rule from §4.

#### `func NewServer(...) *Server`

Constructor. Signature is yours to design — this is a good place to decide,
concretely, what a handler is allowed to depend on.

#### `func (s *Server) routes() http.Handler`

Builds the `*http.ServeMux`, registers every pattern from the table below
with `mux.HandleFunc("METHOD /path/{param}", handler)`, wraps the whole
thing in the middleware chain from Part B, and returns it. This is the one
function where the full endpoint list has to be typed out, so get the table
right against the frontend calls in `web/src/lib/api/client.js:47-67`.

**Public routes:**

| Method | Path | Calls | Notes |
|---|---|---|---|
| GET | `/healthz` | — | Liveness. Always 200 if the process is up. |
| GET | `/readyz` | `pool.Ping` | Readiness. 503 when the DB is unreachable. |
| POST | `/v1/auth/register` | `AuthService.Register` | Returns a session. |
| POST | `/v1/auth/login` | `AuthService.Login` | Rate-limit candidate — see the note in Part D. |
| POST | `/v1/auth/refresh` | `AuthService.Refresh` | Rotates the token pair. |
| POST | `/v1/auth/logout` | `AuthService.Logout` | Idempotent; 204 even if already signed out. |
| POST | `/v1/auth/password/forgot` | `AuthService.BeginPasswordReset` | Always 202 — see Part D, this one has a trap. |
| POST | `/v1/auth/password/reset` | `AuthService.CompletePasswordReset` | Burns the token. |
| GET | `/v1/courts/{courtID}/availability` | `BookingService.Availability` | Reads `?date=YYYY-MM-DD`. |

**Authenticated routes** (behind the auth middleware from Part B):

| Method | Path | Calls | Notes |
|---|---|---|---|
| GET | `/v1/me` | — | See the callout below — there's no service method yet. |
| POST | `/v1/auth/password/change` | `AuthService.ChangePassword` | Proves the old password. |
| POST | `/v1/bookings` | `BookingService.Create` | Takes a hold. 201. |
| GET | `/v1/bookings` | `BookingService.ListMine` | `?limit=`, newest first. |
| DELETE | `/v1/bookings/{bookingID}` | `BookingService.Cancel` | 204. Ownership is enforced inside the SQL `UPDATE`, not by a check you write in the handler. |

Thirteen endpoints, plus two health probes.

**Why `DELETE` for cancel, not `POST /{id}/cancel`:** `BookingService.Cancel`
(`internal/service/booking.go:151`) is already idempotent-ish and
ownership-scoped by the repository's `UPDATE` — the verb matches the
semantics. If your client environment can't send `DELETE`, that's a reason
to switch to `POST`, but make it a deliberate choice, not an accident.

**`GET /v1/me` has no service method behind it yet.** `UserRepo.ByID`
exists in `internal/postgres`, but handlers must not call a repository
directly — only a service. You have two honest options:

- **(a)** Write a small `ProfileService` in `internal/service` with a `Me`
  method wrapping `UserRepo.ByID`. About 15 lines. Recommended — it's the
  kind of gap you'll hit constantly and the fix is always "add the missing
  service method," never "reach past it."
- **(b)** Drop `/v1/me` from your first pass and let the frontend rely on
  the user object already returned by login/register/refresh
  (`sessionDTO.user`). Come back to it later.

Either is fine for this assignment; just don't call `UserRepo` from
`internal/api`.

### `health.go`

#### `func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request)`

Always writes 200. No dependencies checked — this answers "is the process
running," not "is it working."

#### `func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request)`

Calls `pool.Ping(ctx)` (or whatever narrow interface you gave `Server` for
this — don't hand the whole `*pgxpool.Pool` to `Server` just for this one
call if you're trying to keep the "no `pgx` import" rule honest; a
one-method interface like `type Pinger interface { Ping(context.Context) error }`
works and is satisfied by `*pgxpool.Pool` without an import). Writes 200 if
the ping succeeds, 503 with `Retry-After` if it doesn't.

---

## Part D — `auth.go`

Eight handlers, one per `AuthService` method plus `/v1/me`. Each one follows
the same shape: decode (Part A), call the service, map the error (Part A) or
encode the success DTO (Part A). Write that shape once mentally before you
write the first handler — every one of these eight is the same three lines
with different types.

#### `handleRegister` — `POST /v1/auth/register`

Decodes into a `domain.Registration` (check `internal/domain/user.go` for
its fields and `Validate()` method — you don't re-validate in the handler,
`AuthService.Register` already calls `reg.Validate()` for you at
`internal/service/auth.go:64`). Builds a `postgres.SessionContext` from the
request (Part B). Calls `AuthService.Register`. On success, encodes a
`sessionDTO` with 201.

#### `handleLogin` — `POST /v1/auth/login`

Decodes `{email, password}`. Calls `AuthService.Login(ctx, email, password, sc)`.
200 with a `sessionDTO` on success. This is a rate-limit candidate — see the
note at the end of this section; you can defer that and come back to it.

#### `handleRefresh` — `POST /v1/auth/refresh`

Reads the refresh token, calls `AuthService.Refresh`, returns a new
`sessionDTO`. **Where does the refresh token come from?** This is the one
decision that reshapes several handlers, so decide it before writing any of
Register/Login/Refresh/Logout, not after:

| | httpOnly cookie | JSON body |
|---|---|---|
| XSS exposure | Token unreachable from JS | Reachable if the client stores it in `localStorage` |
| Mobile clients | Awkward | Natural |
| CORS | Needs `credentials: 'include'` and explicit origins | Simple |
| CSRF | Needs `SameSite` handling | Not applicable |
| Testing with `curl` | Fiddly (`-c`/`-b` cookie jar) | Easy |

`web/src/lib/api/client.js:27` already sends `credentials: 'include'` on
every request, which is a hint the frontend was written expecting a cookie —
but you can override that by changing `client.js` too if you'd rather do a
JSON body. Whichever you pick, **isolate it**: write one
`readRefreshToken(r *http.Request) string` and one
`writeSession(w http.ResponseWriter, s service.Session)` and have Register,
Login, and Refresh all call through them, so changing your mind later means
editing two functions instead of four handlers.

#### `handleLogout` — `POST /v1/auth/logout`

Reads the refresh token the same way `handleRefresh` does, calls
`AuthService.Logout`. Always 204, even if there was no token to revoke —
`AuthService.Logout` already treats an empty token as "already signed out"
(`internal/service/auth.go:164-169`), so don't add your own check in front
of it.

#### `handlePasswordForgot` — `POST /v1/auth/password/forgot`

Read this one carefully before you write it — it's the one place in this
assignment where the "obvious" implementation is a security bug.

`AuthService.BeginPasswordReset` (`internal/service/auth.go:202`) returns
the plaintext reset token to its caller. **Your handler must never put that
token in the HTTP response.** If you do, anyone can reset anyone's password
by calling this endpoint and reading the reply. There is no email sender
wired up yet (out of scope, §3), so:

- The endpoint always returns **202 with an empty body**, regardless of
  whether the address is registered. `BeginPasswordReset` already returns a
  zero-value `domain.User{}` and empty token for an unknown address without
  erroring — that's the anti-enumeration property; your handler shouldn't
  undo it by branching on "was a token returned."
- Until email delivery exists, log the token at `slog.Info` level **only
  when not in production** — gate it on however you're reading `APP_ENV`
  (see `internal/platform/config/config.go`). That's enough to test the
  full reset flow by reading your own server's stdout locally, and
  structurally impossible to hit once deployed.

#### `handlePasswordReset` — `POST /v1/auth/password/reset`

Decodes `{token, new_password}`, calls
`AuthService.CompletePasswordReset(ctx, token, newPassword)`. 204 on
success — it burns the token as a side effect, nothing to return.

#### `handlePasswordChange` — `POST /v1/auth/password/change` (authenticated)

Reads the user ID off the request context (Part B put it there). Decodes
`{current_password, new_password}`. Calls
`AuthService.ChangePassword(ctx, userID, currentPassword, newPassword)`.
204 on success.

#### `handleMe` — `GET /v1/me` (authenticated)

See the callout in Part C about the missing service method. Once you have
one, this handler is: read user ID from context, call it, encode a
`userDTO`.

**Rate limiting, deferred but worth a comment where you'd add it:**
`domain.CodeRateLimited` exists (`internal/domain/errors.go:35`) and nothing
currently produces it. `Login` and `password/forgot` are the two endpoints
that need it — the former is an online password-guessing target, the latter
can be used to spray an inbox. An in-process token bucket keyed by client IP
is small (well under 100 lines) and needs no new dependency; it just doesn't
survive you running more than one instance of the API. Decide whether that
matters for your setup. Fine to skip for a first pass — leave a `// TODO:
rate limit` comment on `handleLogin` and `handlePasswordForgot` if you do.

---

## Part E — `bookings.go`

#### `handleAvailability` — `GET /v1/courts/{courtID}/availability`

Reads `courtID` from the path with `r.PathValue("courtID")`, parses it as a
`uuid.UUID`. Reads `?date=` from the query string, parses as `YYYY-MM-DD`
(`time.Parse("2006-01-02", ...)`) — a bad or missing date is a 400 via
`domain.Invalid`, not a panic or a 500. Calls
`BookingService.Availability(ctx, courtID, date)`
(`internal/service/booking.go:119`), maps the `[]domain.GridSlot` to
`[]gridSlotDTO`, wraps in the response shape from Part A's DTO example.

#### `handleCreateBooking` — `POST /v1/bookings` (authenticated)

Decodes into whatever request shape you define (court ID, `starts_at`,
`ends_at`, optional `note`, optional `team_id` — compare against
`web/src/lib/api/mock.js:319` for the fields the frontend already sends).
Builds a `service.CreateBookingInput` (`internal/service/booking.go:61`)
with the user ID pulled from context, not from the request body — a client
must never be able to create a booking *as* someone else by putting a
different `user_id` in the JSON. Calls `BookingService.Create`. 201 with a
`bookingDTO`.

Notice what the request does *not* include: a price. `CreateBookingInput`
deliberately has no price field — the service resolves it server-side from
pricing rules (`internal/service/booking.go:102-103`). If your DTO or
handler passes a client-supplied price through anywhere, that's a bug; the
whole point of that design is that the displayed price is advisory and the
authoritative one is computed on the server.

#### `handleListBookings` — `GET /v1/bookings` (authenticated)

Reads `?limit=` (default sensible if absent — check what
`web/src/lib/api/client.js:61` sends: `limit = 20`). Calls
`BookingService.ListMine(ctx, userID, limit)`, maps
`[]domain.BookingDetail` to `[]bookingDetailDTO`, newest first (the service
already orders them — don't re-sort).

#### `handleCancelBooking` — `DELETE /v1/bookings/{bookingID}` (authenticated)

Reads `bookingID` from the path, parses as UUID. Calls
`BookingService.Cancel(ctx, bookingID, userID)`. 204 on success. Ownership
is enforced inside the repository's `UPDATE` (§ note in Part C) — your
handler does not need to, and should not, first load the booking to check
who owns it. That "load then check" pattern is exactly the race condition
`internal/service/booking.go:146-150`'s doc comment explains was
deliberately avoided.

---

## Part F — `cmd/api/main.go`

This is where everything gets wired together and where a mistake in
shutdown order turns a clean deploy into a burst of 500s for whoever's
mid-request when it happens. Read `cmd/migrate/main.go` first for the
`signal.NotifyContext` half of this — it's already built and does exactly
that part correctly.

Rough order (yours doesn't have to match this exactly, but the *shutdown*
order at the bottom is not optional):

```
config.Load()                              fails fast, reports every problem at once
build an slog handler                      JSON in production, text in development
postgres.Connect(ctx, cfg.Database)
build repositories: UserRepo, SessionRepo, BookingRepo
token.NewIssuer(secret, issuer, accessTTL)
build services: AuthService, BookingService, Janitor
api.NewServer(...)  ->  http.Handler
http.Server{ Addr, Handler, ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout }
go janitor.Run(ctx)
go server.ListenAndServe()
<-ctx.Done()                               SIGINT / SIGTERM via signal.NotifyContext
server.Shutdown(shutdownCtx)               stop accepting new conns, drain in-flight ones
cancel the janitor's context, wait for it to return
pool.Close()
```

**Shutdown order, spelled out, because getting it backwards is the easy
mistake:** drain HTTP first, *then* stop the janitor, *then* close the
pool. If you close the pool while a request is still being handled, that
request's database call fails and the client gets a 500 instead of the
response it was about to get — the opposite of what "graceful" means here.

**`ReadHeaderTimeout` is not optional.** Without it, a client that opens a
connection and sends headers one byte every 30 seconds holds that connection
open indefinitely — read the [Slowloris](https://en.wikipedia.org/wiki/Slowloris_(computer_security))
attack if you want the "why" in full; `http.Server`'s docs explain the field
itself.

---

## Part G — Testing

Handlers become testable without a database by declaring narrow interfaces
in package `api` — the same pattern `BookingStore` and `UserStore` already
use one layer down (`internal/service/booking.go:22`,
`internal/service/auth.go:16`), applied one layer up:

```go
type AuthAPI interface {
    Register(ctx context.Context, reg domain.Registration, sc postgres.SessionContext) (service.Session, error)
    Login(ctx context.Context, email, password string, sc postgres.SessionContext) (service.Session, error)
    // ... one method per AuthService method your handlers call
}
```

`*service.AuthService` satisfies this without you writing any adapter code —
Go interfaces are structural. A test fake satisfies it without a database or
a real token issuer. Write the same for `BookingAPI`.

| File | What it should cover |
|---|---|
| `errors_test.go` | Every `domain.Code` → status pairing; that `CodeInternal` and an unrecognized error never leak their cause into the response body; field-error rendering for single- and multi-field errors |
| `middleware_test.go` | Auth middleware with a valid, an expired, a malformed, and a missing token; panic recovery turning a deliberate `panic()` into a clean 500 |
| `auth_test.go` | Each handler's happy path plus its one most-important failure mode, via `net/http/httptest` |
| `bookings_test.go` | Same, plus `?date=` parsing edge cases on availability (missing, malformed, valid) |

Run everything with `go test ./... -race` — all of this is in-process, no
database required, so it should run fast.

**One environment thing to fix while you're in here:** if your CI config
pins an older Go version than your `go.mod` declares, `GOTOOLCHAIN=auto`
will silently download a newer toolchain on every run instead of failing —
worth aligning the two rather than leaving that surprise for later.

---

## 5. Suggested order

Each step should compile and be independently testable before you move to
the next one — don't write all nine files and then try to make the whole
thing build at once.

1. **`errors.go` + `json.go` + `dto.go`**, with tests. Nothing else can be
   honestly tested until error mapping is right.
2. **`middleware.go`**, with tests. Decide the refresh-token transport
   (Part D) here, even though you won't use it until step 4 — it affects
   how you write the auth middleware's counterpart on the write side.
3. **`server.go` + `health.go`.** You should be able to `go run ./cmd/api`
   (once step 6 exists) and `curl localhost:8080/healthz` before writing a
   single business-logic handler.
4. **`auth.go`**, with tests.
5. **`bookings.go`**, with tests.
6. **`cmd/api/main.go`.** This is when a real server first exists.
7. **Verify:** `go vet ./...`, `go build ./...`, `go test ./... -race`, and
   if you have Postgres running locally, a manual pass with `curl` through
   the full register → login → check availability → create booking →
   list bookings → cancel flow.
8. **Point the frontend at it.** `web/src/lib/api/index.js` decides between
   the mock and the real `client.js` — once your server is up, switch it
   over and click through the actual UI in a browser. This is the real test
   of whether your DTOs match what pages expect; a passing `go test` doesn't
   prove the frontend can talk to you.

---

## 6. What this leaves for later

Don't try to build these as part of this assignment — they're bigger,
separate pieces of work:

1. **Payment adapters + a `PaymentRepo`.** The last thing standing between
   an unpaid hold and a confirmed booking. The eSewa/Khalti callbacks are
   attacker-reachable and deserve a focused pass on their own, not to be
   folded into general API work where signature verification gets rushed.
2. **Email delivery**, to make password reset actually usable end to end.
3. **Repositories for teams, tournaments, and matchmaking**, then their
   endpoints — the domain types already exist in `internal/domain/team.go`,
   `tournament.go`, and `matchmaking.go`, but nothing stores them yet.
4. **Arena and court management** — owner-facing writes, and the first
   place `domain.AccountArenaOwner` and `domain.CodeForbidden` do real
   authorization work rather than just existing as an enum value.
5. **Rate limiting**, if you deferred it in Part D.
