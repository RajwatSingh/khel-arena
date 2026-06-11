"use client";

// TeamsClient — /teams composition. Routes actions to live server actions
// or local demo simulators, matching the TournamentsClient pattern.

import { useState } from "react";
import TeamHub from "@/components/TeamHub";
import {
  createTeam,
  getTeamWithMembers,
  addMemberByUsername,
  removeMember,
  joinByCode,
  regenerateJoinCode,
  type CreateTeamInput,
} from "@/actions/teams";
import type { ActionResult, Team, TeamMember } from "@/lib/types";
import { DEMO_MEMBERS } from "@/lib/demo";

interface TeamsClientProps {
  demoMode: boolean;
  teams: Team[];
}

export default function TeamsClient({ demoMode, teams: initial }: TeamsClientProps) {
  const [list, setList] = useState<Team[]>(initial);

  const handleCreate = async (input: CreateTeamInput): Promise<ActionResult<Team>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 500));
      const tag = input.tag.toUpperCase();
      const created: Team = {
        id: crypto.randomUUID(),
        name: input.name,
        tag,
        crest_url: null,
        captain_id: "demo-user",
        home_arena: null,
        join_code: `${tag}-${Math.random().toString(36).slice(2, 6).toUpperCase()}`,
        created_at: new Date().toISOString(),
        member_count: 1,
      };
      setList((prev) => [created, ...prev]);
      return { ok: true, data: created };
    }
    const res = await createTeam(input);
    if (res.ok) setList((prev) => [res.data, ...prev]);
    return res;
  };

  const handleGetMembers = async (
    teamId: string
  ): Promise<ActionResult<{ team: Team; members: TeamMember[] }>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 300));
      const team = list.find((t) => t.id === teamId);
      if (!team) return { ok: false, error: "Team not found." };
      const members = DEMO_MEMBERS[teamId] ?? [];
      return { ok: true, data: { team, members } };
    }
    return getTeamWithMembers(teamId);
  };

  const handleAddMember = async (
    teamId: string,
    username: string
  ): Promise<ActionResult<TeamMember>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 400));
      const member: TeamMember = {
        team_id: teamId,
        user_id: crypto.randomUUID(),
        role: "player",
        joined_at: new Date().toISOString(),
        username,
        full_name: username.replace(/[._]/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
        avatar_url: null,
      };
      setList((prev) =>
        prev.map((t) => (t.id === teamId ? { ...t, member_count: t.member_count + 1 } : t))
      );
      return { ok: true, data: member };
    }
    const res = await addMemberByUsername(teamId, username);
    if (res.ok) {
      setList((prev) =>
        prev.map((t) => (t.id === teamId ? { ...t, member_count: t.member_count + 1 } : t))
      );
    }
    return res;
  };

  const handleRemoveMember = async (
    teamId: string,
    userId: string
  ): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 300));
      setList((prev) =>
        prev.map((t) => (t.id === teamId ? { ...t, member_count: Math.max(0, t.member_count - 1) } : t))
      );
      return { ok: true, data: null };
    }
    const res = await removeMember(teamId, userId);
    if (res.ok) {
      setList((prev) =>
        prev.map((t) => (t.id === teamId ? { ...t, member_count: Math.max(0, t.member_count - 1) } : t))
      );
    }
    return res;
  };

  const handleJoinByCode = async (code: string): Promise<ActionResult<Team>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 400));
      const team = list.find((t) => t.join_code === code.toUpperCase());
      if (!team) return { ok: false, error: "Invalid join code. Check with your captain." };
      return { ok: true, data: team };
    }
    const res = await joinByCode(code);
    if (res.ok) {
      // Refresh the team into our list if it's new
      setList((prev) => {
        if (prev.find((t) => t.id === res.data.id)) return prev;
        return [...prev, res.data];
      });
    }
    return res;
  };

  const handleRegenerateCode = async (teamId: string): Promise<ActionResult<string>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 300));
      const team = list.find((t) => t.id === teamId);
      const newCode = `${team?.tag ?? "DEMO"}-${Math.random().toString(36).slice(2, 6).toUpperCase()}`;
      setList((prev) =>
        prev.map((t) => (t.id === teamId ? { ...t, join_code: newCode } : t))
      );
      return { ok: true, data: newCode };
    }
    return regenerateJoinCode(teamId);
  };

  return (
    <main>
      <TeamHub
        teams={list}
        onCreateTeam={handleCreate}
        onGetMembers={handleGetMembers}
        onAddMember={handleAddMember}
        onRemoveMember={handleRemoveMember}
        onJoinByCode={handleJoinByCode}
        onRegenerateCode={handleRegenerateCode}
      />
    </main>
  );
}
