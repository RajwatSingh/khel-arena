"use server";

// ============================================================================
// Team server actions — create squads, manage rosters, join by code.
// ============================================================================

import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import type { ActionResult, Team, TeamMember } from "@/lib/types";

// ── Schemas ─────────────────────────────────────────────────────────────────

const CreateTeamSchema = z.object({
  name: z.string().min(2, "Name must be at least 2 characters.").max(40, "Name must be under 40 characters."),
  tag: z
    .string()
    .min(2, "Tag must be 2-5 characters.")
    .max(5, "Tag must be 2-5 characters.")
    .regex(/^[A-Z0-9]+$/, "Tag must be uppercase letters and digits only.")
    .transform((v) => v.toUpperCase()),
  homeArena: z.string().uuid().nullable().optional(),
});

export type CreateTeamInput = z.input<typeof CreateTeamSchema>;

// ── Helpers ─────────────────────────────────────────────────────────────────

function generateJoinCode(tag: string): string {
  const suffix = Math.random().toString(36).slice(2, 6).toUpperCase();
  return `${tag}-${suffix}`;
}

// ── Actions ─────────────────────────────────────────────────────────────────

/** Teams where the authenticated user is captain or member. */
export async function getMyTeams(): Promise<ActionResult<Team[]>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to view your teams.", code: "AUTH_REQUIRED" };

  // Get team IDs where user is a member
  const { data: memberships, error: memErr } = await supabase
    .from("team_members")
    .select("team_id")
    .eq("user_id", user.id);

  if (memErr) return { ok: false, error: "Could not load your teams." };

  const teamIds = memberships?.map((m) => m.team_id) ?? [];
  if (teamIds.length === 0) return { ok: true, data: [] };

  const { data: teams, error: teamErr } = await supabase
    .from("teams")
    .select("*")
    .in("id", teamIds)
    .order("created_at", { ascending: false });

  if (teamErr) return { ok: false, error: "Could not load your teams." };

  // Get member counts
  const { data: counts } = await supabase
    .from("team_members")
    .select("team_id")
    .in("team_id", teamIds);

  const countMap: Record<string, number> = {};
  (counts ?? []).forEach((c) => {
    countMap[c.team_id] = (countMap[c.team_id] || 0) + 1;
  });

  return {
    ok: true,
    data: (teams ?? []).map((t) => ({
      ...t,
      member_count: countMap[t.id] || 0,
    })) as Team[],
  };
}

/** Single team with full roster (profiles joined). */
export async function getTeamWithMembers(
  teamId: string
): Promise<ActionResult<{ team: Team; members: TeamMember[] }>> {
  const valid = z.string().uuid().safeParse(teamId);
  if (!valid.success) return { ok: false, error: "Invalid team.", code: "VALIDATION" };

  const supabase = await createClient();

  const { data: team, error: teamErr } = await supabase
    .from("teams")
    .select("*")
    .eq("id", teamId)
    .single();

  if (teamErr || !team) return { ok: false, error: "Team not found." };

  const { data: members, error: memErr } = await supabase
    .from("team_members")
    .select("team_id, user_id, role, joined_at, profiles(username, full_name, avatar_url)")
    .eq("team_id", teamId)
    .order("joined_at", { ascending: true });

  if (memErr) return { ok: false, error: "Could not load team members." };

  const roster: TeamMember[] = (members ?? []).map((m) => {
    const p = m.profiles as unknown as { username: string; full_name: string; avatar_url: string | null };
    return {
      team_id: m.team_id,
      user_id: m.user_id,
      role: m.role as "captain" | "player",
      joined_at: m.joined_at,
      username: p?.username ?? "",
      full_name: p?.full_name ?? "",
      avatar_url: p?.avatar_url ?? null,
    };
  });

  return {
    ok: true,
    data: {
      team: { ...team, member_count: roster.length } as Team,
      members: roster,
    },
  };
}

/** Create a team — the authenticated user becomes captain and first member. */
export async function createTeam(input: CreateTeamInput): Promise<ActionResult<Team>> {
  const parsed = CreateTeamSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to create a team.", code: "AUTH_REQUIRED" };

  const joinCode = generateJoinCode(v.tag);

  const { data: team, error: insertErr } = await supabase
    .from("teams")
    .insert({
      name: v.name,
      tag: v.tag,
      captain_id: user.id,
      home_arena: v.homeArena ?? null,
      join_code: joinCode,
    })
    .select()
    .single();

  if (insertErr) {
    if (insertErr.message.includes("duplicate")) {
      if (insertErr.message.includes("teams_name_key")) {
        return { ok: false, error: "A team with that name already exists." };
      }
      if (insertErr.message.includes("teams_tag_key")) {
        return { ok: false, error: "That tag is already taken." };
      }
    }
    return { ok: false, error: "Could not create team. Try again." };
  }

  // Add captain as first member
  await supabase.from("team_members").insert({
    team_id: team.id,
    user_id: user.id,
    role: "captain",
  });

  revalidatePath("/teams");
  return { ok: true, data: { ...team, member_count: 1 } as Team };
}

/** Captain adds a member by username. */
export async function addMemberByUsername(
  teamId: string,
  username: string
): Promise<ActionResult<TeamMember>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in first.", code: "AUTH_REQUIRED" };

  // Verify captain
  const { data: team } = await supabase
    .from("teams")
    .select("id, captain_id")
    .eq("id", teamId)
    .single();

  if (!team) return { ok: false, error: "Team not found." };
  if (team.captain_id !== user.id) {
    return { ok: false, error: "Only the captain can add players." };
  }

  // Lookup profile
  const { data: profile } = await supabase
    .from("profiles")
    .select("id, username, full_name, avatar_url")
    .eq("username", username)
    .single();

  if (!profile) return { ok: false, error: `No player found with username "${username}".` };

  // Check not already a member
  const { data: existing } = await supabase
    .from("team_members")
    .select("user_id")
    .eq("team_id", teamId)
    .eq("user_id", profile.id)
    .maybeSingle();

  if (existing) return { ok: false, error: `${profile.full_name} is already on the team.` };

  // Insert
  const { error: insertErr } = await supabase.from("team_members").insert({
    team_id: teamId,
    user_id: profile.id,
    role: "player",
  });

  if (insertErr) return { ok: false, error: "Could not add player. Try again." };

  revalidatePath("/teams");
  return {
    ok: true,
    data: {
      team_id: teamId,
      user_id: profile.id,
      role: "player",
      joined_at: new Date().toISOString(),
      username: profile.username,
      full_name: profile.full_name,
      avatar_url: profile.avatar_url,
    },
  };
}

/** Captain removes a player, or a player leaves the team. */
export async function removeMember(
  teamId: string,
  userId: string
): Promise<ActionResult<null>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in first.", code: "AUTH_REQUIRED" };

  // Check the team exists and get captain
  const { data: team } = await supabase
    .from("teams")
    .select("captain_id")
    .eq("id", teamId)
    .single();

  if (!team) return { ok: false, error: "Team not found." };

  const isCaptain = team.captain_id === user.id;
  const isSelf = userId === user.id;

  if (!isCaptain && !isSelf) {
    return { ok: false, error: "Only the captain can remove players." };
  }

  if (isCaptain && isSelf) {
    return { ok: false, error: "The captain can't leave. Transfer captaincy or disband the team." };
  }

  const { error } = await supabase
    .from("team_members")
    .delete()
    .eq("team_id", teamId)
    .eq("user_id", userId);

  if (error) return { ok: false, error: "Could not remove player. Try again." };

  revalidatePath("/teams");
  return { ok: true, data: null };
}

/** Join a team using a join code. */
export async function joinByCode(code: string): Promise<ActionResult<Team>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to join a team.", code: "AUTH_REQUIRED" };

  const { data: team } = await supabase
    .from("teams")
    .select("*")
    .eq("join_code", code.trim().toUpperCase())
    .single();

  if (!team) return { ok: false, error: "Invalid join code. Check with your captain." };

  // Check not already a member
  const { data: existing } = await supabase
    .from("team_members")
    .select("user_id")
    .eq("team_id", team.id)
    .eq("user_id", user.id)
    .maybeSingle();

  if (existing) return { ok: false, error: "You're already on this team." };

  const { error: insertErr } = await supabase.from("team_members").insert({
    team_id: team.id,
    user_id: user.id,
    role: "player",
  });

  if (insertErr) return { ok: false, error: "Could not join team. Try again." };

  revalidatePath("/teams");
  return { ok: true, data: { ...team, member_count: 0 } as Team };
}

/** Captain regenerates the join code for a team. */
export async function regenerateJoinCode(teamId: string): Promise<ActionResult<string>> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in first.", code: "AUTH_REQUIRED" };

  const { data: team } = await supabase
    .from("teams")
    .select("captain_id, tag")
    .eq("id", teamId)
    .single();

  if (!team) return { ok: false, error: "Team not found." };
  if (team.captain_id !== user.id) {
    return { ok: false, error: "Only the captain can regenerate the join code." };
  }

  const newCode = generateJoinCode(team.tag);

  const { error } = await supabase
    .from("teams")
    .update({ join_code: newCode })
    .eq("id", teamId);

  if (error) return { ok: false, error: "Could not regenerate code. Try again." };

  revalidatePath("/teams");
  return { ok: true, data: newCode };
}
