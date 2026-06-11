"use server";

// ============================================================================
// Booking server actions
// The heavy lifting (advisory lock + EXCLUDE constraint) lives in Postgres —
// see supabase/functions.sql. This layer validates input, translates database
// errors into human-readable messages, and revalidates the UI.
// ============================================================================

import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import type { ActionResult, Booking, Court, GridSlot, MyBooking } from "@/lib/types";

/** Fetches all active courts with their arena info. */
export async function getCourts(): Promise<
  ActionResult<(Court & { arenaName: string; arenaArea: string })[]>
> {
  const supabase = await createClient();
  const { data, error } = await supabase
    .from("courts")
    .select("id, arena_id, label, sport, side_count, base_price, arenas!inner(name, area)")
    .eq("is_active", true)
    .eq("arenas.is_active", true)
    .order("label");

  if (error) return { ok: false, error: "Could not load courts." };

  const courts = (data ?? []).map((row: Record<string, unknown>) => {
    const arena = row.arenas as { name: string; area: string };
    return {
      id: row.id as string,
      arena_id: row.arena_id as string,
      label: row.label as string,
      sport: row.sport as string,
      side_count: row.side_count as number,
      base_price: row.base_price as number,
      arenaName: arena.name,
      arenaArea: arena.area,
    };
  });

  return { ok: true, data: courts };
}

const CreateBookingSchema = z
  .object({
    courtId: z.string().uuid(),
    startsAt: z.coerce.date(),
    endsAt: z.coerce.date(),
    teamId: z.string().uuid().nullable().optional(),
    note: z.string().max(280).optional(),
  })
  .refine((v) => v.endsAt > v.startsAt, { message: "End must be after start." })
  .refine((v) => v.startsAt.getTime() > Date.now() - 60_000, {
    message: "That slot has already started.",
  })
  .refine(
    (v) => v.endsAt.getTime() - v.startsAt.getTime() <= 4 * 60 * 60 * 1000,
    { message: "Bookings are limited to 4 hours." }
  );

/** Maps raised Postgres exceptions to copy the player actually understands. */
const DB_ERROR_COPY: Record<string, string> = {
  SLOT_TAKEN: "Someone confirmed this slot seconds before you. Pick another time.",
  SLOT_IN_PAST: "That slot has already started.",
  OUTSIDE_OPERATING_HOURS: "The arena is closed at that time.",
  COURT_NOT_FOUND: "This court is no longer available.",
  AUTH_REQUIRED: "Sign in to book a court.",
};

function translateDbError(message: string): { error: string; code?: string } {
  const code = Object.keys(DB_ERROR_COPY).find((k) => message.includes(k));
  return code
    ? { error: DB_ERROR_COPY[code], code }
    : { error: "Booking failed. Nothing was charged — please try again." };
}

export interface CreateBookingInput {
  courtId: string;
  startsAt: string | Date;
  endsAt: string | Date;
  teamId?: string | null;
  note?: string;
}

export async function createBooking(
  input: CreateBookingInput
): Promise<ActionResult<Booking>> {
  const parsed = CreateBookingSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const { courtId, startsAt, endsAt, teamId, note } = parsed.data;

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: DB_ERROR_COPY.AUTH_REQUIRED, code: "AUTH_REQUIRED" };

  // Single RPC: lock → overlap check → server-side pricing → insert.
  const { data, error } = await supabase.rpc("create_booking", {
    p_court_id: courtId,
    p_starts: startsAt.toISOString(),
    p_ends: endsAt.toISOString(),
    p_team_id: teamId ?? null,
    p_note: note ?? null,
  });

  if (error) return { ok: false, ...translateDbError(error.message) };

  revalidatePath("/book");
  revalidatePath("/community");
  return { ok: true, data: data as Booking };
}

/** Fetches the live availability matrix for one court + Kathmandu date. */
export async function getAvailabilityGrid(
  courtId: string,
  date: string // "YYYY-MM-DD"
): Promise<ActionResult<GridSlot[]>> {
  const valid = z
    .object({ courtId: z.string().uuid(), date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/) })
    .safeParse({ courtId, date });
  if (!valid.success) return { ok: false, error: "Invalid request.", code: "VALIDATION" };

  const supabase = await createClient();
  const { data, error } = await supabase.rpc("get_availability_grid", {
    p_court_id: courtId,
    p_date: date,
  });

  if (error) return { ok: false, error: "Could not load availability." };
  return { ok: true, data: (data ?? []) as GridSlot[] };
}

export async function cancelBooking(bookingId: string): Promise<ActionResult<null>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: DB_ERROR_COPY.AUTH_REQUIRED };

  const { error } = await supabase
    .from("bookings")
    .update({ status: "cancelled", open_to_join: false })
    .eq("id", bookingId)
    .eq("user_id", user.id)
    .in("status", ["pending", "confirmed"]);

  if (error) return { ok: false, error: "Could not cancel this booking." };
  revalidatePath("/book");
  revalidatePath("/my-bookings");
  return { ok: true, data: null };
}

/** Parses Postgres tstzrange text like ["2024-01-01T12:00:00+00","2024-01-01T13:00:00+00") */
function parseSlotRange(slot: string): { starts_at: string; ends_at: string } {
  const m = slot.match(/["[](.+?)[",]+(.+?)[")\]]/);
  return {
    starts_at: m ? new Date(m[1]).toISOString() : "",
    ends_at: m ? new Date(m[2]).toISOString() : "",
  };
}

/** All bookings for the authenticated user, enriched with court + arena context. */
export async function getMyBookings(): Promise<ActionResult<MyBooking[]>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: DB_ERROR_COPY.AUTH_REQUIRED, code: "AUTH_REQUIRED" };

  const { data, error } = await supabase
    .from("bookings")
    .select("*, courts!inner(label, arenas!inner(name, area))")
    .eq("user_id", user.id)
    .order("created_at", { ascending: false })
    .limit(50);

  if (error) return { ok: false, error: "Could not load your bookings." };

  const bookings: MyBooking[] = (data ?? []).map((row: Record<string, unknown>) => {
    const court = row.courts as { label: string; arenas: { name: string; area: string } };
    const { starts_at, ends_at } = parseSlotRange(row.slot as string);
    return {
      id: row.id as string,
      court_id: row.court_id as string,
      user_id: row.user_id as string,
      team_id: (row.team_id as string) ?? null,
      slot: row.slot as string,
      price_npr: row.price_npr as number,
      is_peak: row.is_peak as boolean,
      status: row.status as Booking["status"],
      open_to_join: row.open_to_join as boolean,
      created_at: row.created_at as string,
      court_label: court.label,
      arena_name: court.arenas.name,
      arena_area: court.arenas.area,
      starts_at,
      ends_at,
    };
  });

  return { ok: true, data: bookings };
}
