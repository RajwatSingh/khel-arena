import { env } from '$env/dynamic/private';
import { forwardToApi, isApiPath } from '$lib/server/api-proxy.js';

/**
 * Sends server-side API calls straight to the API container.
 *
 * Only for sub-requests: the `fetch` a `load` function uses. SvelteKit has
 * already attached the visitor's cookies by this point and will copy any
 * Set-Cookie on the reply back to the page, so only the last hop changes.
 * Browser requests for /v1 never reach this server; Caddy sends them to the
 * API directly.
 *
 * Unset API_INTERNAL_URL (as in development, where the vite proxy serves
 * /v1) and calls go to the page's own origin as before.
 */
export async function handle({ event, resolve }) {
	const base = env.API_INTERNAL_URL;

	if (base && event.isSubRequest && isApiPath(event.url.pathname)) {
		return forwardToApi(event.request, { base, clientAddress: clientAddress(event) });
	}

	return resolve(event);
}

// adapter-node throws when ADDRESS_HEADER is set but the header is missing,
// e.g. a request that didn't come through Caddy. Sending no address is better
// than failing the page: the API then falls back to our own address.
function clientAddress(event) {
	try {
		return event.getClientAddress();
	} catch {
		return undefined;
	}
}
