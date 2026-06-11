// /my-bookings — see upcoming/past bookings, cancel, or open to the community.

import type { Metadata } from "next";
import MyBookingsClient from "@/components/MyBookingsClient";
import { getMyBookings } from "@/actions/bookings";
import { DEMO_MY_BOOKINGS, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "My Bookings — Khel Arena" };

export default async function MyBookingsPage() {
  if (isDemoMode()) {
    return <MyBookingsClient demoMode bookings={DEMO_MY_BOOKINGS} />;
  }
  const res = await getMyBookings();
  return <MyBookingsClient demoMode={false} bookings={res.ok ? res.data : []} />;
}
