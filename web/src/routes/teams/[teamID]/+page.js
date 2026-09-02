import { error, redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

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
