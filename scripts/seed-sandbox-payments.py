#!/usr/bin/env python3
"""Connect eSewa and Khalti *sandbox* accounts to every seeded local venue.

Signs in as each owner that `make seed` and `make seed-futsals` create, and
saves test credentials on every venue they own, with "live" unchecked. The
API then sends checkouts to the gateways' test servers, so you can walk a
booking through eSewa or Khalti locally and pay with a test wallet.

    make seed-sandbox                    # API on :8080
    BASE_URL=http://localhost:5173 python3 scripts/seed-sandbox-payments.py

Needs PAYMENT_ENC_KEY set in .env (the API refuses to store keys without it).
Safe to re-run. A venue whose account is already marked live is left alone.

The credentials are the public test ones each gateway publishes. They move no
real money, and this refuses to run against anything but localhost.
"""

import json
import os
import re
import sys
import urllib.error
import urllib.request

BASE = os.environ.get("BASE_URL", "http://localhost:8080").rstrip("/")
PASSWORD = os.environ.get("SEED_PASSWORD", "kathmandu2026")

# The owners created by scripts/seed-dev.sh and scripts/seed-futsals.py.
OWNERS = [
    "owner@khelarena.np",
    "fiveaside@khelarena.np",
    "dhobighat@khelarena.np",
    "field@khelarena.np",
    "royal@khelarena.np",
    "dhanyantari@khelarena.np",
]

# Published sandbox credentials.
#   eSewa:  developer.esewa.com.np, "EPAYTEST" and its shared UAT secret.
#   Khalti: the test key from docs.khalti.com, accepted by dev.khalti.com.
SANDBOX_ACCOUNTS = {
    "esewa": {"merchant_code": "EPAYTEST", "secret_key": "8gBm/:&EnhH.1/q"},
    "khalti": {"merchant_code": "", "secret_key": "05bf95cc57244045b8df5fad06748dab"},
}

TEST_WALLETS = """
Pay with a test wallet:
  eSewa   ID 9806800001 (to ...005)   password Nepal@123   token 123456
  Khalti  ID 9800000000 (to ...005)   MPIN 1111            OTP 987654
"""


def say(msg):
    print("  " + msg, file=sys.stderr)


def call(method, path, body=None, token=None):
    """One API request. Returns (status, parsed JSON or None)."""
    headers = {}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
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


def message(resp):
    return (resp or {}).get("error", {}).get("message", resp)


def connect(token, arena):
    status, accounts = call("GET", f"/owner/arenas/{arena['id']}/payment-accounts", token=token)
    if status != 200:
        say(f"FAILED  {arena['name']}: {message(accounts)}")
        return False

    existing = {a["provider"]: a for a in accounts or []}
    for provider, creds in SANDBOX_ACCOUNTS.items():
        if existing.get(provider, {}).get("live"):
            say(f"skip    {arena['name']} · {provider} is a live account")
            continue

        status, resp = call(
            "PUT",
            f"/owner/arenas/{arena['id']}/payment-accounts/{provider}",
            {**creds, "live": False, "enabled": True},
            token=token,
        )
        if status != 204:
            say(f"FAILED  {arena['name']} · {provider}: {message(resp)}")
            return False
        say(f"sandbox {arena['name']} · {provider}")
    return True


def main():
    if not re.match(r"^https?://(localhost|127\.0\.0\.1)(:\d+)?$", BASE):
        sys.exit("seed-sandbox: local only. Point BASE_URL at localhost.")

    try:
        urllib.request.urlopen(BASE + "/v1/arenas")
    except Exception:
        sys.exit(f"seed-sandbox: no API at {BASE}/v1, start it with 'make run' first")

    print(f"Connecting sandbox payments in {BASE}")
    connected = 0
    for email in OWNERS:
        status, resp = call("POST", "/auth/login", {"email": email, "password": PASSWORD})
        if status != 200:
            say(f"skip    {email} (not seeded)")
            continue
        token = resp["access_token"]

        status, arenas = call("GET", "/owner/arenas", token=token)
        if status != 200:
            say(f"FAILED  {email}: {message(arenas)}")
            continue
        for arena in arenas or []:
            if connect(token, arena):
                connected += 1

    if connected == 0:
        sys.exit("seed-sandbox: no venues connected. Run 'make seed' first, and check "
                 "PAYMENT_ENC_KEY is set in .env.")
    print(f"Done. {connected} venue(s) take eSewa and Khalti in sandbox mode.")
    print(TEST_WALLETS)


if __name__ == "__main__":
    main()
