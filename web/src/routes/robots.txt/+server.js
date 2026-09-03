/** Personal, auth and machine routes stay out of search indexes. */
const DISALLOW = ['/bookings', '/dashboard', '/login', '/manage', '/register', '/v1/'];

export function GET({ url }) {
	const body = [
		'User-agent: *',
		'Allow: /',
		...DISALLOW.map((path) => `Disallow: ${path}`),
		'',
		`Sitemap: ${url.origin}/sitemap.xml`,
		''
	].join('\n');

	return new Response(body, {
		headers: { 'content-type': 'text/plain; charset=utf-8' }
	});
}
