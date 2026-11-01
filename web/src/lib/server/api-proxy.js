/**
 * Sends the web server's own API calls straight to the Go service.
 *
 * Without this, a server-side `load` that fetches `/v1/...` goes the long way
 * round. SvelteKit has no route for it, so it makes a real request to the
 * public ORIGIN: out through DNS and TLS, back in through Caddy, and only then
 * to the API on the same machine. That adds a TLS handshake to every server
 * render, and the API's view of the caller is the host's own address.
 *
 * Kept free of SvelteKit imports so it can be tested as plain JavaScript; the
 * hook in src/hooks.server.js decides when to call it.
 */

const API_PREFIX = '/v1';

// Headers that describe the hop rather than the request, or that fetch works
// out for itself. Forwarding them would be wrong for the new connection. The
// forwarding headers are dropped so a caller can't smuggle in their own.
const DROPPED_HEADERS = [
	'host',
	'connection',
	'keep-alive',
	'transfer-encoding',
	'content-length',
	'forwarded',
	'x-forwarded-for',
	'x-real-ip'
];

/** True for paths the Go service owns. */
export function isApiPath(pathname) {
	return pathname === API_PREFIX || pathname.startsWith(`${API_PREFIX}/`);
}

/**
 * Replays `request` against the API at `base` (e.g. http://api:8080).
 *
 * `clientAddress` becomes X-Forwarded-For. The API trusts one proxy hop
 * (TRUSTED_PROXY_HOPS=1), and this server is now that hop, so the API keys
 * its rate limits and session records on the visitor rather than on us.
 */
export async function forwardToApi(request, { base, clientAddress, fetch = globalThis.fetch }) {
	const url = new URL(request.url);
	const target = new URL(url.pathname + url.search, base);

	const headers = new Headers(request.headers);
	for (const name of DROPPED_HEADERS) headers.delete(name);
	if (clientAddress) headers.set('x-forwarded-for', clientAddress);

	const hasBody = request.method !== 'GET' && request.method !== 'HEAD';

	const response = await fetch(target, {
		method: request.method,
		headers,
		body: hasBody ? await request.arrayBuffer() : undefined,
		redirect: request.redirect,
		signal: request.signal
	});

	// A fresh Response so its headers are mutable; SvelteKit edits them.
	return new Response(response.body, response);
}
