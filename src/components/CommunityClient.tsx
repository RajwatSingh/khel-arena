"use client";

// CommunityClient — /community composition. Routes joins + request management
// to the live server actions or a demo simulator.
import CommunityHub from "@/components/CommunityHub";
import { approveResponse, declineResponse, respondToPost } from "@/actions/matchmaking";
import type { ActionResult, MatchmakingCall, MatchmakingPost } from "@/lib/types";

interface CommunityClientProps {
  demoMode: boolean;
  posts: MatchmakingPost[];
  myCalls: MatchmakingCall[];
}

export default function CommunityClient({ demoMode, posts, myCalls }: CommunityClientProps) {
  const handleJoin = async (postId: string): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 400));
      return { ok: true, data: null };
    }
    return respondToPost(postId);
  };

  const handleApprove = async (postId: string, userId: string): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 300));
      return { ok: true, data: null };
    }
    return approveResponse(postId, userId);
  };

  const handleDecline = async (postId: string, userId: string): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 300));
      return { ok: true, data: null };
    }
    return declineResponse(postId, userId);
  };

  return (
    <main>
      <CommunityHub
        posts={posts}
        myCalls={myCalls}
        onJoin={handleJoin}
        onApprove={handleApprove}
        onDecline={handleDecline}
      />
    </main>
  );
}
