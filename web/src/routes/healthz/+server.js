/**
 * Liveness for the SvelteKit server. It checks nothing on purpose.
 *
 * The container healthcheck cannot point at `/`: that page's load makes three
 * API calls, so a `/` probe reports the API's health rather than this
 * process's, and Docker would restart the web server every time the Go
 * service went down -- which fixes nothing and takes the pages that still
 * render down with it.
 *
 * Same split the Go service already makes between /healthz and /readyz, for
 * the same reason. There is no /readyz here because this server has no
 * dependency to be ready for: it renders pages whether the API answers or not.
 */
export function GET() {
	return new Response('ok\n', {
		headers: {
			'content-type': 'text/plain; charset=utf-8',
			'cache-control': 'no-store'
		}
	});
}
