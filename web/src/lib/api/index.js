/**
 * The one seam between the interface and its data.
 *
 * `cmd/api` is not written yet (README, "Status"), so every page reads from
 * the mock in `mock.js`. When the Go service answers, re-export `./client.js`
 * from here for the endpoints it covers — no page changes.
 */

export * as api from './mock.js';
export { ApiError, HOLD_WINDOW_MS } from './mock.js';
export { SPORT_LABELS } from './fixtures.js';
