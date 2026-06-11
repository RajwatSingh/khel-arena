// ============================================================================
// Khalti return handler.
// Khalti redirects here with ?pidx=… — the pidx is looked up server-to-server
// and only a "Completed" lookup marks the payment verified. The amount is
// cross-checked against the intent to reject tampered or partial payments.
// ============================================================================

import { NextRequest, NextResponse } from "next/server";
import { verifyKhaltiPayment } from "@/lib/payments/khalti";
import { createAdminClient, isPaymentsConfigured } from "@/lib/supabase/admin";

const confirmation = (req: NextRequest, params: Record<string, string>) =>
  NextResponse.redirect(
    new URL(`/book/confirmation?${new URLSearchParams(params)}`, req.nextUrl.origin)
  );

export async function GET(req: NextRequest) {
  const pidx = req.nextUrl.searchParams.get("pidx");
  if (!pidx || !isPaymentsConfigured()) {
    return confirmation(req, { status: "failed", provider: "khalti" });
  }

  const admin = createAdminClient();
  const { data: intent } = await admin
    .from("payments")
    .select("transaction_uuid, booking_id, amount_npr, status")
    .eq("provider_ref", pidx)
    .single();

  if (!intent) return confirmation(req, { status: "failed", provider: "khalti" });

  const result = await verifyKhaltiPayment(pidx);
  const amountMatches =
    typeof result.amountNpr !== "number" || result.amountNpr === intent.amount_npr;

  if (!result.verified || !amountMatches) {
    if (result.status === "Expired" || result.status === "User canceled") {
      await admin
        .from("payments")
        .update({ status: "failed", raw_response: result.raw ?? null })
        .eq("transaction_uuid", intent.transaction_uuid)
        .eq("status", "initiated");
    }
    return confirmation(req, { status: "failed", provider: "khalti" });
  }

  await admin
    .from("payments")
    .update({
      status: "verified",
      raw_response: result.raw ?? null,
      verified_at: new Date().toISOString(),
    })
    .eq("transaction_uuid", intent.transaction_uuid)
    .in("status", ["initiated", "verified"]);

  await admin
    .from("bookings")
    .update({ status: "confirmed" })
    .eq("id", intent.booking_id)
    .eq("status", "pending");

  return confirmation(req, { status: "success", provider: "khalti", ref: pidx });
}
