import { error } from '@sveltejs/kit';
import { browser } from '$app/environment';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		const tournament = await api.getTournament(params.slug, { fetch });

		// The teams the viewer captains, so the entry button knows what it can
		// offer. Signed-out visitors get an empty list and a sign-in prompt.
		//
		// Browser only: the server renders this page for every visitor and
		// holds no session. The page re-runs this load once the visitor's
		// session is restored (see `personal`).
		let teams = [];
		if (browser) {
			try {
				teams = await api.myTeams({ fetch });
			} catch {
				teams = [];
			}
		}

		return { tournament, teams, personal: browser };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No tournament at that address.');
		}
		throw err;
	}
}
