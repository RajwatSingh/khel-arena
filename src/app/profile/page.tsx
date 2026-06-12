// /profile — players customize their editorial card; futsal owners manage
// their arena (profile, hours, court prices) instead.

import type { Metadata } from "next";
import ProfileClient from "@/components/ProfileClient";
import AuthPanel from "@/components/AuthPanel";
import AccountBar from "@/components/AccountBar";
import ArenaStudio from "@/components/ArenaStudio";
import { getMyProfile } from "@/actions/profile";
import { getMyArena } from "@/actions/arena";
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

  if (res.data.profile.account_type === "futsal_owner") {
    const arenaRes = await getMyArena();
    return (
      <>
        <AccountBar username={res.data.profile.username} />
        <main>
          <ArenaStudio
            arena={arenaRes.ok ? arenaRes.data.arena : null}
            courts={arenaRes.ok ? arenaRes.data.courts : []}
          />
        </main>
      </>
    );
  }

  return (
    <>
      <AccountBar username={res.data.profile.username} />
      <ProfileClient demoMode={false} profile={res.data.profile} highlights={res.data.highlights} />
    </>
  );
}
