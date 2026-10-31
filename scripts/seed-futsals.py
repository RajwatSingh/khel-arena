#!/usr/bin/env python3
"""
Seed five real Nepali futsal venues for local development.

Each venue gets its own arena_owner account, one court, a couple of pricing
rules and two gallery photos. Run with the API up (make run):

    python3 scripts/seed-futsals.py
    BASE_URL=http://localhost:5173 python3 scripts/seed-futsals.py

Safe to re-run: existing owners and venues are reused, and only missing
photos are uploaded.

Where the data comes from
-------------------------
Names, areas and phone numbers come from public futsal directories
(sourcenepal.com, collegesnepal.com) and news reports. Those lists are old,
so treat the details as unverified. Opening hours and most prices are
plausible guesses, not the venues' real rates.

The photos are NOT of these venues. They are freely licensed photos of
artificial-turf futsal courts from Wikimedia Commons, credited in each
caption as their licences require. Swap in the venues' own photos (with
permission) before using any of this outside a laptop.

Development only: every account here has a known password.
"""

import json
import os
import re
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import uuid

BASE = os.environ.get("BASE_URL", "http://localhost:8080").rstrip("/")
# The published default is for a laptop only. Anywhere else, set SEED_PASSWORD
# to something only you know -- this file is public.
PASSWORD = os.environ.get("SEED_PASSWORD", "kathmandu2026")
UA = "khel-arena-dev-seed/1.0 (local development)"

ALL_DAYS = [1, 2, 3, 4, 5, 6, 7]  # ISO weekdays, 1 = Monday
SUN_TO_FRI = [7, 1, 2, 3, 4, 5]   # Saturday is the weekend in Nepal

VENUES = [
    {
        "owner": {"email": "fiveaside@khelarena.np", "username": "fiveaside_futsal",
                  "full_name": "5A Side Indoor Futsal"},
        "arena": {
            "name": "5A Side Indoor Futsal", "area": "Sanepa", "city": "Lalitpur",
            "phone": "015535453", "opens_at": "06:00", "closes_at": "21:00",
            "amenities": ["Indoor", "Training sessions", "Tournaments"],
            "description": "Indoor five-a-side futsal in Sanepa, open to players of all ages. "
                           "Also runs training sessions and tournaments.",
        },
        "court": {"name": "Indoor Court", "format": "5-a-side", "side_count": 5, "base_price_npr": 1200},
        "rules": [
            {"label": "Evening Peak", "days": ALL_DAYS, "start_hour": 17, "end_hour": 21,
             "price_npr": 1600, "is_peak": True},
        ],
        "photos": ["File:Indoor soccer singapore z.JPG", "File:Futsal Court.jpg"],
    },
    {
        "owner": {"email": "dhobighat@khelarena.np", "username": "dhobighat_futsal",
                  "full_name": "Dhobighat Futsal"},
        "arena": {
            "name": "Dhobighat Futsal", "area": "Dhobighat", "city": "Lalitpur",
            "opens_at": "06:00", "closes_at": "21:00",
            "amenities": ["Floodlit", "Tournaments"],
            "description": "Futsal ground in Dhobighat, Lalitpur, and a regular host of local "
                           "5-a-side tournaments.",
        },
        "court": {"name": "Court A", "format": "5-a-side", "side_count": 5, "base_price_npr": 1200},
        "rules": [
            {"label": "Evening Peak", "days": ALL_DAYS, "start_hour": 17, "end_hour": 21,
             "price_npr": 1600, "is_peak": True},
        ],
        "photos": ["File:Bong da mini.jpg", "File:Mini football outdoors in Vietnam.jpg"],
    },
    {
        "owner": {"email": "field@khelarena.np", "username": "field_futsal",
                  "full_name": "Field Futsal"},
        "arena": {
            "name": "Field Futsal", "area": "Sanepa", "city": "Lalitpur",
            "opens_at": "06:00", "closes_at": "21:00",
            "amenities": ["Floodlit", "Parking"],
            "description": "Futsal ground in Sanepa, Lalitpur, popular for youth and community tournaments.",
        },
        "court": {"name": "Court A", "format": "5-a-side", "side_count": 5, "base_price_npr": 1100},
        "rules": [
            {"label": "Evening Peak", "days": ALL_DAYS, "start_hour": 17, "end_hour": 21,
             "price_npr": 1500, "is_peak": True},
        ],
        "photos": ["File:Futsal Tennis Court.jpg", "File:Five-a-side football enclosed pitch, The Oval, Bebington.jpg"],
    },
    {
        "owner": {"email": "royal@khelarena.np", "username": "royal_futsal",
                  "full_name": "Royal Futsal"},
        "arena": {
            "name": "Royal Futsal", "area": "Thapagaun, Anamnagar", "city": "Kathmandu",
            "phone": "9808192257", "opens_at": "06:00", "closes_at": "21:00",
            "amenities": ["Changing rooms", "Floodlit"],
            "description": "Single-court futsal in Thapagaun, Anamnagar, a few minutes from New Baneshwor.",
        },
        "court": {"name": "Main Court", "format": "5-a-side", "side_count": 5, "base_price_npr": 1200},
        "rules": [
            {"label": "Evening Peak", "days": ALL_DAYS, "start_hour": 17, "end_hour": 21,
             "price_npr": 1600, "is_peak": True},
        ],
        "photos": ["File:Futsal Ground Miyashita Park Tokyo.jpg", "File:フットサルコート.JPG"],
    },
    {
        "owner": {"email": "dhanyantari@khelarena.np", "username": "dhanyantari_futsal",
                  "full_name": "Dhanyantari Futsal"},
        "arena": {
            "name": "Dhanyantari Futsal", "area": "Hadigaun", "city": "Kathmandu",
            "phone": "014009635", "opens_at": "06:00", "closes_at": "21:00",
            "amenities": ["Wi-Fi", "Café", "Showers", "Swimming pool"],
            "description": "One of the best-known futsal grounds in Kathmandu, on Dhanawantari Marg "
                           "in Hadigaun. Café, Wi-Fi, showers and a swimming pool on site.",
        },
        "court": {"name": "Court A", "format": "5-a-side", "side_count": 5, "base_price_npr": 1200},
        "rules": [
            {"label": "Daytime", "days": SUN_TO_FRI, "start_hour": 10, "end_hour": 16,
             "price_npr": 1000, "is_peak": False},
            {"label": "Evening Peak", "days": ALL_DAYS, "start_hour": 17, "end_hour": 21,
             "price_npr": 1700, "is_peak": True},
        ],
        "photos": ["File:Tokyo Yoyogi futsal court.JPG",
                   'File:Artificial "five a side" football pitch - geograph.org.uk - 1915895.jpg'],
    },
]


def say(msg):
    print("  " + msg, file=sys.stderr)


def call(method, path, body=None, token=None, raw=None, content_type=None):
    """One API request. Returns (status, parsed JSON or None)."""
    headers = {}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    elif raw is not None:
        data = raw
        headers["Content-Type"] = content_type
    if token:
        headers["Authorization"] = "Bearer " + token

    req = urllib.request.Request(BASE + "/v1" + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req) as resp:
            text = resp.read()
            return resp.status, (json.loads(text) if text else None)
    except urllib.error.HTTPError as e:
        text = e.read()
        try:
            return e.code, json.loads(text)
        except ValueError:
            return e.code, {"error": {"message": text.decode(errors="replace")}}


def sign_in(owner):
    """Register the owner, or sign in if they already exist. Returns a token."""
    status, resp = call("POST", "/auth/register",
                        {**owner, "password": PASSWORD, "account_type": "arena_owner"})
    if status == 201:
        say(f"created owner  {owner['email']}")
        return resp["access_token"]

    status, resp = call("POST", "/auth/login", {"email": owner["email"], "password": PASSWORD})
    if status == 200:
        say(f"exists  owner  {owner['email']}")
        return resp["access_token"]

    say(f"FAILED  {owner['email']}: {resp}")
    return None


def commons_photo(title, dest_dir):
    """Download a Commons file (at most 1280px wide). Returns (path, caption)."""
    q = urllib.parse.urlencode({
        "action": "query", "format": "json", "prop": "imageinfo", "titles": title,
        "iiprop": "url|size|extmetadata", "iiurlwidth": "1280",
    })
    req = urllib.request.Request("https://commons.wikimedia.org/w/api.php?" + q,
                                 headers={"User-Agent": UA})
    page = next(iter(json.load(urllib.request.urlopen(req))["query"]["pages"].values()))
    info = page["imageinfo"][0]
    meta = info.get("extmetadata", {})

    # Commons refuses to upscale, so small originals are fetched as they are.
    url = info["thumburl"] if info["width"] > 1280 else info["url"]
    artist = re.sub(r"<[^>]+>", "", meta.get("Artist", {}).get("value", "")).strip() or "Unknown"
    licence = meta.get("LicenseShortName", {}).get("value", "see Commons")

    data = urllib.request.urlopen(urllib.request.Request(url, headers={"User-Agent": UA})).read()
    path = os.path.join(dest_dir, uuid.uuid4().hex + os.path.splitext(url)[1].lower())
    with open(path, "wb") as f:
        f.write(data)

    # The column holds 120 characters, so name the source rather than link it.
    caption = f"Photo: {artist}, {licence} (Wikimedia Commons)"
    if len(caption) > 120:
        caption = f"Photo: {artist[:120 - len(licence) - 30]}…, {licence} (Wikimedia Commons)"
    return path, caption


def upload_photo(token, arena_id, path, caption, sort_order):
    boundary = uuid.uuid4().hex
    with open(path, "rb") as f:
        file_bytes = f.read()

    parts = []
    for name, value in (("caption", caption), ("sort_order", str(sort_order))):
        parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n'
                     f"{value}\r\n".encode())
    parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; '
                 f'filename="{os.path.basename(path)}"\r\nContent-Type: application/octet-stream\r\n\r\n'
                 .encode() + file_bytes + b"\r\n")
    parts.append(f"--{boundary}--\r\n".encode())

    return call("POST", f"/owner/arenas/{arena_id}/photos/upload", token=token,
                raw=b"".join(parts), content_type=f"multipart/form-data; boundary={boundary}")


def create_venue(venue, token):
    status, arena = call("POST", "/owner/arenas", venue["arena"], token=token)
    if status != 201:
        say(f"FAILED  venue {venue['arena']['name']}: {arena}")
        return None
    say(f"venue   {arena['name']}  ({arena['area']}, {arena['city']})")

    status, court = call("POST", f"/owner/arenas/{arena['id']}/courts", venue["court"], token=token)
    if status != 201:
        say(f"FAILED  court: {court}")
        return arena
    for rule in venue["rules"]:
        status, resp = call("POST", f"/owner/courts/{court['id']}/pricing", rule, token=token)
        if status != 201:
            say(f"FAILED  rate {rule['label']}: {resp}")
    say(f"court   {venue['court']['name']} · NPR {venue['court']['base_price_npr']}/hr, "
        f"{len(venue['rules'])} rate(s)")
    return arena


def seed(venue, tmp):
    token = sign_in(venue["owner"])
    if not token:
        return

    status, existing = call("GET", "/owner/arenas", token=token)
    if status == 200 and existing:
        arena = existing[0]
        say(f"venue   {arena['name']} already exists")
    else:
        arena = create_venue(venue, token)
        if not arena:
            return

    # Photos are topped up on every run, so a failed upload can be retried.
    _, have = call("GET", f"/arenas/{arena['id']}/photos")
    have = have or []
    taken = {p["sort_order"] for p in have}
    cover = arena.get("cover_url") or (have[0]["url"] if have else None)
    needs_cover = not arena.get("cover_url")

    for i, title in enumerate(venue["photos"]):
        if i in taken:
            continue
        try:
            path, caption = commons_photo(title, tmp)
        except Exception as e:  # network trouble shouldn't sink the whole seed
            say(f"FAILED  download {title}: {e}")
            continue
        status, photo = upload_photo(token, arena["id"], path, caption, i)
        if status != 201:
            say(f"FAILED  upload {title}: {photo}")
            continue
        cover = cover or photo["url"]
        say(f"photo   {photo['url']}")

    if cover and needs_cover:
        # PUT replaces the whole venue, so send the full form back with the cover.
        status, resp = call("PUT", f"/owner/arenas/{arena['id']}",
                            {**venue["arena"], "cover_url": cover}, token=token)
        if status != 200:
            say(f"FAILED  cover photo: {resp}")


def main():
    try:
        urllib.request.urlopen(BASE + "/v1/arenas")
    except Exception:
        sys.exit(f"seed-futsals: no API at {BASE}/v1, start it with 'make run' first")

    local = re.match(r"^https?://(localhost|127\.0\.0\.1)(:\d+)?$", BASE)
    if not local and "SEED_PASSWORD" not in os.environ:
        sys.exit("seed-futsals: set SEED_PASSWORD when seeding anything but localhost")
    if len(PASSWORD) < 10:
        sys.exit("seed-futsals: SEED_PASSWORD needs at least 10 characters")

    print(f"Seeding futsal venues into {BASE}")
    with tempfile.TemporaryDirectory() as tmp:
        for venue in VENUES:
            seed(venue, tmp)
    print("Done. Owners sign in with the seed password.")


if __name__ == "__main__":
    main()
