// /book/confirmation loading skeleton — mirrors the centered verdict layout
// (eyebrow, headline, blurb, receipt chip, and the two CTAs) so it doesn't
// inherit the booking-grid skeleton from the parent segment.
export default function ConfirmationLoading() {
  return (
    <main className="grain relative flex min-h-[calc(100svh-57px)] items-center bg-canvas">
      <div className="mx-auto flex w-full max-w-2xl flex-col items-center px-6 py-24">
        {/* eyebrow */}
        <div className="mb-6 h-2.5 w-36 animate-pulse bg-hairline-2" />

        {/* headline (two lines) */}
        <div className="h-12 w-[26rem] max-w-full animate-pulse bg-surface-2" />
        <div className="mt-3 h-12 w-64 max-w-full animate-pulse bg-surface-2" />

        {/* blurb */}
        <div className="mt-8 h-3 w-80 max-w-full animate-pulse bg-hairline-2" />
        <div className="mt-2 h-3 w-72 max-w-full animate-pulse bg-hairline-2" />

        {/* receipt chip */}
        <div className="mt-8 h-10 w-48 animate-pulse bg-surface" />

        {/* CTAs */}
        <div className="mt-12 flex items-center justify-center gap-8">
          <div className="h-12 w-44 animate-pulse bg-surface-2" />
          <div className="h-3 w-32 animate-pulse bg-hairline-2" />
        </div>
      </div>
    </main>
  );
}
