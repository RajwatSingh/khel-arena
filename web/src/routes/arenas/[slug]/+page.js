import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		return { arena: await api.getArena(params.slug, { fetch }) };
	} catch (err) {
		// A slug nobody has is a 404 page, not an error banner: the address
		// itself is wrong, so there is nothing on this page to recover to.
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No arena at that address.');
		}
		throw err;
	}
}
