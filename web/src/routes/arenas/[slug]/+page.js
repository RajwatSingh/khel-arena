import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

export async function load({ params, fetch }) {
	try {
		const arena = await api.getArena(params.slug, { fetch });

		// The gallery is public and belongs to the page's first paint. Reviews
		// are fetched by their own panel instead: what it shows depends on who
		// is signed in, which this load does not know.
		const photos = await api.arenaPhotos(arena.id, { fetch });

		return { arena, photos };
	} catch (err) {
		// A slug nobody has is a 404 page, not an error banner: the address
		// itself is wrong, so there is nothing on this page to recover to.
		if (err instanceof ApiError && err.code === 'not_found') {
			error(404, 'No arena at that address.');
		}
		throw err;
	}
}
