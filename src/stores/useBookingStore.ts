// ============================================================================
// Booking flow store (Zustand)
// Holds ephemeral UI state for the matrix: selected court, date, and slots.
// Server state (availability) stays in server components / RPC responses —
// this store never caches authority over what is bookable.
// ============================================================================

import { create } from "zustand";
import type { GridSlot } from "@/lib/types";

interface BookingState {
  courtId: string | null;
  dateISO: string; // "YYYY-MM-DD" in Asia/Kathmandu
  selected: GridSlot[];
  isSubmitting: boolean;
  lastError: string | null;

  setCourt: (courtId: string) => void;
  setDate: (dateISO: string) => void;
  toggleSlot: (slot: GridSlot, allSlots?: GridSlot[]) => void;
  clearSelection: () => void;
  setSubmitting: (v: boolean) => void;
  setError: (msg: string | null) => void;

  totalNpr: () => number;
}

/** Slots must be contiguous on the same court — enforced at selection time. */
function isAdjacent(selection: GridSlot[], slot: GridSlot): boolean {
  if (selection.length === 0) return true;
  return selection.some(
    (s) => s.ends_at === slot.starts_at || s.starts_at === slot.ends_at
  );
}

export const useBookingStore = create<BookingState>((set, get) => ({
  courtId: null,
  dateISO: new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Kathmandu" }).format(
    new Date()
  ),
  selected: [],
  isSubmitting: false,
  lastError: null,

  setCourt: (courtId) => set({ courtId, selected: [], lastError: null }),
  setDate: (dateISO) => set({ dateISO, selected: [], lastError: null }),

  toggleSlot: (slot, allSlots) => {
    const { selected } = get();
    const already = selected.find((s) => s.starts_at === slot.starts_at);

    if (already) {
      // Removing a middle slot would split the range — keep the longest run.
      const remaining = selected.filter((s) => s.starts_at !== slot.starts_at);
      const sorted = [...remaining].sort((a, b) => a.starts_at.localeCompare(b.starts_at));
      const contiguous: GridSlot[] = [];
      for (const s of sorted) {
        if (contiguous.length === 0 || contiguous[contiguous.length - 1].ends_at === s.starts_at) {
          contiguous.push(s);
        } else break;
      }
      set({ selected: contiguous, lastError: null });
      return;
    }

    if (slot.is_booked || slot.is_past) return;

    if (selected.length === 0 || !allSlots) {
      set({ selected: [slot], lastError: null });
      return;
    }

    if (isAdjacent(selected, slot)) {
      if (selected.length >= 4) {
        set({ lastError: "Bookings are limited to 4 hours." });
        return;
      }
      set({
        selected: [...selected, slot].sort((a, b) => a.starts_at.localeCompare(b.starts_at)),
        lastError: null,
      });
      return;
    }

    // Non-adjacent: select the full range between current selection and clicked slot.
    const sorted = [...allSlots].sort((a, b) => a.starts_at.localeCompare(b.starts_at));
    const selStart = selected[0].starts_at;
    const selEnd = selected[selected.length - 1].starts_at;
    const clickedTime = slot.starts_at;

    let rangeStart: string, rangeEnd: string;
    if (clickedTime < selStart) {
      rangeStart = clickedTime;
      rangeEnd = selEnd;
    } else {
      rangeStart = selStart;
      rangeEnd = clickedTime;
    }

    const range = sorted.filter(
      (s) => s.starts_at >= rangeStart && s.starts_at <= rangeEnd
    );

    // Check: no booked/past slots in range
    const blocked = range.find((s) => s.is_booked || s.is_past);
    if (blocked) {
      set({ lastError: "There's a taken or closed slot in that range." });
      return;
    }

    // Check: contiguous (no gaps in range)
    for (let i = 1; i < range.length; i++) {
      if (range[i - 1].ends_at !== range[i].starts_at) {
        set({ lastError: "There's a gap in that range." });
        return;
      }
    }

    if (range.length > 4) {
      set({ lastError: "Bookings are limited to 4 hours." });
      return;
    }

    set({ selected: range, lastError: null });
  },

  clearSelection: () => set({ selected: [], lastError: null }),
  setSubmitting: (v) => set({ isSubmitting: v }),
  setError: (msg) => set({ lastError: msg }),

  totalNpr: () => get().selected.reduce((sum, s) => sum + s.price_npr, 0),
}));
