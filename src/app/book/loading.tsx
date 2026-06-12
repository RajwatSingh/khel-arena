// /book loading skeleton — mirrors BookingMatrix's structure (court selector,
// 7-day date strip, legend, and the hourly slot grid) so the layout doesn't
// shift when the live availability arrives.
export default function BookLoading() {
  return (
    <section className="grain relative overflow-hidden bg-canvas py-28">
      <div className="relative mx-auto max-w-6xl px-6">
        {/* Section head */}
        <div className="mb-16">
          <div className="mb-4 h-2.5 w-32 animate-pulse bg-hairline-2" />
          <div className="h-12 w-72 max-w-full animate-pulse bg-surface-2" />
        </div>

        {/* Court selector */}
        <div className="mb-10 flex flex-wrap gap-px bg-hairline">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="flex-1 basis-48 space-y-2 bg-canvas px-5 py-4">
              <div className="h-5 w-32 animate-pulse bg-surface-2" />
              <div className="h-2.5 w-40 animate-pulse bg-hairline-2" />
            </div>
          ))}
        </div>

        {/* Date strip */}
        <div className="mb-2 grid grid-cols-7 border-y border-hairline">
          {Array.from({ length: 7 }).map((_, i) => (
            <div key={i} className="flex flex-col items-center gap-2 py-5">
              <div className="h-2.5 w-8 animate-pulse bg-hairline-2" />
              <div className="h-7 w-9 animate-pulse bg-surface-2" />
            </div>
          ))}
        </div>

        {/* Legend */}
        <div className="mb-8 flex flex-wrap gap-8 py-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-2.5 w-20 animate-pulse bg-hairline-2" />
          ))}
        </div>

        {/* The slot grid */}
        <div className="grid grid-cols-2 gap-px bg-hairline sm:grid-cols-4">
          {Array.from({ length: 16 }).map((_, i) => (
            <div
              key={i}
              className="h-24 animate-pulse bg-surface"
              style={{ animationDelay: `${i * 40}ms` }}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
