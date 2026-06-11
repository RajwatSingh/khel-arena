"use server";

// ============================================================================
// Payment server actions
// payForBooking() creates a payment intent (payments row) and returns an
// instruction the client executes:
//   · eSewa  → { kind: "form" }     — browser POSTs the signed form
//   · Khalti → { kind: "redirect" } — browser navigates to payment_url
// Verification NEVER happens here; the gateway callbacks at
// /api/payments/{provider}/callback confirm server-to-server and only then
// flip payments → verified and bookings → confirmed.
// ============================================================================

import { randomUUID } from "crypto";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import { createAdminClient, isPaymentsConfigured } from "@/lib/supabase/admin";
import { buildEsewaPayment } from "@/lib/payments/esewa";
import { initiateKhaltiPayment } from "@/lib/payments/khalti";
import type { ActionResult } from "@/lib/types";

export type PaymentInstruction =
  | { kind: "form"; action: string; fields: Record<string, string> }
  | { kind: "redirect"; url: string };

const Input = z.object({
  bookingId: z.string().uuid(),
  provider: z.enum(["esewa", "khalti"]),
});

const site = () => process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";

export async function payForBooking(
  input: z.input<typeof Input>
): Promise<ActionResult<PaymentInstruction>> {
  const parsed = Input.safeParse(input);
  if (!parsed.success) return { ok: false, error: "Invalid payment request.", code: "VALIDATION" };
  const { bookingId, provider } = parsed.data;

  if (!isPaymentsConfigured()) {
    return { ok: false, error: "Payments are not configured on this environment.", code: "NOT_CONFIGURED" };
  }

  // Identity comes from the session; ownership is checked under RLS.
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) return { ok: false, error: "Sign in to pay for a booking.", code: "AUTH_REQUIRED" };

  const { data: booking, error: bookingError } = await supabase
    .from("bookings")
    .select("id, user_id, price_npr, status, court_id, slot")
    .eq("id", bookingId)
    .single();

  if (bookingError || !booking) return { ok: false, error: "Booking not found." };
  if (booking.user_id !== user.id)
    return { ok: false, error: "Only the booking owner can pay for it." };
  if (booking.status !== "pending")
    return { ok: false, error: "This booking is not awaiting payment." };

  // Create the payment intent with the service client (payments has no
  // client insert policy by design).
  const admin = createAdminClient();
  const transactionUuid = randomUUID();
  const { error: intentError } = await admin.from("payments").insert({
    booking_id: booking.id,
    provider,
    amount_npr: booking.price_npr,
    status: "initiated",
    transaction_uuid: transactionUuid,
  });
  if (intentError) return { ok: false, error: "Could not start the payment. Try again." };

  if (provider === "esewa") {
    const payload = buildEsewaPayment({
      amountNpr: booking.price_npr,
      transactionUuid,
      successUrl: `${site()}/api/payments/esewa/callback`,
      failureUrl: `${site()}/book/confirmation?status=failed&provider=esewa`,
    });
    return { ok: true, data: { kind: "form", action: payload.action, fields: payload.fields } };
  }

  // Khalti
  const init = await initiateKhaltiPayment({
    amountNpr: booking.price_npr,
    transactionUuid,
    bookingLabel: `Khel Arena booking ${booking.id.slice(0, 8)}`,
    returnUrl: `${site()}/api/payments/khalti/callback`,
  });
  if (!init.ok || !init.paymentUrl || !init.pidx) {
    await admin.from("payments").update({ status: "failed" }).eq("transaction_uuid", transactionUuid);
    return { ok: false, error: init.error ?? "Khalti could not start this payment." };
  }

  // Store pidx so the callback can reconcile without trusting query params.
  await admin
    .from("payments")
    .update({ provider_ref: init.pidx })
    .eq("transaction_uuid", transactionUuid);

  return { ok: true, data: { kind: "redirect", url: init.paymentUrl } };
}
