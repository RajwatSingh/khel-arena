"use client";

// ============================================================================
// FutsalPitchPicker — choose your position by tapping the court.
// A top-down futsal pitch rendered in the site's hairline language; each of
// the five futsal roles maps to its real zone on the floor (Goleiro in the
// goal mouth, Fixo deep, Ala on the flanks, Pivô high, Universal at the
// pivot of the center circle). The chosen zone lights gold.
// ============================================================================

import { m } from "framer-motion";
import type { FutsalPosition } from "@/lib/types";

interface Zone {
  pos: FutsalPosition;
  cx: number;
  cy: number;
  blurb: string;
}

// viewBox is 320 (wide) × 200 (tall): a futsal court seen from above,
// our goal on the left, attack to the right.
const ZONES: Zone[] = [
  { pos: "Goleiro", cx: 24, cy: 100, blurb: "Last line. Sweeper-keeper, starts the play." },
  { pos: "Fixo", cx: 90, cy: 100, blurb: "The anchor. Reads the game, holds the back." },
  { pos: "Ala", cx: 175, cy: 40, blurb: "Engine on the flank. Up and down all game." },
  { pos: "Pivô", cx: 270, cy: 100, blurb: "Target up top. Backs in, holds, finishes." },
  { pos: "Universal", cx: 160, cy: 100, blurb: "Plays every role. Total futsal." },
];

export default function FutsalPitchPicker({
  value,
  onChange,
}: {
  value: FutsalPosition | null;
  onChange: (pos: FutsalPosition) => void;
}) {
  const active = ZONES.find((z) => z.pos === value);

  return (
    <div>
      <svg
        viewBox="0 0 320 200"
        className="w-full select-none"
        role="group"
        aria-label="Pick your position on the court"
      >
        {/* Floor */}
        <rect x="6" y="6" width="308" height="188" fill="var(--surface-2)" stroke="var(--hairline-2)" strokeWidth="1.5" />
        {/* Halfway line + center circle */}
        <line x1="160" y1="6" x2="160" y2="194" stroke="var(--hairline-2)" strokeWidth="1.2" />
        <circle cx="160" cy="100" r="34" fill="none" stroke="var(--hairline-2)" strokeWidth="1.2" />
        {/* Penalty arcs */}
        <path d="M 6 56 A 56 56 0 0 1 6 144" fill="none" stroke="var(--hairline-2)" strokeWidth="1.2" />
        <path d="M 314 56 A 56 56 0 0 0 314 144" fill="none" stroke="var(--hairline-2)" strokeWidth="1.2" />
        {/* Goals */}
        <rect x="2" y="84" width="4" height="32" fill="var(--gold)" opacity="0.5" />
        <rect x="314" y="84" width="4" height="32" fill="var(--hairline-2)" />

        {/* Zone taps */}
        {ZONES.map((z) => {
          const selected = z.pos === value;
          return (
            <g
              key={z.pos}
              role="radio"
              aria-checked={selected}
              aria-label={z.pos}
              tabIndex={0}
              className="cursor-pointer"
              onClick={() => onChange(z.pos)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onChange(z.pos);
                }
              }}
            >
              {/* generous invisible hit area */}
              <circle cx={z.cx} cy={z.cy} r="26" fill="transparent" />
              {selected && (
                <m.circle
                  layoutId="pos-halo"
                  cx={z.cx}
                  cy={z.cy}
                  r="20"
                  fill="var(--gold-soft)"
                  stroke="var(--gold)"
                  strokeWidth="1.5"
                />
              )}
              <circle
                cx={z.cx}
                cy={z.cy}
                r="7"
                fill={selected ? "var(--gold)" : "var(--surface)"}
                stroke={selected ? "var(--gold)" : "var(--hairline-2)"}
                strokeWidth="1.5"
                className="transition-colors"
              />
              <text
                x={z.cx}
                y={z.cy - 16}
                textAnchor="middle"
                className="pointer-events-none"
                style={{
                  fontFamily: "var(--font-mono)",
                  fontSize: 8,
                  letterSpacing: "0.12em",
                  textTransform: "uppercase",
                  fill: selected ? "var(--gold)" : "var(--ink-faint)",
                }}
              >
                {z.pos}
              </text>
            </g>
          );
        })}
      </svg>

      <p className="mt-3 min-h-[2.5rem] text-sm leading-relaxed text-ink-dim">
        {active ? (
          <>
            <span className="font-display text-ink">{active.pos}.</span> {active.blurb}
          </>
        ) : (
          "Tap a spot on the floor to set your position."
        )}
      </p>
    </div>
  );
}
