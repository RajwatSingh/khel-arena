"use server";

// ============================================================================
// Tournament server actions
// Creation goes through RLS (organizer inserts own row); registration goes
// through register_team_for_tournament() — the advisory-lock function that
// makes over-filling a tournament impossible under concurrency.
// ============================================================================

import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import type { ActionResult, Tournament } from "@/lib/types";

const CreateTournamentSchema = z
  .object({
    name: z.string().min(4, "Give it a name of at least 4 characters.").max(80),
    format: z.enum(["knockout", "league", "group_knockout"]),
    sideCount: z.number().int().min(4).max(7),
    squadCap: z.number().int().min(5).max(15),
    maxTeams: z.number().int().min(4, "At least 4 teams.").max(32, "32 teams maximum."),
    entryFeeNpr: z.number().int().min(0).max(100_000),
    prizePoolNpr: z.number().int().min(0).max(5_000_000),
    prizeSplit: z
      .array(z.number().int().min(0).max(100))
      .min(1)
      .max(4)
      .refine((s) => s.reduce((a, b) => a + b, 0) === 100, {
        message: "Prize split must add up to 100%.",
      }),
    skill: z.enum(["casual", "intermediate", "competitive", "semi_pro"]),
    description: z.string().max(500).optional(),
    rules: z.string().max(2000).optional(),
    startsOn: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, "Pick a start date."),
    registerBy: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, "Pick a registration deadline."),
    arenaId: z.string().uuid().nullable().optional(),
  })
  .refine((v) => v.registerBy <= v.startsOn, {
    message: "Registration must close on or before kick-off.",
  })
  .refine((v) => v.startsOn >= new Date().toISOString().slice(0, 10), {
    message: "Kick-off can't be in the past.",
  });

export type CreateTournamentInput = z.input<typeof CreateTournamentSchema>;

const slugify = (name: string) =>
  name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "")
    .slice(0, 60) + "-" + Math.random().toString(36).slice(2, 6);

export async function createTournament(
  input: CreateTournamentInput
): Promise<ActionResult<Tournament>> {
  const parsed = CreateTournamentSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to host a tournament.", code: "AUTH_REQUIRED" };

  const { data, error } = await supabase
    .from("tournaments")
    .insert({
      organizer_id: user.id,
      arena_id: v.arenaId ?? null,
      name: v.name,
      slug: slugify(v.name),
      format: v.format,
      side_count: v.sideCount,
      squad_cap: v.squadCap,
      max_teams: v.maxTeams,
      entry_fee_npr: v.entryFeeNpr,
      prize_pool_npr: v.prizePoolNpr,
      prize_split: v.prizeSplit,
      skill: v.skill,
      description: v.description ?? null,
      rules: v.rules ?? null,
      starts_on: v.startsOn,
      register_by: v.registerBy,
    })
    .select()
    .single();

  if (error) return { ok: false, error: "Could not create the tournament. Try again." };

  revalidatePath("/tournaments");
  revalidatePath("/community");
  return { ok: true, data: { ...data, team_count: 0 } as Tournament };
}

/** Open + upcoming tournaments, soonest kick-off first. */
export async function getTournaments(): Promise<ActionResult<Tournament[]>> {
  const supabase = await createClient();
  const { data, error } = await supabase
    .from("tournament_board")
    .select("*")
    .in("status", ["open", "full", "ongoing"])
    .order("starts_on", { ascending: true })
    .limit(20);

  if (error) return { ok: false, error: "Could not load tournaments." };
  return { ok: true, data: (data ?? []) as Tournament[] };
}

const REGISTER_ERRORS: Record<string, string> = {
  NOT_CAPTAIN: "Only the team captain can register the team.",
  REGISTRATION_CLOSED: "Registration for this tournament has closed.",
  DEADLINE_PASSED: "The registration deadline has passed.",
  TOURNAMENT_FULL: "All spots are taken — someone beat you to the last one.",
  ALREADY_REGISTERED: "Your team is already registered.",
  AUTH_REQUIRED: "Sign in to register your team.",
};

/** Registers the captain's team via the race-proof database function. */
export async function registerTeam(tournamentId: string): Promise<ActionResult<null>> {
  const valid = z.string().uuid().safeParse(tournamentId);
  if (!valid.success) return { ok: false, error: "Invalid tournament.", code: "VALIDATION" };

  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: REGISTER_ERRORS.AUTH_REQUIRED, code: "AUTH_REQUIRED" };

  // MVP: register the first team this user captains.
  const { data: team } = await supabase
    .from("teams")
    .select("id")
    .eq("captain_id", user.id)
    .limit(1)
    .maybeSingle();

  if (!team) {
    return { ok: false, error: "You need to captain a team before registering. Create one first." };
  }

  const { error } = await supabase.rpc("register_team_for_tournament", {
    p_tournament_id: tournamentId,
    p_team_id: team.id,
  });

  if (error) {
    const code = Object.keys(REGISTER_ERRORS).find((k) => error.message.includes(k));
    return { ok: false, error: code ? REGISTER_ERRORS[code] : "Registration failed. Try again." };
  }

  revalidatePath("/tournaments");
  return { ok: true, data: null };
}
