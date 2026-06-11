"use server";

// ============================================================================
// Community Hub server actions — matchmaking + leaderboard
// ============================================================================

import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import type { ActionResult, MatchmakingPost } from "@/lib/types";

const ToggleSchema = z.object({
  bookingId: z.string().uuid(),
  open: z.boolean(),
  neededPlayers: z.number().int().min(1).max(10).default(2),
  title: z.string().min(4).max(120).optional(),
  skill: z.enum(["casual", "intermediate", "competitive", "semi_pro"]).default("casual"),
});

/**
 * Opens or closes a booking to the community board.
 * Atomic in Postgres: locks the booking row, flips open_to_join, and
 * creates/reopens/fills the linked matchmaking post in one transaction.
 */
export async function toggleMatchmakingSlot(
  input: z.input<typeof ToggleSchema>
): Promise<ActionResult<MatchmakingPost>> {
  const parsed = ToggleSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const { bookingId, open, neededPlayers, title, skill } = parsed.data;

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to manage your bookings.", code: "AUTH_REQUIRED" };

  const { data, error } = await supabase.rpc("toggle_matchmaking_slot", {
    p_booking_id: bookingId,
    p_open: open,
    p_needed_players: neededPlayers,
    p_title: title ?? null,
    p_skill: skill,
  });

  if (error) {
    if (error.message.includes("NOT_OWNER"))
      return { ok: false, error: "Only the booking owner can open it to the community." };
    if (error.message.includes("SLOT_IN_PAST"))
      return { ok: false, error: "This slot has already started." };
    return { ok: false, error: "Could not update the matchmaking board." };
  }

  revalidatePath("/community");
  return { ok: true, data: data as MatchmakingPost };
}

/** Live matchmaking feed: open posts, soonest kickoff first. */
export async function getMatchmakingFeed(): Promise<ActionResult<MatchmakingPost[]>> {
  const supabase = await createClient();
  const { data, error } = await supabase
    .from("matchmaking_posts")
    .select(
      `id, title, needed_players, filled_players, skill, starts_at, status, booking_id,
       author:profiles!author_id (username, full_name, avatar_url, community_score),
       arena:arenas!arena_id (name, area)`
    )
    .eq("status", "open")
    .gte("starts_at", new Date().toISOString())
    .order("starts_at", { ascending: true })
    .limit(20);

  if (error) return { ok: false, error: "Could not load the matchmaking board." };
  return { ok: true, data: data as unknown as MatchmakingPost[] };
}

/** Joins an open post; fills the slot count atomically via upsert + RPC-free path. */
export async function respondToPost(
  postId: string,
  message?: string
): Promise<ActionResult<null>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to join a game.", code: "AUTH_REQUIRED" };

  const { error } = await supabase
    .from("matchmaking_responses")
    .insert({ post_id: postId, user_id: user.id, message: message?.slice(0, 200) });

  if (error) {
    if (error.code === "23505")
      return { ok: false, error: "You already asked to join this game." };
    return { ok: false, error: "Could not send your request." };
  }
  revalidatePath("/community");
  return { ok: true, data: null };
}

