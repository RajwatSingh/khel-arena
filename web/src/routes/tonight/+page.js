import { api } from '$lib/api/index.js';

const SPORTS = ['futsal', 'basketball', 'badminton', 'cricket_net'];

export function load({ url }) {
	const dates = api.dates.rail(7);
	const date = dates.includes(url.searchParams.get('date'))
		? url.searchParams.get('date')
		: dates[0];
	const sport = SPORTS.includes(url.searchParams.get('sport'))
		? url.searchParams.get('sport')
		: 'futsal';
	const areas = api.listAreas();
	const area = areas.includes(url.searchParams.get('area')) ? url.searchParams.get('area') : 'all';

	return { dates, date, sport, sports: SPORTS, areas, area };
}
