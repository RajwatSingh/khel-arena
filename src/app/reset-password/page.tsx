"use client";

// /reset-password — reached via the email link → /auth/callback (which sets a
// recovery session) → here. Sets a new password, then sends the player to
// their profile.
import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { AnimatePresence, m } from "framer-motion";
import { updatePassword } from "@/actions/auth";

const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
const labelClass = "eyebrow mb-2 block";

export default function ResetPasswordPage() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [isPending, startTransition] = useTransition();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (password !== confirm) {
      setError("Those passwords don't match.");
      return;
    }
    startTransition(async () => {
      const res = await updatePassword(password);
      if (res.ok) {
        setDone(true);
        setTimeout(() => router.push("/profile"), 1200);
      } else {
        setError(res.error);
      }
    });
  };

  return (
    <section className="grain relative flex min-h-[calc(100svh-57px)] items-center overflow-hidden bg-canvas px-6 py-28">
      <div className="mx-auto w-full max-w-md">
        <p className="eyebrow mb-4">Reset &middot; पासवर्ड</p>
        <h1 className="font-display text-5xl tracking-tight text-ink">Set a new password</h1>
        <p className="mt-4 text-sm leading-relaxed text-ink-dim">
          Choose a new password for your account. You&rsquo;ll be signed in once it&rsquo;s saved.
        </p>

        <form onSubmit={handleSubmit} className="mt-10 space-y-6">
          <div>
            <label htmlFor="r-password" className={labelClass}>New password</label>
            <input
              id="r-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              placeholder="At least 8 characters"
              className={inputClass}
            />
          </div>
          <div>
            <label htmlFor="r-confirm" className={labelClass}>Confirm password</label>
            <input
              id="r-confirm"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="new-password"
              placeholder="Re-enter it"
              className={inputClass}
            />
          </div>

          <button
            type="submit"
            disabled={isPending || done}
            className="w-full border border-gold/60 px-8 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50"
          >
            {done ? "Saved" : isPending ? "Saving…" : "Save password"}
          </button>

          <AnimatePresence>
            {error && (
              <m.p
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                role="alert"
                className="font-mono text-xs text-ember"
              >
                {error}
              </m.p>
            )}
            {done && (
              <m.p
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                role="status"
                className="font-mono text-xs text-sage"
              >
                Password updated — taking you to your profile…
              </m.p>
            )}
          </AnimatePresence>
        </form>
      </div>
    </section>
  );
}
