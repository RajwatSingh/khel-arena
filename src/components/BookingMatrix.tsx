"use client";

// ============================================================================
// BookingMatrix — the signature surface.
// The grid is drawn like a stadium elevation: hairline columns, monospaced
// times, NPR pricing inline. Slot states:
//   · available off-peak  — sage tick, quiet
//   · available peak      — gold price, ◆ marker
//   · selected            — filled gold, ink type
//   · booked              — struck through, untouchable
//   · past                — faded out of play
// Selection enforces contiguous hours; pricing totals live in the dock.
// Confirmation calls the server action — the database, not this component,
// has the final word on availability.
// ============================================================================

import { useMemo, useState, useTransition } from "react";
import Link from "next/link";
import { AnimatePresence, m } from "framer-motion";
import { useBookingStore } from "@/stores/useBookingStore";
import type { ActionResult, Booking, Court, GridSlot, PaymentProvider } from "@/lib/types";

export interface BookingMatrixProps {
  courts: (Court & { arenaName: string; arenaArea: string; arenaSlug: string })[];
  /** Availability for the active court + date (from get_availability_grid). */
  slots: GridSlot[];
  /** True while a fresh grid is being fetched — render skeletons, not stale slots. */
  loading?: boolean;
  onCourtChange?: (courtId: string) => void;
  onDateChange?: (dateISO: string) => void;
  onConfirm: (payload: {
    courtId: string;
    startsAt: string;
    endsAt: string;
    provider: PaymentProvider;
  }) => Promise<ActionResult<Booking>>;
}

const NPR = new Intl.NumberFormat("en-IN");

/** Next 7 days, rendered in Kathmandu time regardless of viewer locale. */
function upcomingDays(): { iso: string; dow: string; day: string }[] {
  const fmtIso = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Kathmandu" });
  const fmtDow = new Intl.DateTimeFormat("en-US", { timeZone: "Asia/Kathmandu", weekday: "short" });
  const fmtDay = new Intl.DateTimeFormat("en-US", { timeZone: "Asia/Kathmandu", day: "2-digit" });
  return Array.from({ length: 7 }, (_, i) => {
    const d = new Date(Date.now() + i * 86_400_000);
    return { iso: fmtIso.format(d), dow: fmtDow.format(d), day: fmtDay.format(d) };
  });
}

const timeFmt = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Kathmandu",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

export default function BookingMatrix({
  courts,
  slots,
  loading = false,
  onCourtChange,
  onDateChange,
  onConfirm,
}: BookingMatrixProps) {
  const {
    courtId,
    dateISO,
    selected,
    lastError,
    setCourt,
    setDate,
    toggleSlot,
    clearSelection,
    setError,
    totalNpr,
  } = useBookingStore();

  const [provider, setProvider] = useState<PaymentProvider>("esewa");
  const [confirmedId, setConfirmedId] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const days = useMemo(upcomingDays, []);
  const activeCourt = courts.find((c) => c.id === courtId) ?? courts[0];

  const selectCourt = (id: string) => {
    setCourt(id);
    setConfirmedId(null);
    onCourtChange?.(id);
  };
  const selectDate = (iso: string) => {
    setDate(iso);
    setConfirmedId(null);
    onDateChange?.(iso);
  };

  const handleConfirm = () => {
    if (selected.length === 0 || !activeCourt) return;
    startTransition(async () => {
      const result = await onConfirm({
        courtId: activeCourt.id,
        startsAt: selected[0].starts_at,
        endsAt: selected[selected.length - 1].ends_at,
        provider,
      });
      if (result.ok) {
        setConfirmedId(result.data.id);
        clearSelection();
      } else {
        setError(result.error);
      }
    });
  };

  return (
    <section id="book" className="relative bg-canvas py-28">
      <div className="mx-auto max-w-6xl px-6">
        {/* Section head */}
        <div className="mb-16 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="eyebrow mb-4">Reserve · आरक्षण</p>
            <h2 className="font-display text-5xl tracking-tight text-ink">The matrix</h2>
          </div>
          <p className="max-w-xs text-sm leading-relaxed text-ink-dim">
            Every hour, every court, priced live. Gold marks peak hours —
            evenings and Saturdays carry the premium.
          </p>
        </div>

        {/* Court selector */}
        <div className="mb-10 flex flex-wrap gap-px bg-hairline">
          {courts.map((c) => {
            const active = c.id === activeCourt?.id;
            return (
              <button
                key={c.id}
                onClick={() => selectCourt(c.id)}
                className={`flex-1 basis-48 px-5 py-4 text-left transition-colors duration-300 ${
                  active ? "bg-surface-2" : "bg-canvas hover:bg-surface"
                }`}
              >
                <span className={`block font-display text-lg ${active ? "text-gold" : "text-ink"}`}>
                  {c.arenaName}
                </span>
                <span className="mt-1 block font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                  {c.label} · {c.side_count}-a-side · {c.arenaArea}
                </span>
              </button>
            );
          })}
        </div>

        {/* Arena profile link for the active court */}
        {activeCourt && (
          <div className="-mt-6 mb-10 text-right">
            <Link
              href={`/arenas/${activeCourt.arenaSlug}`}
              className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim underline decoration-hairline-2 underline-offset-8 transition-colors hover:text-gold"
            >
              About {activeCourt.arenaName} — photos &amp; reviews &rarr;
            </Link>
          </div>
        )}

        {/* Date strip */}
        <div role="tablist" aria-label="Choose a date" className="mb-2 grid grid-cols-7 border-y border-hairline">
          {days.map((d) => {
            const active = d.iso === dateISO;
            return (
              <button
                key={d.iso}
                role="tab"
                aria-selected={active}
                onClick={() => selectDate(d.iso)}
                className="group relative py-5 text-center"
              >
                <span className="eyebrow block">{d.dow}</span>
                <span
                  className={`mt-1 block font-display text-2xl transition-colors ${
                    active ? "text-gold" : "text-ink-dim group-hover:text-ink"
                  }`}
                >
                  {d.day}
                </span>
                {active && (
                  <m.span
                    layoutId="date-underline"
                    className="absolute inset-x-4 bottom-0 h-px bg-gold"
                  />
                )}
              </button>
            );
          })}
        </div>

        {/* Legend */}
        <div className="mb-8 flex flex-wrap gap-8 py-4 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
          <span className="flex items-center gap-2"><i className="h-2 w-2 bg-sage" /> Open</span>
          <span className="flex items-center gap-2"><i className="h-2 w-2 rotate-45 bg-gold" /> Peak rate</span>
          <span className="flex items-center gap-2"><i className="h-2 w-2 bg-hairline-2" /> Taken</span>
        </div>

        {/* THE GRID */}
        {loading ? (
          <div
            aria-busy="true"
            aria-label="Loading availability"
            className="grid grid-cols-2 gap-px bg-hairline sm:grid-cols-4"
          >
            {Array.from({ length: slots.length || 16 }).map((_, i) => (
              <div key={i} className="h-24 animate-pulse bg-surface/60" />
            ))}
          </div>
        ) : (
        <m.div
          key={`${activeCourt?.id}-${dateISO}`}
          initial="hidden"
          animate="show"
          variants={{ show: { transition: { staggerChildren: 0.025 } } }}
          className="grid grid-cols-2 gap-px bg-hairline sm:grid-cols-4"
        >
          {slots.map((slot) => {
            const isSelected = selected.some((s) => s.starts_at === slot.starts_at);
            const disabled = slot.is_booked || slot.is_past;
            return (
              <m.button
                key={slot.starts_at}
                variants={{
                  hidden: { opacity: 0, y: 10 },
                  show: { opacity: 1, y: 0, transition: { duration: 0.45, ease: [0.22, 1, 0.36, 1] } },
                }}
                whileTap={disabled ? undefined : { scale: 0.985 }}
                disabled={disabled}
                aria-pressed={isSelected}
                aria-label={`${timeFmt.format(new Date(slot.starts_at))}, ${
                  slot.is_booked ? "taken" : `रू ${NPR.format(slot.price_npr)}${slot.is_peak ? ", peak" : ""}`
                }`}
                onClick={() => toggleSlot(slot, slots)}
                className={`group relative flex h-24 flex-col justify-between p-4 text-left transition-colors duration-300 ${
                  isSelected
                    ? "bg-gold text-ink"
                    : disabled
                      ? "bg-canvas"
                      : "bg-surface hover:bg-surface-2"
                }`}
              >
                <span
                  className={`font-mono text-sm tabular-nums ${
                    isSelected
                      ? "text-ink"
                      : slot.is_past
                        ? "text-ink-faint/40"
                        : slot.is_booked
                          ? "text-ink-faint line-through decoration-hairline-2"
                          : "text-ink"
                  }`}
                >
                  {timeFmt.format(new Date(slot.starts_at))}
                </span>

                <span className="flex items-end justify-between">
                  {slot.is_booked ? (
                    <span className="font-mono text-[0.6rem] uppercase tracking-editorial text-ink-faint">
                      Taken
                    </span>
                  ) : slot.is_past ? (
                    <span className="font-mono text-[0.6rem] uppercase tracking-editorial text-ink-faint/40">
                      Closed
                    </span>
                  ) : (
                    <span
                      className={`font-mono text-xs tabular-nums ${
                        isSelected ? "text-ink" : slot.is_peak ? "text-gold" : "text-sage"
                      }`}
                    >
                      रू {NPR.format(slot.price_npr)}
                    </span>
                  )}
                  {slot.is_peak && !slot.is_booked && !slot.is_past && (
                    <i
                      aria-hidden
                      className={`h-1.5 w-1.5 rotate-45 ${isSelected ? "bg-canvas" : "bg-gold"}`}
                    />
                  )}
                </span>
              </m.button>
            );
          })}
        </m.div>
        )}

        {/* Confirmation + errors */}
        <AnimatePresence>
          {lastError && (
            <m.p
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              className="mt-6 font-mono text-xs text-ember"
              role="alert"
            >
              {lastError}
            </m.p>
          )}
          {confirmedId && (
            <m.p
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              className="mt-6 font-mono text-xs text-sage"
              role="status"
            >
              Slot held — confirmation #{confirmedId.slice(0, 8)}. Taking you to payment…
            </m.p>
          )}
        </AnimatePresence>
      </div>

      {/* Selection dock */}
      <AnimatePresence>
        {selected.length > 0 && activeCourt && (
          <m.aside
            initial={{ y: "120%" }}
            animate={{ y: 0 }}
            exit={{ y: "120%" }}
            transition={{ type: "spring", stiffness: 320, damping: 34 }}
            className="fixed inset-x-0 bottom-6 z-50 mx-auto w-[min(92vw,56rem)] border border-hairline-2 bg-surface/95 backdrop-blur-md"
          >
            <div className="flex flex-col gap-5 p-6 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="eyebrow mb-1">{activeCourt.arenaName} · {activeCourt.label}</p>
                <p className="font-mono text-sm tabular-nums text-ink">
                  {timeFmt.format(new Date(selected[0].starts_at))} —{" "}
                  {timeFmt.format(new Date(selected[selected.length - 1].ends_at))}
                  <span className="text-ink-faint"> · {selected.length} hr</span>
                </p>
              </div>

              <div className="flex items-center gap-6">
                {/* Payment provider */}
                <div className="flex border border-hairline-2" role="radiogroup" aria-label="Pay with">
                  {(["esewa", "khalti"] as const).map((p) => (
                    <button
                      key={p}
                      role="radio"
                      aria-checked={provider === p}
                      onClick={() => setProvider(p)}
                      className={`px-4 py-2 font-mono text-[0.62rem] uppercase tracking-editorial transition-colors ${
                        provider === p ? "bg-ink text-canvas" : "text-ink-dim hover:text-ink"
                      }`}
                    >
                      {p}
                    </button>
                  ))}
                </div>

                <p className="font-display text-3xl tabular-nums text-gold">
                  रू {NPR.format(totalNpr())}
                </p>

                <button
                  onClick={handleConfirm}
                  disabled={isPending}
                  className="border border-gold/60 px-6 py-3 font-mono text-[0.65rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50"
                >
                  {isPending ? "Holding…" : "Confirm &amp; pay"}
                </button>
              </div>
            </div>
          </m.aside>
        )}
      </AnimatePresence>
    </section>
  );
}
