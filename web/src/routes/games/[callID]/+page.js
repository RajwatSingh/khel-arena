import { error } from '@sveltejs/kit';
import { browser } from '$app/environment';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		// On the server this is the anonymous view: it renders for every
		// visitor and holds no session. The page reloads it in the browser once
		// the viewer is known, for the parts that are theirs (see `personal`).
		return { call: await api.getCall(params.callID, { fetch }), personal: browser };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No game at that address.');
		}
		throw err;
	}
}
