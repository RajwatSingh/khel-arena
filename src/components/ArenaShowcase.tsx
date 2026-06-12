"use client";

// ============================================================================
// ArenaShowcase — /arenas/[slug] composition.
// Public face of a futsal: hours, amenities, courts & prices, the owner's
// photo gallery, and the community's star reviews. Signed-in players leave
// (or replace) one review; demo mode simulates the submit locally.
// ============================================================================

import { useMemo, useState, useTransition } from "react";
import Link from "next/link";
import { AnimatePresence, m } from "framer-motion";
import { PitchBackdrop, PitchDivider } from "@/components/PitchLines";
import { submitReview } from "@/actions/arena";
import { DEMO_PROFILE } from "@/lib/demo";
import type { ArenaProfile, ArenaReview } from "@/lib/types";

const NPR = new Intl.NumberFormat("en-IN");
const dateFmt = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Kathmandu",
  day: "2-digit",
  month: "short",
  year: "numeric",
});

const toHHMM = (t: string) => t.slice(0, 5);

const ease = [0.22, 1, 0.36, 1] as const;
const rowAnim = {
  hidden: { opacity: 0, y: 14 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.55, ease, delay: i * 0.07 },
  }),
};

function Stars({ value, className = "" }: { value: number; className?: string }) {
  return (
    <span
      aria-label={`${value} out of 5 stars`}
      className={`leading-none tracking-[0.15em] ${className}`}
    >
      {[1, 2, 3, 4, 5].map((n) => (
        <span key={n} className={n <= Math.round(value) ? "text-gold" : "text-hairline-2"}>
          ★
        </span>
      ))}
    </span>
  );
}

export default function ArenaShowcase({
  demoMode,
  profile,
}: {
  demoMode: boolean;
  profile: ArenaProfile;
}) {
  const { arena, courts, photos } = profile;

  const [reviews, setReviews] = useState<ArenaReview[]>(profile.reviews);
  const [myReview, setMyReview] = useState<ArenaReview | null>(profile.myReview);
  const [rating, setRating] = useState(profile.myReview?.rating ?? 0);
  const [hovered, setHovered] = useState(0);
  const [comment, setComment] = useState(profile.myReview?.comment ?? "");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const average = useMemo(() => {
    if (reviews.length === 0) return arena.rating ?? 0;
    return reviews.reduce((sum, r) => sum + r.rating, 0) / reviews.length;
  }, [reviews, arena.rating]);

  const handleSubmit = () => {
    setError(null);
    setNotice(null);
    if (rating < 1) {
      setError("Pick a star rating first.");
      return;
    }

    startTransition(async () => {
      if (demoMode) {
        await new Promise((r) => setTimeout(r, 400));
        const review: ArenaReview = {
          id: myReview?.id ?? `local-${Date.now()}`,
          arena_id: arena.id,
          user_id: DEMO_PROFILE.id,
          rating,
          comment: comment.trim() || null,
          created_at: new Date().toISOString(),
          author: {
            username: DEMO_PROFILE.username,
            full_name: DEMO_PROFILE.full_name,
            avatar_url: DEMO_PROFILE.avatar_url,
          },
        };
        setReviews((prev) => [review, ...prev.filter((r) => r.id !== review.id)]);
        setMyReview(review);
        setNotice(myReview ? "Your review was updated." : "Thanks — your review is live.");
        return;
      }

      const res = await submitReview({
        arenaId: arena.id,
        slug: arena.slug,
        rating,
        comment: comment.trim() || null,
      });
      if (!res.ok) {
        setError(res.error);
        return;
      }
      setReviews((prev) => [res.data, ...prev.filter((r) => r.user_id !== res.data.user_id)]);
      setMyReview(res.data);
      setNotice(myReview ? "Your review was updated." : "Thanks — your review is live.");
    });
  };

  return (
    <section className="grain relative min-h-screen overflow-hidden bg-canvas py-28">
      <PitchBackdrop />

      {/* Devanagari watermark */}
      <m.span
        aria-hidden
        initial={{ opacity: 0 }}
        animate={{ opacity: 0.04 }}
        transition={{ duration: 2, delay: 0.6 }}
        className="absolute -right-8 top-1/2 -translate-y-1/2 select-none font-display text-[17rem] leading-none text-ink"
      >
        मैदान
      </m.span>

      <div className="relative mx-auto max-w-6xl px-6">
        {/* Page header */}
        <div className="mb-16 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="eyebrow mb-4">Arena &middot; मैदान</p>
            <h2 className="font-display text-5xl tracking-tight text-ink">
              {arena.name}
            </h2>
            <p className="mt-3 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
              {arena.area} &middot; {arena.city}
              <span className="mx-2 text-hairline-2">|</span>
              <span className="text-ink">
                {toHHMM(arena.opens_at)} – {toHHMM(arena.closes_at)} daily
              </span>
            </p>
          </div>
          <div className="text-left sm:text-right">
            <Stars value={average} className="text-xl" />
            <p className="mt-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
              {average > 0 ? average.toFixed(1) : "—"} &middot; {reviews.length}{" "}
              {reviews.length === 1 ? "review" : "reviews"}
            </p>
          </div>
        </div>

        {arena.description && (
          <p className="mb-12 max-w-2xl text-base leading-relaxed text-ink-dim">
            {arena.description}
          </p>
        )}

        {arena.amenities.length > 0 && (
          <div className="mb-16 flex flex-wrap gap-2">
            {arena.amenities.map((a) => (
              <span
                key={a}
                className="border border-hairline-2 px-3 py-1.5 font-mono text-[0.6rem] uppercase tracking-editorial text-ink-dim"
              >
                {a}
              </span>
            ))}
          </div>
        )}

        <div className="grid gap-16 lg:grid-cols-5">
          {/* ────────────── LEFT: Gallery + reviews ────────────── */}
          <div className="lg:col-span-3">
            {/* Photo gallery */}
            <div className="mb-6 flex items-center justify-between border-b border-hairline-2 pb-4">
              <h3 className="font-display text-2xl tracking-tight text-ink">From the pitch</h3>
              <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                {photos.length} {photos.length === 1 ? "photo" : "photos"}
              </span>
            </div>

            {photos.length === 0 ? (
              <div className="border border-dashed border-hairline-2 p-12 text-center">
                <p className="font-display text-xl text-ink-dim">No photos yet.</p>
                <p className="mt-2 text-sm text-ink-faint">
                  The owner hasn&rsquo;t posted any pictures of this futsal.
                </p>
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                {photos.map((p, i) => (
                  <m.figure
                    key={p.id}
                    custom={i}
                    variants={rowAnim}
                    initial="hidden"
                    whileInView="show"
                    viewport={{ once: true, margin: "-60px" }}
                    className={i === 0 ? "sm:col-span-2" : undefined}
                  >
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={p.url}
                      alt={p.caption ?? `${arena.name} photo`}
                      className="aspect-[4/3] w-full border border-hairline object-cover sm:aspect-[16/9]"
                      loading="lazy"
                    />
                    {p.caption && (
                      <figcaption className="mt-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                        {p.caption}
                      </figcaption>
                    )}
                  </m.figure>
                ))}
              </div>
            )}

            {/* Reviews */}
            <div className="mb-6 mt-16 flex items-center justify-between border-b border-hairline-2 pb-4">
              <h3 className="font-display text-2xl tracking-tight text-ink">
                What players say
              </h3>
              <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                {reviews.length} total
              </span>
            </div>

            {reviews.length === 0 ? (
              <div className="border border-dashed border-hairline-2 p-12 text-center">
                <p className="font-display text-xl text-ink-dim">No reviews yet.</p>
                <p className="mt-2 text-sm text-ink-faint">
                  Played here? Be the first to rate it.
                </p>
              </div>
            ) : (
              <ul>
                {reviews.map((r, i) => (
                  <m.li
                    key={r.id}
                    custom={i}
                    variants={rowAnim}
                    initial="hidden"
                    whileInView="show"
                    viewport={{ once: true, margin: "-60px" }}
                    className="border-b border-hairline py-6"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <p className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink">
                        {r.author?.full_name ?? "A player"}
                        <span className="ml-2 text-ink-faint">
                          @{r.author?.username ?? "unknown"}
                        </span>
                      </p>
                      <span className="font-mono text-[0.6rem] uppercase tracking-editorial text-ink-faint">
                        {dateFmt.format(new Date(r.created_at))}
                      </span>
                    </div>
                    <Stars value={r.rating} className="mt-2 block text-sm" />
                    {r.comment && (
                      <p className="mt-3 max-w-xl text-sm leading-relaxed text-ink-dim">
                        {r.comment}
                      </p>
                    )}
                  </m.li>
                ))}
              </ul>
            )}
          </div>

          {/* ────────────── RIGHT: Courts + review form (sticky) ────────────── */}
          <div className="lg:col-span-2">
            <div className="sticky top-24 space-y-10">
              {/* Courts & prices */}
              <div className="relative overflow-hidden border border-hairline-2 bg-surface">
                <span
                  aria-hidden
                  className="pointer-events-none absolute -right-3 -top-6 select-none font-display text-[10rem] leading-none text-ink"
                  style={{ opacity: 0.04 }}
                >
                  ⚽
                </span>
                <div className="relative p-8">
                  <h3 className="font-display text-3xl tracking-tight text-ink">
                    Courts &amp; rates
                  </h3>
                  <PitchDivider className="my-6" />
                  {courts.length === 0 ? (
                    <p className="text-sm text-ink-faint">No courts listed yet.</p>
                  ) : (
                    <ul className="space-y-4">
                      {courts.map((c) => (
                        <li
                          key={c.id}
                          className="flex items-baseline justify-between gap-4 border-b border-hairline pb-4 last:border-b-0 last:pb-0"
                        >
                          <div>
                            <p className="font-display text-xl tracking-tight text-ink">
                              {c.label}
                            </p>
                            <p className="mt-1 font-mono text-[0.6rem] uppercase tracking-editorial text-ink-faint">
                              {c.side_count}-a-side
                            </p>
                          </div>
                          <p className="text-right">
                            <span className="font-display text-2xl tabular-nums text-gold">
                              रू {NPR.format(c.base_price)}
                            </span>
                            <span className="block font-mono text-[0.55rem] uppercase tracking-editorial text-ink-faint">
                              per hour, off-peak
                            </span>
                          </p>
                        </li>
                      ))}
                    </ul>
                  )}
                  <PitchDivider className="my-6" />
                  <Link
                    href="/book"
                    className="group flex w-full items-center justify-center gap-3 border border-gold/60 px-6 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink"
                  >
                    Book a slot here
                    <span
                      aria-hidden
                      className="transition-transform duration-300 group-hover:translate-x-1"
                    >
                      &rarr;
                    </span>
                  </Link>
                </div>
              </div>

              {/* Review form */}
              <div className="border border-hairline-2 bg-surface p-8">
                <h3 className="font-display text-2xl tracking-tight text-ink">
                  {myReview ? "Update your review" : "Rate this futsal"}
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-ink-faint">
                  One review per player — submitting again replaces your old one.
                </p>

                <div
                  role="radiogroup"
                  aria-label="Star rating"
                  className="mt-6 flex gap-1"
                  onMouseLeave={() => setHovered(0)}
                >
                  {[1, 2, 3, 4, 5].map((n) => (
                    <button
                      key={n}
                      type="button"
                      role="radio"
                      aria-checked={rating === n}
                      aria-label={`${n} star${n > 1 ? "s" : ""}`}
                      onClick={() => setRating(n)}
                      onMouseEnter={() => setHovered(n)}
                      className={`text-3xl leading-none transition-colors duration-150 ${
                        n <= (hovered || rating) ? "text-gold" : "text-hairline-2 hover:text-gold/60"
                      }`}
                    >
                      ★
                    </button>
                  ))}
                </div>

                <textarea
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  maxLength={500}
                  rows={4}
                  placeholder="How was the turf, the lights, the vibe?"
                  className="mt-5 w-full border border-hairline-2 bg-canvas px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none"
                />

                <button
                  type="button"
                  onClick={handleSubmit}
                  disabled={isPending}
                  className="mt-4 w-full border border-ink bg-ink px-6 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-canvas transition-colors duration-300 hover:bg-transparent hover:text-ink disabled:opacity-50"
                >
                  {isPending ? "Saving…" : myReview ? "Update review" : "Post review"}
                </button>

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
                  {notice && (
                    <m.p
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      className="mt-4 font-mono text-xs text-sage"
                    >
                      {notice}
                    </m.p>
                  )}
                </AnimatePresence>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
