"use client";

// Shared site navigation — persists across all routes.
// The profile link is an avatar: the player's photo when they have one,
// their initial when they don't, and a silhouette when signed out. It
// refetches on route changes and on the "khel:auth" event fired by the
// sign-in / sign-out flows, so it tracks the session without a reload.
import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
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
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={identity.avatarUrl}
        alt=""
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

  const profileActive = pathname.startsWith("/profile");

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
          <Link
            href="/profile"
            aria-label="Profile"
            aria-current={profileActive ? "page" : undefined}
            title={identity ? `@${identity.username}` : "Profile"}
            className="group"
          >
            <ProfileAvatar identity={identity} active={profileActive} />
          </Link>
        </nav>
      </div>
    </header>
  );
}
