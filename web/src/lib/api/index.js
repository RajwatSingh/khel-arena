/**
 * The one seam between the interface and its data.
 *
 * `cmd/api` now answers, but not for everything the interface asks for: the
 * arena reads (listArenas, getArena, listAreas, cityLedger) and payBooking
 * have no endpoint behind them yet, so pages still read from `mock.js`.
 *
 * Switching over is this line, once those endpoints exist — but note that the
 * two transports differ in more than their source. The mock answers
 * synchronously where the client returns a Promise, so `listBookings()` in a
 * $derived has to become an awaited call at the same time. That is the work
 * this seam defers, not work it removes.
 *
 * `client.js` itself is finished and tested against the running service; it is
 * imported directly by session.svelte.js for the auth flow.
 */

export * as api from './mock.js';
export { ApiError } from './errors.js';
export { HOLD_WINDOW_MS } from './mock.js';
export { SPORT_LABELS } from './fixtures.js';
