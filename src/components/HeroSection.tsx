"use client";

// ============================================================================
// HeroSection — the opening statement.
// One orchestrated load sequence: the court markings chalk themselves in, the headline
// rises line by line, then the arena ticker and live stats settle. The
// Devanagari मैदान ("the pitch") watermark anchors the page in Kathmandu.
// ============================================================================

import Link from "next/link";
import { m, useReducedMotion } from "framer-motion";
import { PitchBackdrop } from "@/components/PitchLines";

const ARENAS = [
  "Dhuku Futsal · Jhamsikhel",
  "Hattiban Sports Arena · Lalitpur",
  "Baluwatar Turf · Kathmandu",
  "Yala Futsal Park · Patan",
  "Boudha Five-A-Side · Boudha",
  "Kirtipur Greens · Kirtipur",
];

const STATS = [
  { value: "42", label: "Arenas live" },
  { value: "06–22", label: "Booking hours" },
  { value: "1,900+", label: "Players matched" },
];

const ease = [0.22, 1, 0.36, 1] as const;

export default function HeroSection() {
  const reduceMotion = useReducedMotion();

  const rise = {
    hidden: { y: reduceMotion ? 0 : "110%" },
    show: (i: number) => ({
      y: "0%",
      transition: { duration: 1.1, ease, delay: 0.15 + i * 0.12 },
    }),
  };

  return (
    <section className="grain relative min-h-[calc(100svh-57px)] overflow-hidden bg-canvas">
      {/* The court itself, chalked in hairlines — halfway line, center
          circle, penalty arcs. The page opens standing on the pitch. */}
      <PitchBackdrop withBall />

      {/* Devanagari watermark */}
      <m.span
        aria-hidden
        initial={{ opacity: 0 }}
        animate={{ opacity: 0.05 }}
        transition={{ duration: 2, delay: 0.8 }}
        className="absolute -right-8 top-1/2 -translate-y-1/2 select-none font-display text-[26rem] leading-none text-ink"
      >
        मैदान
      </m.span>

      <div className="relative mx-auto flex min-h-[calc(100svh-57px)] max-w-6xl flex-col justify-between px-6 pb-10 pt-8">
        {/* Headline */}
        <div className="py-20">
          <m.p
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.1 }}
            className="eyebrow mb-8"
          >
            The arena network of the valley
          </m.p>

          <h1 className="font-display text-[clamp(3rem,9vw,7.5rem)] leading-[0.95] tracking-tight text-ink">
            {["Book the pitch.", "Find your five."].map((line, i) => (
              <span key={line} className="block overflow-hidden pb-2">
                <m.span
                  custom={i}
                  variants={rise}
                  initial="hidden"
                  animate="show"
                  className="block"
                >
                  {i === 1 ? (
                    <>
                      Find your <em className="not-italic text-gold">five.</em>
                    </>
                  ) : (
                    line
                  )}
                </m.span>
              </span>
            ))}
          </h1>

          <m.p
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.9, ease, delay: 0.7 }}
            className="mt-8 max-w-md text-base leading-relaxed text-ink-dim"
          >
            Live availability across Kathmandu&rsquo;s best turfs. Pick an hour,
            pay with eSewa or Khalti, and let the community fill your last two
            spots.
          </m.p>

          <m.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.9, ease, delay: 0.85 }}
            className="mt-12 flex flex-wrap items-center gap-6"
          >
            <Link
              href="/book"
              className="group relative inline-flex items-center gap-3 border border-gold/60 px-8 py-4 font-mono text-[0.7rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink"
            >
              Reserve a court
              <span aria-hidden className="transition-transform duration-300 group-hover:translate-x-1">→</span>
            </Link>
            <Link
              href="/community"
              className="font-mono text-[0.7rem] uppercase tracking-editorial text-ink-dim underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-ink"
            >
              Tonight&rsquo;s open games
            </Link>
          </m.div>
        </div>

        {/* Footer strip: stats + arena ticker */}
        <m.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 1, delay: 1.1 }}
        >
          <div className="rule-x mb-6" />
          <div className="flex flex-col gap-8 sm:flex-row sm:items-end sm:justify-between">
            <dl className="flex gap-12">
              {STATS.map((s) => (
                <div key={s.label}>
                  <dt className="eyebrow mb-2">{s.label}</dt>
                  <dd className="font-display text-3xl text-ink">{s.value}</dd>
                </div>
              ))}
            </dl>

            {/* Marquee of partner arenas */}
            <div className="relative w-full max-w-sm overflow-hidden" aria-hidden>
              <div className="pointer-events-none absolute inset-y-0 left-0 z-10 w-12 bg-gradient-to-r from-canvas to-transparent" />
              <div className="pointer-events-none absolute inset-y-0 right-0 z-10 w-12 bg-gradient-to-l from-canvas to-transparent" />
              <m.div
                className="flex w-max gap-10 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint"
                animate={reduceMotion ? undefined : { x: ["0%", "-50%"] }}
                transition={{ duration: 28, ease: "linear", repeat: Infinity }}
              >
                {[...ARENAS, ...ARENAS].map((a, i) => (
                  <span key={i} className="whitespace-nowrap">{a}</span>
                ))}
              </m.div>
            </div>
          </div>
        </m.div>
      </div>
    </section>
  );
}
