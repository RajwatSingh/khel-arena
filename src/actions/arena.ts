"use server";

// ============================================================================
// Arena server actions — the futsal owner's side of the house.
// Owners (profiles.account_type = 'futsal_owner') create one arena profile,
// then manage its hours, courts, and per-court pricing. All writes go through
// the user's own session and are guarded by the RLS policies in
// supabase/04_arena_owners.sql — only the arena's owner can touch its rows.
// ============================================================================

import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import type { ActionResult, Arena, Court } from "@/lib/types";

const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

const ArenaSchema = z
  .object({
    name: z.string().trim().min(3, "Give your futsal a name.").max(80),
    area: z.string().trim().min(2, "Which area is it in?").max(60),
    city: z.string().trim().min(2).max(60).default("Kathmandu"),
    description: z.string().trim().max(500, "Keep the description under 500 characters.").nullable(),
    amenities: z.array(z.string().trim().min(1).max(30)).max(12).default([]),
    opensAt: z.string().regex(TIME_RE, "Opening time must look like 06:00."),
    closesAt: z.string().regex(TIME_RE, "Closing time must look like 22:00."),
  })
  .refine((v) => v.opensAt < v.closesAt, {
    message: "Closing time must be after opening time.",
    path: ["closesAt"],
  });

export type ArenaInput = z.input<typeof ArenaSchema>;

const CourtSchema = z.object({
  label: z.string().trim().min(1, "Name the court — e.g. Court A.").max(40),
  sideCount: z.number().int().min(3).max(11),
  basePrice: z.number().int().min(100, "Price must be at least Rs. 100.").max(100000),
});
export type CourtInput = z.input<typeof CourtSchema>;

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)+/g, "")
    .slice(0, 60);
}

async function requireOwner(): Promise<
  | { ok: true; supabase: Awaited<ReturnType<typeof createClient>>; userId: string }
  | { ok: false; error: string; code?: string }
> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in first.", code: "AUTH_REQUIRED" };

  const { data: profile } = await supabase
    .from("profiles")
    .select("account_type")
    .eq("id", user.id)
    .single();
  if (profile?.account_type !== "futsal_owner") {
    return { ok: false, error: "Only futsal owner accounts can manage an arena.", code: "NOT_OWNER" };
  }
  return { ok: true, supabase, userId: user.id };
}

export async function getMyArena(): Promise<
  ActionResult<{ arena: Arena | null; courts: Court[] }>
> {
  const auth = await requireOwner();
  if (!auth.ok) return auth;

  const { data: arena } = await auth.supabase
    .from("arenas")
    .select("*")
    .eq("owner_id", auth.userId)
    .maybeSingle();

  if (!arena) return { ok: true, data: { arena: null, courts: [] } };

  const { data: courts } = await auth.supabase
    .from("courts")
    .select("*")
    .eq("arena_id", arena.id)
    .order("label");

  return { ok: true, data: { arena: arena as Arena, courts: (courts ?? []) as Court[] } };
}

export async function createArena(input: ArenaInput): Promise<ActionResult<Arena>> {
  const parsed = ArenaSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  const auth = await requireOwner();
  if (!auth.ok) return auth;

  const { data: existing } = await auth.supabase
    .from("arenas")
    .select("id")
    .eq("owner_id", auth.userId)
    .maybeSingle();
  if (existing) return { ok: false, error: "You already have an arena profile." };

  // Suffix the slug if another arena already claimed it.
  const base = slugify(v.name) || "arena";
  const { data: clash } = await auth.supabase
    .from("arenas")
    .select("id")
    .eq("slug", base)
    .maybeSingle();
  const slug = clash ? `${base}-${Math.random().toString(36).slice(2, 6)}` : base;

  const { data, error } = await auth.supabase
    .from("arenas")
    .insert({
      owner_id: auth.userId,
      name: v.name,
      slug,
      area: v.area,
      city: v.city,
      description: v.description,
      amenities: v.amenities,
      opens_at: v.opensAt,
      closes_at: v.closesAt,
    })
    .select()
    .single();

  if (error || !data) return { ok: false, error: "Could not create your arena profile." };
  revalidatePath("/profile");
  revalidatePath("/book");
  return { ok: true, data: data as Arena };
}

export async function updateArena(input: ArenaInput): Promise<ActionResult<null>> {
  const parsed = ArenaSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  const auth = await requireOwner();
  if (!auth.ok) return auth;

  const { error } = await auth.supabase
    .from("arenas")
    .update({
      name: v.name,
      area: v.area,
      city: v.city,
      description: v.description,
      amenities: v.amenities,
      opens_at: v.opensAt,
      closes_at: v.closesAt,
    })
    .eq("owner_id", auth.userId);

  if (error) return { ok: false, error: "Could not save your arena." };
  revalidatePath("/profile");
  revalidatePath("/book");
  return { ok: true, data: null };
}

export async function addCourt(input: CourtInput): Promise<ActionResult<Court>> {
  const parsed = CourtSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  const auth = await requireOwner();
  if (!auth.ok) return auth;

  const { data: arena } = await auth.supabase
    .from("arenas")
    .select("id")
    .eq("owner_id", auth.userId)
    .maybeSingle();
  if (!arena) return { ok: false, error: "Create your arena profile first." };

  const { data, error } = await auth.supabase
    .from("courts")
    .insert({
      arena_id: arena.id,
      label: v.label,
      side_count: v.sideCount,
      base_price: v.basePrice,
    })
    .select()
    .single();

  if (error || !data) {
    return { ok: false, error: "Could not add the court — is the label already used?" };
  }
  revalidatePath("/profile");
  revalidatePath("/book");
  return { ok: true, data: data as Court };
}

export async function updateCourtPrice(
  courtId: string,
  basePrice: number
): Promise<ActionResult<null>> {
  if (!Number.isInteger(basePrice) || basePrice < 100 || basePrice > 100000) {
    return { ok: false, error: "Price must be between Rs. 100 and Rs. 100,000.", code: "VALIDATION" };
  }

  const auth = await requireOwner();
  if (!auth.ok) return auth;

  // RLS restricts the update to courts in the owner's own arena.
  const { error } = await auth.supabase
    .from("courts")
    .update({ base_price: basePrice })
    .eq("id", courtId);

  if (error) return { ok: false, error: "Could not update the price." };
  revalidatePath("/profile");
  revalidatePath("/book");
  return { ok: true, data: null };
}
