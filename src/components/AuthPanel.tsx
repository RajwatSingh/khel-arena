"use client";

// ============================================================================
// AuthPanel — the signed-out gate on /profile.
// Toggles between Sign in and Create account, both backed by the auth server
// actions. On success it refreshes the route so the page re-renders with the
// player's real card. If the Supabase project requires email confirmation,
// sign-up surfaces a "check your inbox" note instead of logging straight in.
// ============================================================================

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { AnimatePresence, m } from "framer-motion";
import { requestPasswordReset, signIn, signUp } from "@/actions/auth";
import { AUTH_EVENT } from "@/components/Nav";
import type { AccountType } from "@/lib/types";

type Mode = "signin" | "signup" | "reset";

const ACCOUNT_TYPES: { value: AccountType; title: string; blurb: string }[] = [
  { value: "player", title: "Player", blurb: "Book courts, join games, build your player card." },
  { value: "futsal_owner", title: "Futsal owner", blurb: "List your arena, set hours and court prices." },
];

const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
const labelClass = "eyebrow mb-2 block";

export default function AuthPanel() {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>("signin");
  const [isPending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [fullName, setFullName] = useState("");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [accountType, setAccountType] = useState<AccountType>("player");

  const switchMode = (next: Mode) => {
    setMode(next);
    setError(null);
    setNotice(null);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setNotice(null);
    startTransition(async () => {
      if (mode === "reset") {
        const res = await requestPasswordReset(email);
        if (res.ok) {
          setNotice("If that email has an account, a reset link is on its way. Check your inbox.");
          setMode("signin");
        } else {
          setError(res.error);
        }
        return;
      }
      if (mode === "signin") {
        const res = await signIn({ email, password });
        if (res.ok) {
          window.dispatchEvent(new Event(AUTH_EVENT));
          router.refresh();
        } else setError(res.error);
        return;
      }
      if (password !== confirmPassword) {
        setError("Those passwords don't match.");
        return;
      }
      const res = await signUp({ fullName, username, email, password, accountType });
      if (!res.ok) {
        setError(res.error);
        return;
      }
      if (res.data.needsConfirmation) {
        setNotice("Account created. Check your inbox to confirm your email, then sign in.");
        setMode("signin");
        setPassword("");
        setConfirmPassword("");
      } else {
        window.dispatchEvent(new Event(AUTH_EVENT));
        router.refresh();
      }
    });
  };

  return (
    <section className="relative bg-canvas py-28">
      <div className="mx-auto max-w-md px-6">
        <p className="eyebrow mb-4">Profile · खेलाडी</p>
        <h2 className="font-display text-5xl tracking-tight text-ink">
          {mode === "signin" ? "Welcome back" : mode === "reset" ? "Reset password" : "Join Khel"}
        </h2>
        <p className="mt-4 text-sm leading-relaxed text-ink-dim">
          {mode === "reset"
            ? "Enter your email and we'll send a link to set a new password."
            : mode === "signin"
              ? "Sign in to manage your bookings, teams, and player card."
              : accountType === "futsal_owner"
                ? "Create an owner account to list your futsal, set hours, and manage prices."
                : "Create an account to book courts and build your player card."}
        </p>

        {/* Mode toggle (hidden during password reset) */}
        {mode === "reset" ? (
          <button
            type="button"
            onClick={() => switchMode("signin")}
            className="mt-10 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-ink"
          >
            ← Back to sign in
          </button>
        ) : (
          <div className="mt-10 flex border border-hairline-2" role="tablist" aria-label="Auth mode">
            {(["signin", "signup"] as const).map((m_) => (
              <button
                key={m_}
                role="tab"
                aria-selected={mode === m_}
                onClick={() => switchMode(m_)}
                className={`flex-1 px-4 py-3 font-mono text-[0.62rem] uppercase tracking-editorial transition-colors ${
                  mode === m_ ? "bg-ink text-canvas" : "text-ink-dim hover:text-ink"
                }`}
              >
                {m_ === "signin" ? "Sign in" : "Create account"}
              </button>
            ))}
          </div>
        )}

        <form onSubmit={handleSubmit} className="mt-8 space-y-6">
          {mode === "signup" && (
            <>
              <div role="radiogroup" aria-label="Account type" className="space-y-0">
                <span className={labelClass}>I am a…</span>
                <div className="divide-y divide-hairline-2 border border-hairline-2">
                  {ACCOUNT_TYPES.map((t) => {
                    const selected = accountType === t.value;
                    return (
                      <button
                        key={t.value}
                        type="button"
                        role="radio"
                        aria-checked={selected}
                        onClick={() => setAccountType(t.value)}
                        className={`flex w-full items-start gap-3 px-4 py-3 text-left transition-colors ${
                          selected ? "bg-surface-2" : "hover:bg-surface"
                        }`}
                      >
                        <span
                          aria-hidden
                          className={`mt-0.5 block h-3 w-3 shrink-0 rounded-full border ${
                            selected ? "border-gold bg-gold" : "border-hairline-2"
                          }`}
                        />
                        <span>
                          <span
                            className={`block font-mono text-[0.62rem] uppercase tracking-editorial ${
                              selected ? "text-ink" : "text-ink-dim"
                            }`}
                          >
                            {t.title}
                          </span>
                          <span className="mt-1 block text-xs leading-relaxed text-ink-faint">
                            {t.blurb}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
              <div>
                <label htmlFor="a-name" className={labelClass}>Full name</label>
                <input
                  id="a-name"
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  autoComplete="name"
                  placeholder="Sajan Maharjan"
                  className={inputClass}
                />
              </div>
              <div>
                <label htmlFor="a-username" className={labelClass}>Username</label>
                <input
                  id="a-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value.toLowerCase())}
                  autoComplete="username"
                  placeholder="sajan_ktm"
                  className={inputClass}
                />
                <p className="mt-2 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
                  3–24 chars · a–z, 0–9, _
                </p>
              </div>
            </>
          )}

          <div>
            <label htmlFor="a-email" className={labelClass}>Email</label>
            <input
              id="a-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              placeholder="you@example.com"
              className={inputClass}
            />
          </div>

          {mode !== "reset" && (
            <div>
              <label htmlFor="a-password" className={labelClass}>Password</label>
              <input
                id="a-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete={mode === "signin" ? "current-password" : "new-password"}
                placeholder={mode === "signup" ? "At least 8 characters" : "••••••••"}
                className={inputClass}
              />
              {mode === "signin" && (
                <button
                  type="button"
                  onClick={() => switchMode("reset")}
                  className="mt-2 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint underline decoration-hairline-2 underline-offset-4 transition-colors hover:text-ink"
                >
                  Forgot password?
                </button>
              )}
            </div>
          )}

          {mode === "signup" && (
            <div>
              <label htmlFor="a-confirm" className={labelClass}>Confirm password</label>
              <input
                id="a-confirm"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                autoComplete="new-password"
                placeholder="Re-enter your password"
                className={inputClass}
              />
            </div>
          )}

          <button
            type="submit"
            disabled={isPending}
            className="w-full border border-gold/60 px-8 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50"
          >
            {isPending
              ? mode === "signin"
                ? "Signing in…"
                : mode === "reset"
                  ? "Sending…"
                  : "Creating…"
              : mode === "signin"
                ? "Sign in"
                : mode === "reset"
                  ? "Send reset link"
                  : "Create account"}
          </button>

          <AnimatePresence>
            {error && (
              <m.p
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className="font-mono text-xs text-ember"
                role="alert"
              >
                {error}
              </m.p>
            )}
            {notice && (
              <m.p
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className="font-mono text-xs text-sage"
                role="status"
              >
                {notice}
              </m.p>
            )}
          </AnimatePresence>
        </form>
      </div>
    </section>
  );
}
