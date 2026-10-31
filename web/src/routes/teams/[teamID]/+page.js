import { error, redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

// Private page: rendered in the browser only. A session lives in the visitor's
// own tab; the server renders for everyone at once and keeps none, so it
// cannot load this page's data on anyone's behalf.
export const ssr = false;

export async function load({ params, fetch }) {
	try {
		return { team: await api.getTeam(params.teamID, { fetch }) };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, `/login?next=/teams/${params.teamID}`);
		}
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No team at that address.');
		}
		throw err;
	}
}
