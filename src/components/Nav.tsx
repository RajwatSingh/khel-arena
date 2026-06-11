"use client";

// Shared site navigation — persists across all routes.
import Link from "next/link";
import { usePathname } from "next/navigation";

const LINKS = [
  { href: "/book", label: "Book" },
  { href: "/my-bookings", label: "My Bookings" },
  { href: "/tournaments", label: "Tournaments" },
  { href: "/teams", label: "Teams" },
  { href: "/community", label: "Community" },
  { href: "/profile", label: "Profile" },
];

export default function Nav() {
  const pathname = usePathname();
  return (
    <header className="sticky top-0 z-40 border-b border-hairline bg-canvas/90 backdrop-blur-md">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <Link href="/" className="font-display text-xl tracking-tight text-ink">
          Khel<span className="text-gold">.</span>
        </Link>

        <nav className="flex items-center gap-5 font-mono sm:gap-7 text-[0.65rem] uppercase tracking-editorial">
          {LINKS.map((l) => {
            const active = pathname.startsWith(l.href);
            return (
              <Link
                key={l.href}
                href={l.href}
                aria-current={active ? "page" : undefined}
                className={`relative pb-1 transition-colors ${
                  active
                    ? "text-gold after:absolute after:inset-x-0 after:bottom-0 after:h-px after:bg-gold"
                    : "text-ink-dim hover:text-ink"
                }`}
              >
                {l.label}
              </Link>
            );
          })}
          <span className="hidden items-center gap-2 text-sage sm:flex">
            <span className="relative flex h-1.5 w-1.5">
              <span className="absolute h-full w-full animate-ping rounded-full bg-sage opacity-60" />
              <span className="h-1.5 w-1.5 rounded-full bg-sage" />
            </span>
            Kathmandu · Live
          </span>
        </nav>
      </div>
    </header>
  );
}
