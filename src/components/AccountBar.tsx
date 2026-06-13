"use client";

// Signed-in bar above the profile studio: handle, a Settings disclosure for
// email/password changes, and Log out. Signing out refreshes the route so
// /profile falls back to the AuthPanel.
import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { AnimatePresence, m } from "framer-motion";
import { signOut, updateEmail, updatePassword } from "@/actions/auth";
import { AUTH_EVENT } from "@/components/Nav";

const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
const btnClass =
  "border border-hairline-2 px-4 py-2.5 font-mono text-[0.6rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-gold hover:text-gold disabled:opacity-50";

type Msg = { ok: boolean; text: string } | null;

export default function AccountBar({ username }: { username: string }) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [open, setOpen] = useState(false);

  const [email, setEmail] = useState("");
  const [emailMsg, setEmailMsg] = useState<Msg>(null);
  const [password, setPassword] = useState("");
  const [pwMsg, setPwMsg] = useState<Msg>(null);

  const handleSignOut = () =>
    startTransition(async () => {
      await signOut();
      window.dispatchEvent(new Event(AUTH_EVENT));
      router.refresh();
    });

  const submitEmail = (e: React.FormEvent) => {
    e.preventDefault();
    setEmailMsg(null);
    startTransition(async () => {
      const res = await updateEmail(email);
      if (res.ok) {
        setEmailMsg({ ok: true, text: "Confirmation sent — click the link in your new inbox to finish." });
        setEmail("");
      } else {
        setEmailMsg({ ok: false, text: res.error });
      }
    });
  };

  const submitPassword = (e: React.FormEvent) => {
    e.preventDefault();
    setPwMsg(null);
    startTransition(async () => {
      const res = await updatePassword(password);
      if (res.ok) {
        setPwMsg({ ok: true, text: "Password updated." });
        setPassword("");
      } else {
        setPwMsg({ ok: false, text: res.error });
      }
    });
  };

  return (
    <div className="border-b border-hairline bg-canvas">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <p className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
          Signed in as <span className="text-ink">@{username}</span>
        </p>
        <div className="flex items-center gap-3">
          <button
            onClick={() => setOpen((o) => !o)}
            aria-expanded={open}
            aria-controls="account-settings"
            className="border border-hairline-2 px-4 py-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-gold hover:text-gold"
          >
            {open ? "Close" : "Settings"}
          </button>
          <button
            onClick={handleSignOut}
            disabled={isPending}
            className="border border-hairline-2 px-4 py-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-ember hover:text-ember disabled:opacity-50"
          >
            {isPending ? "Logging out…" : "Log out"}
          </button>
        </div>
      </div>

      <AnimatePresence>
        {open && (
          <m.div
            id="account-settings"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
            className="overflow-hidden border-t border-hairline"
          >
            <div className="mx-auto grid max-w-6xl gap-8 px-6 py-7 sm:grid-cols-2">
              {/* Change email */}
              <form onSubmit={submitEmail} className="space-y-3">
                <p className="eyebrow">Change email</p>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  autoComplete="email"
                  placeholder="new@email.com"
                  className={inputClass}
                />
                <button type="submit" disabled={isPending || !email.trim()} className={btnClass}>
                  Send confirmation
                </button>
                {emailMsg && (
                  <p className={`font-mono text-xs ${emailMsg.ok ? "text-sage" : "text-ember"}`}>
                    {emailMsg.text}
                  </p>
                )}
              </form>

              {/* Change password */}
              <form onSubmit={submitPassword} className="space-y-3">
                <p className="eyebrow">Change password</p>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="new-password"
                  placeholder="New password (8+ characters)"
                  className={inputClass}
                />
                <button type="submit" disabled={isPending || !password} className={btnClass}>
                  Update password
                </button>
                {pwMsg && (
                  <p className={`font-mono text-xs ${pwMsg.ok ? "text-sage" : "text-ember"}`}>
                    {pwMsg.text}
                  </p>
                )}
              </form>
            </div>
          </m.div>
        )}
      </AnimatePresence>
    </div>
  );
}
