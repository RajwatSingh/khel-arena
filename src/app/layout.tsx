import type { Metadata } from "next";
import { Fraunces, Archivo, IBM_Plex_Mono } from "next/font/google";
import Nav from "@/components/Nav";
import MotionProvider from "@/components/MotionProvider";
import "./globals.css";

const display = Fraunces({
  subsets: ["latin"],
  axes: ["opsz"],
  variable: "--font-display",
});
const sans = Archivo({ subsets: ["latin"], variable: "--font-sans" });
const mono = IBM_Plex_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  variable: "--font-mono",
});

export const metadata: Metadata = {
  title: "Khel Arena — Book the pitch. Find your five.",
  description:
    "Premium futsal and sports arena booking across Kathmandu. Live availability, instant confirmation with eSewa and Khalti, and a community that fills your last two spots.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${display.variable} ${sans.variable} ${mono.variable}`}>
      <body className="font-sans">
        <MotionProvider>
        <Nav />
        {children}
        <footer className="border-t border-hairline bg-canvas py-12">
          <div className="mx-auto flex max-w-6xl flex-col items-baseline justify-between gap-4 px-6 sm:flex-row">
            <p className="font-display text-lg text-ink">
              Khel<span className="text-gold">.</span>
            </p>
            <p className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
              Kathmandu Valley · eSewa &amp; Khalti accepted
            </p>
          </div>
        </footer>
        </MotionProvider>
      </body>
    </html>
  );
}
