import { api } from '$lib/api/index.js';

const SKILLS = ['casual', 'intermediate', 'competitive', 'semi_pro'];

/**
 * The board of open games.
 *
 * Filters live in the URL like the court board's do, so a filtered view is
 * shareable and the back button walks back through them — and so the fetch
 * belongs here, where every change already re-runs.
 */
export async function load({ url, fetch }) {
	const skill = SKILLS.includes(url.searchParams.get('skill'))
		? url.searchParams.get('skill')
		: 'all';

	const areas = await api.listAreas({ fetch });
	const area = areas.includes(url.searchParams.get('area')) ? url.searchParams.get('area') : 'all';

	const calls = await api.callFeed({ skill, area, fetch });

	return { calls, skill, area, areas, skills: SKILLS };
}
