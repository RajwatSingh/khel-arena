"use client";

// ============================================================================
// BookClient — /book composition.
// Confirm flow: createBooking (race-safe in Postgres) → payForBooking →
// hand the browser to the gateway (eSewa signed form POST or Khalti
// redirect). The gateway returns to /api/payments/{provider}/callback,
// which verifies server-to-server and lands on /book/confirmation.
// Demo mode simulates the whole loop locally.
// ============================================================================

import { useCallback, useState } from "react";
import { useRouter } from "next/navigation";
import BookingMatrix from "@/components/BookingMatrix";
import { useBookingStore } from "@/stores/useBookingStore";
import { createBooking, getAvailabilityGrid } from "@/actions/bookings";
import { payForBooking } from "@/actions/payments";
import { demoGrid } from "@/lib/demo";
import type { ActionResult, Booking, Court, GridSlot, PaymentProvider } from "@/lib/types";

interface BookClientProps {
  demoMode: boolean;
  courts: (Court & { arenaName: string; arenaArea: string })[];
  initialSlots: GridSlot[];
}

/** Builds and submits a hidden POST form — how eSewa's ePay v2 is entered. */
function submitGatewayForm(action: string, fields: Record<string, string>) {
  const form = document.createElement("form");
  form.method = "POST";
  form.action = action;
  for (const [name, value] of Object.entries(fields)) {
    const input = document.createElement("input");
    input.type = "hidden";
    input.name = name;
    input.value = value;
    form.appendChild(input);
  }
  document.body.appendChild(form);
  form.submit();
}

export default function BookClient({ demoMode, courts, initialSlots }: BookClientProps) {
  const router = useRouter();
  const [slots, setSlots] = useState<GridSlot[]>(initialSlots);
  const [loading, setLoading] = useState(false);
  const [locallyBooked, setLocallyBooked] = useState<Set<string>>(new Set());

  const refreshGrid = useCallback(
    async (courtId: string, dateISO: string) => {
      if (demoMode) {
        setSlots(
          demoGrid(courtId, dateISO).map((s) =>
            locallyBooked.has(`${courtId}|${s.starts_at}`) ? { ...s, is_booked: true } : s
          )
        );
        return;
      }
      // Mark loading synchronously so the matrix swaps to skeletons at once,
      // rather than lingering on the previous date's slots during the fetch.
      setLoading(true);
      try {
        const res = await getAvailabilityGrid(courtId, dateISO);
        if (res.ok) setSlots(res.data);
      } finally {
        setLoading(false);
      }
    },
    [demoMode, locallyBooked]
  );

  const handleCourtChange = (courtId: string) =>
    void refreshGrid(courtId, useBookingStore.getState().dateISO);
  const handleDateChange = (dateISO: string) =>
    void refreshGrid(useBookingStore.getState().courtId ?? courts[0].id, dateISO);

  const handleConfirm = async (payload: {
    courtId: string;
    startsAt: string;
    endsAt: string;
    provider: PaymentProvider;
  }): Promise<ActionResult<Booking>> => {
    if (demoMode) {
      // Simulate the booking hold, then the gateway round-trip.
      await new Promise((r) => setTimeout(r, 600));
      setLocallyBooked((prev) => {
        const next = new Set(prev);
        for (
          let t = new Date(payload.startsAt).getTime();
          t < new Date(payload.endsAt).getTime();
          t += 3_600_000
        ) {
          next.add(`${payload.courtId}|${new Date(t).toISOString()}`);
        }
        return next;
      });
      setTimeout(() => {
        router.push(
          `/book/confirmation?status=success&provider=${payload.provider}&ref=DEMO-${Date.now()
            .toString(36)
            .toUpperCase()}&demo=1`
        );
      }, 900);
      return {
        ok: true,
        data: {
          id: crypto.randomUUID(),
          court_id: payload.courtId,
          user_id: "demo",
          team_id: null,
          slot: `[${payload.startsAt},${payload.endsAt})`,
          price_npr: 0,
          is_peak: false,
          status: "pending",
          open_to_join: false,
          created_at: new Date().toISOString(),
        },
      };
    }

    // 1. Hold the slot — Postgres guarantees no double booking.
    const booked = await createBooking(payload);
    if (!booked.ok) return booked;

    // 2. Create the payment intent and hand off to the gateway.
    const intent = await payForBooking({
      bookingId: booked.data.id,
      provider: payload.provider as "esewa" | "khalti",
    });
    if (!intent.ok) {
      return {
        ok: false,
        error: `${intent.error} Your slot is held as #${booked.data.id.slice(0, 8)} — retry payment from your bookings.`,
      };
    }

    if (intent.data.kind === "form") {
      submitGatewayForm(intent.data.action, intent.data.fields);
    } else {
      window.location.assign(intent.data.url);
    }
    return booked;
  };

  return (
    <main>
      <BookingMatrix
        courts={courts}
        slots={slots}
        loading={loading}
        onCourtChange={handleCourtChange}
        onDateChange={handleDateChange}
        onConfirm={handleConfirm}
      />
    </main>
  );
}
