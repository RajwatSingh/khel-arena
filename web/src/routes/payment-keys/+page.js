import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

// Private page: rendered in the browser only, like the dashboard. The session
// lives in the visitor's own tab, so the server has nobody to load this for.
export const ssr = false;

/**
 * Every venue the owner runs, with the eSewa and Khalti keys saved on each.
 *
 * Reached from the profile menu. This is where a saved key is changed or
 * removed; the venue page only connects a provider for the first time and
 * then shows that it is active.
 */
export async function load({ fetch }) {
	let me;
	try {
		me = await api.me({ fetch });
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/payment-keys');
		}
		throw err;
	}

	if (me.account_type !== 'arena_owner') {
		redirect(303, '/bookings');
	}

	const arenas = await api.myArenas({ fetch });

	// One accounts call per venue: an owner's own handful, in parallel.
	const venues = await Promise.all(
		arenas.map((arena) =>
			api.arenaPaymentAccounts(arena.id, { fetch }).then((accounts) => ({ arena, accounts }))
		)
	);

	return { venues };
}
