// Shared editorial UI tokens — the form field + label styles and the list-row
// motion variant reused across the hub surfaces (teams, tournaments, …).

export const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
export const labelClass = "eyebrow mb-2 block";

export const ease = [0.22, 1, 0.36, 1] as const;

/** Staggered fade-up for editorial list rows; `custom={i}` drives the delay. */
export const rowAnim = {
  hidden: { opacity: 0, y: 14 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.55, ease, delay: i * 0.07 },
  }),
};
