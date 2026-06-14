import { beforeEach, describe, expect, it } from "vitest";
import { useBookingStore } from "@/stores/useBookingStore";
import type { GridSlot } from "@/lib/types";

// Contiguous, on-the-hour slots for a single day.
const slot = (h: number, over: Partial<GridSlot> = {}): GridSlot => ({
  starts_at: `2026-06-20T${String(h).padStart(2, "0")}:00:00.000Z`,
  ends_at: `2026-06-20T${String(h + 1).padStart(2, "0")}:00:00.000Z`,
  price_npr: 1000,
  is_peak: false,
  is_booked: false,
  is_past: false,
  ...over,
});

const ALL = [16, 17, 18, 19, 20, 21].map((h) => slot(h));
const hours = () =>
  useBookingStore
    .getState()
    .selected.map((s) => Number(s.starts_at.slice(11, 13)));

beforeEach(() => useBookingStore.setState({ selected: [], lastError: null }));

describe("useBookingStore.toggleSlot", () => {
  it("selects a single open slot", () => {
    useBookingStore.getState().toggleSlot(slot(18));
    expect(hours()).toEqual([18]);
  });

  it("ignores booked or past slots", () => {
    useBookingStore.getState().toggleSlot(slot(18, { is_booked: true }));
    useBookingStore.getState().toggleSlot(slot(19, { is_past: true }));
    expect(hours()).toEqual([]);
  });

  it("adds adjacent hours and keeps them sorted", () => {
    const { toggleSlot } = useBookingStore.getState();
    toggleSlot(slot(19), ALL);
    toggleSlot(slot(18), ALL);
    expect(hours()).toEqual([18, 19]);
  });

  it("fills the whole range when a non-adjacent slot is clicked", () => {
    const { toggleSlot } = useBookingStore.getState();
    toggleSlot(slot(17), ALL);
    toggleSlot(slot(20), ALL);
    expect(hours()).toEqual([17, 18, 19, 20]);
  });

  it("rejects a range longer than 4 hours", () => {
    const { toggleSlot } = useBookingStore.getState();
    toggleSlot(slot(17), ALL);
    toggleSlot(slot(21), ALL); // 17–21 = 5 slots
    expect(hours()).toEqual([17]); // selection unchanged
    expect(useBookingStore.getState().lastError).toMatch(/4 hours/i);
  });

  it("deselecting a slot keeps the contiguous run", () => {
    const { toggleSlot } = useBookingStore.getState();
    toggleSlot(slot(18), ALL);
    toggleSlot(slot(19), ALL);
    toggleSlot(slot(19), ALL); // remove
    expect(hours()).toEqual([18]);
  });

  it("totalNpr sums the selected slots", () => {
    const { toggleSlot } = useBookingStore.getState();
    toggleSlot(slot(18, { price_npr: 1200 }), ALL);
    toggleSlot(slot(19, { price_npr: 1800 }), ALL);
    expect(useBookingStore.getState().totalNpr()).toBe(3000);
  });
});
