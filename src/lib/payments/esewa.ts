// ============================================================================
// eSewa ePay v2 hook
// Flow: build a signed form payload server-side → browser POSTs it to eSewa →
// eSewa redirects back with a base64 payload → verify the signature AND
// re-confirm via the status-check endpoint before marking the payment
// verified. Never trust the redirect alone.
// ============================================================================

import { createHmac } from "crypto";

const ESEWA = {
  formUrl:
    process.env.ESEWA_ENV === "production"
      ? "https://epay.esewa.com.np/api/epay/main/v2/form"
      : "https://rc-epay.esewa.com.np/api/epay/main/v2/form",
  statusUrl:
    process.env.ESEWA_ENV === "production"
      ? "https://epay.esewa.com.np/api/epay/transaction/status/"
      : "https://rc.esewa.com.np/api/epay/transaction/status/",
  productCode: process.env.ESEWA_PRODUCT_CODE ?? "EPAYTEST",
  secretKey: process.env.ESEWA_SECRET_KEY ?? "",
};

/** eSewa requires HMAC-SHA256 over exactly these comma-joined fields. */
function sign(fields: Record<string, string>, signedFieldNames: string): string {
  const message = signedFieldNames
    .split(",")
    .map((name) => `${name}=${fields[name]}`)
    .join(",");
  return createHmac("sha256", ESEWA.secretKey).update(message).digest("base64");
}

export interface EsewaFormPayload {
  action: string;
  fields: Record<string, string>;
}

/**
 * Builds the auto-submitting form payload for a booking payment.
 * `transactionUuid` must match payments.transaction_uuid so the callback
 * can be reconciled.
 */
export function buildEsewaPayment(params: {
  amountNpr: number;
  transactionUuid: string;
  successUrl: string;
  failureUrl: string;
}): EsewaFormPayload {
  const signedFieldNames = "total_amount,transaction_uuid,product_code";
  const fields: Record<string, string> = {
    amount: String(params.amountNpr),
    tax_amount: "0",
    total_amount: String(params.amountNpr),
    transaction_uuid: params.transactionUuid,
    product_code: ESEWA.productCode,
    product_service_charge: "0",
    product_delivery_charge: "0",
    success_url: params.successUrl,
    failure_url: params.failureUrl,
    signed_field_names: signedFieldNames,
  };
  fields.signature = sign(fields, signedFieldNames);
  return { action: ESEWA.formUrl, fields };
}

export interface EsewaCallbackResult {
  verified: boolean;
  transactionUuid?: string;
  providerRef?: string;
  /** Amount eSewa confirms was paid, in NPR — cross-check against the intent. */
  amountNpr?: number;
  raw?: unknown;
}

/** eSewa returns total_amount as a string, sometimes like "1,000.0". */
function parseAmount(value: unknown): number | undefined {
  if (value == null) return undefined;
  const n = Number(String(value).replace(/,/g, ""));
  return Number.isFinite(n) ? Math.round(n) : undefined;
}

/** Verifies the base64 payload eSewa appends to the success redirect. */
export async function verifyEsewaCallback(encodedData: string): Promise<EsewaCallbackResult> {
  let payload: Record<string, string>;
  try {
    payload = JSON.parse(Buffer.from(encodedData, "base64").toString("utf8"));
  } catch {
    return { verified: false };
  }

  // 1. Signature check on the callback itself.
  const expected = sign(payload, payload.signed_field_names ?? "");
  if (expected !== payload.signature || payload.status !== "COMPLETE") {
    return { verified: false, raw: payload };
  }

  // 2. Server-to-server status confirmation (defends against forged callbacks).
  const qs = new URLSearchParams({
    product_code: ESEWA.productCode,
    total_amount: payload.total_amount,
    transaction_uuid: payload.transaction_uuid,
  });
  const res = await fetch(`${ESEWA.statusUrl}?${qs}`, { cache: "no-store" });
  if (!res.ok) return { verified: false, raw: payload };
  const status = await res.json();

  return {
    verified: status.status === "COMPLETE",
    transactionUuid: payload.transaction_uuid,
    providerRef: status.ref_id ?? payload.transaction_code,
    amountNpr: parseAmount(status.total_amount ?? payload.total_amount),
    raw: status,
  };
}
