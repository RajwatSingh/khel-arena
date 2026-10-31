import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

// Private page: rendered in the browser only. A session lives in the visitor's
// own tab; the server renders for everyone at once and keeps none, so it
// cannot load this page's data on anyone's behalf.
export const ssr = false;

/**
 * Signed-out visitors go to sign in, the same as every other private page.
 * Without this the page rendered a "sign in" notice instead, which also left
 * it reachable with Back after signing out.
 */
export async function load({ fetch }) {
	try {
		await api.me({ fetch });
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/bookings');
		}
		throw err;
	}
	return {};
}
