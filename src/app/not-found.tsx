// 404 — shown for unknown routes and notFound() calls.
import Link from "next/link";

export default function NotFound() {
  return (
    <section className="grain relative flex min-h-screen items-center justify-center overflow-hidden bg-canvas px-6 py-28">
      <div className="max-w-md text-center">
        <p className="eyebrow mb-4">404 &middot; हराएको</p>
        <h1 className="font-display text-6xl tracking-tight text-ink">Off the pitch</h1>
        <p className="mt-4 text-sm leading-relaxed text-ink-dim">
          This page isn&rsquo;t on the field. Check the address, or head back to book a court.
        </p>
        <div className="mt-8 flex items-center justify-center gap-5">
          <Link
            href="/"
            className="border border-gold/60 px-6 py-3 font-mono text-[0.65rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink"
          >
            Back home
          </Link>
          <Link
            href="/book"
            className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-ink"
          >
            Book a court
          </Link>
        </div>
      </div>
    </section>
  );
}
