import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

// Private page: rendered in the browser only. A session lives in the visitor's
// own tab; the server renders for everyone at once and keeps none, so it
// cannot load this page's data on anyone's behalf.
export const ssr = false;

/**
 * A game is always posted for one of the caller's own bookings, so the page
 * starts by listing them. Only confirmed bookings that haven't started
 * qualify: an unpaid hold can lapse and leave the game with no court.
 *
 * A booking can carry one game. The caller's games are fetched alongside so
 * a booking that already has one links to it instead of offering a second.
 */
export async function load({ fetch, url }) {
	try {
		const [bookings, calls] = await Promise.all([
			api.listBookings(50, { fetch }),
			api.myCalls({ fetch })
		]);
		const now = Date.now();

		const gameFor = Object.fromEntries(
			(calls ?? []).filter((c) => c.booking_id).map((c) => [c.booking_id, c.id])
		);

		return {
			bookings: bookings
				.filter((b) => b.status === 'confirmed' && new Date(b.starts_at).getTime() > now)
				.sort((a, b) => a.starts_at.localeCompare(b.starts_at))
				.map((b) => ({ ...b, game_id: gameFor[b.id] ?? null })),
			// Lets a link elsewhere open the form with a booking already picked.
			preselect: url.searchParams.get('booking')
		};
	} catch (err) {
		// Signed out: send them to sign in and come back here afterwards.
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/games/new');
		}
		throw err;
	}
}
