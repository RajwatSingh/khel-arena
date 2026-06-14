import { describe, it, expect } from "vitest";
import { DEMO_COURTS, demoGrid } from "@/lib/demo";

const courtId = DEMO_COURTS[0].id; // base_price 1200
const date = "2090-01-15"; // far future → never "past"
const grid = demoGrid(courtId, date);

describe("demoGrid", () => {
  it("projects 16 hourly slots (06:00–21:00)", () => {
    expect(grid).toHaveLength(16);
  });

  it("prices off-peak at base and evening peak at 1.5x", () => {
    expect(grid[0].is_peak).toBe(false); // 06:00
    expect(grid[0].price_npr).toBe(1200);
    expect(grid[12].is_peak).toBe(true); // 18:00
    expect(grid[12].price_npr).toBe(1800);
  });

  it("is deterministic for the same court + date", () => {
    const again = demoGrid(courtId, date);
    expect(again.map((s) => s.is_booked)).toEqual(grid.map((s) => s.is_booked));
  });

  it("emits hour-long, time-ordered slots", () => {
    for (let i = 1; i < grid.length; i++) {
      expect(grid[i].starts_at > grid[i - 1].starts_at).toBe(true);
    }
    const ms =
      new Date(grid[0].ends_at).getTime() - new Date(grid[0].starts_at).getTime();
    expect(ms).toBe(3_600_000);
  });
});
