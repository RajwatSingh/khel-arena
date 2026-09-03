import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ fetch }) {
	let me;
	try {
		me = await api.me({ fetch });
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/manage');
		}
		throw err;
	}

	// The back office is for venue operators. A player who lands here can only
	// register an arena the API would then refuse them — send them home.
	if (me.account_type !== 'arena_owner') {
		redirect(303, '/bookings');
	}

	return { arenas: await api.myArenas({ fetch }) };
}
