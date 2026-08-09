import { api } from '$lib/api/index.js';

export function load() {
	const date = api.dates.today();
	return {
		date,
		dates: api.dates.rail(7),
		areas: api.listAreas(),
		ledger: api.cityLedger(date),
		arenas: api.listArenas()
	};
}
