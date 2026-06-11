"use client";

// Shared LazyMotion provider — loads the lightweight domAnimation feature set
// instead of the full framer-motion engine. Cuts ~60% of framer-motion's
// bundle. Wrap the app once in the root layout.

import { LazyMotion, domAnimation } from "framer-motion";

export default function MotionProvider({ children }: { children: React.ReactNode }) {
  return <LazyMotion features={domAnimation}>{children}</LazyMotion>;
}
