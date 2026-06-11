// /profile — customize your editorial player card.

import type { Metadata } from "next";
import ProfileClient from "@/components/ProfileClient";
import { getMyProfile } from "@/actions/profile";
import { DEMO_HIGHLIGHTS, DEMO_PROFILE, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "Your profile — Khel Arena" };

export default async function ProfilePage() {
  if (isDemoMode()) {
    return <ProfileClient demoMode profile={DEMO_PROFILE} highlights={DEMO_HIGHLIGHTS} />;
  }
  const res = await getMyProfile();
  if (!res.ok) {
    // Not signed in (or no profile) — fall back to the demo card as a preview.
    return <ProfileClient demoMode profile={DEMO_PROFILE} highlights={DEMO_HIGHLIGHTS} />;
  }
  return (
    <ProfileClient demoMode={false} profile={res.data.profile} highlights={res.data.highlights} />
  );
}
