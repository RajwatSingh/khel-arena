/**
 * The real transport: one function per row of the endpoint table in
 * apiPlan.md §3, talking to `cmd/api`.
 *
 * Three things here are worth reading before changing anything.
 *
 * **The base URL is relative, and that is only half a decision.** In a browser
 * `/v1/...` resolves against the page's own origin, which is what we want: the
 * API is served from the same origin (the vite proxy in development, a reverse
 * proxy in production) so the httpOnly refresh cookie travels without any CORS
 * or SameSite negotiation. On the server there is no page origin, and Node's
 * global fetch rejects a relative URL outright — so every call made from a
 * `load` function must pass the `fetch` SvelteKit provides. That one resolves
 * relative URLs against the incoming request and forwards its cookies, which
 * is also what makes an authenticated server-rendered call possible at all.
 * `resolveFetch` below refuses to fall back silently when that is missed.
 *
 * **The access token is held here, not in storage.** It lives for fifteen
 * minutes and is re-obtainable from the refresh cookie, so persisting it would
 * add an XSS-readable copy of a credential in exchange for nothing.
 *
 * **A 401 is recoverable exactly once.** See `request`.
 */

import { ApiError } from './errors.js';

const BASE = '/v1';

/** The in-memory access token. Set by the session store on sign-in. */
let accessToken = null;

/**
 * The refresh currently in flight, if any.
 *
 * Without this, a page that fires four requests at once on a lapsed token
 * gets four simultaneous refreshes, and since refreshing *rotates* the token
 * the last three present a credential the first has already retired — which
 * the server is right to treat as reuse of a stolen token. One shared promise
 * means one rotation.
 */
let refreshInFlight = null;

export function setAccessToken(token) {
	accessToken = token ?? null;
}

export function getAccessToken() {
	return accessToken;
}

/**
 * Picks the fetch to use, and fails loudly rather than cryptically.
 *
 * A `load` function that forgets to pass its own fetch would otherwise get
 * `TypeError: Failed to parse URL from /v1/...` from deep inside Node, which
 * says nothing about the actual mistake.
 *
 * The browser check is made per call rather than once at module load, so that
 * the answer cannot depend on when this module happened to be imported.
 */
function resolveFetch(given) {
	if (given) return given;
	if (typeof window !== 'undefined') return globalThis.fetch.bind(globalThis);

	throw new Error(
		'api: a server-side call needs the fetch SvelteKit passes to load(), ' +
			'because the base URL is relative and Node has no page origin to ' +
			"resolve it against. Pass it through: api.availability(id, date, { fetch })."
	);
}

/** Turns the error envelope from apiPlan.md §4 into an ApiError. */
function toApiError(payload, status) {
	const error = payload?.error ?? {};
	return new ApiError(
		error.code ?? fallbackCode(status),
		error.message ?? 'Something went wrong on our side.',
		error.fields ?? []
	);
}

/**
 * A status with no envelope behind it — a proxy's own 502, say, or a network
 * failure. Pages switch on `code`, so it still needs to be one of the domain's.
 */
function fallbackCode(status) {
	if (status === 401) return 'unauthenticated';
	if (status === 403) return 'forbidden';
	if (status === 404) return 'not_found';
	if (status === 409) return 'conflict';
	if (status === 429) return 'rate_limited';
	if (status >= 500) return 'internal';
	return 'invalid';
}

/**
 * One request, with a single automatic recovery from an expired access token.
 *
 * Access tokens last fifteen minutes; the refresh cookie lasts a month. So a
 * 401 on an authenticated call usually means "the short credential lapsed",
 * not "you are signed out" — and the fix is one refresh and one replay, which
 * is invisible to the caller. It is attempted only once (`retry` guards the
 * recursion), only for authenticated calls, and never for the refresh
 * endpoint itself, so a genuinely dead session ends as a 401 rather than a
 * loop.
 */
async function request(path, { method = 'GET', body, auth = false, fetch: given, retry = true } = {}) {
	const doFetch = resolveFetch(given);

	const headers = {};
	if (body !== undefined) headers['Content-Type'] = 'application/json';
	if (auth && accessToken) headers.Authorization = `Bearer ${accessToken}`;

	let response;
	try {
		response = await doFetch(`${BASE}${path}`, {
			method,
			headers,
			// The refresh cookie is httpOnly, so it only travels if credentials
			// are included. Same-origin in both environments, so this is not a
			// cross-site grant.
			credentials: 'include',
			body: body === undefined ? undefined : JSON.stringify(body)
		});
	} catch {
		// DNS failure, connection refused, offline. There is no status and no
		// envelope, but a page should still catch one type.
		throw new ApiError('unavailable', "We couldn't reach the server. Check your connection.", []);
	}

	if (response.status === 401 && auth && retry && path !== '/auth/refresh') {
		const refreshed = await refreshQuietly(doFetch);
		if (refreshed) {
			return request(path, { method, body, auth, fetch: given, retry: false });
		}
	}

	if (response.status === 204) return null;

	const payload = await response.json().catch(() => null);

	if (!response.ok) throw toApiError(payload, response.status);

	return payload;
}

/**
 * Refreshes the token pair, reporting success as a boolean.
 *
 * "Quietly" because its failure is not the caller's error: the caller asked
 * for a booking list, and if the session cannot be renewed it should see the
 * original 401, not a confusing error about refreshing.
 */
async function refreshQuietly(doFetch) {
	refreshInFlight ??= (async () => {
		try {
			const session = await request('/auth/refresh', { method: 'POST', fetch: doFetch });
			setAccessToken(session?.access_token ?? null);
			return Boolean(session?.access_token);
		} catch {
			setAccessToken(null);
			return false;
		} finally {
			// Cleared in a microtask rather than here, so every caller that
			// awaited this same refresh observes the settled result before the
			// slot reopens.
			queueMicrotask(() => {
				refreshInFlight = null;
			});
		}
	})();

	return refreshInFlight;
}

// ------------------------------------------------------------------- auth --
//
// Register, login and refresh all return a session: `{ user, access_token }`.
// The refresh token is not in that body by design — it is in an httpOnly
// cookie the browser sends back on its own.

export const register = (input, opts) =>
	request('/auth/register', { method: 'POST', body: input, ...opts });

export const login = (credentials, opts) =>
	request('/auth/login', { method: 'POST', body: credentials, ...opts });

export const refresh = (opts) => request('/auth/refresh', { method: 'POST', ...opts });

export const logout = (opts) => request('/auth/logout', { method: 'POST', ...opts });

export const me = (opts) => request('/me', { auth: true, ...opts });

export const changePassword = (input, opts) =>
	request('/auth/password/change', { method: 'POST', body: input, auth: true, ...opts });

export const forgotPassword = (email, opts) =>
	request('/auth/password/forgot', { method: 'POST', body: { email }, ...opts });

export const resetPassword = (input, opts) =>
	request('/auth/password/reset', { method: 'POST', body: input, ...opts });

// --------------------------------------------------------------- bookings --

export const availability = (courtId, date, opts) =>
	request(`/courts/${courtId}/availability?date=${encodeURIComponent(date)}`, opts);

export const listBookings = (limit = 20, opts) =>
	request(`/bookings?limit=${limit}`, { auth: true, ...opts });

export const createBooking = (input, opts) =>
	request('/bookings', { method: 'POST', body: input, auth: true, ...opts });

export const cancelBooking = (id, opts) =>
	request(`/bookings/${id}`, { method: 'DELETE', auth: true, ...opts });
export { ApiError };
