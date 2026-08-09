import { api } from '$lib/api/index.js';

/** Public routes only — the private ones are listed in robots.txt. */
export function GET({ url }) {
	const now = new Date().toISOString().slice(0, 10);
	const paths = [
		{ path: '/', priority: '1.0' },
		{ path: '/tonight', priority: '0.9' },
		{ path: '/arenas', priority: '0.8' },
		...api.listArenas().map((arena) => ({ path: `/arenas/${arena.slug}`, priority: '0.7' }))
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
