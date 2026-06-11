// ============================================================================
// / — landing. The cinematic hero plus two editorial doorways into the
// product's halves: the booking matrix and the community hub.
// ============================================================================

import Link from "next/link";
import HeroSection from "@/components/HeroSection";

const DOORS = [
  {
    href: "/book",
    eyebrow: "Reserve · आरक्षण",
    title: "The matrix",
    body: "Every hour, every court, priced live. Pick your slot, pay with eSewa or Khalti, done in under a minute.",
    cta: "Open the booking matrix",
  },
  {
    href: "/tournaments",
    eyebrow: "Tournaments · प्रतियोगिता",
    title: "Silverware season",
    body: "Enter a cup or run your own — set the format, the purse, and the deadline, and let the valley's teams come to you.",
    cta: "Browse the competitions",
  },
  {
    href: "/community",
    eyebrow: "Community · समुदाय",
    title: "The valley plays together",
    body: "Short a striker? Post your booking. Looking for a game? Tonight's open calls live here.",
    cta: "See tonight's open games",
  },
];

export default function HomePage() {
  return (
    <main>
      <HeroSection />
      <section className="border-t border-hairline bg-surface">
        <div className="mx-auto grid max-w-6xl sm:grid-cols-3">
          {DOORS.map((d, i) => (
            <Link
              key={d.href}
              href={d.href}
              className={`group flex flex-col justify-between p-10 transition-colors hover:bg-surface-2 sm:p-14 ${
i < 2 ? "sm:border-r sm:border-hairline" : ""
              }`}
            >
              <div>
                <p className="eyebrow mb-4">{d.eyebrow}</p>
                <h2 className="font-display text-4xl tracking-tight text-ink">{d.title}</h2>
                <p className="mt-4 max-w-sm text-sm leading-relaxed text-ink-dim">{d.body}</p>
              </div>
              <p className="mt-10 font-mono text-[0.65rem] uppercase tracking-editorial text-gold">
                {d.cta}{" "}
                <span aria-hidden className="inline-block transition-transform duration-300 group-hover:translate-x-1">
                  →
                </span>
              </p>
            </Link>
          ))}
        </div>
      </section>
    </main>
  );
}
