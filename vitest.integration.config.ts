import { defineConfig } from "vitest/config";

// Integration tests hit a real Postgres (DATABASE_URL). Kept separate from the
// fast unit suite so `npm test` never needs a database.
export default defineConfig({
  test: {
    environment: "node",
    include: ["tests/integration/**/*.test.ts"],
    testTimeout: 60_000,
    hookTimeout: 60_000,
  },
});
