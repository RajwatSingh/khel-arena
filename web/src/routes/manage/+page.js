import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ fetch }) {
	try {
		return { arenas: await api.myArenas({ fetch }) };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/manage');
		}
		throw err;
	}
}
