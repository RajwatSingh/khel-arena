import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ fetch }) {
	try {
		return { teams: await api.myTeams({ fetch }) };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/teams');
		}
		throw err;
	}
}
