/**
 * In-memory stand-in for the Go service described in apiPlan.md §3.
 *
 * It answers the same endpoint set with the same payload shapes, so wiring the
 * real API later means changing which module `$lib/api` re-exports — not
 * touching a page. Two rules the real service holds are reproduced faithfully
 * because the interface depends on them:
 *
 *   - The availability grid is projected, never stored: hours are walked from
 *     opening to closing and priced by rule, exactly as `domain.BuildGrid` and
 *     `domain.ResolvePrice` do.
 *   - A new booking is an unpaid hold that expires. `BOOKING_HOLD_WINDOW`
 *     defaults to 15 minutes; the slot goes back on the board when it lapses.
 */

import { ApiError } from './errors.js';
import { arenas, arenaBySlug, courtById, courts } from './fixtures.js';
import { addDays, dayTimeMinutes, instantAt, isoWeekday, localDate } from '../time.js';

export const HOLD_WINDOW_MS = 15 * 60 * 1000;

// Re-exported so the many pages that already import ApiError from here keep
// working. It is defined in ./errors.js, which client.js shares.
export { ApiError };

// --------------------------------------------------------------- pricing ---

/** Highest priority wins; ties break toward the narrower window. */
function resolvePrice(court, date, hour) {
	const weekday = isoWeekday(date);
	let winner = null;

	for (const rule of court.rules ?? []) {
		if (!rule.days.includes(weekday)) continue;
		if (hour < rule.start_hour || hour >= rule.end_hour) continue;
		if (!winner) {
			winner = rule;
			continue;
		}
		if (rule.priority > winner.priority) {
			winner = rule;
		} else if (rule.priority === winner.priority) {
			const span = rule.end_hour - rule.start_hour;
			const winning = winner.end_hour - winner.start_hour;
			if (span < winning) winner = rule;
		}
	}

	return winner
		? { price_npr: winner.price_npr, is_peak: winner.is_peak, rule: winner.label }
		: { price_npr: court.base_price_npr, is_peak: false, rule: 'Base rate' };
}

// -------------------------------------------------------------- occupancy --

/**
 * Deterministic stand-in for bookings already in the database.
 *
 * Stable for a given court, date and hour so the same grid renders on the
 * server and in the browser, and so a page can be reloaded without the pitch
 * appearing to empty out.
 */
function hash(...parts) {
	let h = 2166136261;
	const key = parts.join('|');
	for (let i = 0; i < key.length; i++) {
		h ^= key.charCodeAt(i);
		h = Math.imul(h, 16777619);
	}
	return (h >>> 0) % 100;
}

function seededBooked(courtId, date, hour) {
	// Evenings go first, mornings almost never fill.
	const pressure = hour >= 17 && hour < 21 ? 72 : hour >= 15 ? 44 : hour >= 10 ? 26 : 12;
	return hash(courtId, date, hour) < pressure;
}

// ------------------------------------------------------------------ state --

const held = new Map(); // `${courtId}|${startsAt}` -> booking id
let bookings = [];
let session = null;
let nextRef = 4817;

/**
 * The mock keeps its state in sessionStorage so a reload behaves the way the
 * real thing will: the refresh-token cookie survives it, and so do your
 * bookings. Nothing here outlives the browser tab.
 */
const STORE_KEY = 'khel-arena/mock';

function persist() {
	if (typeof sessionStorage === 'undefined') return;
	sessionStorage.setItem(
		STORE_KEY,
		JSON.stringify({ session, bookings, nextRef, held: [...held] })
	);
}

/** Rehydrates on the client after a reload. Returns the restored session. */
export function restore() {
	if (typeof sessionStorage === 'undefined') return session;
	const raw = sessionStorage.getItem(STORE_KEY);
	if (!raw) return session;
	try {
		const saved = JSON.parse(raw);
		session = saved.session ?? null;
		bookings = saved.bookings ?? [];
		nextRef = saved.nextRef ?? nextRef;
		held.clear();
		for (const [key, value] of saved.held ?? []) held.set(key, value);
	} catch {
		sessionStorage.removeItem(STORE_KEY);
	}
	return session;
}

function isHeldElsewhere(courtId, startsAt) {
	return held.has(`${courtId}|${startsAt}`);
}

function delay(ms = 180) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

function requireSession() {
	if (!session) {
		throw new ApiError('unauthenticated', 'Sign in to continue.');
	}
	return session;
}

// ------------------------------------------------------------ availability -

/** `GET /v1/courts/{courtID}/availability?date=YYYY-MM-DD` */
export function availability(courtId, date, now = new Date()) {
	const court = courtById(courtId);
	if (!court) throw new ApiError('not_found', 'That court is not listed.');

	const openMinutes = dayTimeMinutes(court.opens_at);
	const closeMinutes = dayTimeMinutes(court.closes_at);
	const slots = [];

	for (let offset = openMinutes; offset + 60 <= closeMinutes; offset += 60) {
		const start = instantAt(date, offset);
		const end = instantAt(date, offset + 60);
    const hour = Math.floor(offset / 60)
		const { price_npr, is_peak, rule } = resolvePrice(court, date, hour);
		const startsAt = start.toISOString();
		const isBooked = seededBooked(courtId, date, hour) || isHeldElsewhere(courtId, startsAt);
		const isPast = start.getTime() < now.getTime();

		slots.push({
			starts_at: startsAt,
			ends_at: end.toISOString(),
			price_npr,
			is_peak,
			rule,
			is_booked: isBooked,
			is_past: isPast,
			available: !isBooked && !isPast
		});
	}

	return { court_id: courtId, date, slots };
}

/**
 * The ledger behind the home page: every futsal court in the city, one row
 * each, for one date. Not an endpoint in apiPlan.md §3 — it is the same
 * availability projection run per court, which the real client will do too.
 */
export function cityLedger(
	date = localDate(),
	{ sport = 'futsal', area = 'all', now = new Date() } = {}
) {
	const rows = courts
		.filter((court) => sport === 'all' || court.sport === sport)
		.filter((court) => area === 'all' || court.arena_area === area)
		.map((court) => {
			const grid = availability(court.id, date, now);
			return {
				court_id: court.id,
				court_name: court.name,
				sport: court.sport,
				format: court.format,
				// The arena a court belongs to, carried on every row: the ledger
				// links each cell to /arenas/{slug} and labels it with the venue,
				// and a row without these renders a nameless link to /arenas/undefined.
				arena_id: court.arena_id,
				arena_name: court.arena_name,
				arena_slug: court.arena_slug,
				arena_area: court.arena_area,
				slots: grid.slots
			};
		});

	const openHours = rows.reduce(
		(total, row) => total + row.slots.filter((s) => s.available).length,
		0
	);
	const cheapest = rows
		.flatMap((row) => row.slots.filter((s) => s.available))
		.reduce((min, slot) => (min === null || slot.price_npr < min ? slot.price_npr : min), null);

	return { date, rows, open_hours: openHours, cheapest_npr: cheapest };
}

// ------------------------------------------------------------------ arenas -

export function listArenas() {
	return arenas.map((arena) => ({
		...arena,
		court_count: arena.courts.length,
		from_price_npr: Math.min(...arena.courts.map((c) => c.base_price_npr))
	}));
}

export function getArena(slug) {
	const arena = arenaBySlug(slug);
	if (!arena) throw new ApiError('not_found', 'No arena at that address.');
	return arena;
}

// -------------------------------------------------------------------- auth -

const accounts = new Map([
	[
		'rajwat@khelarena.np',
		{
			password: 'kathmandu2026',
			user: {
				id: 'u0000000-0000-4000-8000-000000000001',
				full_name: 'Rajwat Singh',
				username: 'rajwat',
				email: 'rajwat@khelarena.np',
				account_type: 'player',
				skill: 'intermediate',
				position: 'Ala',
				jersey_number: 7,
				preferred_foot: 'left'
			}
		}
	]
]);

export async function login({ email, password }) {
	await delay();
	const record = accounts.get(String(email).trim().toLowerCase());
	if (!record || record.password !== password) {
		// One message for both cases: a distinct "no such account" reply is an
		// account-enumeration oracle.
		throw new ApiError('unauthenticated', 'The email or password do not match.');
	}
	session = { 
    user: record.user, 
    access_token: 'mock.access.token' 
  };
	persist();
	return session;
}

export async function register(input) {
	await delay(320);
	const fields = [];
	const email = String(input.email ?? '')
		.trim()
		.toLowerCase();

	if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email)) {
		fields.push({ field: 'email', message: 'That does not look like an email address.' });
	}
	if (accounts.has(email)) {
		fields.push({ field: 'email', message: 'That email already has an account.' });
	}
	if ((input.full_name ?? '').trim().length < 2) {
		fields.push({ field: 'full_name', message: 'Tell us what to call you.' });
	}
	if ((input.username ?? '').trim().length < 3) {
		fields.push({ field: 'username', message: 'Usernames need at least 3 characters.' });
	}
	if ((input.password ?? '').length < 10) {
		fields.push({ field: 'password', message: 'Use at least 10 characters.' });
	}
	if (fields.length) {
		throw new ApiError('invalid', fields.map((f) => f.message).join(' '), fields);
	}

	const user = {
		id: `u0000000-0000-4000-8000-${String(accounts.size + 2).padStart(12, '0')}`,
		full_name: input.full_name.trim(),
		username: input.username.trim(),
		email,
		account_type: input.account_type ?? 'player',
		skill: input.skill ?? 'casual',
		position: input.position ?? null,
		jersey_number: null,
		preferred_foot: null
	};
	accounts.set(email, { password: input.password, user });
	session = { user, access_token: 'mock.access.token' };
	persist();
	return session;
}

export function logout() {
	session = null;
	bookings = [];
	held.clear();
	persist();
}

export function currentSession() {
	return session;
}

// ---------------------------------------------------------------- bookings -

/** `POST /v1/bookings` — takes a hold, 201. */
export async function createBooking({ court_id, starts_at, note }) {
	await delay(420);
	const user = requireSession();
	const court = courtById(court_id);
	if (!court) throw new ApiError('not_found', 'That court is not listed.');

	const date = localDate(new Date(starts_at));
	const grid = availability(court_id, date);
	const slot = grid.slots.find((s) => s.starts_at === starts_at);
	if (!slot) throw new ApiError('invalid', 'That hour is not part of the court schedule.');
	if (slot.is_past) throw new ApiError('invalid', 'That hour has already passed.');
	if (!slot.available) {
		// What the exclusion constraint produces when two people race.
		throw new ApiError('conflict', 'Someone took that hour first. Pick another.');
	}

	const booking = {
		id: crypto.randomUUID(),
		reference: `KA-${nextRef++}`,
		court_id,
		starts_at,
		ends_at: slot.ends_at,
		price_npr: slot.price_npr,
		status: 'pending',
		note: note ?? '',
		hold_expires_at: new Date(Date.now() + HOLD_WINDOW_MS).toISOString(),
		created_at: new Date().toISOString(),
		user_id: user.user.id
	};

	held.set(`${court_id}|${starts_at}`, booking.id);
	bookings = [booking, ...bookings];
	persist();
	return booking;
}

/** `GET /v1/bookings` — newest first. */
export function listBookings() {
	requireSession();
	return bookings.map(reconcile);
}

export function getBooking(id) {
	requireSession();
	const booking = bookings.find((b) => b.id === id);
	if (!booking) throw new ApiError('not_found', 'No booking with that reference.');
	return reconcile(booking);
}

/**
 * Applies the janitor's rule client-side: a pending booking whose hold has
 * lapsed no longer blocks its slot.
 */
function reconcile(booking) {
	if (booking.status === 'pending' && new Date(booking.hold_expires_at) <= new Date()) {
		held.delete(`${booking.court_id}|${booking.starts_at}`);
		return { ...booking, status: 'expired' };
	}
	return booking;
}

/** Stands in for the eSewa/Khalti callback that verifies a payment. */
export async function payBooking(id) {
	await delay(700);
	requireSession();
	const booking = bookings.find((b) => b.id === id);
	if (!booking) throw new ApiError('not_found', 'No booking with that reference.');
	if (booking.status !== 'pending') {
		throw new ApiError('conflict', 'This booking is no longer waiting on payment.');
	}
	if (new Date(booking.hold_expires_at) <= new Date()) {
		held.delete(`${booking.court_id}|${booking.starts_at}`);
		throw new ApiError('conflict', 'The hold ran out and the hour went back on the board.');
	}
	booking.status = 'confirmed';
	booking.hold_expires_at = null;
	booking.paid_at = new Date().toISOString();
	persist();
	return { ...booking };
}

/** `DELETE /v1/bookings/{bookingID}` — 204. */
export async function cancelBooking(id) {
	await delay(240);
	requireSession();
	const booking = bookings.find((b) => b.id === id);
	if (!booking) throw new ApiError('not_found', 'No booking with that reference.');
	booking.status = 'cancelled';
	held.delete(`${booking.court_id}|${booking.starts_at}`);
	persist();
}

/** Every area with at least one court, for the search control. */
export function listAreas() {
	return [...new Set(courts.map((court) => court.arena_area))].sort();
}

export const dates = {
	today: () => localDate(),
	rail: (span = 7) => {
		const start = localDate();
		return Array.from({ length: span }, (_, i) => addDays(start, i));
	}
};
