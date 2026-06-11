// ============================================================================
// Khalti ePayment hook
// Flow: server initiates → receives pidx + payment_url → browser redirects →
// Khalti calls back with pidx → server confirms via the lookup endpoint.
// The lookup response is the only source of truth for marking a payment
// verified.
// ============================================================================

const KHALTI = {
  base:
    process.env.KHALTI_ENV === "production"
      ? "https://khalti.com/api/v2"
      : "https://dev.khalti.com/api/v2",
  secretKey: process.env.KHALTI_SECRET_KEY ?? "",
};

function headers() {
  return {
    Authorization: `Key ${KHALTI.secretKey}`,
    "Content-Type": "application/json",
  };
}

export interface KhaltiInitiateResult {
  ok: boolean;
  pidx?: string;
  paymentUrl?: string;
  error?: string;
}

/**
 * Initiates a Khalti payment for a booking.
 * Khalti amounts are in PAISA — convert from NPR exactly once, here.
 */
export async function initiateKhaltiPayment(params: {
  amountNpr: number;
  transactionUuid: string; // payments.transaction_uuid
  bookingLabel: string;    // "Court A · Dhuku Futsal · 19:00"
  returnUrl: string;
  customer?: { name: string; phone?: string };
}): Promise<KhaltiInitiateResult> {
  const res = await fetch(`${KHALTI.base}/epayment/initiate/`, {
    method: "POST",
    headers: headers(),
    cache: "no-store",
    body: JSON.stringify({
      return_url: params.returnUrl,
      website_url: process.env.NEXT_PUBLIC_SITE_URL,
      amount: Math.round(params.amountNpr * 100), // NPR → paisa
      purchase_order_id: params.transactionUuid,
      purchase_order_name: params.bookingLabel,
      customer_info: params.customer,
    }),
  });

  const body = await res.json().catch(() => null);
  if (!res.ok || !body?.pidx) {
    return { ok: false, error: body?.detail ?? "Khalti could not start this payment." };
  }
  return { ok: true, pidx: body.pidx, paymentUrl: body.payment_url };
}

export interface KhaltiLookupResult {
  verified: boolean;
  pidx: string;
  status?: string; // Completed | Pending | Expired | Refunded | "User canceled"
  amountNpr?: number;
  raw?: unknown;
}

/** Confirms payment state. Call from the return-URL handler — never trust query params. */
export async function verifyKhaltiPayment(pidx: string): Promise<KhaltiLookupResult> {
  const res = await fetch(`${KHALTI.base}/epayment/lookup/`, {
    method: "POST",
    headers: headers(),
    cache: "no-store",
    body: JSON.stringify({ pidx }),
  });

  const body = await res.json().catch(() => null);
  if (!res.ok || !body) return { verified: false, pidx };

  return {
    verified: body.status === "Completed",
    pidx,
    status: body.status,
    amountNpr: typeof body.total_amount === "number" ? body.total_amount / 100 : undefined,
    raw: body,
  };
}
