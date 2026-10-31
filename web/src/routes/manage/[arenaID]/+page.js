import { error, redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

// Private page: rendered in the browser only. A session lives in the visitor's
// own tab; the server renders for everyone at once and keeps none, so it
// cannot load this page's data on anyone's behalf.
export const ssr = false;

/**
 * The management view for one venue.
 *
 * The arena itself is read through the public endpoint — it carries the
 * courts and their pricing rules, which the owner listing does not — and the
 * ownership check is the payments call: that one is owner-scoped, so a
 * stranger who guessed the id gets a 404 here rather than a page.
 */
export async function load({ params, fetch }) {
	let arenas;
	try {
		arenas = await api.myArenas({ fetch });
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, `/login?next=/manage/${params.arenaID}`);
		}
		throw err;
	}

	const listing = arenas.find((a) => a.id === params.arenaID);
	if (!listing) {
		error(404, 'No venue of yours at that address.');
	}

	const [arena, payments, photos, paymentAccounts] = await Promise.all([
		api.getArena(listing.slug, { fetch }),
		api.arenaPayments(params.arenaID, 50, { fetch }),
		api.arenaPhotos(params.arenaID, { fetch }),
		api.arenaPaymentAccounts(params.arenaID, { fetch })
	]);

	return { arena, listing, payments, photos, paymentAccounts };
}
