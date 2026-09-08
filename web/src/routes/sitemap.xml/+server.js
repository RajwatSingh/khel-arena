import { api } from '$lib/api/index.js';

/**
 * Public routes only — the private ones are listed in robots.txt.
 *
 * The arena list is fetched, which makes two things load-bearing. The `fetch`
 * SvelteKit hands this endpoint has to be passed through: the API client's
 * base URL is relative and Node has no page origin to resolve it against, so
 * calling without it throws. And the call has to be awaited before `.map`,
 * because `request` is async — mapping the Promise itself is a TypeError that
 * only ever shows up as a 500 on a route nothing in development requests.
 *
 * A failure degrades to the static routes rather than failing the response. A
 * crawler that arrives while the API is restarting should get the pages that
 * are always there, not a 500 that teaches it the sitemap is broken.
 */
export async function GET({ url, fetch }) {
	const now = new Date().toISOString().slice(0, 10);

	let arenas = [];
	try {
		arenas = await api.listArenas({ fetch });
	} catch {
		arenas = [];
	}

	const paths = [
		{ path: '/', priority: '1.0' },
		{ path: '/tonight', priority: '0.9' },
		{ path: '/arenas', priority: '0.8' },
		...arenas.map((arena) => ({ path: `/arenas/${arena.slug}`, priority: '0.7' }))
	];

	const body = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${paths
	.map(
		({ path, priority }) => `	<url>
		<loc>${url.origin}${path}</loc>
		<lastmod>${now}</lastmod>
		<changefreq>daily</changefreq>
		<priority>${priority}</priority>
	</url>`
	)
	.join('\n')}
</urlset>
`;

	return new Response(body, {
		headers: { 'content-type': 'application/xml; charset=utf-8' }
	});
}
