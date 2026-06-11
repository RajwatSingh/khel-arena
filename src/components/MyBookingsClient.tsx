"use client";

// MyBookingsClient — /my-bookings composition. Routes cancel + community
// toggle to live server actions or local demo simulators.

import { useState } from "react";
import MyBookingsHub from "@/components/MyBookingsHub";
import { cancelBooking } from "@/actions/bookings";
import { toggleMatchmakingSlot } from "@/actions/matchmaking";
import type { ActionResult, MatchmakingPost, MyBooking } from "@/lib/types";

interface MyBookingsClientProps {
  demoMode: boolean;
  bookings: MyBooking[];
}

export default function MyBookingsClient({
  demoMode,
  bookings: initial,
}: MyBookingsClientProps) {
  const [list, setList] = useState<MyBooking[]>(initial);

  const handleCancel = async (bookingId: string): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 400));
      setList((prev) =>
        prev.map((b) =>
          b.id === bookingId
            ? { ...b, status: "cancelled" as const, open_to_join: false }
            : b
        )
      );
      return { ok: true, data: null };
    }
    const res = await cancelBooking(bookingId);
    if (res.ok) {
      setList((prev) =>
        prev.map((b) =>
          b.id === bookingId
            ? { ...b, status: "cancelled" as const, open_to_join: false }
            : b
        )
      );
    }
    return res;
  };

  const handleToggleCommunity = async (input: {
    bookingId: string;
    open: boolean;
    neededPlayers?: number;
    title?: string;
    skill?: string;
  }): Promise<ActionResult<MatchmakingPost>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 400));
      setList((prev) =>
        prev.map((b) =>
          b.id === input.bookingId ? { ...b, open_to_join: input.open } : b
        )
      );
      return {
        ok: true,
        data: {
          id: crypto.randomUUID(),
          author_id: "demo-user",
          booking_id: input.bookingId,
          arena_id: null,
          title: input.title ?? "Open slot",
          needed_players: input.neededPlayers ?? 2,
          filled_players: 0,
          skill: (input.skill as "casual") ?? "casual",
          starts_at: new Date().toISOString(),
          status: input.open ? "open" : "filled",
        },
      };
    }
    const res = await toggleMatchmakingSlot({
      bookingId: input.bookingId,
      open: input.open,
      neededPlayers: input.neededPlayers ?? 2,
      title: input.title,
      skill: (input.skill as "casual") ?? "casual",
    });
    if (res.ok) {
      setList((prev) =>
        prev.map((b) =>
          b.id === input.bookingId ? { ...b, open_to_join: input.open } : b
        )
      );
    }
    return res;
  };

  return (
    <main>
      <MyBookingsHub
        bookings={list}
        onCancel={handleCancel}
        onToggleCommunity={handleToggleCommunity}
      />
    </main>
  );
}
