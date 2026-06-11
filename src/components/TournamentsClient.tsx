"use client";

// TournamentsClient — /tournaments composition. Routes create/register to
// live server actions or local demo simulators.
import { useState } from "react";
import TournamentBoard from "@/components/TournamentBoard";
import { createTournament, registerTeam, type CreateTournamentInput } from "@/actions/tournaments";
import type { ActionResult, Tournament } from "@/lib/types";

interface TournamentsClientProps {
  demoMode: boolean;
  tournaments: Tournament[];
}

export default function TournamentsClient({ demoMode, tournaments }: TournamentsClientProps) {
  const [list, setList] = useState<Tournament[]>(tournaments);

  const handleCreate = async (
    input: CreateTournamentInput
  ): Promise<ActionResult<Tournament>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 600));
      const created: Tournament = {
        id: crypto.randomUUID(),
        organizer_id: "demo",
        arena_id: null,
        name: input.name,
        slug: input.name.toLowerCase().replace(/\s+/g, "-"),
        format: input.format,
        side_count: input.sideCount,
        squad_cap: input.squadCap,
        max_teams: input.maxTeams,
        entry_fee_npr: input.entryFeeNpr,
        prize_pool_npr: input.prizePoolNpr,
        prize_split: input.prizeSplit,
        skill: input.skill,
        description: input.description ?? null,
        rules: null,
        starts_on: input.startsOn,
        register_by: input.registerBy,
        status: "open",
        team_count: 0,
        arena_name: null,
        arena_area: null,
        organizer_username: "you",
      };
      setList((prev) => [created, ...prev]);
      return { ok: true, data: created };
    }
    const res = await createTournament(input);
    if (res.ok) setList((prev) => [res.data, ...prev]);
    return res;
  };

  const handleRegister = async (id: string): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 400));
      setList((prev) =>
        prev.map((t) => (t.id === id ? { ...t, team_count: t.team_count + 1 } : t))
      );
      return { ok: true, data: null };
    }
    return registerTeam(id);
  };

  return (
    <main>
      <TournamentBoard tournaments={list} onCreate={handleCreate} onRegister={handleRegister} />
    </main>
  );
}
