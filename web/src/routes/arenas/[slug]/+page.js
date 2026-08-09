import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export function load({ params }) {
	try {
		return { arena: api.getArena(params.slug) };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No arena at that address.');
		}
		throw err;
	}
}
