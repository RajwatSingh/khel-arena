"use client";

// ============================================================================
// CommunityHub — the matchmaking board.
// Open calls for players, soonest kick-off first, rendered as an editorial
// ledger: hairline rows, mono data, display-type titles. Each row shows the
// needed-player gauge and a one-tap Join.
// ============================================================================

import { useState, useTransition } from "react";
import { AnimatePresence, m } from "framer-motion";
import type { ActionResult, MatchmakingPost, SkillTier } from "@/lib/types";

export interface CommunityHubProps {
  posts: MatchmakingPost[];
  onJoin: (postId: string) => Promise<ActionResult<null>>;
}

const SKILL_LABEL: Record<SkillTier, string> = {
  casual: "Casual",
  intermediate: "Intermediate",
  competitive: "Competitive",
  semi_pro: "Semi-pro",
};

const kickoffFmt = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Kathmandu",
  weekday: "short",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const ease = [0.22, 1, 0.36, 1] as const;
const row = {
  hidden: { opacity: 0, y: 14 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.55, ease, delay: i * 0.06 },
  }),
};

export default function CommunityHub({ posts, onJoin }: CommunityHubProps) {
  const [joined, setJoined] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const handleJoin = (postId: string) => {
    setError(null);
    startTransition(async () => {
      const res = await onJoin(postId);
      if (res.ok) setJoined((prev) => new Set(prev).add(postId));
      else setError(res.error);
    });
  };

  return (
    <section id="community" className="grain relative bg-surface py-28">
      <div className="mx-auto max-w-6xl px-6">
        <div className="mb-16 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="eyebrow mb-4">Community · समुदाय</p>
            <h2 className="font-display text-5xl tracking-tight text-ink">
              The valley plays together
            </h2>
          </div>
          <p className="max-w-xs text-sm leading-relaxed text-ink-dim">
            Short a striker? Post your booking. Looking for a game? Tonight&rsquo;s
            open calls are below — soonest kickoff first.
          </p>
        </div>

        <div>
          {/* ───────────────── Matchmaking board ───────────────── */}
          <div>
            <div className="mb-6 flex items-baseline justify-between border-b border-hairline-2 pb-4">
              <h3 className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim">
                Open calls
              </h3>
              <span className="flex items-center gap-2 font-mono text-[0.62rem] uppercase tracking-editorial text-sage">
                <i className="h-1.5 w-1.5 animate-pulse rounded-full bg-sage" />
                {posts.length} live
              </span>
            </div>

            {posts.length === 0 ? (
              <div className="border border-dashed border-hairline-2 p-12 text-center">
                <p className="font-display text-xl text-ink-dim">The board is quiet.</p>
                <p className="mt-2 text-sm text-ink-faint">
                  Open one of your bookings to the community and it appears here.
                </p>
              </div>
            ) : (
              <ul>
                {posts.map((post, i) => {
                  const spotsLeft = post.needed_players - post.filled_players;
                  const hasJoined = joined.has(post.id);
                  return (
                    <m.li
                      key={post.id}
                      custom={i}
                      variants={row}
                      initial="hidden"
                      whileInView="show"
                      viewport={{ once: true, margin: "-60px" }}
                      className="group border-b border-hairline py-6 transition-colors hover:bg-surface-2/50"
                    >
                      <div className="flex flex-wrap items-start justify-between gap-4">
                        <div className="min-w-0">
                          <p className="font-display text-xl leading-snug text-ink">
                            {post.title}
                          </p>
                          <p className="mt-2 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
                            {post.arena ? `${post.arena.name} · ${post.arena.area}` : "Venue TBD"}
                            <span className="mx-3 text-hairline-2">|</span>
                            {kickoffFmt.format(new Date(post.starts_at))}
                            <span className="mx-3 text-hairline-2">|</span>
                            {SKILL_LABEL[post.skill]}
                          </p>
                          {post.author && (
                            <p className="mt-2 text-xs text-ink-dim">
                              Posted by{" "}
                              <span className="text-ink">@{post.author.username}</span>
                              <span className="ml-2 font-mono text-[0.6rem] text-gold">
                                ★ {post.author.community_score}
                              </span>
                            </p>
                          )}
                        </div>

                        <div className="flex items-center gap-5">
                          {/* Spots indicator: one diamond per needed player */}
                          <div className="text-right">
                            <div className="mb-1.5 flex justify-end gap-1.5" aria-hidden>
                              {Array.from({ length: post.needed_players }, (_, n) => (
                                <i
                                  key={n}
                                  className={`h-1.5 w-1.5 rotate-45 ${
                                    n < post.filled_players ? "bg-ink-faint" : "bg-gold"
                                  }`}
                                />
                              ))}
                            </div>
                            <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim">
                              {spotsLeft} {spotsLeft === 1 ? "spot" : "spots"} left
                            </span>
                          </div>

                          <button
                            onClick={() => handleJoin(post.id)}
                            disabled={hasJoined || isPending || spotsLeft === 0}
                            className={`border px-5 py-2.5 font-mono text-[0.62rem] uppercase tracking-editorial transition-colors duration-300 ${
                              hasJoined
                                ? "border-sage/50 text-sage"
                                : "border-hairline-2 text-ink-dim hover:border-gold hover:text-gold"
                            } disabled:cursor-default`}
                          >
                            {hasJoined ? "Requested" : "Join"}
                          </button>
                        </div>
                      </div>
                    </m.li>
                  );
                })}
              </ul>
            )}

            <AnimatePresence>
              {error && (
                <m.p
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  role="alert"
                  className="mt-4 font-mono text-xs text-ember"
                >
                  {error}
                </m.p>
              )}
            </AnimatePresence>
          </div>
        </div>
      </div>
    </section>
  );
}
