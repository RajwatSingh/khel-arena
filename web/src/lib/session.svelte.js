import { api } from './api/index.js';
import { setAccessToken } from './api/client.js';

/**
 * Who is signed in, and the one place the access token is handed to the
 * transport.
 *
 * Every path that produces a session — sign in, sign up, and the silent
 * restore on boot — goes through `adopt` below, so there is exactly one line
 * that calls setAccessToken and no way to end up signed in with the client
 * still unauthenticated. That was the bug this replaced: client.js exported
 * setAccessToken and nothing anywhere called it, so every authenticated
 * request went out with no Authorization header.
 */

let current = $state(null);

/** Takes a session from any source: stores the user, arms the transport. */
function adopt(next) {
	current = next ?? null;
	setAccessToken(next?.access_token ?? null);
	return current;
}

export const session = {
	/**
	 * Picks the signed-in user back up after a reload. Client only.
	 *
	 * Against the real API there is nothing stored to read: the access token
	 * was deliberately never persisted, and the refresh token is in an
	 * httpOnly cookie the browser will send but JavaScript cannot see. So
	 * "restore" means asking the server — POST /v1/auth/refresh, which either
	 * returns a fresh session on the strength of that cookie or 401s, in which
	 * case we were signed out and now know it.
	 *
	 * Asynchronous for that reason, where the mock's was synchronous. Callers
	 * that do not await it still work: the store is reactive, so the header
	 * updates when the answer lands.
	 */
	async restore() {
		// The mock keeps its session in sessionStorage and hands it back
		// synchronously. Feature-detected rather than branched on a flag so
		// that switching $lib/api over to the real client needs no edit here.
		if (typeof api.restore === 'function') {
			return adopt(api.restore());
		}

		try {
			return adopt(await api.refresh());
		} catch {
			// No cookie, or an expired one. Signed out is the correct answer,
			// not an error worth showing anybody.
			return adopt(null);
		}
	},

	get user() {
		return current?.user ?? null;
	},

	get signedIn() {
		return current !== null;
	},

	async signIn(credentials) {
		return adopt(await api.login(credentials));
	},

	async signUp(input) {
		return adopt(await api.register(input));
	},

	/**
	 * Clears the session locally whatever the server says.
	 *
	 * The request revokes the refresh token, which matters, but if it fails
	 * the user still asked to be signed out — leaving them apparently signed
	 * in on a shared machine because a network call failed is the worse
	 * outcome of the two.
	 */
	async signOut() {
		try {
			await api.logout();
		} finally {
			adopt(null);
		}
	}
};

/** Ticks once a second while any hold is counting down. */
export function clock() {
	let now = $state(Date.now());

	$effect(() => {
		const id = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(id);
	});

	return {
		get now() {
			return now;
		}
	};
}
