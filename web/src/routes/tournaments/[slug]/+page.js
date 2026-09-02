import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		const tournament = await api.getTournament(params.slug, { fetch });

		// The teams the viewer captains, so the entry button knows what it can
		// offer. Signed-out visitors get an empty list and a sign-in prompt.
		let teams = [];
		try {
			teams = await api.myTeams({ fetch });
		} catch {
			teams = [];
		}

		return { tournament, teams };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No tournament at that address.');
		}
		throw err;
	}
}
