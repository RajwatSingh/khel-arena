import { describe, expect, it } from 'vitest';
import { formatDate, formatDateLong } from './time.js';

describe('formatDate / formatDateLong', () => {
	it('formats a calendar date', () => {
		expect(formatDate('2026-11-03')).toBe('Tue 3 Nov');
		expect(formatDateLong('2026-11-03')).toBe('Tuesday 3 November');
	});

	// API fields like starts_at are full timestamps; these used to come out as
	// "undefined NaN November".
	it('formats a timestamp as its date in Kathmandu', () => {
		expect(formatDateLong('2026-11-03T12:15:00Z')).toBe('Tuesday 3 November');
		expect(formatDate(new Date('2026-11-03T12:15:00Z'))).toBe('Tue 3 Nov');
	});

	it('rolls over to the next day after midnight in Kathmandu', () => {
		// 20:00 UTC is 01:45 the next morning at UTC+05:45.
		expect(formatDate('2026-11-02T20:00:00Z')).toBe('Tue 3 Nov');
	});
});
