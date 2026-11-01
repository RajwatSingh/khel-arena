import { describe, expect, it } from 'vitest';
import { markLeftForGateway, takeGatewayReturn } from './gatewayReturn.js';

function memoryStorage() {
	const data = new Map();
	return {
		getItem: (k) => (data.has(k) ? data.get(k) : null),
		setItem: (k, v) => data.set(k, String(v)),
		removeItem: (k) => data.delete(k)
	};
}

describe('gateway return note', () => {
	it('is false when the tab never left for a gateway', () => {
		expect(takeGatewayReturn(1_000, memoryStorage())).toBe(false);
	});

	it('is true once after leaving, then cleared', () => {
		const storage = memoryStorage();
		markLeftForGateway(1_000, storage);

		expect(takeGatewayReturn(61_000, storage)).toBe(true);
		expect(takeGatewayReturn(62_000, storage)).toBe(false);
	});

	it('expires once the hold would be long gone', () => {
		const storage = memoryStorage();
		markLeftForGateway(0, storage);

		expect(takeGatewayReturn(31 * 60_000, storage)).toBe(false);
		// And the stale note is cleared rather than left to fire later.
		expect(storage.getItem('khel:left-for-gateway')).toBeNull();
	});

	it('survives storage that throws', () => {
		const broken = {
			getItem() {
				throw new Error('blocked');
			},
			setItem() {
				throw new Error('blocked');
			},
			removeItem() {}
		};

		expect(() => markLeftForGateway(0, broken)).not.toThrow();
		expect(takeGatewayReturn(0, broken)).toBe(false);
	});
});
