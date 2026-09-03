import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/index.js';

/**
 * The venue operator's home: every arena they run, with the bookings, the
 * money taken and the cash still owed rolled up across all of them.
 *
 * Guarded twice over. An unauthenticated caller is bounced to sign in; a
 * signed-in *player* is sent to their booking list, because this portal is
 * only meaningful for an arena_owner account and the owner endpoints below
 * would hand them nothing anyway.
 */
export async function load({ fetch }) {
	let me;
	try {
		me = await api.me({ fetch });
	} catch (err) {
		if (err instanceof ApiError && err.code === 'unauthenticated') {
			redirect(303, '/login?next=/dashboard');
		}
		throw err;
	}

	if (me.account_type !== 'arena_owner') {
		redirect(303, '/bookings');
	}

	const arenas = await api.myArenas({ fetch });

	// One payments call per venue. The list is an operator's own arenas —
	// a handful, not a feed — so the fan-out is bounded and parallel.
	const ledgers = await Promise.all(
		arenas.map((a) =>
			api
				.arenaPayments(a.id, 200, { fetch })
				.then((payments) => ({ arena: a, payments }))
		)
	);

	const venues = ledgers.map(({ arena, payments }) => {
		const earned = payments
			.filter((p) => p.status === 'verified')
			.reduce((sum, p) => sum + p.amount_npr, 0);
		const paidBookings = payments.filter((p) => p.status === 'verified').length;
		const awaitingCash = payments.filter(
			(p) => p.provider === 'cash' && p.status === 'initiated'
		);

		return {
			id: arena.id,
			slug: arena.slug,
			name: arena.name,
			area: arena.area,
			city: arena.city,
			is_active: arena.is_active,
			court_count: arena.court_count,
			earned_npr: earned,
			paid_bookings: paidBookings,
			awaiting_cash_count: awaitingCash.length,
			awaiting_cash_npr: awaitingCash.reduce((sum, p) => sum + p.amount_npr, 0)
		};
	});

	const totals = venues.reduce(
		(t, v) => ({
			earned_npr: t.earned_npr + v.earned_npr,
			paid_bookings: t.paid_bookings + v.paid_bookings,
			awaiting_cash_count: t.awaiting_cash_count + v.awaiting_cash_count,
			awaiting_cash_npr: t.awaiting_cash_npr + v.awaiting_cash_npr
		}),
		{ earned_npr: 0, paid_bookings: 0, awaiting_cash_count: 0, awaiting_cash_npr: 0 }
	);

	return { operator: me, venues, totals };
}
