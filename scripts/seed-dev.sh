#!/usr/bin/env bash
#
# Seed a local Khel Arena for development.
#
# Creates two accounts and, for the owner, one venue with a court and an
# evening-peak rate so there is something to book straight away:
#
#   player   rajwat@khelarena.np   / kathmandu2026   (shown on the login page in dev)
#   owner    owner@khelarena.np    / kathmandu2026
#
#   ./scripts/seed-dev.sh
#   BASE_URL=http://localhost:5173 ./scripts/seed-dev.sh   # through the web proxy
#
# Idempotent: an account that already exists is left alone. Talks to a running
# API over HTTP, so it needs the server up (make run) and nothing else but
# curl and python3. Never point this at production — it puts known-password
# accounts in the database.

set -uo pipefail

BASE="${BASE_URL:-http://localhost:8080}"
PASSWORD="kathmandu2026"

say()  { printf '  %s\n' "$*" >&2; }
json() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get(sys.argv[1],""))' "$1"; }

# register EMAIL USERNAME "FULL NAME" ACCOUNT_TYPE  ->  prints access_token (new or existing)
register() {
  local email="$1" username="$2" name="$3" acct="$4" body resp code
  body=$(printf '{"email":"%s","username":"%s","full_name":"%s","password":"%s","account_type":"%s"}' \
    "$email" "$username" "$name" "$PASSWORD" "$acct")

  resp=$(curl -s -w $'\n%{http_code}' -X POST "$BASE/v1/auth/register" \
    -H 'content-type: application/json' -d "$body")
  code=${resp##*$'\n'}
  resp=${resp%$'\n'*}

  if [ "$code" = "201" ]; then
    say "created $acct  $email"
    printf '%s' "$resp" | json access_token
    return
  fi

  # Already there (409) or any other non-fatal case: sign in instead.
  resp=$(curl -s -X POST "$BASE/v1/auth/login" -H 'content-type: application/json' \
    -d "$(printf '{"email":"%s","password":"%s"}' "$email" "$PASSWORD")")
  local tok
  tok=$(printf '%s' "$resp" | json access_token)
  if [ -n "$tok" ]; then
    say "exists  $acct  $email"
    printf '%s' "$tok"
  else
    say "FAILED  $email  (register $code, and sign-in did not work)"
    printf '%s\n' "$resp" >&2
  fi
}

# ---------------------------------------------------------------------------

if ! curl -sf -o /dev/null "$BASE/v1/arenas" 2>/dev/null; then
  echo "seed-dev: no API reachable at $BASE/v1 — start it with 'make run' first" >&2
  exit 1
fi

echo "Seeding $BASE"

PLAYER_TOKEN=$(register "rajwat@khelarena.np" "rajwat" "Rajwat Singh" "player")
OWNER_TOKEN=$(register "owner@khelarena.np"  "dhuku"  "Dhuku Futsal"  "arena_owner")

if [ -z "$OWNER_TOKEN" ]; then
  echo "seed-dev: could not obtain an owner session; skipping the venue" >&2
  exit 0
fi

# Give the owner a venue only if they have none.
ARENA_COUNT=$(curl -s "$BASE/v1/owner/arenas" -H "authorization: Bearer $OWNER_TOKEN" \
  | python3 -c 'import sys,json; print(len(json.load(sys.stdin)))' 2>/dev/null || echo 0)

if [ "$ARENA_COUNT" != "0" ]; then
  say "venue   already has $ARENA_COUNT — leaving it"
  echo "Done."
  exit 0
fi

ARENA=$(curl -s -X POST "$BASE/v1/owner/arenas" -H "authorization: Bearer $OWNER_TOKEN" \
  -H 'content-type: application/json' \
  -d '{"name":"Jhamsikhel Futsal Club","area":"Jhamsikhel","city":"Lalitpur","opens_at":"06:00","closes_at":"23:00","amenities":["Covered","Floodlit"],"phone":"9851000000","description":"Demo venue — two covered courts behind the Jhamsikhel bus stop."}')
ARENA_ID=$(printf '%s' "$ARENA" | json id)
if [ -z "$ARENA_ID" ]; then
  say "venue   could not create (name/slug may be taken) — skipping court + rate"
  printf '%s\n' "$ARENA" >&2
  echo "Done (accounts seeded; venue skipped)."
  exit 0
fi
say "venue   Jhamsikhel Futsal Club ($ARENA_ID)"

COURT=$(curl -s -X POST "$BASE/v1/owner/arenas/$ARENA_ID/courts" -H "authorization: Bearer $OWNER_TOKEN" \
  -H 'content-type: application/json' \
  -d '{"name":"Court A","format":"5-a-side","side_count":5,"base_price_npr":1200}')
COURT_ID=$(printf '%s' "$COURT" | json id)
[ -n "$COURT_ID" ] && say "court   Court A ($COURT_ID)" || { say "court   FAILED"; printf '%s\n' "$COURT" >&2; exit 1; }

curl -s -o /dev/null -X POST "$BASE/v1/owner/courts/$COURT_ID/pricing" -H "authorization: Bearer $OWNER_TOKEN" \
  -H 'content-type: application/json' \
  -d '{"label":"Evening Peak","days":[1,2,3,4,5,6,7],"start_hour":17,"end_hour":22,"price_npr":1800,"is_peak":true}'
say "rate    Evening Peak 17:00–22:00 · NPR 1800"

echo "Done. Sign in at /login as rajwat@khelarena.np / $PASSWORD"
