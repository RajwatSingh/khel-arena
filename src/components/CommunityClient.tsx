"use client";

// CommunityClient — /community composition. Routes joins to the live server
// action or a demo simulator.
import CommunityHub from "@/components/CommunityHub";
import { respondToPost } from "@/actions/matchmaking";
import type { ActionResult, MatchmakingPost } from "@/lib/types";

interface CommunityClientProps {
  demoMode: boolean;
  posts: MatchmakingPost[];
}

export default function CommunityClient({ demoMode, posts }: CommunityClientProps) {
  const handleJoin = async (postId: string): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 400));
      return { ok: true, data: null };
    }
    return respondToPost(postId);
  };

  return (
    <main>
      <CommunityHub posts={posts} onJoin={handleJoin} />
    </main>
  );
}
