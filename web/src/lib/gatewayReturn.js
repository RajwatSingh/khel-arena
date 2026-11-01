/**
 * Bringing a player back cleanly when they press Back on eSewa or Khalti.
 *
 * Leaving for a gateway is a full page navigation. Pressing Back returns to
 * whatever page started it, and that page is in no fit state: restored from
 * the back/forward cache it is frozen mid-"Opening Khalti…" with its buttons
 * disabled, and reloaded instead it has forgotten the hold it took (the arena
 * page keeps holds in memory). /bookings is the one page that always shows
 * every hold with all its options (each gateway, cash, cancel), so that is
 * where a returning player belongs.
 *
 * So just before leaving we leave a note in sessionStorage (per tab, gone when
 * the tab closes), and the layout reads it on the way back in. The note is
 * read once and expires, so a tab reopened much later behaves normally.
 */

const KEY = 'khel:left-for-gateway';

// Comfortably past a booking hold (15 minutes). After that there is nothing
// left to pay for, and a redirect would only be a surprise.
const MAX_AGE_MS = 30 * 60_000;

function sessionStore() {
	try {
		return globalThis.sessionStorage ?? null;
	} catch {
		// Blocked storage throws on access in some browsers.
		return null;
	}
}

/** Notes that this tab is about to leave for a payment gateway. */
export function markLeftForGateway(now = Date.now(), storage = sessionStore()) {
	try {
		storage?.setItem(KEY, String(now));
	} catch {
		// Full or blocked storage: Back just behaves as it did before.
	}
}

/**
 * True if this tab left for a gateway recently. Clears the note either way,
 * so it answers true at most once per trip.
 */
export function takeGatewayReturn(now = Date.now(), storage = sessionStore()) {
	try {
		const raw = storage?.getItem(KEY);
		if (raw == null) return false;
		storage.removeItem(KEY);
		const at = Number(raw);
		return Number.isFinite(at) && now - at >= 0 && now - at < MAX_AGE_MS;
	} catch {
		return false;
	}
}
