// Shared social-card artwork for the OpenGraph + Twitter image routes.
// Rendered by next/og (satori): every element with more than one child must
// set display:flex, and we lean on the default font (no remote font fetch).
import type { ReactElement } from "react";

export const OG_ALT = "Khel Arena — Book the pitch. Find your five.";

export function OgCard(): ReactElement {
  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
        background: "#faf7f0",
        color: "#211d14",
        padding: "80px",
        fontFamily: "sans-serif",
      }}
    >
      {/* Top row: wordmark + locale */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ display: "flex", alignItems: "baseline", fontSize: 44, fontWeight: 700 }}>
          <span>Khel</span>
          <span style={{ color: "#a47e33" }}>.</span>
        </div>
        <div style={{ fontSize: 20, letterSpacing: "0.28em", textTransform: "uppercase", color: "#9a9180" }}>
          Kathmandu
        </div>
      </div>

      {/* Headline + blurb */}
      <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            fontSize: 88,
            fontWeight: 700,
            lineHeight: 1.05,
            letterSpacing: "-0.03em",
          }}
        >
          <div>Book the pitch.</div>
          <div>Find your five.</div>
        </div>
        <div style={{ fontSize: 28, lineHeight: 1.4, color: "#6b6354", maxWidth: 820 }}>
          Live futsal availability, instant eSewa &amp; Khalti checkout, and a community that fills
          your last two spots.
        </div>
      </div>

      {/* Footer rail */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 16,
          fontSize: 20,
          letterSpacing: "0.2em",
          textTransform: "uppercase",
          color: "#9a9180",
        }}
      >
        <div style={{ width: 12, height: 12, background: "#a47e33", transform: "rotate(45deg)" }} />
        <span>Reserve · Tournaments · Teams · Community</span>
      </div>
    </div>
  );
}
