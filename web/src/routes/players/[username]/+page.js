import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		return { player: await api.player(params.username, { fetch }) };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No player by that name.');
		}
		throw err;
	}
}
