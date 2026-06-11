// /profile — customize your editorial player card.

import type { Metadata } from "next";
import ProfileClient from "@/components/ProfileClient";
import AuthPanel from "@/components/AuthPanel";
import AccountBar from "@/components/AccountBar";
import { getMyProfile } from "@/actions/profile";
import { DEMO_HIGHLIGHTS, DEMO_PROFILE, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "Your profile — Khel Arena" };

export default async function ProfilePage() {
  if (isDemoMode()) {
    return <ProfileClient demoMode profile={DEMO_PROFILE} highlights={DEMO_HIGHLIGHTS} />;
  }
  const res = await getMyProfile();
  if (!res.ok) {
    // Not signed in — gate the studio behind sign in / sign up.
    return <AuthPanel />;
  }
  return (
    <>
      <AccountBar username={res.data.profile.username} />
      <ProfileClient demoMode={false} profile={res.data.profile} highlights={res.data.highlights} />
    </>
  );
}
