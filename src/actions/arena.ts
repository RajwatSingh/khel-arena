"use server";

// ============================================================================
// Arena server actions — the futsal owner's side of the house.
// Owners (profiles.account_type = 'futsal_owner') create one arena profile,
// then manage its hours, courts, and per-court pricing. All writes go through
// the user's own session and are guarded by the RLS policies in
// supabase/04_arena_owners.sql — only the arena's owner can touch its rows.
// ============================================================================

import { randomUUID } from "crypto";
import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import type {
  ActionResult,
  Arena,
  ArenaPhoto,
  ArenaProfile,
  ArenaReview,
  Court,
} from "@/lib/types";

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
  ActionResult<{ arena: Arena | null; courts: Court[]; photos: ArenaPhoto[] }>
> {
  const auth = await requireOwner();
  if (!auth.ok) return auth;

  const { data: arena } = await auth.supabase
    .from("arenas")
    .select("*")
    .eq("owner_id", auth.userId)
    .maybeSingle();

  if (!arena) return { ok: true, data: { arena: null, courts: [], photos: [] } };

  const [{ data: courts }, { data: photos }] = await Promise.all([
    auth.supabase.from("courts").select("*").eq("arena_id", arena.id).order("label"),
    auth.supabase
      .from("arena_photos")
      .select("*")
      .eq("arena_id", arena.id)
      .order("created_at", { ascending: false }),
  ]);

  return {
    ok: true,
    data: {
      arena: arena as Arena,
      courts: (courts ?? []) as Court[],
      photos: (photos ?? []) as ArenaPhoto[],
    },
  };
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

// ============================================================================
// Public arena profiles, reviews & photos — supabase/07_arena_reviews_photos.sql
// ============================================================================

const PHOTO_BUCKET = "arena-photos";
const PHOTO_TYPES = ["image/jpeg", "image/png", "image/webp"];
const MAX_PHOTO_BYTES = 4 * 1024 * 1024;

/** Everything the public /arenas/[slug] page needs, in one call. */
export async function getArenaProfile(slug: string): Promise<ActionResult<ArenaProfile>> {
  const supabase = await createClient();

  const { data: arena } = await supabase
    .from("arenas")
    .select("*")
    .eq("slug", slug)
    .eq("is_active", true)
    .maybeSingle();
  if (!arena) return { ok: false, error: "Arena not found.", code: "NOT_FOUND" };

  const [{ data: courts }, { data: photos }, { data: reviews }, { data: auth }] =
    await Promise.all([
      supabase.from("courts").select("*").eq("arena_id", arena.id).order("label"),
      supabase
        .from("arena_photos")
        .select("*")
        .eq("arena_id", arena.id)
        .order("created_at", { ascending: false }),
      supabase
        .from("arena_reviews")
        .select("*, author:profiles(username, full_name, avatar_url)")
        .eq("arena_id", arena.id)
        .order("created_at", { ascending: false })
        .limit(50),
      supabase.auth.getUser(),
    ]);

  const all = (reviews ?? []) as ArenaReview[];
  const myReview = auth.user ? (all.find((r) => r.user_id === auth.user!.id) ?? null) : null;

  return {
    ok: true,
    data: {
      arena: arena as Arena,
      courts: (courts ?? []) as Court[],
      photos: (photos ?? []) as ArenaPhoto[],
      reviews: all,
      myReview,
      reviewCount: all.length,
    },
  };
}

const ReviewSchema = z.object({
  arenaId: z.string().uuid(),
  slug: z.string().min(1),
  rating: z.number().int().min(1, "Pick a star rating.").max(5),
  comment: z.string().trim().max(500, "Keep your review under 500 characters.").nullable(),
});
export type ReviewInput = z.input<typeof ReviewSchema>;

/** Inserts or replaces the signed-in player's review (one per arena). */
export async function submitReview(input: ReviewInput): Promise<ActionResult<ArenaReview>> {
  const parsed = ReviewSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to leave a review.", code: "AUTH_REQUIRED" };

  const { data, error } = await supabase
    .from("arena_reviews")
    .upsert(
      {
        arena_id: v.arenaId,
        user_id: user.id,
        rating: v.rating,
        comment: v.comment || null,
      },
      { onConflict: "arena_id,user_id" }
    )
    .select("*, author:profiles(username, full_name, avatar_url)")
    .single();

  if (error || !data) {
    // RLS blocks owners from reviewing their own arena.
    return { ok: false, error: "Could not save your review." };
  }
  revalidatePath(`/arenas/${v.slug}`);
  return { ok: true, data: data as ArenaReview };
}

/** Owner posts a photo to their arena's gallery. Field name: "photo". */
export async function uploadArenaPhoto(
  formData: FormData
): Promise<ActionResult<ArenaPhoto>> {
  const file = formData.get("photo");
  const caption = String(formData.get("caption") ?? "").trim().slice(0, 120) || null;
  if (!(file instanceof File)) return { ok: false, error: "Choose an image first." };
  if (!PHOTO_TYPES.includes(file.type))
    return { ok: false, error: "Use a JPEG, PNG, or WebP image." };
  if (file.size > MAX_PHOTO_BYTES) return { ok: false, error: "Keep the image under 4 MB." };

  const auth = await requireOwner();
  if (!auth.ok) return auth;

  const { data: arena } = await auth.supabase
    .from("arenas")
    .select("id, slug")
    .eq("owner_id", auth.userId)
    .maybeSingle();
  if (!arena) return { ok: false, error: "Create your arena profile first." };

  const ext = file.type.split("/")[1];
  const path = `${arena.id}/${randomUUID()}.${ext}`;

  const { error: uploadError } = await auth.supabase.storage
    .from(PHOTO_BUCKET)
    .upload(path, file, { contentType: file.type, upsert: false });
  if (uploadError) return { ok: false, error: "Upload failed. Try a different image." };

  const {
    data: { publicUrl },
  } = auth.supabase.storage.from(PHOTO_BUCKET).getPublicUrl(path);

  const { data, error } = await auth.supabase
    .from("arena_photos")
    .insert({ arena_id: arena.id, url: publicUrl, caption })
    .select()
    .single();
  if (error || !data) return { ok: false, error: "Could not save the photo." };

  revalidatePath("/profile");
  revalidatePath(`/arenas/${arena.slug}`);
  return { ok: true, data: data as ArenaPhoto };
}

export async function removeArenaPhoto(photoId: string): Promise<ActionResult<null>> {
  const auth = await requireOwner();
  if (!auth.ok) return auth;

  // RLS limits the delete to photos of the owner's own arena.
  const { data: photo } = await auth.supabase
    .from("arena_photos")
    .delete()
    .eq("id", photoId)
    .select("url, arenas(slug)")
    .maybeSingle();
  if (!photo) return { ok: false, error: "Could not remove the photo." };

  // Best-effort cleanup of the storage object behind the public URL.
  const marker = `/${PHOTO_BUCKET}/`;
  const idx = photo.url.indexOf(marker);
  if (idx !== -1) {
    await auth.supabase.storage
      .from(PHOTO_BUCKET)
      .remove([photo.url.slice(idx + marker.length)]);
  }

  revalidatePath("/profile");
  const arenaJoin = photo.arenas as { slug: string } | { slug: string }[] | null;
  const slug = Array.isArray(arenaJoin) ? arenaJoin[0]?.slug : arenaJoin?.slug;
  if (slug) revalidatePath(`/arenas/${slug}`);
  return { ok: true, data: null };
}
