import { redirect } from '@sveltejs/kit';

// The board used to live here. Old links and bookmarks land on the home
// page with their filters intact.
export function GET({ url }) {
	redirect(308, '/' + url.search);
}
