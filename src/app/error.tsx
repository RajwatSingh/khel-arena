"use client";

// Route-level error boundary. Catches render/data errors in any page segment
// (the layout, Nav, and footer stay mounted) and offers a retry.
import { useEffect } from "react";
import Link from "next/link";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <section className="grain relative flex min-h-screen items-center justify-center overflow-hidden bg-canvas px-6 py-28">
      <div className="max-w-md text-center">
        <p className="eyebrow mb-4">Error &middot; त्रुटि</p>
        <h1 className="font-display text-5xl tracking-tight text-ink">Something went sideways</h1>
        <p className="mt-4 text-sm leading-relaxed text-ink-dim">
          A play broke down on our end. Try again — if it keeps happening, head back home.
        </p>
        <div className="mt-8 flex items-center justify-center gap-5">
          <button
            onClick={reset}
            className="border border-gold/60 px-6 py-3 font-mono text-[0.65rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink"
          >
            Try again
          </button>
          <Link
            href="/"
            className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-ink"
          >
            Go home
          </Link>
        </div>
      </div>
    </section>
  );
}
