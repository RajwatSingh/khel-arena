"use client";

// Shared site navigation — persists across all routes.
// Desktop shows the links inline; on small screens they collapse into a
// hamburger drawer so the bar doesn't crowd. The profile avatar stays visible
// at all sizes: the player's photo when they have one, their initial when they
// don't, and a silhouette when signed out. Identity refetches on route changes
// and on the "khel:auth" event from the sign-in / sign-out flows.
import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { AnimatePresence, m } from "framer-motion";
import AvatarImage from "@/components/AvatarImage";
import { getNavIdentity, type NavIdentity } from "@/actions/profile";

const LINKS = [
  { href: "/book", label: "Book" },
  { href: "/my-bookings", label: "My Bookings" },
  { href: "/tournaments", label: "Tournaments" },
  { href: "/teams", label: "Teams" },
  { href: "/community", label: "Community" },
];

export const AUTH_EVENT = "khel:auth";

function ProfileAvatar({ identity, active }: { identity: NavIdentity | null; active: boolean }) {
  const ring = active
    ? "border-gold"
    : "border-hairline-2 group-hover:border-ink";

  if (identity?.avatarUrl) {
    return (
      <AvatarImage
        src={identity.avatarUrl}
        size={28}
        className={`h-7 w-7 rounded-full border object-cover transition-colors ${ring}`}
      />
    );
  }

  if (identity) {
    return (
      <span
        className={`flex h-7 w-7 items-center justify-center rounded-full border bg-surface-2 font-mono text-[0.65rem] uppercase text-ink transition-colors ${ring}`}
      >
        {identity.username.charAt(0)}
      </span>
    );
  }

  return (
    <span
      className={`flex h-7 w-7 items-center justify-center rounded-full border text-ink-dim transition-colors group-hover:text-ink ${ring}`}
    >
      <svg aria-hidden viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5}>
        <circle cx="12" cy="8.5" r="3.5" />
        <path d="M5 19.5c1.4-3 4-4.5 7-4.5s5.6 1.5 7 4.5" strokeLinecap="round" />
      </svg>
    </span>
  );
}

export default function Nav() {
  const pathname = usePathname();
  const [identity, setIdentity] = useState<NavIdentity | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const refresh = () => {
      getNavIdentity()
        .then((id) => {
          if (!cancelled) setIdentity(id);
        })
        .catch(() => {});
    };
    refresh();
    window.addEventListener(AUTH_EVENT, refresh);
    return () => {
      cancelled = true;
      window.removeEventListener(AUTH_EVENT, refresh);
    };
  }, [pathname]);

  // Close the mobile drawer on navigation.
  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  // Close the mobile drawer on Escape.
  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [menuOpen]);

  const profileActive = pathname.startsWith("/profile");

  return (
    <header className="sticky top-0 z-40 border-b border-hairline bg-canvas/90 backdrop-blur-md">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <Link href="/" className="font-display text-xl tracking-tight text-ink">
          Khel<span className="text-gold">.</span>
        </Link>

        <div className="flex items-center gap-5 sm:gap-7">
          {/* Desktop links */}
          <nav className="hidden items-center gap-5 font-mono text-[0.65rem] uppercase tracking-editorial sm:flex sm:gap-7">
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
          </nav>

          {/* Profile avatar — always visible */}
          <Link
            href="/profile"
            aria-label="Profile"
            aria-current={profileActive ? "page" : undefined}
            title={identity ? `@${identity.username}` : "Profile"}
            className="group"
          >
            <ProfileAvatar identity={identity} active={profileActive} />
          </Link>

          {/* Hamburger — mobile only */}
          <button
            type="button"
            onClick={() => setMenuOpen((o) => !o)}
            aria-label={menuOpen ? "Close menu" : "Open menu"}
            aria-expanded={menuOpen}
            aria-controls="mobile-nav"
            className="-mr-1 p-1 text-ink-dim transition-colors hover:text-ink sm:hidden"
          >
            <svg aria-hidden viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round">
              {menuOpen ? (
                <>
                  <path d="M6 6l12 12" />
                  <path d="M18 6L6 18" />
                </>
              ) : (
                <>
                  <path d="M4 7h16" />
                  <path d="M4 12h16" />
                  <path d="M4 17h16" />
                </>
              )}
            </svg>
          </button>
        </div>
      </div>

      {/* Mobile drawer */}
      <AnimatePresence>
        {menuOpen && (
          <m.nav
            id="mobile-nav"
            key="mobile-nav"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
            className="overflow-hidden border-t border-hairline bg-canvas/95 backdrop-blur-md sm:hidden"
          >
            <div className="mx-auto flex max-w-6xl flex-col px-6">
              {LINKS.map((l) => {
                const active = pathname.startsWith(l.href);
                return (
                  <Link
                    key={l.href}
                    href={l.href}
                    aria-current={active ? "page" : undefined}
                    className={`border-b border-hairline py-4 font-mono text-[0.7rem] uppercase tracking-editorial transition-colors last:border-b-0 ${
                      active ? "text-gold" : "text-ink-dim hover:text-ink"
                    }`}
                  >
                    {l.label}
                  </Link>
                );
              })}
            </div>
          </m.nav>
        )}
      </AnimatePresence>
    </header>
  );
}
