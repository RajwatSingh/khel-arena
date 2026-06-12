"use client";

// ============================================================================
// MyBookingsHub — /my-bookings composition.
// Left: booking ledger — upcoming/past segmented toggle, editorial rows with
// court + arena, time, price in display gold, status badges, cancel + open-
// to-community actions. Right: sticky summary card with aggregate stats,
// book-another CTA, and community link.
// ============================================================================

import { useEffect, useMemo, useState, useTransition } from "react";
import Link from "next/link";
import { AnimatePresence, m } from "framer-motion";
import { PitchBackdrop, PitchDivider } from "@/components/PitchLines";
import type { ActionResult, BookingStatus, MatchmakingPost, MyBooking } from "@/lib/types";

export interface MyBookingsHubProps {
  bookings: MyBooking[];
  onCancel: (bookingId: string) => Promise<ActionResult<null>>;
  onToggleCommunity: (input: {
    bookingId: string;
    open: boolean;
    neededPlayers?: number;
    title?: string;
    skill?: string;
    description?: string;
  }) => Promise<ActionResult<MatchmakingPost>>;
}

const NPR = new Intl.NumberFormat("en-IN");
const timeFmt = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Kathmandu",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});
const dayFmt = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Kathmandu",
  weekday: "short",
  day: "2-digit",
  month: "short",
});

const STATUS_STYLE: Record<BookingStatus, { label: string; color: string }> = {
  confirmed: { label: "Confirmed", color: "border-sage/60 text-sage" },
  pending: { label: "Pending", color: "border-gold/60 text-gold" },
  completed: { label: "Completed", color: "border-ink-dim/40 text-ink-dim" },
  cancelled: { label: "Cancelled", color: "border-ember/40 text-ember" },
  no_show: { label: "No show", color: "border-ember/40 text-ember" },
};

const ease = [0.22, 1, 0.36, 1] as const;

/** Modal card: players needed + a note on the game — then the slot goes live. */
function OpenSlotDialog({
  booking,
  onClose,
  onConfirm,
}: {
  booking: MyBooking;
  onClose: () => void;
  onConfirm: MyBookingsHubProps["onToggleCommunity"];
}) {
  const [count, setCount] = useState(2);
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const submit = () => {
    setError(null);
    const plural = count > 1 ? "s" : "";
    const title = `Need ${count} player${plural} at ${booking.arena_name} — come play`;
    startTransition(async () => {
      const res = await onConfirm({
        bookingId: booking.id,
        open: true,
        neededPlayers: count,
        title,
        skill: "casual",
        description: description.trim() || undefined,
      });
      if (res.ok) onClose();
      else setError(res.error);
    });
  };

  return (
    <m.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 z-[60] flex items-center justify-center bg-ink/40 px-6 backdrop-blur-sm"
      onClick={onClose}
    >
      <m.div
        role="dialog"
        aria-modal="true"
        aria-label="Open this slot to the community"
        initial={{ opacity: 0, y: 24, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: 12, scale: 0.98 }}
        transition={{ duration: 0.35, ease }}
        className="grain relative w-full max-w-md border border-hairline-2 bg-canvas p-8"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Watermark */}
        <span
          aria-hidden
          className="pointer-events-none absolute -right-2 -top-8 select-none font-display text-[7rem] leading-none text-ink"
          style={{ opacity: 0.05 }}
        >
          खेल
        </span>

        <p className="eyebrow mb-3">Community &middot; समुदाय</p>
        <h3 className="font-display text-3xl tracking-tight text-ink">Open this slot</h3>
        <p className="mt-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
          {booking.arena_name} &middot; {booking.court_label}
          <span className="mx-2 text-hairline-2">|</span>
          {dayFmt.format(new Date(booking.starts_at))},{" "}
          {timeFmt.format(new Date(booking.starts_at))}
        </p>

        <PitchDivider className="my-6" />

        <p className="eyebrow mb-3">Players needed</p>
        <div
          role="radiogroup"
          aria-label="Players needed"
          className="grid grid-cols-5 gap-px bg-hairline"
        >
          {Array.from({ length: 15 }, (_, i) => i + 1).map((n) => (
            <button
              key={n}
              type="button"
              role="radio"
              aria-checked={count === n}
              onClick={() => setCount(n)}
              className={`py-2.5 font-display text-lg tabular-nums transition-colors duration-200 ${
                count === n
                  ? "bg-gold text-ink"
                  : "bg-surface text-ink-dim hover:bg-surface-2 hover:text-ink"
              }`}
            >
              {n}
            </button>
          ))}
        </div>

        <p className="eyebrow mb-3 mt-7">How will the game be?</p>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          maxLength={280}
          rows={4}
          placeholder="Friendly 5-a-side, rolling subs, bring both bibs. Looking for a goleiro and two alas…"
          className="w-full resize-none border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none"
        />
        <p className="mt-2 font-mono text-[0.55rem] uppercase tracking-editorial text-ink-faint">
          Mention the positions you need — keeper, fixo, ala, pivô
        </p>

        <PitchDivider className="my-6" />

        <button
          type="button"
          onClick={submit}
          disabled={isPending}
          className="group flex w-full items-center justify-center gap-3 border border-gold/60 px-6 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50"
        >
          {isPending ? "Posting…" : "Post to the community"}
          <span
            aria-hidden
            className="transition-transform duration-300 group-hover:translate-x-1"
          >
            &rarr;
          </span>
        </button>
        <button
          type="button"
          onClick={onClose}
          className="mt-4 block w-full text-center font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-ink"
        >
          Never mind
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
        </AnimatePresence>
      </m.div>
    </m.div>
  );
}

const rowAnim = {
  hidden: { opacity: 0, y: 14 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.55, ease, delay: i * 0.07 },
  }),
};

export default function MyBookingsHub({
  bookings,
  onCancel,
  onToggleCommunity,
}: MyBookingsHubProps) {
  const [tab, setTab] = useState<"upcoming" | "past">("upcoming");
  const [isPending, startTransition] = useTransition();
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [dialogBooking, setDialogBooking] = useState<MyBooking | null>(null);

  const now = useMemo(() => new Date().toISOString(), []);

  const upcoming = useMemo(
    () =>
      bookings
        .filter((b) => b.ends_at > now && b.status !== "cancelled")
        .sort((a, b) => a.starts_at.localeCompare(b.starts_at)),
    [bookings, now]
  );
  const past = useMemo(
    () =>
      bookings
        .filter((b) => b.ends_at <= now || b.status === "cancelled")
        .sort((a, b) => b.starts_at.localeCompare(a.starts_at)),
    [bookings, now]
  );

  const visible = tab === "upcoming" ? upcoming : past;

  const totalSpent = useMemo(
    () =>
      bookings
        .filter((b) => b.status !== "cancelled")
        .reduce((sum, b) => sum + b.price_npr, 0),
    [bookings]
  );

  const handleCancel = (id: string) => {
    setCancelError(null);
    startTransition(async () => {
      const res = await onCancel(id);
      if (!res.ok) setCancelError(res.error);
    });
  };

  const handleToggleCommunity = (b: MyBooking) => {
    if (!b.open_to_join) {
      // Opening asks for details first — the dialog posts to the board.
      setDialogBooking(b);
      return;
    }
    startTransition(async () => {
      await onToggleCommunity({ bookingId: b.id, open: false });
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
        className="absolute -right-8 top-1/2 -translate-y-1/2 select-none font-display text-[22rem] leading-none text-ink"
      >
        बुकिङ
      </m.span>

      <div className="relative mx-auto max-w-6xl px-6">
        {/* Page header */}
        <div className="mb-16 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="eyebrow mb-4">Bookings &middot; बुकिङ</p>
            <h2 className="font-display text-5xl tracking-tight text-ink">
              Your <em className="not-italic text-gold">pitches</em>
            </h2>
          </div>
          <p className="max-w-xs text-sm leading-relaxed text-ink-dim">
            Every court you&rsquo;ve reserved &mdash; cancel upcoming slots, or open
            them to the community so others can join your game.
          </p>
        </div>

        <div className="grid gap-16 lg:grid-cols-5">
          {/* ────────────── LEFT: Booking ledger ────────────── */}
          <div className="lg:col-span-3">
            {/* Segmented toggle */}
            <div className="mb-6 flex items-center justify-between border-b border-hairline-2 pb-4">
              <div
                role="radiogroup"
                aria-label="Booking filter"
                className="flex border border-hairline-2"
              >
                {(["upcoming", "past"] as const).map((t) => (
                  <button
                    key={t}
                    type="button"
                    role="radio"
                    aria-checked={tab === t}
                    onClick={() => setTab(t)}
                    className={`px-5 py-2.5 font-mono text-[0.62rem] uppercase tracking-editorial transition-colors ${
                      tab === t
                        ? "bg-ink text-canvas"
                        : "text-ink-dim hover:bg-surface-2 hover:text-ink"
                    }`}
                  >
                    {t === "upcoming"
                      ? `Upcoming (${upcoming.length})`
                      : `Past (${past.length})`}
                  </button>
                ))}
              </div>
              <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                {bookings.length} total
              </span>
            </div>

            {visible.length === 0 ? (
              <div className="border border-dashed border-hairline-2 p-12 text-center">
                <p className="font-display text-xl text-ink-dim">
                  {tab === "upcoming" ? "No upcoming bookings." : "No past bookings."}
                </p>
                <p className="mt-2 text-sm text-ink-faint">
                  {tab === "upcoming"
                    ? "Reserve a court and it will appear here."
                    : "Your booking history will build up over time."}
                </p>
              </div>
            ) : (
              <ul>
                {visible.map((b, i) => {
                  const s = STATUS_STYLE[b.status];
                  const canCancel =
                    tab === "upcoming" &&
                    (b.status === "pending" || b.status === "confirmed");
                  const canToggle =
                    tab === "upcoming" && b.status === "confirmed";

                  return (
                    <m.li
                      key={b.id}
                      custom={i}
                      variants={rowAnim}
                      initial="hidden"
                      whileInView="show"
                      viewport={{ once: true, margin: "-60px" }}
                      className="border-b border-hairline py-7"
                    >
                      <div className="flex flex-wrap items-start justify-between gap-5">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-3">
                            <h4 className="font-display text-2xl leading-tight tracking-tight text-ink">
                              {b.court_label}
                            </h4>
                            <span className={`border px-2 py-0.5 font-mono text-[0.58rem] uppercase tracking-editorial ${s.color}`}>
                              {s.label}
                            </span>
                            {b.is_peak && (
                              <span className="border border-gold/30 px-2 py-0.5 font-mono text-[0.55rem] uppercase tracking-editorial text-gold">
                                Peak
                              </span>
                            )}
                          </div>
                          <p className="mt-2 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
                            {b.arena_name} &middot; {b.arena_area}
                          </p>
                          <p className="mt-2 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
                            {dayFmt.format(new Date(b.starts_at))}
                            <span className="mx-2 text-hairline-2">|</span>
                            <span className="text-ink">
                              {timeFmt.format(new Date(b.starts_at))}
                              {" – "}
                              {timeFmt.format(new Date(b.ends_at))}
                            </span>
                          </p>

                          {b.open_to_join && (
                            <p className="mt-2 font-mono text-[0.6rem] uppercase tracking-editorial text-sage">
                              Open to the community
                            </p>
                          )}
                        </div>

                        <div className="flex flex-col items-end gap-3">
                          <p className="text-right">
                            <span className="eyebrow block">Price</span>
                            <span className="font-display text-3xl tabular-nums text-gold">
                              रू {NPR.format(b.price_npr)}
                            </span>
                          </p>

                          {/* Time progress diamonds */}
                          <div
                            className="flex max-w-[5rem] flex-wrap justify-end gap-1"
                            aria-hidden
                          >
                            {(() => {
                              const start = new Date(b.starts_at).getTime();
                              const end = new Date(b.ends_at).getTime();
                              const hours = Math.max(1, Math.round((end - start) / 3_600_000));
                              const elapsed = Math.min(
                                hours,
                                Math.max(0, Math.round((Date.now() - start) / 3_600_000))
                              );
                              return Array.from({ length: hours }, (_, n) => (
                                <i
                                  key={n}
                                  className={`h-1.5 w-1.5 rotate-45 ${
                                    n < elapsed ? "bg-gold" : "bg-hairline-2"
                                  }`}
                                />
                              ));
                            })()}
                          </div>

                          {/* Actions */}
                          <div className="flex items-center gap-3">
                            {canToggle && (
                              <button
                                onClick={() => handleToggleCommunity(b)}
                                disabled={isPending}
                                className={`border px-4 py-2 font-mono text-[0.6rem] uppercase tracking-editorial transition-colors duration-300 ${
                                  b.open_to_join
                                    ? "border-sage/60 text-sage hover:border-ink-dim hover:text-ink-dim"
                                    : "border-hairline-2 text-ink-dim hover:border-sage hover:text-sage"
                                } disabled:opacity-50`}
                              >
                                {b.open_to_join ? "Close to community" : "Open to community"}
                              </button>
                            )}
                            {canCancel && (
                              <button
                                onClick={() => handleCancel(b.id)}
                                disabled={isPending}
                                className="border border-hairline-2 px-4 py-2 font-mono text-[0.6rem] uppercase tracking-editorial text-ink-faint transition-colors duration-300 hover:border-ember hover:text-ember disabled:opacity-50"
                              >
                                Cancel
                              </button>
                            )}
                          </div>
                        </div>
                      </div>
                    </m.li>
                  );
                })}
              </ul>
            )}

            <AnimatePresence>
              {cancelError && (
                <m.p
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  role="alert"
                  className="mt-4 font-mono text-xs text-ember"
                >
                  {cancelError}
                </m.p>
              )}
            </AnimatePresence>
          </div>

          {/* ────────────── RIGHT: Summary card (sticky) ────────────── */}
          <div className="lg:col-span-2">
            <div className="sticky top-24">
              <p className="eyebrow mb-4">
                Season summary
              </p>

              <div className="relative overflow-hidden border border-hairline-2 bg-surface">
                {/* Watermark */}
                <span
                  aria-hidden
                  className="pointer-events-none absolute -right-3 -top-6 select-none font-display text-[10rem] leading-none text-ink"
                  style={{ opacity: 0.04 }}
                >
                  #
                </span>

                <div className="relative p-8">
                  <h3 className="font-display text-3xl tracking-tight text-ink">
                    At a glance
                  </h3>

                  <PitchDivider className="my-6" />

                  {/* Stats grid */}
                  <div className="grid grid-cols-3 gap-px bg-hairline text-center">
                    <div className="bg-surface py-4">
                      <p className="font-display text-2xl tabular-nums text-ink">
                        {bookings.length}
                      </p>
                      <p className="eyebrow mt-1">Total</p>
                    </div>
                    <div className="bg-surface py-4">
                      <p className="font-display text-2xl tabular-nums text-gold">
                        {upcoming.length}
                      </p>
                      <p className="eyebrow mt-1">Upcoming</p>
                    </div>
                    <div className="bg-surface py-4">
                      <p className="font-display text-2xl tabular-nums text-ink">
                        रू {NPR.format(totalSpent)}
                      </p>
                      <p className="eyebrow mt-1">Spent</p>
                    </div>
                  </div>

                  {/* Upcoming next */}
                  {upcoming.length > 0 && (
                    <div className="mt-6">
                      <p className="eyebrow mb-3">Next up</p>
                      <div className="border border-hairline bg-canvas px-4 py-3">
                        <p className="font-display text-lg tracking-tight text-ink">
                          {upcoming[0].court_label}
                        </p>
                        <p className="mt-1 font-mono text-[0.6rem] uppercase tracking-editorial text-ink-faint">
                          {upcoming[0].arena_name} &middot;{" "}
                          {dayFmt.format(new Date(upcoming[0].starts_at))},{" "}
                          {timeFmt.format(new Date(upcoming[0].starts_at))}
                        </p>
                      </div>
                    </div>
                  )}

                  <PitchDivider className="my-6" />

                  {/* CTAs */}
                  <div className="space-y-4">
                    <Link
                      href="/book"
                      className="group flex w-full items-center justify-center gap-3 border border-gold/60 px-6 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink"
                    >
                      Book another slot
                      <span
                        aria-hidden
                        className="transition-transform duration-300 group-hover:translate-x-1"
                      >
                        &rarr;
                      </span>
                    </Link>
                    <Link
                      href="/community"
                      className="block text-center font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-ink"
                    >
                      Browse open games
                    </Link>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <AnimatePresence>
        {dialogBooking && (
          <OpenSlotDialog
            booking={dialogBooking}
            onClose={() => setDialogBooking(null)}
            onConfirm={onToggleCommunity}
          />
        )}
      </AnimatePresence>
    </section>
  );
}
