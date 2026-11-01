/**
 * The user score rules, as the interface explains them. The server applies
 * them (internal/domain/booking.go); these only let a page warn a player
 * before they cancel, so keep the numbers in step.
 */

export const USER_SCORE_MAX = 100;
export const LATE_CANCEL_WINDOW_MS = 5 * 60 * 60 * 1000;
export const LATE_CANCEL_PENALTY = 20;
export const BOOKING_BAN_FLOOR = 10;

/** True when cancelling `booking` now would cost points: confirmed, and
 *  less than five hours before it starts. Unpaid holds never cost points. */
export function isLateCancel(booking, now = Date.now()) {
	if (booking?.status !== 'confirmed') return false;
	return new Date(booking.starts_at).getTime() - now < LATE_CANCEL_WINDOW_MS;
}

/** What a late cancellation would do to `score`. */
export function penaltyPreview(score) {
	const after = Math.max(score - LATE_CANCEL_PENALTY, 0);
	return { before: score, after, bans: after <= BOOKING_BAN_FLOOR };
}

/** True while a ban date is in the future. */
export function isBanned(bannedUntil, now = Date.now()) {
	return Boolean(bannedUntil) && new Date(bannedUntil).getTime() > now;
}
