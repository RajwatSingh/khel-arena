// ============================================================================
// /book/confirmation — where the player lands after the gateway round-trip.
// The verdict shown here was decided by the server-side verification in the
// callback routes; this page only renders the outcome.
// ============================================================================

import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "Booking confirmation — Khel Arena" };

const PROVIDER_LABEL: Record<string, string> = { esewa: "eSewa", khalti: "Khalti" };

export default async function ConfirmationPage({
  searchParams,
}: {
  searchParams: Promise<{ status?: string; provider?: string; ref?: string; demo?: string }>;
}) {
  const { status, provider, ref, demo } = await searchParams;
  const success = status === "success";
  const providerName = PROVIDER_LABEL[provider ?? ""] ?? "the gateway";

  return (
    <main className="grain relative flex min-h-[calc(100svh-57px)] items-center bg-canvas">
      <div className="mx-auto w-full max-w-2xl px-6 py-24 text-center">
        <p className="eyebrow mb-6">{success ? "Payment verified" : "Payment incomplete"}</p>

        <h1 className="font-display text-5xl leading-tight tracking-tight text-ink sm:text-6xl">
          {success ? (
            <>
              You&rsquo;re on the <em className="not-italic text-gold">pitch.</em>
            </>
          ) : (
            <>That didn&rsquo;t go through.</>
          )}
        </h1>

        <p className="mx-auto mt-6 max-w-md text-sm leading-relaxed text-ink-dim">
          {success
            ? `${providerName} confirmed your payment and the court is locked in. Bring your boots — the slot is yours.`
            : `${providerName} did not complete this payment, so the court was not confirmed. Your slot is still held briefly — you can retry, or pick another time.`}
        </p>

        {success && ref && (
          <p className="mt-8 inline-block border border-hairline-2 bg-surface px-6 py-3 font-mono text-xs tabular-nums text-ink">
            Receipt&ensp;<span className="text-gold">{ref}</span>
          </p>
        )}
        {demo && (
          <p className="mt-4 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
            Demo mode — no money moved
          </p>
        )}

        <div className="mt-12 flex items-center justify-center gap-8">
          <Link
            href="/book"
            className="group inline-flex items-center gap-3 border border-gold/60 px-8 py-4 font-mono text-[0.7rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink"
          >
            {success ? "Book another slot" : "Try again"}
            <span aria-hidden className="transition-transform duration-300 group-hover:translate-x-1">→</span>
          </Link>
          <Link
            href="/community"
            className="font-mono text-[0.7rem] uppercase tracking-editorial text-ink-dim underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-ink"
          >
            {success ? "Find two more players" : "Browse open games"}
          </Link>
        </div>
      </div>
    </main>
  );
}
