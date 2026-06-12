"use client";

// Thin signed-in bar above the profile studio: shows the player's handle and
// a Log out button. Signing out refreshes the route so /profile falls back to
// the AuthPanel.
import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { signOut } from "@/actions/auth";
import { AUTH_EVENT } from "@/components/Nav";

export default function AccountBar({ username }: { username: string }) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  const handleSignOut = () =>
    startTransition(async () => {
      await signOut();
      window.dispatchEvent(new Event(AUTH_EVENT));
      router.refresh();
    });

  return (
    <div className="border-b border-hairline bg-canvas">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <p className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
          Signed in as <span className="text-ink">@{username}</span>
        </p>
        <button
          onClick={handleSignOut}
          disabled={isPending}
          className="border border-hairline-2 px-4 py-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-ember hover:text-ember disabled:opacity-50"
        >
          {isPending ? "Logging out…" : "Log out"}
        </button>
      </div>
    </div>
  );
}
