// Route-level loading fallback — shown while a page's server data resolves.
// A quiet editorial skeleton: eyebrow, heading, and a grid of shimmer tiles,
// sitting on the same grain canvas as the real pages.
export default function Loading() {
  return (
    <section className="grain relative min-h-screen overflow-hidden bg-canvas py-28">
      <div className="mx-auto max-w-6xl px-6">
        {/* Header */}
        <div className="mb-16">
          <div className="mb-4 h-2.5 w-28 animate-pulse bg-hairline-2" />
          <div className="h-12 w-80 max-w-full animate-pulse bg-surface-2" />
        </div>

        {/* Body tiles */}
        <div className="grid grid-cols-2 gap-px bg-hairline sm:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div
              key={i}
              className="h-28 animate-pulse bg-surface"
              style={{ animationDelay: `${i * 80}ms` }}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
