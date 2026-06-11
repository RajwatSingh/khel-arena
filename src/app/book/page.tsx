// /book — server component. Resolves availability (live or demo) for the
// default court + today in Kathmandu, then hands off to the client.

import type { Metadata } from "next";
import BookClient from "@/components/BookClient";
import { getAvailabilityGrid } from "@/actions/bookings";
import { DEMO_COURTS, demoGrid, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "Book a court — Khel Arena" };

export default async function BookPage() {
  const demoMode = isDemoMode();
  const today = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Kathmandu" }).format(
    new Date()
  );

  if (demoMode) {
    return (
      <BookClient
        demoMode
        courts={DEMO_COURTS}
        initialSlots={demoGrid(DEMO_COURTS[0].id, today)}
      />
    );
  }

  const grid = await getAvailabilityGrid(DEMO_COURTS[0].id, today);
  return (
    <BookClient
      demoMode={false}
      courts={DEMO_COURTS /* MVP: replace with a courts query as arenas onboard */}
      initialSlots={grid.ok ? grid.data : []}
    />
  );
}
