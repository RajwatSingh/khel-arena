"use client";

// ============================================================================
// CommunityHub — the matchmaking board.
// Open calls for players, soonest kick-off first, rendered as an editorial
// ledger. Joining a call files a REQUEST; the call's author approves or
// declines it from the "Your calls" panel above the board.
// ============================================================================

import { useState, useTransition } from "react";
import { AnimatePresence, m } from "framer-motion";
import { PitchBackdrop } from "@/components/PitchLines";
import type {
  ActionResult,
  MatchmakingCall,
  MatchmakingPost,
  SkillTier,
} from "@/lib/types";

export interface CommunityHubProps {
  posts: MatchmakingPost[];
  myCalls: MatchmakingCall[];
  onJoin: (postId: string) => Promise<ActionResult<null>>;
  onApprove: (postId: string, userId: string) => Promise<ActionResult<null>>;
  onDecline: (postId: string, userId: string) => Promise<ActionResult<null>>;
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

export default function CommunityHub({
  posts,
  myCalls,
  onJoin,
  onApprove,
  onDecline,
}: CommunityHubProps) {
  const [calls, setCalls] = useState<MatchmakingCall[]>(myCalls);
  const [joined, setJoined] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  // Author's own calls already live in the manage panel — keep them off the
  // public board so they don't try to "join" themselves.
  const ownIds = new Set(calls.map((c) => c.id));
  const board = posts.filter((p) => !ownIds.has(p.id));

  const handleJoin = (postId: string) => {
    setError(null);
    startTransition(async () => {
      const res = await onJoin(postId);
      if (res.ok) setJoined((prev) => new Set(prev).add(postId));
      else setError(res.error);
    });
  };

  const handleApprove = (postId: string, userId: string) => {
    setError(null);
    startTransition(async () => {
      const res = await onApprove(postId, userId);
      if (res.ok) {
        setCalls((prev) =>
          prev.map((c) =>
            c.id !== postId
              ? c
              : {
                  ...c,
                  filled_players: Math.min(c.filled_players + 1, c.needed_players),
                  responses: c.responses.map((r) =>
                    r.user_id === userId ? { ...r, accepted: true } : r
                  ),
                }
          )
        );
      } else {
        setError(res.error);
      }
    });
  };

  const handleDecline = (postId: string, userId: string) => {
    setError(null);
    startTransition(async () => {
      const res = await onDecline(postId, userId);
      if (res.ok) {
        setCalls((prev) =>
          prev.map((c) => {
            if (c.id !== postId) return c;
            const removed = c.responses.find((r) => r.user_id === userId);
            return {
              ...c,
              filled_players: removed?.accepted
                ? Math.max(c.filled_players - 1, 0)
                : c.filled_players,
              responses: c.responses.filter((r) => r.user_id !== userId),
            };
          })
        );
      } else {
        setError(res.error);
      }
    });
  };

  return (
    <section id="community" className="grain relative min-h-screen overflow-hidden bg-surface py-28">
      <PitchBackdrop />

      {/* Devanagari watermark */}
      <m.span
        aria-hidden
        initial={{ opacity: 0 }}
        animate={{ opacity: 0.04 }}
        transition={{ duration: 2, delay: 0.6 }}
        className="absolute -right-8 top-1/2 -translate-y-1/2 select-none font-display text-[18rem] leading-none text-ink"
      >
        समुदाय
      </m.span>

      <div className="relative mx-auto max-w-6xl px-6">
        <div className="mb-16 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="eyebrow mb-4">Community · समुदाय</p>
            <h2 className="font-display text-5xl tracking-tight text-ink">
              The valley plays together
            </h2>
          </div>
          <p className="max-w-xs text-sm leading-relaxed text-ink-dim">
            Short a striker? Open your booking. Looking for a game? Ask to join an
            open call below — the host waves you in.
          </p>
        </div>

        <AnimatePresence>
          {error && (
            <m.p
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              role="alert"
              className="mb-6 font-mono text-xs text-ember"
            >
              {error}
            </m.p>
          )}
        </AnimatePresence>

        {/* ───────────────── Your calls (author manages requests) ───────────────── */}
        {calls.length > 0 && (
          <div className="mb-16">
            <div className="mb-6 flex items-baseline justify-between border-b border-hairline-2 pb-4">
              <h3 className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim">
                Your calls
              </h3>
              <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                {calls.length} open
              </span>
            </div>

            <ul className="space-y-8">
              {calls.map((call) => {
                const pending = call.responses.filter((r) => !r.accepted);
                const accepted = call.responses.filter((r) => r.accepted);
                const spotsLeft = call.needed_players - call.filled_players;
                return (
                  <li key={call.id} className="border border-hairline-2 bg-canvas p-6">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="font-display text-xl leading-snug text-ink">{call.title}</p>
                        <p className="mt-2 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
                          {call.arena ? `${call.arena.name} · ${call.arena.area}` : "Venue TBD"}
                          <span className="mx-3 text-hairline-2">|</span>
                          {kickoffFmt.format(new Date(call.starts_at))}
                        </p>
                      </div>
                      <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim">
                        {call.filled_players}/{call.needed_players} filled
                        {spotsLeft > 0 && (
                          <span className="text-sage"> · {spotsLeft} open</span>
                        )}
                      </span>
                    </div>

                    {/* Accepted players */}
                    {accepted.length > 0 && (
                      <div className="mt-5">
                        <p className="eyebrow mb-2">In the squad</p>
                        <ul className="flex flex-wrap gap-2">
                          {accepted.map((r) => (
                            <li
                              key={r.user_id}
                              className="flex items-center gap-2 border border-sage/40 px-3 py-1.5"
                            >
                              <span className="font-mono text-[0.62rem] text-ink">
                                @{r.responder?.username ?? "player"}
                              </span>
                              <button
                                onClick={() => handleDecline(call.id, r.user_id)}
                                disabled={isPending}
                                aria-label={`Remove @${r.responder?.username ?? "player"}`}
                                className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint transition-colors hover:text-ember disabled:opacity-50"
                              >
                                Remove
                              </button>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}

                    {/* Pending requests */}
                    <div className="mt-5">
                      <p className="eyebrow mb-2">
                        Requests {pending.length > 0 && `(${pending.length})`}
                      </p>
                      {pending.length === 0 ? (
                        <p className="text-sm text-ink-faint">No pending requests yet.</p>
                      ) : (
                        <ul className="space-y-2">
                          {pending.map((r) => (
                            <li
                              key={r.user_id}
                              className="flex flex-wrap items-center justify-between gap-3 border-b border-hairline py-2"
                            >
                              <div className="min-w-0">
                                <p className="text-sm text-ink">
                                  @{r.responder?.username ?? "player"}
                                  {typeof r.responder?.community_score === "number" && (
                                    <span className="ml-2 font-mono text-[0.6rem] text-gold">
                                      ★ {r.responder.community_score}
                                    </span>
                                  )}
                                </p>
                                {r.message && (
                                  <p className="mt-0.5 truncate text-xs text-ink-dim">{r.message}</p>
                                )}
                              </div>
                              <div className="flex items-center gap-2">
                                <button
                                  onClick={() => handleApprove(call.id, r.user_id)}
                                  disabled={isPending || spotsLeft <= 0}
                                  className="border border-sage/50 px-4 py-1.5 font-mono text-[0.6rem] uppercase tracking-editorial text-sage transition-colors hover:bg-sage hover:text-ink disabled:opacity-40"
                                >
                                  Approve
                                </button>
                                <button
                                  onClick={() => handleDecline(call.id, r.user_id)}
                                  disabled={isPending}
                                  className="border border-hairline-2 px-4 py-1.5 font-mono text-[0.6rem] uppercase tracking-editorial text-ink-faint transition-colors hover:border-ember hover:text-ember disabled:opacity-50"
                                >
                                  Decline
                                </button>
                              </div>
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          </div>
        )}

        {/* ───────────────── Matchmaking board ───────────────── */}
        <div>
          <div className="mb-6 flex items-baseline justify-between border-b border-hairline-2 pb-4">
            <h3 className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim">
              Open calls
            </h3>
            <span className="flex items-center gap-2 font-mono text-[0.62rem] uppercase tracking-editorial text-sage">
              <i className="h-1.5 w-1.5 animate-pulse rounded-full bg-sage" />
              {board.length} live
            </span>
          </div>

          {board.length === 0 ? (
            <div className="border border-dashed border-hairline-2 p-12 text-center">
              <p className="font-display text-xl text-ink-dim">The board is quiet.</p>
              <p className="mt-2 text-sm text-ink-faint">
                Open one of your bookings to the community and it appears here.
              </p>
            </div>
          ) : (
            <ul>
              {board.map((post, i) => {
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
                        <p className="font-display text-xl leading-snug text-ink">{post.title}</p>
                        <p className="mt-2 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
                          {post.arena ? `${post.arena.name} · ${post.arena.area}` : "Venue TBD"}
                          <span className="mx-3 text-hairline-2">|</span>
                          {kickoffFmt.format(new Date(post.starts_at))}
                          <span className="mx-3 text-hairline-2">|</span>
                          {SKILL_LABEL[post.skill]}
                        </p>
                        {post.author && (
                          <p className="mt-2 text-xs text-ink-dim">
                            Posted by <span className="text-ink">@{post.author.username}</span>
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
                          {hasJoined ? "Requested" : "Ask to join"}
                        </button>
                      </div>
                    </div>
                  </m.li>
                );
              })}
            </ul>
          )}
        </div>
      </div>
    </section>
  );
}
