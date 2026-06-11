// ============================================================================
// eSewa return handler.
// eSewa redirects the player here with ?data=<base64 payload>. The payload's
// signature is checked AND the transaction is re-confirmed against eSewa's
// status API before anything is marked verified. Query params alone are
// never trusted.
// ============================================================================

import { NextRequest, NextResponse } from "next/server";
import { verifyEsewaCallback } from "@/lib/payments/esewa";
import { createAdminClient, isPaymentsConfigured } from "@/lib/supabase/admin";

const confirmation = (req: NextRequest, params: Record<string, string>) =>
  NextResponse.redirect(
    new URL(`/book/confirmation?${new URLSearchParams(params)}`, req.nextUrl.origin)
  );

export async function GET(req: NextRequest) {
  const encoded = req.nextUrl.searchParams.get("data");
  if (!encoded || !isPaymentsConfigured()) {
    return confirmation(req, { status: "failed", provider: "esewa" });
  }

  const result = await verifyEsewaCallback(encoded);
  if (!result.verified || !result.transactionUuid) {
    return confirmation(req, { status: "failed", provider: "esewa" });
  }

  const admin = createAdminClient();

  // Idempotent: a replayed callback finds the row already verified.
  const { data: payment } = await admin
    .from("payments")
    .update({
      status: "verified",
      provider_ref: result.providerRef ?? null,
      raw_response: result.raw ?? null,
      verified_at: new Date().toISOString(),
    })
    .eq("transaction_uuid", result.transactionUuid)
    .in("status", ["initiated", "verified"])
    .select("booking_id")
    .single();

  if (!payment) return confirmation(req, { status: "failed", provider: "esewa" });

  await admin
    .from("bookings")
    .update({ status: "confirmed" })
    .eq("id", payment.booking_id)
    .eq("status", "pending");

  return confirmation(req, {
    status: "success",
    provider: "esewa",
    ref: result.providerRef ?? result.transactionUuid,
  });
}
