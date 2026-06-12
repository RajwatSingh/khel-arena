"use server";

// ============================================================================
// Profile server actions — the player's editorial card.
// Avatar uploads go to the public `avatars` storage bucket under the user's
// own prefix; everything else is plain RLS-guarded updates.
// ============================================================================

import { revalidatePath } from "next/cache";
import { randomUUID } from "crypto";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import { DEMO_PROFILE, isDemoMode } from "@/lib/demo";
import type { ActionResult, Profile, ProfileHighlight } from "@/lib/types";

export interface NavIdentity {
  username: string;
  avatarUrl: string | null;
}

/** Lightweight identity for the nav avatar — null when signed out. */
export async function getNavIdentity(): Promise<NavIdentity | null> {
  if (isDemoMode()) {
    return { username: DEMO_PROFILE.username, avatarUrl: DEMO_PROFILE.avatar_url };
  }

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return null;

  const { data } = await supabase
    .from("profiles")
    .select("username, avatar_url")
    .eq("id", user.id)
    .maybeSingle();
  if (!data) return null;
  return { username: data.username, avatarUrl: data.avatar_url };
}

const UpdateProfileSchema = z.object({
  fullName: z.string().min(2, "Tell us your name.").max(60),
  position: z.enum(["Goleiro", "Fixo", "Ala", "Pivô", "Universal"]).nullable(),
  jerseyNumber: z.number().int().min(0).max(99).nullable(),
  preferredFoot: z.enum(["left", "right", "both"]).nullable(),
  bio: z.string().max(280, "Bios are capped at 280 characters.").nullable(),
  skill: z.enum(["casual", "intermediate", "competitive", "semi_pro"]),
});

export type UpdateProfileInput = z.input<typeof UpdateProfileSchema>;

export async function getMyProfile(): Promise<
  ActionResult<{ profile: Profile; highlights: ProfileHighlight[] }>
> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to edit your profile.", code: "AUTH_REQUIRED" };

  const [{ data: profile }, { data: highlights }] = await Promise.all([
    supabase.from("profiles").select("*").eq("id", user.id).single(),
    supabase
      .from("profile_highlights")
      .select("*")
      .eq("user_id", user.id)
      .order("created_at", { ascending: false }),
  ]);

  if (!profile) return { ok: false, error: "Profile not found." };
  return {
    ok: true,
    data: { profile: profile as Profile, highlights: (highlights ?? []) as ProfileHighlight[] },
  };
}

export async function updateProfile(input: UpdateProfileInput): Promise<ActionResult<null>> {
  const parsed = UpdateProfileSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to edit your profile.", code: "AUTH_REQUIRED" };

  const { error } = await supabase
    .from("profiles")
    .update({
      full_name: v.fullName,
      position: v.position,
      jersey_number: v.jerseyNumber,
      preferred_foot: v.preferredFoot,
      bio: v.bio,
      skill: v.skill,
    })
    .eq("id", user.id);

  if (error) return { ok: false, error: "Could not save your profile." };
  revalidatePath("/profile");
  return { ok: true, data: null };
}

const MAX_AVATAR_BYTES = 4 * 1024 * 1024;
const AVATAR_TYPES = ["image/jpeg", "image/png", "image/webp"];

/** Receives FormData with an `avatar` file; stores it and updates the profile. */
export async function uploadAvatar(formData: FormData): Promise<ActionResult<{ url: string }>> {
  const file = formData.get("avatar");
  if (!(file instanceof File)) return { ok: false, error: "Choose an image first." };
  if (!AVATAR_TYPES.includes(file.type))
    return { ok: false, error: "Use a JPEG, PNG, or WebP image." };
  if (file.size > MAX_AVATAR_BYTES)
    return { ok: false, error: "Keep the image under 4 MB." };

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to update your photo.", code: "AUTH_REQUIRED" };

  const ext = file.type.split("/")[1];
  const path = `${user.id}/${randomUUID()}.${ext}`;

  const { error: uploadError } = await supabase.storage
    .from("avatars")
    .upload(path, file, { contentType: file.type, upsert: false });
  if (uploadError) return { ok: false, error: "Upload failed. Try a different image." };

  const {
    data: { publicUrl },
  } = supabase.storage.from("avatars").getPublicUrl(path);

  const { error } = await supabase
    .from("profiles")
    .update({ avatar_url: publicUrl })
    .eq("id", user.id);
  if (error) return { ok: false, error: "Could not save your new photo." };

  revalidatePath("/profile");
  return { ok: true, data: { url: publicUrl } };
}

const HighlightSchema = z.object({
  title: z.string().min(2, "Give the clip a title.").max(80),
  url: z.string().url("Paste a full link, starting with https://"),
});

export async function addHighlight(
  input: z.input<typeof HighlightSchema>
): Promise<ActionResult<ProfileHighlight>> {
  const parsed = HighlightSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to add highlights.", code: "AUTH_REQUIRED" };

  const { data, error } = await supabase
    .from("profile_highlights")
    .insert({ user_id: user.id, title: parsed.data.title, url: parsed.data.url })
    .select()
    .single();

  if (error) return { ok: false, error: "Could not add the highlight." };
  revalidatePath("/profile");
  return { ok: true, data: data as ProfileHighlight };
}

export async function removeHighlight(id: string): Promise<ActionResult<null>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in first.", code: "AUTH_REQUIRED" };

  const { error } = await supabase
    .from("profile_highlights")
    .delete()
    .eq("id", id)
    .eq("user_id", user.id);

  if (error) return { ok: false, error: "Could not remove the highlight." };
  revalidatePath("/profile");
  return { ok: true, data: null };
}
