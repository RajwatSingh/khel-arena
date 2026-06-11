import type { Config } from "tailwindcss";

export default {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        canvas: "var(--canvas)",
        surface: "var(--surface)",
        "surface-2": "var(--surface-2)",
        hairline: "var(--hairline)",
        "hairline-2": "var(--hairline-2)",
        ink: "var(--ink)",
        "ink-dim": "var(--ink-dim)",
        "ink-faint": "var(--ink-faint)",
        gold: "var(--gold)",
        "gold-bright": "var(--gold-bright)",
        sage: "var(--sage)",
        ember: "var(--ember)",
      },
      fontFamily: {
        display: ["var(--font-display)", "serif"],
        sans: ["var(--font-sans)", "system-ui", "sans-serif"],
        mono: ["var(--font-mono)", "monospace"],
      },
      letterSpacing: {
        editorial: "0.28em",
      },
    },
  },
  plugins: [],
} satisfies Config;
