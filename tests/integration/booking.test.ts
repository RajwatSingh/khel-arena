// ============================================================================
// Integration tests for the race-proof booking core (supabase/functions.sql +
// 05_release_stale_holds.sql) against a REAL Postgres.
//
// Set DATABASE_URL to a disposable database to run these (the suite resets the
// public + auth schemas on every run). Without DATABASE_URL they skip, so the
// fast unit suite never needs a database.
//
//   DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres \
//     npm run test:integration
//
// A Supabase shim (auth schema, auth.uid(), the anon/authenticated/service_role
// roles) lets the unmodified production SQL load on vanilla Postgres. The
// "current user" is driven by the `app.user_id` GUC, which our shim's
// auth.uid() reads — set per connection.
// ============================================================================

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";
import pg from "pg";

const { Client } = pg;
const DB = process.env.DATABASE_URL;

const USER_ID = "00000000-0000-4000-8000-000000000001";
const ARENA_ID = "00000000-0000-4000-8000-0000000000a1";
const COURT_ID = "00000000-0000-4000-8000-0000000000c1";

const sqlFile = (name: string) =>
  readFileSync(resolve(process.cwd(), "supabase", name), "utf8");

const BOOTSTRAP = `
  drop schema if exists public cascade;
  create schema public;
  drop schema if exists auth cascade;
  create schema auth;
  create table auth.users (id uuid primary key);
  create or replace function auth.uid() returns uuid language sql stable as $fn$
    select nullif(current_setting('app.user_id', true), '')::uuid;
  $fn$;
  do $roles$ begin
    if not exists (select from pg_roles where rolname = 'anon') then create role anon noinherit; end if;
    if not exists (select from pg_roles where rolname = 'authenticated') then create role authenticated noinherit; end if;
    if not exists (select from pg_roles where rolname = 'service_role') then create role service_role noinherit; end if;
  end $roles$;
`;

const SEED = `
  insert into auth.users (id) values ('${USER_ID}');
  insert into public.profiles (id, username, full_name) values ('${USER_ID}', 'tester', 'Tester');
  insert into public.arenas (id, owner_id, name, slug, area)
    values ('${ARENA_ID}', '${USER_ID}', 'Test Arena', 'test-arena', 'Test');
  insert into public.courts (id, arena_id, label, base_price)
    values ('${COURT_ID}', '${ARENA_ID}', 'Court A', 1000);
`;

/** A future slot, expressed in Kathmandu local time so it sits inside hours. */
function slotAt(hour: number, hours = 1) {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + 3);
  const date = d.toISOString().slice(0, 10);
  const p = (h: number) => String(h).padStart(2, "0");
  return {
    starts: `${date}T${p(hour)}:00:00+05:45`,
    ends: `${date}T${p(hour + hours)}:00:00+05:45`,
  };
}

async function connect(userId: string | null = USER_ID) {
  const c = new Client({ connectionString: DB });
  await c.connect();
  await c.query("select set_config('app.user_id', $1, false)", [userId ?? ""]);
  return c;
}

const bookSql = "select * from public.create_booking($1::uuid, $2::timestamptz, $3::timestamptz)";

describe.skipIf(!DB)("create_booking — race-proof core", () => {
  let main: pg.Client;

  beforeAll(async () => {
    main = new Client({ connectionString: DB });
    await main.connect();
    await main.query(BOOTSTRAP);
    await main.query(sqlFile("schema.sql"));
    await main.query(sqlFile("functions.sql"));
    await main.query(sqlFile("05_release_stale_holds.sql"));
    await main.query(SEED);
    await main.query("select set_config('app.user_id', $1, false)", [USER_ID]);
  });

  afterAll(async () => {
    await main?.end();
  });

  beforeEach(async () => {
    await main.query("delete from public.bookings");
  });

  it("books an open slot as pending", async () => {
    const { starts, ends } = slotAt(14);
    const { rows } = await main.query(bookSql, [COURT_ID, starts, ends]);
    expect(rows).toHaveLength(1);
    expect(rows[0].status).toBe("pending");
    expect(rows[0].court_id).toBe(COURT_ID);
  });

  it("requires authentication", async () => {
    const anon = await connect(null); // app.user_id = ''
    const { starts, ends } = slotAt(9);
    await expect(anon.query(bookSql, [COURT_ID, starts, ends])).rejects.toThrow(/AUTH_REQUIRED/);
    await anon.end();
  });

  it("lets exactly one of two concurrent bookings win the same slot", async () => {
    const { starts, ends } = slotAt(15);
    const [a, b] = await Promise.all([connect(), connect()]);

    const results = await Promise.allSettled([
      a.query(bookSql, [COURT_ID, starts, ends]),
      b.query(bookSql, [COURT_ID, starts, ends]),
    ]);
    await Promise.all([a.end(), b.end()]);

    const won = results.filter((r) => r.status === "fulfilled");
    const lost = results.filter((r) => r.status === "rejected") as PromiseRejectedResult[];
    expect(won).toHaveLength(1);
    expect(lost).toHaveLength(1);
    expect(String(lost[0].reason)).toContain("SLOT_TAKEN");

    const { rows } = await main.query(
      "select count(*)::int as n from public.bookings where status <> 'cancelled'"
    );
    expect(rows[0].n).toBe(1);
  });

  it("rejects an overlapping range", async () => {
    const a = slotAt(16, 2); // 16:00–18:00
    const b = slotAt(17, 2); // 17:00–19:00 (overlaps)
    await main.query(bookSql, [COURT_ID, a.starts, a.ends]);
    await expect(main.query(bookSql, [COURT_ID, b.starts, b.ends])).rejects.toThrow(/SLOT_TAKEN/);
  });

  it("allows adjacent, non-overlapping slots", async () => {
    const a = slotAt(10); // 10:00–11:00
    const b = slotAt(11); // 11:00–12:00 (touches, doesn't overlap)
    await expect(main.query(bookSql, [COURT_ID, a.starts, a.ends])).resolves.toBeTruthy();
    await expect(main.query(bookSql, [COURT_ID, b.starts, b.ends])).resolves.toBeTruthy();
  });

  it("frees a cancelled slot for rebooking", async () => {
    const { starts, ends } = slotAt(12);
    const first = await main.query(bookSql, [COURT_ID, starts, ends]);
    await main.query("update public.bookings set status = 'cancelled' where id = $1", [
      first.rows[0].id,
    ]);
    await expect(main.query(bookSql, [COURT_ID, starts, ends])).resolves.toBeTruthy();
  });

  it("releases an expired unpaid hold and books over it", async () => {
    const { starts, ends } = slotAt(13);
    // A pending hold created 20 minutes ago (past the 15-minute window).
    const stale = await main.query(
      `insert into public.bookings (court_id, user_id, slot, price_npr, is_peak, status, created_at)
       values ($1, $2, tstzrange($3::timestamptz, $4::timestamptz, '[)'), 1000, false, 'pending',
               now() - interval '20 minutes')
       returning id`,
      [COURT_ID, USER_ID, starts, ends]
    );
    const staleId = stale.rows[0].id;

    const fresh = await main.query(bookSql, [COURT_ID, starts, ends]);
    expect(fresh.rows[0].status).toBe("pending");
    expect(fresh.rows[0].id).not.toBe(staleId);

    const old = await main.query("select status from public.bookings where id = $1", [staleId]);
    expect(old.rows[0].status).toBe("cancelled");
  });
});
