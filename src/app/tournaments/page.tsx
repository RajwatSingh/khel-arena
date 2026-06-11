// /tournaments — enter a cup or host your own.

import type { Metadata } from "next";
import TournamentsClient from "@/components/TournamentsClient";
import { getTournaments } from "@/actions/tournaments";
import { DEMO_TOURNAMENTS, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "Tournaments — Khel Arena" };

export default async function TournamentsPage() {
  if (isDemoMode()) {
    return <TournamentsClient demoMode tournaments={DEMO_TOURNAMENTS} />;
  }
  const res = await getTournaments();
  return <TournamentsClient demoMode={false} tournaments={res.ok ? res.data : []} />;
}
