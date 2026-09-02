/**
 * The one seam between the interface and its data.
 *
 * This now points at the real service. `mock.js` and `fixtures.js` are no
 * longer imported by anything and are kept only until the payment flow lands,
 * since that is the last call with nothing behind it -- `client.payBooking`
 * rejects with a readable message rather than pretending.
 *
 * The two transports differ in more than their source, which is why switching
 * this line was not the whole job: the mock answered synchronously and the
 * client returns a Promise, so every `load` became async and the call sites
 * that read data inside a `$derived` had to become effects.
 *
 * On the server, a `load` must pass the `fetch` SvelteKit gives it --
 * `api.listArenas({ fetch })`. The client's base URL is relative and Node has
 * no page origin to resolve it against; that fetch does, and forwards cookies
 * besides. Calling without it throws a message saying so.
 */

export * as api from './client.js';
export { ApiError } from './errors.js';
export { SPORT_LABELS } from '../sports.js';
