#!/usr/bin/env bash
#
# Smoke-test a running Khel Arena API.
#
# Walks the endpoints a real client uses, in the order a real client uses them:
# health, sign up, sign in, read the availability grid, take a hold, fail to
# double-book it, list it, cancel it. Every step asserts a status code, so a
# regression shows up as a failed line rather than as output somebody has to
# read carefully.
#
#   ./scripts/smoke.sh                       # against localhost:8080
#   BASE_URL=https://api.example.com ./scripts/smoke.sh
#   COURT_ID=<uuid> ./scripts/smoke.sh       # skip court discovery
#
# Exits non-zero if any check fails, so it is usable as a deploy gate.
#
# Needs: curl, python3. The booking half also needs a court to exist -- it is
# discovered through psql if DATABASE_URL is reachable, and skipped with a
# note if not, because there is no endpoint that lists courts yet.

set -uo pipefail

BASE="${BASE_URL:-http://localhost:8080}"

# A fresh account per run. Re-running must not fail on a unique constraint,
# and the API has no delete-account endpoint to clean up with.
STAMP=$(date +%s)
EMAIL="smoke${STAMP}@khelarena.np"
USERNAME="smoke_${STAMP}"
PASSWORD="kathmandu2026"

TMP=$(mktemp -d)
JAR="$TMP/cookies.txt"
trap 'rm -rf "$TMP"' EXIT

if [ -t 1 ]; then
  GREEN=$(tput setaf 2); RED=$(tput setaf 1); DIM=$(tput dim); RESET=$(tput sgr0)
else
  GREEN=""; RED=""; DIM=""; RESET=""
fi

PASSED=0
FAILED=0
TOKEN=""
REGISTERED=no

ok()   { printf '  %sPASS%s %s\n' "$GREEN" "$RESET" "$1"; PASSED=$((PASSED + 1)); }
bad()  { printf '  %sFAIL%s %s\n' "$RED" "$RESET" "$1"; FAILED=$((FAILED + 1)); }
note() { printf '  %s%s%s\n' "$DIM" "$1" "$RESET"; }
head_() { printf '\n%s\n' "$1"; }

# request METHOD PATH [BODY] [auth|noauth]
#
# Leaves the response in $BODY and the status in $STATUS. The cookie jar is
# always used, because the refresh token is httpOnly and lives nowhere else.
request() {
  local method="$1" path="$2" body="${3:-}" auth="${4:-auth}"
  local args=(-s -o "$TMP/body" -w '%{http_code}' -X "$method" "$BASE$path" -b "$JAR" -c "$JAR")

  if [ -n "$body" ]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi
  if [ "$auth" = auth ] && [ -n "$TOKEN" ]; then
    args+=(-H "Authorization: Bearer $TOKEN")
  fi

  : > "$TMP/body"
  STATUS=$(curl "${args[@]}")
  BODY=$(cat "$TMP/body" 2>/dev/null)
}

# check DESC WANT_STATUS — asserts the status of the last request.
check() {
  local desc="$1" want="$2"
  if [ "$STATUS" = "$want" ]; then
    ok "$desc ($STATUS)"
  else
    bad "$desc — got $STATUS, want $want${BODY:+ — $BODY}"
  fi
}

# json FIELD — pulls one top-level field out of the last response.
json() {
  printf '%s' "$BODY" | python3 -c "
import sys, json
try:
    print(json.load(sys.stdin).get('$1', ''))
except Exception:
    print('')
"
}

# ---------------------------------------------------------------- health --

head_ "Health"

request GET /healthz "" noauth
check "GET /healthz" 200

request GET /readyz "" noauth
check "GET /readyz" 200

# ------------------------------------------------------------------ auth --

head_ "Auth"

request POST /v1/auth/register "$(printf '{
  "full_name": "Smoke Test",
  "username": "%s",
  "email": "%s",
  "password": "%s",
  "skill": "casual"
}' "$USERNAME" "$EMAIL" "$PASSWORD")" noauth
check "POST /v1/auth/register" 201
[ "$STATUS" = 201 ] && REGISTERED=yes

# The refresh token is the long-lived credential; it belongs in the httpOnly
# cookie and nowhere a script could read it.
case "$BODY" in
  *refresh_token*) bad "refresh token appears in the register response body" ;;
  *)               ok  "refresh token is not in the response body" ;;
esac

request POST /v1/auth/login "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" noauth
check "POST /v1/auth/login" 200
TOKEN=$(json access_token)
if [ -n "$TOKEN" ]; then ok "access token issued"; else bad "no access token in the login response"; fi

request GET /v1/me
check "GET /v1/me" 200

request GET /v1/me "" noauth
check "GET /v1/me without a token is refused" 401

request POST /v1/auth/login "{\"email\":\"$EMAIL\",\"password\":\"wrong-password\"}" noauth
check "POST /v1/auth/login with a bad password" 401

request POST /v1/auth/refresh
check "POST /v1/auth/refresh" 200
TOKEN=$(json access_token)

# Always 202, and never the token: a different answer for an unregistered
# address would turn this endpoint into an account-enumeration oracle.
request POST /v1/auth/password/forgot "{\"email\":\"$EMAIL\"}" noauth
check "POST /v1/auth/password/forgot (known address)" 202
request POST /v1/auth/password/forgot '{"email":"nobody-here@khelarena.np"}' noauth
check "POST /v1/auth/password/forgot (unknown address)" 202

request POST /v1/auth/register '{"user_name":"typo","email":"t@k.np","password":"kathmandu2026"}' noauth
check "an unknown JSON field is rejected" 400

# --------------------------------------------------------------- courts ---

head_ "Availability"

COURT="${COURT_ID:-}"
if [ -z "$COURT" ] && command -v psql >/dev/null 2>&1 && [ -n "${DATABASE_URL:-}" ]; then
  COURT=$(psql "$DATABASE_URL" -tA -c \
    'select id from courts where is_active limit 1' 2>/dev/null | tr -d '[:space:]')
fi

if [ -z "$COURT" ]; then
  note "No court to test against. Set COURT_ID=<uuid>, or DATABASE_URL so it"
  note "can be looked up. Skipping availability and booking."
else
  note "court $COURT"

  # Tomorrow, so the grid is not mostly in the past. GNU and BSD date disagree
  # about how to say that, hence python.
  DATE=$(python3 -c 'import datetime;print(datetime.date.today()+datetime.timedelta(days=1))')

  request GET "/v1/courts/$COURT/availability?date=$DATE" "" noauth
  check "GET availability?date=$DATE" 200

  SLOTS=$(printf '%s' "$BODY" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["slots"]))' 2>/dev/null || echo 0)
  if [ "$SLOTS" -gt 0 ]; then ok "$SLOTS slots in the grid"; else bad "the grid came back empty"; fi

  # Pick the first bookable hour rather than hardcoding one, so the script
  # keeps working whatever the arena's hours are and whatever is already taken.
  read -r START END < <(printf '%s' "$BODY" | python3 -c '
import sys, json
free = [s for s in json.load(sys.stdin)["slots"] if s["available"]]
print(free[0]["starts_at"], free[0]["ends_at"]) if free else print("", "")
')

  request GET "/v1/courts/$COURT/availability" "" noauth
  check "availability with no ?date= is refused" 400
  request GET "/v1/courts/$COURT/availability?date=tomorrow" "" noauth
  check "availability with an unparseable date is refused" 400
  request GET "/v1/courts/not-a-uuid/availability?date=$DATE" "" noauth
  check "availability for a malformed court id is refused" 400

  # ------------------------------------------------------------ bookings --

  head_ "Bookings"

  if [ -z "$START" ]; then
    note "Every hour on $DATE is already taken or past. Skipping the booking flow."
  else
    note "booking $START"
    BOOK="{\"court_id\":\"$COURT\",\"starts_at\":\"$START\",\"ends_at\":\"$END\",\"note\":\"smoke test\"}"

    request POST /v1/bookings "$BOOK"
    check "POST /v1/bookings" 201
    BOOKING_ID=$(json id)
    PRICE=$(json price_npr)
    if [ -n "$PRICE" ] && [ "$PRICE" -gt 0 ] 2>/dev/null; then
      ok "price resolved server-side: $PRICE NPR"
    else
      bad "no price on the booking"
    fi

    # The exclusion constraint, seen from the outside.
    request POST /v1/bookings "$BOOK"
    check "the same hour again is a conflict" 409

    request GET "/v1/bookings?limit=20"
    check "GET /v1/bookings" 200
    case "$BODY" in
      *"$BOOKING_ID"*) ok "the new booking is in the list" ;;
      *)               bad "the new booking is missing from the list" ;;
    esac

    request GET "/v1/bookings?limit=twenty"
    check "a non-numeric ?limit= is refused" 400

    request DELETE "/v1/bookings/$BOOKING_ID"
    check "DELETE /v1/bookings/{id}" 204

    request DELETE "/v1/bookings/not-a-uuid"
    check "DELETE with a malformed id is refused" 400

    request GET /v1/bookings
    case "$BODY" in
      *cancelled*) ok "the booking now reads as cancelled" ;;
      *)           bad "the booking did not end up cancelled" ;;
    esac
  fi
fi

# --------------------------------------------------------------- logout ---

head_ "Logout"

request POST /v1/auth/logout
check "POST /v1/auth/logout" 204
request POST /v1/auth/logout
check "POST /v1/auth/logout again is still fine" 204

# --------------------------------------------------------------- result ---

printf '\n%d passed, %d failed\n' "$PASSED" "$FAILED"
if [ "$REGISTERED" = yes ]; then
  note "left behind: the account $EMAIL (there is no delete-account endpoint yet)"
fi

[ "$FAILED" -eq 0 ]
