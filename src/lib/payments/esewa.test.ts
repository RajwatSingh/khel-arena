import { describe, it, expect } from "vitest";
import { buildEsewaPayment } from "@/lib/payments/esewa";

const base = {
  transactionUuid: "tx-123",
  successUrl: "https://khel.test/api/payments/esewa/callback",
  failureUrl: "https://khel.test/book/confirmation?status=failed",
};

describe("buildEsewaPayment", () => {
  it("emits the amount, transaction id, and the exact signed-field list", () => {
    const { action, fields } = buildEsewaPayment({ amountNpr: 1500, ...base });

    expect(fields.amount).toBe("1500");
    expect(fields.total_amount).toBe("1500");
    expect(fields.transaction_uuid).toBe("tx-123");
    expect(fields.signed_field_names).toBe("total_amount,transaction_uuid,product_code");
    expect(fields.signature).toMatch(/^[A-Za-z0-9+/]+=*$/); // base64 HMAC
    expect(action).toContain("esewa");
  });

  it("is tamper-evident: changing the amount changes the signature", () => {
    const a = buildEsewaPayment({ amountNpr: 1500, ...base }).fields.signature;
    const b = buildEsewaPayment({ amountNpr: 9999, ...base }).fields.signature;
    expect(a).not.toBe(b);
  });

  it("is tamper-evident: changing the transaction id changes the signature", () => {
    const a = buildEsewaPayment({ amountNpr: 1500, ...base }).fields.signature;
    const b = buildEsewaPayment({ amountNpr: 1500, ...base, transactionUuid: "tx-999" })
      .fields.signature;
    expect(a).not.toBe(b);
  });
});
