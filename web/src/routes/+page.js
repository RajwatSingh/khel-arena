import { api } from '$lib/api/index.js';
import { dates as dateHelper } from '$lib/time.js';

const SPORTS = ['futsal', 'basketball', 'badminton', 'cricket_net'];

/**
 * Every filter is read from the URL and validated against what actually
 * exists, so a hand-edited query string lands on a sensible page rather than
 * an empty one -- and so the server is never asked for a sport it would
 * reject.
 */
export async function load({ url, fetch }) {
	const dates = dateHelper.rail(7);
	const date = dates.includes(url.searchParams.get('date')) ? url.searchParams.get('date') : dates[0];
	const sport = SPORTS.includes(url.searchParams.get('sport'))
		? url.searchParams.get('sport')
		: 'futsal';

	const areas = await api.listAreas({ fetch });
	const area = areas.includes(url.searchParams.get('area')) ? url.searchParams.get('area') : 'all';

	// The board is fetched here rather than in the component because it depends
	// on nothing but these filters, and the filters live in the URL -- so every
	// change already re-runs this load. Fetching it here means the board is
	// server-rendered and shareable, where an effect in the component would
	// render an empty grid first and fill it in afterwards.
	const ledger = await api.cityLedger(date, { sport, area, fetch });

	return { dates, date, sport, sports: SPORTS, areas, area, ledger };
}
