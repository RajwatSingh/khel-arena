/**
 * Stand-in inventory for the arenas Khel Arena books.
 *
 * Shapes follow apiPlan.md §5–6: snake_case keys, `HH:MM` day times, money as
 * whole NPR integers. Pricing rules mirror `domain.PricingRule` — an absolute
 * rate for the half-open hour window [start_hour, end_hour) on the listed ISO
 * weekdays, highest priority winning.
 */

export const ARENA_TZ = 'Asia/Kathmandu';

const WEEKNIGHTS = [1, 2, 3, 4, 5];
const WEEKEND = [6, 7];
const EVERY_DAY = [1, 2, 3, 4, 5, 6, 7];

export const arenas = [
	{
		id: 'a1b0d2f4-0000-4000-8000-000000000001',
		slug: 'dhuku-futsal',
		name: 'Dhuku Futsal',
		area: 'Jhamsikhel',
		city: 'Kathmandu',
		opens_at: '06:00',
		closes_at: '22:00',
		rating: 4.7,
		review_count: 312,
		phone: '+977 1 5548812',
		description:
			'Two covered courts behind the Jhamsikhel bus stop, laid with 40mm turf in 2023. The 7-a-side court takes the league fixtures; the smaller one is where most weeknight games happen.',
		amenities: ['Covered', 'Floodlit', 'Changing rooms', 'Parking', 'Bibs provided'],
		courts: [
			{
				id: 'c0000000-0000-4000-8000-000000000011',
				name: 'Court A',
				sport: 'futsal',
				format: '7-a-side',
				surface: '40mm turf, covered',
				base_price_npr: 1400,
				rules: [
					{
						label: 'Evening peak',
						days: WEEKNIGHTS,
						start_hour: 17,
						end_hour: 21,
						price_npr: 2100,
						is_peak: true,
						priority: 20
					},
					{
						label: 'Weekend evening',
						days: WEEKEND,
						start_hour: 17,
						end_hour: 21,
						price_npr: 2300,
						is_peak: true,
						priority: 20
					},
					{
						label: 'Weekend daytime',
						days: WEEKEND,
						start_hour: 10,
						end_hour: 17,
						price_npr: 1700,
						is_peak: false,
						priority: 10
					},
					{
						label: 'Morning rate',
						days: EVERY_DAY,
						start_hour: 6,
						end_hour: 10,
						price_npr: 1100,
						is_peak: false,
						priority: 15
					}
				]
			},
			{
				id: 'c0000000-0000-4000-8000-000000000012',
				name: 'Court B',
				sport: 'futsal',
				format: '5-a-side',
				surface: '40mm turf, covered',
				base_price_npr: 1100,
				rules: [
					{
						label: 'Evening peak',
						days: WEEKNIGHTS,
						start_hour: 17,
						end_hour: 21,
						price_npr: 1700,
						is_peak: true,
						priority: 20
					},
					{
						label: 'Weekend evening',
						days: WEEKEND,
						start_hour: 17,
						end_hour: 21,
						price_npr: 1850,
						is_peak: true,
						priority: 20
					},
					{
						label: 'Weekend daytime',
						days: WEEKEND,
						start_hour: 10,
						end_hour: 17,
						price_npr: 1350,
						is_peak: false,
						priority: 10
					}
				]
			}
		]
	},
	{
		id: 'a1b0d2f4-0000-4000-8000-000000000002',
		slug: 'bagmati-turf',
		name: 'Bagmati Turf',
		area: 'Kupondole',
		city: 'Kathmandu',
		opens_at: '05:00',
		closes_at: '23:00',
		rating: 4.4,
		review_count: 188,
		phone: '+977 1 5260904',
		description:
			'Open-air, on the embankment road. Opens at five for the morning crowd and runs to eleven. The cricket net at the far end is booked separately and takes half the ground when it is in use.',
		amenities: ['Floodlit', 'Cricket net', 'Tea shop', 'Parking'],
		courts: [
			{
				id: 'c0000000-0000-4000-8000-000000000021',
				name: 'Main ground',
				sport: 'futsal',
				format: '7-a-side',
				surface: '50mm turf, open air',
				base_price_npr: 1200,
				rules: [
					{
						label: 'Evening peak',
						days: EVERY_DAY,
						start_hour: 18,
						end_hour: 22,
						price_npr: 1900,
						is_peak: true,
						priority: 20
					},
					{
						label: 'Dawn rate',
						days: EVERY_DAY,
						start_hour: 5,
						end_hour: 8,
						price_npr: 800,
						is_peak: false,
						priority: 25
					}
				]
			},
			{
				id: 'c0000000-0000-4000-8000-000000000022',
				name: 'Cricket net',
				sport: 'cricket_net',
				format: 'Single lane',
				surface: 'Matting over concrete',
				base_price_npr: 700,
				rules: [
					{
						label: 'Evening',
						days: EVERY_DAY,
						start_hour: 17,
						end_hour: 21,
						price_npr: 1000,
						is_peak: true,
						priority: 20
					}
				]
			}
		]
	},
	{
		id: 'a1b0d2f4-0000-4000-8000-000000000003',
		slug: 'chandragiri-sports-hub',
		name: 'Chandragiri Sports Hub',
		area: 'Bhaisepati',
		city: 'Lalitpur',
		opens_at: '06:00',
		closes_at: '22:00',
		rating: 4.8,
		review_count: 96,
		phone: '+977 1 5591220',
		description:
			'Purpose-built indoor hall with a sprung basketball floor and two badminton courts marked over it. The futsal court is separate and the only one in the valley with a proper sports-vinyl surface.',
		amenities: ['Indoor', 'Air-conditioned', 'Showers', 'Spectator seating', 'Café'],
		courts: [
			{
				id: 'c0000000-0000-4000-8000-000000000031',
				name: 'Vinyl court',
				sport: 'futsal',
				format: '5-a-side',
				surface: 'Sports vinyl, indoor',
				base_price_npr: 1600,
				rules: [
					{
						label: 'Evening peak',
						days: EVERY_DAY,
						start_hour: 17,
						end_hour: 21,
						price_npr: 2400,
						is_peak: true,
						priority: 20
					}
				]
			},
			{
				id: 'c0000000-0000-4000-8000-000000000032',
				name: 'Main hall',
				sport: 'basketball',
				format: 'Full court',
				surface: 'Sprung maple',
				base_price_npr: 1800,
				rules: [
					{
						label: 'Evening peak',
						days: EVERY_DAY,
						start_hour: 17,
						end_hour: 21,
						price_npr: 2600,
						is_peak: true,
						priority: 20
					}
				]
			},
			{
				id: 'c0000000-0000-4000-8000-000000000033',
				name: 'Badminton 1',
				sport: 'badminton',
				format: 'Singles or doubles',
				surface: 'Mat over maple',
				base_price_npr: 600,
				rules: [
					{
						label: 'Evening peak',
						days: EVERY_DAY,
						start_hour: 17,
						end_hour: 21,
						price_npr: 900,
						is_peak: true,
						priority: 20
					}
				]
			}
		]
	},
	{
		id: 'a1b0d2f4-0000-4000-8000-000000000004',
		slug: 'ranipokhari-courts',
		name: 'Ranipokhari Courts',
		area: 'Chabahil',
		city: 'Kathmandu',
		opens_at: '06:00',
		closes_at: '21:00',
		rating: 4.1,
		review_count: 241,
		phone: '+977 1 4471335',
		description:
			'The oldest turf on this side of the ring road and priced like it. Rooftop, so games are called off when the monsoon arrives, and the last slot ends at nine.',
		amenities: ['Rooftop', 'Floodlit', 'Bibs provided'],
		courts: [
			{
				id: 'c0000000-0000-4000-8000-000000000041',
				name: 'Rooftop turf',
				sport: 'futsal',
				format: '5-a-side',
				surface: '30mm turf, rooftop',
				base_price_npr: 900,
				rules: [
					{
						label: 'Evening peak',
						days: EVERY_DAY,
						start_hour: 17,
						end_hour: 21,
						price_npr: 1400,
						is_peak: true,
						priority: 20
					}
				]
			}
		]
	}
];

/** Every court, flattened, each carrying a back-reference to its arena. */
export const courts = arenas.flatMap((arena) =>
	arena.courts.map((court) => ({
		...court,
		arena_id: arena.id,
		arena_name: arena.name,
		arena_slug: arena.slug,
		arena_area: arena.area,
		opens_at: arena.opens_at,
		closes_at: arena.closes_at
	}))
);

export function arenaBySlug(slug) {
	return arenas.find((a) => a.slug === slug);
}

export function courtById(id) {
	return courts.find((c) => c.id === id);
}

export const SPORT_LABELS = {
	futsal: 'Futsal',
	basketball: 'Basketball',
	badminton: 'Badminton',
	cricket_net: 'Cricket net',
	tennis: 'Tennis'
};
