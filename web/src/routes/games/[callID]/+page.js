import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		return { call: await api.getCall(params.callID, { fetch }) };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No game at that address.');
		}
		throw err;
	}
}
