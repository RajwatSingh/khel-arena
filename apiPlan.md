# Khel Arena — HTTP API Plan

The plan for the layer `documentation.md` §12 calls "the next phase": `cmd/api`,
routing, JSON, middleware, and the `domain.Code` → status mapping that `Code`
was designed for.

Nothing here is built yet. This document is for deciding *what* to build; the
open questions are collected in §11 and none of the code should be written
until those are settled.

---

## 1. Scope

### In

Two services have repositories behind them, so those are what can honestly be
exposed:

- **`AuthService`** — 8 methods, all reachable.
- **`BookingService`** — 4 of 5 reachable (`ReleaseStaleHolds` belongs to the
  janitor, not to a request).

Plus the transport machinery that has no equivalent today: error mapping, JSON
encode/decode, middleware, the `cmd/api` binary, and graceful shutdown.

### Out, and why

- **Teams, tournaments, matchmaking, arena management.** Domain types and
  tables exist; `internal/postgres` has no repositories for them. Endpoints
  would have nothing to call. This is a storage-layer job first, roughly
  comparable in size to everything below.
- **Payments.** `domain.Payment` holds the security-critical logic but there is
  no `PaymentRepo` and no eSewa/Khalti adapter. §12 is right that callbacks are
  attacker-reachable and deserve their own focused pass — folding them into
  general API work is how signature verification gets rushed.
- **Email delivery.** Affects what `/auth/password/forgot` can do; see §9.

### Dependencies

None added. Go 1.22+ `net/http.ServeMux` handles method-and-wildcard patterns
(`POST /v1/bookings/{id}`) natively, so `go.mod` stays at its four direct
requires. This matches the existing no-framework posture rather than fighting
it.

---

## 2. Package layout

```
cmd/api/main.go              Config, wiring, janitor, graceful shutdown

internal/api/                The transport layer. Imports service; never pgx.
  server.go                  Server struct, dependency interfaces, routes()
  errors.go                  domain.Code -> status, error response writing
  json.go                    decode/encode helpers, body size limits
  middleware.go              request ID, logging, recovery, CORS, auth
  auth.go                    handlers for /v1/auth/*
  bookings.go                handlers for /v1/bookings, /v1/courts/*
  health.go                  /healthz, /readyz
  dto.go                     wire types + domain -> wire conversion
```

**On the package name.** `internal/http` would shadow `net/http` at every use
site inside it. `internal/api` (package `api`) avoids that; the alternative is
`internal/transport/http` with `package httpapi`, which is more explicit but
adds a directory level for one package. Recommending `internal/api`.

**Dependency direction.** `api` imports `service` and `domain`. It must not
import `pgx` — same rule the service layer follows. See §11.4 for the one place
that rule is currently awkward.

---

## 3. Endpoints

### Public

| Method | Path | Calls | Notes |
|---|---|---|---|
| GET | `/healthz` | — | Liveness. Always 200 if the process is up. |
| GET | `/readyz` | `pool.Ping` | Readiness. 503 when the DB is unreachable. |
| POST | `/v1/auth/register` | `Register` | Returns a session. |
| POST | `/v1/auth/login` | `Login` | Rate-limit candidate (§11.3). |
| POST | `/v1/auth/refresh` | `Refresh` | Rotates the pair. |
| POST | `/v1/auth/logout` | `Logout` | Idempotent; 204 even if already out. |
| POST | `/v1/auth/password/forgot` | `BeginPasswordReset` | Always 202 (§9). |
| POST | `/v1/auth/password/reset` | `CompletePasswordReset` | Burns the token. |
| GET | `/v1/courts/{courtID}/availability` | `Availability` | `?date=YYYY-MM-DD`. |

### Authenticated

| Method | Path | Calls | Notes |
|---|---|---|---|
| GET | `/v1/me` | — | Needs a small addition; see §11.5. |
| POST | `/v1/auth/password/change` | `ChangePassword` | Proves the old password. |
| POST | `/v1/bookings` | `Create` | Takes a hold. 201. |
| GET | `/v1/bookings` | `ListMine` | `?limit=` , newest first. |
| DELETE | `/v1/bookings/{bookingID}` | `Cancel` | 204. Ownership enforced in SQL. |

Thirteen endpoints plus two probes.

`DELETE` for cancel rather than `POST /{id}/cancel`: the repository's `Cancel`
is already idempotent-ish and ownership-scoped in its `UPDATE`, so the verb
matches the semantics. Flag if you'd rather keep cancel as a POST for clients
that can't send DELETE.

---

## 4. Error mapping

This is the centrepiece and the reason `domain.Code` exists. One function, one
table, no handler making its own decision:

| `domain.Code` | Status | Body |
|---|---|---|
| `CodeInvalid` | 400 | message + per-field errors |
| `CodeUnauthenticated` | 401 | message |
| `CodeForbidden` | 403 | message |
| `CodeNotFound` | 404 | message |
| `CodeConflict` | 409 | message |
| `CodeRateLimited` | 429 | message + `Retry-After` |
| `CodeUnavailable` | 503 | message + `Retry-After` |
| `CodeInternal` | 500 | generic message only |
| *unclassified* | 500 | generic message only |

Three rules the implementation must hold to:

1. **The body always comes from `domain.UserMessage(err)`**, never from
   `err.Error()`. `UserMessage` already collapses `CodeInternal` and
   unclassified errors to the generic apology, which is what stops a pgx error
   putting a connection string in a response. `CodeOf` treating unknown errors
   as `CodeInternal` gives the same protection for status codes.

2. **`CodeInternal` is the only code that logs the cause.** `Error.Unwrap()`
   exposes it for the logger; it is never serialised. Every other code is an
   expected outcome and logging it at error level would bury real defects in
   401 noise.

3. **Field errors come from `Error.FieldErrors()`**, which already normalises
   the single-field and multi-field shapes. `Registration.Validate()` returns
   every problem at once specifically so a signup form can render them all —
   throwing that away at the transport boundary would waste the design.

Error body:

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

`fields` is omitted when empty.

---

## 5. JSON conventions

- **Success responses are the bare resource**, not an envelope. Errors carry
  the `error` key above. Mixing both into `{data, error}` buys little when the
  status code already says which it is.
- **`snake_case`** throughout, matching the SQL columns and the existing
  `Field` values in validation errors (`full_name`, `current_password`) —
  those strings are already user-visible and already snake_case, so the wire
  format should agree.
- **Times are RFC 3339 UTC.** `domain.Slot` normalises to UTC on construction,
  so this is just letting `time.Time` marshal itself.
- **`DayTime` renders as `"HH:MM"`.** It has a `ParseDayTime` already; the
  wire form should round-trip through it.
- **Money is `int` NPR**, never a float. It is `int` everywhere in the domain
  and in the schema.
- Request bodies capped with `http.MaxBytesReader` (64 KiB is generous here);
  decoder uses `DisallowUnknownFields` so a typo'd field is a 400 rather than
  a silently ignored value.

---

## 6. Wire types

Handlers must not marshal domain structs directly. Three concrete reasons, all
present in the current code:

- **`domain.User` carries `Email` and `EmailVerifiedAt`.** Fine for `/v1/me`,
  wrong the moment a user object appears in someone else's view. A separate
  `userDTO` and `publicUserDTO` make the distinction explicit rather than
  hoping no endpoint ever embeds the wrong one. (`PasswordHash` is already
  safe — `user.go` keeps it off `User` entirely, which is the right instinct
  and worth preserving at this layer too.)
- **`domain.Slot` is `{Start, End}` nested.** Clients want
  `starts_at`/`ends_at` flat on the booking, not `slot.start`.
- **`domain.BookingDetail` embeds `Booking`**, so default marshalling would
  nest or flatten depending on tags nobody has written yet. Explicit is better.

Planned DTOs: `sessionDTO`, `userDTO`, `bookingDTO`, `bookingDetailDTO`,
`gridSlotDTO`, `errorDTO`. Each with one `fromDomain` constructor, all in
`dto.go`, so the mapping is reviewable in one file.

Example — availability:

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

`available` is computed from `GridSlot.Available()` rather than left for the
client to re-derive — the rule already exists in the domain and clients
shouldn't reimplement it.

---

## 7. Middleware

Applied outermost to innermost:

1. **Panic recovery** — logs with stack, returns a 500 through the same error
   writer as everything else. Outermost so it catches panics in the others.
2. **Request ID** — accepts an inbound `X-Request-Id` or generates a UUID;
   into the context and onto the response.
3. **Structured logging** — `log/slog` (already a dependency via the janitor).
   One line per request: method, path, status, duration, request ID, user ID
   when authenticated.
4. **CORS** — origins from config. Only needed if a browser client on another
   origin talks to this; see §11.1.
5. **Timeout** — `http.TimeoutHandler` or a context deadline per request, so a
   slow query can't pin a connection indefinitely. The whole stack already
   threads `ctx` properly, so this works end to end.

Then, on protected routes only:

6. **Authentication** — reads `Authorization: Bearer …`, calls
   `AuthService.Authenticate`, puts `userID` and `AccountType` in the context.

`Authenticate` verifies the JWT in-process and touches no database, so this is
cheap enough to sit in front of every protected route without a per-request
round trip — which is the point of the short-lived access token.

Two supporting details:

- **Context keys are an unexported struct type** in the `api` package, with
  typed accessors. Not `string` keys.
- **`SessionContext` construction** — `postgres.SessionContext{UserAgent, IP}`
  is needed by `Register`, `Login` and `Refresh`. `UserAgent` is
  `r.UserAgent()`. `IP` is the problem: `r.RemoteAddr` behind a proxy is the
  proxy, and `X-Forwarded-For` is client-controlled and trivially spoofable
  unless you know how many trusted hops sit in front. This needs a config
  setting (§11.2), because `SessionContext.addr()` silently drops anything
  unparseable — a wrong answer here means session audit rows quietly record
  nothing rather than failing loudly.

---

## 8. `cmd/api/main.go`

```
config.Load()                       fails fast, reports every problem at once
slog handler                        JSON in production, text in development
postgres.Connect(ctx, cfg.Database)
repos: UserRepo, SessionRepo, BookingRepo
token.NewIssuer(secret, issuer, accessTTL)
services: AuthService, BookingService, Janitor
api.NewServer(...) -> http.Handler
http.Server{ Addr, Handler, ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout }
go janitor.Run(ctx)
go server.ListenAndServe()
<-ctx.Done()                        SIGINT / SIGTERM via signal.NotifyContext
server.Shutdown(shutdownCtx)        stop accepting, drain in flight
cancel janitor, wait
pool.Close()
```

Shutdown order matters and is easy to get wrong: **drain HTTP first, then stop
the janitor, then close the pool.** Closing the pool while a request is in
flight turns a clean shutdown into a burst of 500s. `cmd/migrate/main.go`
already demonstrates the `signal.NotifyContext` half of this.

`ReadHeaderTimeout` is not optional — without it the server is trivially held
open by a client that never finishes its headers.

---

## 9. Password reset, specifically

`BeginPasswordReset` returns the reset token to its caller, and §12 notes that
nothing sends email yet. That creates a trap worth naming before any code is
written:

**The token must not appear in the HTTP response.** Returning it would let
anyone reset any account by calling the endpoint and reading the reply — the
exact opposite of what the token is for. The endpoint returns **202 with no
body regardless of whether the address is registered**, which also preserves
the anti-enumeration property the service comment describes.

Until email exists, the token goes to the log at info level **in development
only**, gated on `cfg.IsProduction()`. That is enough to test the flow locally
and impossible to reach in production.

---

## 10. Testing

Handlers become testable by declaring narrow interfaces in the `api` package —
the same consumer-declared pattern `BookingStore` and `UserStore` already use,
applied one layer up:

```go
type AuthAPI interface {
    Register(ctx context.Context, reg domain.Registration, sc postgres.SessionContext) (service.Session, error)
    Login(...)  // etc.
}
```

`*service.AuthService` satisfies it; a fake in a test satisfies it without a
database or a token issuer.

| File | Covers |
|---|---|
| `errors_test.go` | Every `Code` → status; that `CodeInternal` and unclassified errors never leak a cause into the body; field-error rendering |
| `middleware_test.go` | Auth with valid, expired, malformed, and missing tokens; recovery converting a panic to a 500 |
| `auth_test.go` | Each handler's happy path and its main failure, via `httptest` |
| `bookings_test.go` | Same, plus date parsing on availability |

`go test ./... -race` already runs in CI. These are all in-process — no
database needed, so they run in the same job without touching the Postgres
service container.

**Note:** CI pins `go-version: '1.24'` while `go.mod` declares `go 1.26.5`.
With `GOTOOLCHAIN=auto` this downloads a toolchain on every run rather than
failing, but it's an unnecessary download and a confusing signal. Worth
bumping while this branch is open.

---

## 11. Open decisions

These change the code enough to be worth settling first.

### 11.1 Refresh token transport

| | httpOnly cookie | JSON body |
|---|---|---|
| XSS exposure | Token unreachable from JS | Reachable if stored in `localStorage` |
| Mobile clients | Awkward | Natural |
| CORS | Needs `credentials`, explicit origins | Simple |
| CSRF | Needs `SameSite` + consideration | Not applicable |
| curl testing | Fiddly | Easy |

Cookie is the safer default for a browser client; body is the right answer for
a mobile client. A third option accepts both — cookie first, body as fallback —
at the cost of a branch in three handlers.

Whatever is chosen, isolate it: one `readRefreshToken(r)` and one
`writeSession(w, session)` used by login, refresh and logout alike, so changing
the answer later touches two functions rather than every auth handler.

**This is the one decision that most affects the shape of §3.**

### 11.2 Trusted proxy configuration

Will this sit behind a reverse proxy or load balancer? If yes, config needs the
trusted-hop count (or trusted CIDRs) so `X-Forwarded-For` can be read safely.
If it's directly exposed, `r.RemoteAddr` is the honest answer and
`X-Forwarded-For` must be ignored entirely.

### 11.3 Rate limiting — now or later?

`CodeRateLimited` exists and nothing produces it. `Login` and
`password/forgot` are the endpoints that need it: the former is an
online-guessing target, the latter can be used to spray email. An in-process
token bucket keyed by IP is ~60 lines and no new dependency; it does not
survive multiple instances, which may or may not matter yet.

### 11.4 `postgres.SessionContext` in the transport layer

`service` currently imports `postgres` solely for `SessionContext` and
`CourtContext` — the inner layer depending on the outer one. Building the API
makes it worse: handlers will import `postgres` too, just to construct a
two-string struct.

Moving `SessionContext` (and `CourtContext`) into `domain` fixes it. Neither
has a database-specific field, `postgres` already imports `domain`, and the
scan code is untouched. It is a ~15 minute mechanical change and it is much
easier now than after handlers depend on the current shape.

Worth doing as a preliminary step — say if you'd rather not.

### 11.5 Profile endpoints

`GET /v1/me` has no service method behind it. `UserRepo.ByID` and
`UserRepo.UpdateProfile` both exist, but no service wraps them, and handlers
should not call repositories directly.

Options: (a) add a small `ProfileService` with `Me` and `UpdateProfile` — about
40 lines, and it makes `PATCH /v1/me` available too; (b) drop `/v1/me` from
this pass, and let clients rely on the user object returned by login and
refresh. (a) is recommended — the profile is a natural thing for a client to
re-read, and `ProfileUpdate`'s pointer-based partial-update design is already
built for a PATCH.

### 11.6 Versioning

Plan assumes a `/v1` prefix. Cheap now, impossible to add gracefully later.
Confirm.

---

## 12. Sequencing and effort

Each phase compiles and is independently reviewable.

| # | Phase | Output | Est. |
|---|---|---|---|
| 0 | `SessionContext` → `domain` (if §11.4 agreed) | Mechanical refactor | 15 min |
| 1 | Foundation | `errors.go`, `json.go`, `dto.go` + tests | 45 min |
| 2 | Middleware | `middleware.go` + tests | 30 min |
| 3 | Server + health | `server.go`, `health.go`, routing table | 20 min |
| 4 | Auth handlers | `auth.go` + tests | 45 min |
| 5 | Booking handlers | `bookings.go` + tests | 40 min |
| 6 | Binary | `cmd/api/main.go`, graceful shutdown | 30 min |
| 7 | Verify | `go vet`, `go build`, `go test -race`, manual smoke | 25 min |
| 8 | Docs | Update `documentation.md` §12 and the file map | 20 min |

**Roughly 4½ hours of working time** for the complete, tested layer; about
1,200–1,500 lines across 9 files. A running server you can curl arrives at the
end of phase 6 — call it 3½ hours — with phases 7–8 turning it from working
into finished.

Dropping the handler tests saves roughly an hour and is a reasonable trade if
you want it demoable sooner, but it should be a decision rather than a
drift — the error-mapping tests in phase 1 are the ones I'd least want to skip,
since that code is what stands between an internal error and a leaked query.

End-to-end verification needs a live Postgres. If you have one running, say so
and phase 7 includes a real smoke test against it; otherwise it is limited to
build, vet, and the in-process tests.

---

## 13. What this leaves for later

In rough priority order:

1. **Payment adapters + `PaymentRepo`** — the last thing standing between a
   hold and a confirmed booking. Deserves its own pass; the callbacks are
   attacker-reachable.
2. **Email delivery** — makes password reset genuinely usable.
3. **Repositories for teams, tournaments, matchmaking** — then their endpoints.
4. **Arena and court management** — owner-facing writes; the only place
   `AccountArenaOwner` and `CodeForbidden` start doing real work.
5. **Rate limiting**, if deferred at §11.3.
