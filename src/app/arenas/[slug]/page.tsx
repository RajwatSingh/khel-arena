// /arenas/[slug] — public futsal profile. Resolves the arena (live or demo),
// then hands off to the client for the gallery + review interactions.

import type { Metadata } from "next";
import { notFound } from "next/navigation";
import ArenaShowcase from "@/components/ArenaShowcase";
import { getArenaProfile } from "@/actions/arena";
import { demoArenaProfile, isDemoMode } from "@/lib/demo";

export const metadata: Metadata = { title: "Arena — Khel Arena" };

export default async function ArenaPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;

  if (isDemoMode()) {
    const profile = demoArenaProfile(slug);
    if (!profile) notFound();
    return <ArenaShowcase demoMode profile={profile} />;
  }

  const res = await getArenaProfile(slug);
  if (!res.ok) notFound();
  return <ArenaShowcase demoMode={false} profile={res.data} />;
}
