import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

/**
 * The form offers the caller's own upcoming bookings to attach the game to,
 * so "we have Dhuku at 8, need two more" is a choice rather than something
 * typed out. Only bookings that are still live and still ahead qualify.
 */
export async function load({ fetch }) {
	try {
		const bookings = await api.listBookings(20, { fetch });
		const now = Date.now();

		return {
			bookings: bookings.filter(
				(b) =>
					new Date(b.starts_at).getTime() > now &&
					(b.status === 'pending' || b.status === 'confirmed')
			)
		};
	} catch (err) {
		// Signed out: the page itself will send them to sign in, and an empty
		// list is the right state to render in the meantime.
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/games/new');
		}
		throw err;
	}
}
