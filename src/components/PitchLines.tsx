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

      {/* Match ball — drops in and bounces to rest on the center spot while
          the groundskeeper is still chalking the lines. */}
      <m.g
        style={{ transformBox: "fill-box", transformOrigin: "center" }}
        initial={reduceMotion ? { opacity: 0 } : { opacity: 0, y: -460 }}
        animate={
          reduceMotion
            ? { opacity: 1 }
            : { opacity: 1, y: [-460, 0, -150, 0, -55, 0, -18, 0] }
        }
        transition={
          reduceMotion
            ? { duration: 0.6, delay: 1.4 }
            : {
                opacity: { duration: 0.25, delay: 1.0 },
                y: {
                  duration: 1.6,
                  delay: 1.0,
                  times: [0, 0.32, 0.5, 0.64, 0.75, 0.84, 0.92, 1],
                  ease: ["easeIn", "easeOut", "easeIn", "easeOut", "easeIn", "easeOut", "easeIn"],
                },
              }
        }
      >
        <circle
          cx="600" cy="400" r="20"
          fill="var(--canvas)"
          stroke="var(--ink)" strokeOpacity={0.85} strokeWidth={1.5}
        />
        {/* Center pentagon */}
        <path
          d="M600 393 L606.7 397.8 L604.1 405.7 L595.9 405.7 L593.3 397.8 Z"
          fill="var(--ink)" fillOpacity={0.85} stroke="none"
        />
        {/* Seams radiating from the pentagon */}
        <g stroke="var(--ink)" strokeOpacity={0.85} strokeWidth={1.2}>
          <line x1="600" y1="393" x2="600" y2="381" />
          <line x1="606.7" y1="397.8" x2="618" y2="394" />
          <line x1="604.1" y1="405.7" x2="611" y2="415.5" />
          <line x1="595.9" y1="405.7" x2="589" y2="415.5" />
          <line x1="593.3" y1="397.8" x2="582" y2="394" />
        </g>
      </m.g>
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
