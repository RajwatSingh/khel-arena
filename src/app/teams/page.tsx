// /teams — build squads, manage rosters, join by code.

import type { Metadata } from "next";
import TeamsClient from "@/components/TeamsClient";
import { getMyTeams } from "@/actions/teams";
import { DEMO_TEAMS, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "Teams — Khel Arena" };

export default async function TeamsPage() {
  if (isDemoMode()) {
    return <TeamsClient demoMode teams={DEMO_TEAMS} />;
  }
  const res = await getMyTeams();
  return <TeamsClient demoMode={false} teams={res.ok ? res.data : []} />;
}
