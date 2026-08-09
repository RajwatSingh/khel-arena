/**
 * Kathmandu wall-clock helpers.
 *
 * Asia/Kathmandu is a fixed UTC+05:45 with no daylight saving, so the offset
 * can be a constant rather than a timezone database lookup. Arena opening
 * hours and pricing rules are written in this wall clock; instants on the wire
 * are RFC 3339 UTC (apiPlan.md §5).
 */

const OFFSET_MINUTES = 5 * 60 + 45;
const MS_PER_MINUTE = 60_000;

/** `2026-08-09` for the calendar date in Kathmandu at a given instant. */
export function localDate(instant = new Date()) {
	const shifted = new Date(instant.getTime() + OFFSET_MINUTES * MS_PER_MINUTE);
	return shifted.toISOString().slice(0, 10);
}

/** The instant at which `HH:MM` on a `YYYY-MM-DD` Kathmandu date occurs. */
export function instantAt(date, minutesFromMidnight) {
	const [y, m, d] = date.split('-').map(Number);
	return new Date(Date.UTC(y, m - 1, d, 0, 0) + (minutesFromMidnight - OFFSET_MINUTES) * MS_PER_MINUTE);
}

/** Minutes from midnight for a `HH:MM` day time. */
export function dayTimeMinutes(dayTime) {
	const [h, m] = dayTime.split(':').map(Number);
	return h * 60 + m;
}

/** `HH:MM` from minutes from midnight. */
export function minutesToDayTime(minutes) {
	const h = Math.floor(minutes / 60) % 24;
	const m = minutes % 60;
	return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
}

/** ISO weekday for a Kathmandu calendar date: 1 = Monday … 7 = Sunday. */
export function isoWeekday(date) {
	const [y, m, d] = date.split('-').map(Number);
	const js = new Date(Date.UTC(y, m - 1, d)).getUTCDay();
	return js === 0 ? 7 : js;
}

/** Shift a `YYYY-MM-DD` by whole days. */
export function addDays(date, days) {
	const [y, m, d] = date.split('-').map(Number);
	const next = new Date(Date.UTC(y, m - 1, d + days));
	return next.toISOString().slice(0, 10);
}

const WEEKDAY_SHORT = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const MONTH_SHORT = [
	'Jan',
	'Feb',
	'Mar',
	'Apr',
	'May',
	'Jun',
	'Jul',
	'Aug',
	'Sep',
	'Oct',
	'Nov',
	'Dec'
];

/** `Sun 9 Aug` — the form used on date rails and booking rows. */
export function formatDate(date) {
	const [y, m, d] = date.split('-').map(Number);
	const weekday = WEEKDAY_SHORT[new Date(Date.UTC(y, m - 1, d)).getUTCDay()];
	return `${weekday} ${d} ${MONTH_SHORT[m - 1]}`;
}

/** `Sunday 9 August` — long form for section openers. */
export function formatDateLong(date) {
	const [y, m, d] = date.split('-').map(Number);
	const js = new Date(Date.UTC(y, m - 1, d));
	const weekday = [
		'Sunday',
		'Monday',
		'Tuesday',
		'Wednesday',
		'Thursday',
		'Friday',
		'Saturday'
	][js.getUTCDay()];
	const month = [
		'January',
		'February',
		'March',
		'April',
		'May',
		'June',
		'July',
		'August',
		'September',
		'October',
		'November',
		'December'
	][m - 1];
	return `${weekday} ${d} ${month}`;
}

/** `20:00` — the Kathmandu wall-clock time of an RFC 3339 instant. */
export function formatTime(iso) {
	const shifted = new Date(new Date(iso).getTime() + OFFSET_MINUTES * MS_PER_MINUTE);
	return shifted.toISOString().slice(11, 16);
}

/** `1,400` — NPR is always a whole rupee (apiPlan.md §5). */
export function formatNPR(amount) {
	return amount.toLocaleString('en-IN');
}

/** `4:31` — a countdown, for hold expiry. */
export function formatCountdown(ms) {
	const total = Math.max(0, Math.round(ms / 1000));
	const minutes = Math.floor(total / 60);
	const seconds = total % 60;
	return `${minutes}:${String(seconds).padStart(2, '0')}`;
}
