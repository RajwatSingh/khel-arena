// /community — server component. Loads the matchmaking feed.

import type { Metadata } from "next";
import CommunityClient from "@/components/CommunityClient";
import { getMatchmakingFeed } from "@/actions/matchmaking";
import { DEMO_POSTS, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "Community — Khel Arena" };

export default async function CommunityPage() {
  if (isDemoMode()) {
    return <CommunityClient demoMode posts={DEMO_POSTS} />;
  }

  const feed = await getMatchmakingFeed();
  return <CommunityClient demoMode={false} posts={feed.ok ? feed.data : []} />;
}
