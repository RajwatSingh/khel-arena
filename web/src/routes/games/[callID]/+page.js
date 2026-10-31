import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		// The anonymous view: it renders for every visitor and holds no
		// session. The page fetches the game again as the viewer once they are
		// known, for the parts that are theirs (see `personal` in the page).
		return { call: await api.getCall(params.callID, { fetch }) };
	} catch (err) {
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No game at that address.');
		}
		throw err;
	}
}
