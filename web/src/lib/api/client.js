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

// ----------------------------------------------------------------- arenas --

export const listArenas = (opts) => request('/arenas', opts);

export const getArena = (slug, opts) => request(`/arenas/${encodeURIComponent(slug)}`, opts);

export const listAreas = (opts) => request('/areas', opts);

/**
 * The city-wide grid: every matching court's day, in one request.
 *
 * One endpoint rather than a loop over /availability per court. That loop is
 * the N+1 the server side was built to avoid, and moving it into the browser
 * would not make it cheaper -- it would make it slower and racier.
 */
export const cityLedger = (date, { sport = 'all', area = 'all', ...opts } = {}) =>
	request(
		`/ledger?date=${encodeURIComponent(date)}` +
			`&sport=${encodeURIComponent(sport)}&area=${encodeURIComponent(area)}`,
		opts
	);

// --------------------------------------------------------------- bookings --

export const availability = (courtId, date, opts) =>
	request(`/courts/${courtId}/availability?date=${encodeURIComponent(date)}`, opts);

export const listBookings = (limit = 20, opts) =>
	request(`/bookings?limit=${limit}`, { auth: true, ...opts });

export const createBooking = (input, opts) =>
	request('/bookings', { method: 'POST', body: input, auth: true, ...opts });

export const cancelBooking = (id, opts) =>
	request(`/bookings/${id}`, { method: 'DELETE', auth: true, ...opts });

// -------------------------------------- results, reviews and galleries --

export const standings = (opts) => request('/standings', opts);

export const teamMatches = (teamId, opts) => request(`/teams/${teamId}/matches`, opts);

export const reportMatch = (input, opts) =>
	request('/matches', { method: 'POST', body: input, auth: true, ...opts });

/** The *other* captain agreeing. The server refuses a self-confirmation. */
export const confirmMatch = (id, opts) =>
	request(`/matches/${id}/confirm`, { method: 'POST', auth: true, ...opts });

export const withdrawMatch = (id, opts) =>
	request(`/matches/${id}`, { method: 'DELETE', auth: true, ...opts });

export const arenaReviews = (arenaId, opts) => request(`/arenas/${arenaId}/reviews`, opts);

/** What you said and whether you may say anything — a review is earned by
 *  having played there. */
export const myReview = (arenaId, opts) =>
	request(`/arenas/${arenaId}/reviews/mine`, { auth: true, ...opts });

export const reviewArena = (arenaId, input, opts) =>
	request(`/arenas/${arenaId}/reviews/mine`, { method: 'PUT', body: input, auth: true, ...opts });

export const deleteReview = (arenaId, opts) =>
	request(`/arenas/${arenaId}/reviews/mine`, { method: 'DELETE', auth: true, ...opts });

export const arenaPhotos = (arenaId, opts) => request(`/arenas/${arenaId}/photos`, opts);

export const addPhoto = (arenaId, input, opts) =>
	request(`/owner/arenas/${arenaId}/photos`, { method: 'POST', body: input, auth: true, ...opts });

export const deletePhoto = (photoId, opts) =>
	request(`/owner/photos/${photoId}`, { method: 'DELETE', auth: true, ...opts });

export const copyPricingRules = (toCourtId, fromCourtId, opts) =>
	request(`/owner/courts/${toCourtId}/pricing/copy`, {
		method: 'POST',
		body: { from_court_id: fromCourtId },
		auth: true,
		...opts
	});

export const playerHighlights = (userId, opts) => request(`/players/${userId}/highlights`, opts);

export const addHighlight = (input, opts) =>
	request('/me/highlights', { method: 'POST', body: input, auth: true, ...opts });

export const deleteHighlight = (id, opts) =>
	request(`/me/highlights/${id}`, { method: 'DELETE', auth: true, ...opts });

// ------------------------------------------------------------ tournaments --

export const listTournaments = (opts) => request('/tournaments', opts);

export const getTournament = (slug, opts) =>
	request(`/tournaments/${encodeURIComponent(slug)}`, opts);

export const createTournament = (input, opts) =>
	request('/tournaments', { method: 'POST', body: input, auth: true, ...opts });

export const registerTeam = (tournamentId, teamId, opts) =>
	request(`/tournaments/${tournamentId}/teams`, {
		method: 'POST',
		body: { team_id: teamId },
		auth: true,
		...opts
	});

export const withdrawTeam = (tournamentId, teamId, opts) =>
	request(`/tournaments/${tournamentId}/teams/${teamId}`, {
		method: 'DELETE',
		auth: true,
		...opts
	});

export const setEntryPaid = (tournamentId, teamId, paid, opts) =>
	request(`/tournaments/${tournamentId}/teams/${teamId}/paid`, {
		method: 'PUT',
		body: { paid },
		auth: true,
		...opts
	});

export const setTournamentStatus = (tournamentId, status, opts) =>
	request(`/tournaments/${tournamentId}/status`, {
		method: 'PUT',
		body: { status },
		auth: true,
		...opts
	});

// ------------------------------------------------------------------ owner --

export const myArenas = (opts) => request('/owner/arenas', { auth: true, ...opts });

export const createArena = (input, opts) =>
	request('/owner/arenas', { method: 'POST', body: input, auth: true, ...opts });

/**
 * Replaces the venue's details.
 *
 * PUT, not PATCH: the server writes every field it owns, so anything the body
 * leaves out is blanked. Send the whole resource — the edit form is pre-filled
 * from the current values for exactly this reason.
 */
export const updateArena = (id, input, opts) =>
	request(`/owner/arenas/${id}`, { method: 'PUT', body: input, auth: true, ...opts });

export const setArenaActive = (id, active, opts) =>
	request(`/owner/arenas/${id}/active`, { method: 'PUT', body: { active }, auth: true, ...opts });

export const createCourt = (arenaId, input, opts) =>
	request(`/owner/arenas/${arenaId}/courts`, { method: 'POST', body: input, auth: true, ...opts });

export const updateCourt = (courtId, input, opts) =>
	request(`/owner/courts/${courtId}`, { method: 'PUT', body: input, auth: true, ...opts });

export const setCourtActive = (courtId, active, opts) =>
	request(`/owner/courts/${courtId}/active`, { method: 'PUT', body: { active }, auth: true, ...opts });

export const createPricingRule = (courtId, input, opts) =>
	request(`/owner/courts/${courtId}/pricing`, { method: 'POST', body: input, auth: true, ...opts });

export const deletePricingRule = (ruleId, opts) =>
	request(`/owner/pricing/${ruleId}`, { method: 'DELETE', auth: true, ...opts });

export const arenaPayments = (arenaId, limit = 50, opts) =>
	request(`/owner/arenas/${arenaId}/payments?limit=${limit}`, { auth: true, ...opts });

/** The venue confirming that cash changed hands — the only way a cash booking
 *  becomes confirmed. */
export const markCashReceived = (paymentId, opts) =>
	request(`/owner/payments/${paymentId}/received`, { method: 'POST', auth: true, ...opts });

// ------------------------------------------------------------------ teams --

export const myTeams = (opts) => request('/teams', { auth: true, ...opts });

export const getTeam = (id, opts) => request(`/teams/${id}`, { auth: true, ...opts });

export const createTeam = (input, opts) =>
	request('/teams', { method: 'POST', body: input, auth: true, ...opts });

export const updateTeam = (id, input, opts) =>
	request(`/teams/${id}`, { method: 'PUT', body: input, auth: true, ...opts });

/** The code identifies the team, so there is no team id to get wrong. */
export const joinTeam = (code, opts) =>
	request('/teams/join', { method: 'POST', body: { code }, auth: true, ...opts });

export const addTeamMember = (teamId, userId, opts) =>
	request(`/teams/${teamId}/members`, { method: 'POST', body: { user_id: userId }, auth: true, ...opts });

/** One call for both "remove them" and "I'm leaving": the server decides which. */
export const removeTeamMember = (teamId, userId, opts) =>
	request(`/teams/${teamId}/members/${userId}`, { method: 'DELETE', auth: true, ...opts });

export const transferCaptaincy = (teamId, userId, opts) =>
	request(`/teams/${teamId}/captain`, { method: 'PUT', body: { user_id: userId }, auth: true, ...opts });

export const rotateJoinCode = (teamId, opts) =>
	request(`/teams/${teamId}/join-code`, { method: 'POST', auth: true, ...opts });

export const disbandTeam = (teamId, opts) =>
	request(`/teams/${teamId}`, { method: 'DELETE', auth: true, ...opts });

// ------------------------------------------------------------- matchmaking --

/**
 * The board of open games.
 *
 * `all` is what the interface's filters call their default position, and the
 * server reads it as "no filter" — so the two agree without the client having
 * to translate.
 */
export const callFeed = ({ skill = 'all', area = 'all', limit = 50, ...opts } = {}) =>
	request(
		`/calls?skill=${encodeURIComponent(skill)}&area=${encodeURIComponent(area)}&limit=${limit}`,
		opts
	);

export const myCalls = (opts) => request('/calls/mine', { auth: true, ...opts });

export const getCall = (id, opts) => request(`/calls/${id}`, { auth: true, ...opts });

export const createCall = (input, opts) =>
	request('/calls', { method: 'POST', body: input, auth: true, ...opts });

export const updateCall = (id, input, opts) =>
	request(`/calls/${id}`, { method: 'PUT', body: input, auth: true, ...opts });

export const cancelCall = (id, opts) =>
	request(`/calls/${id}/cancel`, { method: 'POST', auth: true, ...opts });

export const deleteCall = (id, opts) =>
	request(`/calls/${id}`, { method: 'DELETE', auth: true, ...opts });

export const respondToCall = (id, message, opts) =>
	request(`/calls/${id}/responses`, { method: 'POST', body: { message }, auth: true, ...opts });

export const withdrawFromCall = (id, opts) =>
	request(`/calls/${id}/responses`, { method: 'DELETE', auth: true, ...opts });

export const acceptResponder = (callId, userId, opts) =>
	request(`/calls/${callId}/responses/${userId}/accept`, { method: 'POST', auth: true, ...opts });

// ---------------------------------------------------------------- payments --

/** Which gateways this deployment can actually take money through. */
export const paymentProviders = (opts) => request('/payments/providers', opts);

/**
 * Starts a payment and returns how to hand the player to the gateway.
 *
 * The amount is not a parameter and cannot be: the server takes it from the
 * booking, which took it from the pricing rules when the hold was made.
 */
export const startCheckout = (bookingId, provider, opts) =>
	request(`/bookings/${bookingId}/checkout`, {
		method: 'POST',
		body: { provider },
		auth: true,
		...opts
	});

/** The state of the latest payment on a booking, for polling after a redirect. */
export const paymentStatus = (bookingId, opts) =>
	request(`/bookings/${bookingId}/payment`, { auth: true, ...opts });

/**
 * Sends the browser to the gateway.
 *
 * Two shapes, because providers differ: Khalti gives a URL to visit, eSewa
 * wants a form POSTed to it with signed fields. A form is built and submitted
 * for the second rather than trying to express it as a link, because the
 * signed fields have to travel in a body.
 *
 * This navigates away, so it never returns.
 */
export function redirectToGateway(checkout) {
	if (checkout.method === 'GET') {
		window.location.assign(checkout.url);
		return;
	}

	const form = document.createElement('form');
	form.method = 'POST';
	form.action = checkout.url;

	for (const [name, value] of Object.entries(checkout.fields ?? {})) {
		const input = document.createElement('input');
		input.type = 'hidden';
		input.name = name;
		input.value = value;
		form.append(input);
	}

	// Must be in the document to submit.
	document.body.append(form);
	form.submit();
}
export { ApiError };
