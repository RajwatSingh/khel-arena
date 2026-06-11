"use client";

// ============================================================================
// PitchLines — futsal court markings as architecture.
// Two exports:
//   <PitchBackdrop /> — halfway line, center circle, and penalty arcs drawn
//     in hairlines behind a section, animated like a groundskeeper chalking
//     the court at dawn.
//   <PitchDivider /> — a section rule built from the halfway line + center
//     circle motif, replacing plain <hr> moments across the site.
// ============================================================================

import { m, useReducedMotion } from "framer-motion";

const ease = [0.22, 1, 0.36, 1] as const;

export function PitchBackdrop() {
  const reduceMotion = useReducedMotion();
  const draw = (delay: number) => ({
    initial: { pathLength: reduceMotion ? 1 : 0, opacity: 0 },
    animate: { pathLength: 1, opacity: 1 },
    transition: { pathLength: { duration: 1.8, ease, delay }, opacity: { duration: 0.4, delay } },
  });

  return (
    <svg
      aria-hidden
      className="pointer-events-none absolute inset-0 h-full w-full"
      viewBox="0 0 1200 800"
      preserveAspectRatio="xMidYMid slice"
      fill="none"
      stroke="var(--hairline-2)"
      strokeOpacity={0.55}
      strokeWidth={1.5}
    >
      {/* Halfway line */}
      <m.line x1="600" y1="0" x2="600" y2="800" {...draw(0.2)} />
      {/* Center circle + spot */}
      <m.circle cx="600" cy="400" r="170" {...draw(0.5)} />
      <m.circle
        cx="600"
        cy="400"
        r="4"
        fill="var(--gold)"
        stroke="none"
        initial={{ opacity: 0, scale: 0 }}
        animate={{ opacity: 0.7, scale: 1 }}
        transition={{ duration: 0.5, delay: 2.1 }}
      />
      {/* Penalty arcs — 6 m semicircles off each goal line */}
      <m.path d="M 0 215 A 185 185 0 0 1 0 585" {...draw(0.8)} />
      <m.path d="M 1200 215 A 185 185 0 0 0 1200 585" {...draw(0.8)} />
      {/* Penalty spots */}
      <m.circle cx="185" cy="400" r="3" fill="var(--hairline-2)" stroke="none"
        initial={{ opacity: 0 }} animate={{ opacity: 0.8 }} transition={{ delay: 2.3 }} />
      <m.circle cx="1015" cy="400" r="3" fill="var(--hairline-2)" stroke="none"
        initial={{ opacity: 0 }} animate={{ opacity: 0.8 }} transition={{ delay: 2.3 }} />
    </svg>
  );
}

export function PitchDivider({ className = "" }: { className?: string }) {
  return (
    <div aria-hidden className={`flex items-center ${className}`}>
      <span className="h-px flex-1 bg-gradient-to-r from-transparent to-hairline-2" />
      <span className="mx-3 block h-5 w-5 rounded-full border border-hairline-2" />
      <span className="h-px flex-1 bg-gradient-to-l from-transparent to-hairline-2" />
    </div>
  );
}
