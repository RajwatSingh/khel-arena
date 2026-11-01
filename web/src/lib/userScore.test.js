import { describe, expect, it } from 'vitest';
import { isBanned, isLateCancel, penaltyPreview } from './userScore.js';

const HOUR = 60 * 60 * 1000;
const now = Date.parse('2026-11-08T10:00:00Z');
const startsIn = (ms) => new Date(now + ms).toISOString();

describe('isLateCancel', () => {
	it('is late for a confirmed booking under five hours away', () => {
		expect(isLateCancel({ status: 'confirmed', starts_at: startsIn(4 * HOUR) }, now)).toBe(true);
	});

	it('is not late five or more hours away', () => {
		expect(isLateCancel({ status: 'confirmed', starts_at: startsIn(5 * HOUR) }, now)).toBe(false);
	});

	it('never counts an unpaid hold', () => {
		expect(isLateCancel({ status: 'pending', starts_at: startsIn(HOUR) }, now)).toBe(false);
	});
});

describe('penaltyPreview', () => {
	it('takes off twenty points', () => {
		expect(penaltyPreview(80)).toEqual({ before: 80, after: 60, bans: false });
	});

	it('bans at ten or below, and never goes under zero', () => {
		expect(penaltyPreview(30)).toEqual({ before: 30, after: 10, bans: true });
		expect(penaltyPreview(10)).toEqual({ before: 10, after: 0, bans: true });
	});
});

describe('isBanned', () => {
	it('is banned only while the date is ahead', () => {
		expect(isBanned(null, now)).toBe(false);
		expect(isBanned(startsIn(HOUR), now)).toBe(true);
		expect(isBanned(startsIn(-HOUR), now)).toBe(false);
	});
});
