"use server";

// ============================================================================
// Community Hub server actions — matchmaking + leaderboard
// ============================================================================

import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import type { ActionResult, MatchmakingCall, MatchmakingPost } from "@/lib/types";

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

const RESPOND_ERRORS: Record<string, string> = {
  AUTH_REQUIRED: "Sign in to join a game.",
  POST_NOT_FOUND: "This game is no longer listed.",
  OWN_POST: "You can't join your own call.",
  POST_CLOSED: "This game is no longer open.",
  SLOT_IN_PAST: "This game has already started.",
  POST_FULL: "This game just filled up.",
  ALREADY_JOINED: "You already asked to join this game.",
};

/**
 * Asks to join an open call. Files a pending request — no spot is taken until
 * the post author approves it (see approveResponse).
 */
export async function respondToPost(
  postId: string,
  message?: string
): Promise<ActionResult<null>> {
  const valid = z.string().uuid().safeParse(postId);
  if (!valid.success) return { ok: false, error: "Invalid game.", code: "VALIDATION" };

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: RESPOND_ERRORS.AUTH_REQUIRED, code: "AUTH_REQUIRED" };

  const { error } = await supabase.rpc("respond_to_matchmaking_post", {
    p_post_id: postId,
    p_message: message?.slice(0, 200) ?? null,
  });

  if (error) {
    const code = Object.keys(RESPOND_ERRORS).find((k) => error.message.includes(k));
    return { ok: false, error: code ? RESPOND_ERRORS[code] : "Could not send your request." };
  }

  revalidatePath("/community");
  return { ok: true, data: null };
}

/** The signed-in author's own calls (open or filled), each with its requests. */
export async function getMyCalls(): Promise<ActionResult<MatchmakingCall[]>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to manage your calls.", code: "AUTH_REQUIRED" };

  const { data, error } = await supabase
    .from("matchmaking_posts")
    .select(
      `id, author_id, booking_id, arena_id, title, needed_players, filled_players,
       skill, starts_at, status,
       arena:arenas!arena_id (name, area),
       responses:matchmaking_responses (
         user_id, message, accepted, created_at,
         responder:profiles!user_id (username, full_name, avatar_url, community_score)
       )`
    )
    .eq("author_id", user.id)
    .in("status", ["open", "filled"])
    .gte("starts_at", new Date().toISOString())
    .order("starts_at", { ascending: true })
    .limit(20);

  if (error) return { ok: false, error: "Could not load your calls." };
  return { ok: true, data: data as unknown as MatchmakingCall[] };
}

const MANAGE_ERRORS: Record<string, string> = {
  AUTH_REQUIRED: "Sign in to manage requests.",
  POST_NOT_FOUND: "This call no longer exists.",
  NOT_OWNER: "Only the call's author can manage requests.",
  POST_FULL: "Every spot is already taken.",
  REQUEST_NOT_FOUND: "That request is no longer pending.",
};

function translateManage(message: string): string {
  const code = Object.keys(MANAGE_ERRORS).find((k) => message.includes(k));
  return code ? MANAGE_ERRORS[code] : "Could not update the request.";
}

const ManageSchema = z.object({
  postId: z.string().uuid(),
  userId: z.string().uuid(),
});

/** Author accepts a pending request — fills the spot. */
export async function approveResponse(
  postId: string,
  userId: string
): Promise<ActionResult<null>> {
  const valid = ManageSchema.safeParse({ postId, userId });
  if (!valid.success) return { ok: false, error: "Invalid request.", code: "VALIDATION" };

  const supabase = await createClient();
  const { error } = await supabase.rpc("approve_response", {
    p_post_id: postId,
    p_user_id: userId,
  });
  if (error) return { ok: false, error: translateManage(error.message) };

  revalidatePath("/community");
  return { ok: true, data: null };
}

/** Author removes a request (and frees the spot if it had been accepted). */
export async function declineResponse(
  postId: string,
  userId: string
): Promise<ActionResult<null>> {
  const valid = ManageSchema.safeParse({ postId, userId });
  if (!valid.success) return { ok: false, error: "Invalid request.", code: "VALIDATION" };

  const supabase = await createClient();
  const { error } = await supabase.rpc("decline_response", {
    p_post_id: postId,
    p_user_id: userId,
  });
  if (error) return { ok: false, error: translateManage(error.message) };

  revalidatePath("/community");
  return { ok: true, data: null };
}

