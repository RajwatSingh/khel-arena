import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import * as api from './client.js';
import { ApiError } from './errors.js';

/**
 * These cover the client's own logic against a stubbed fetch, so they need
 * nothing running and answer in milliseconds. The same paths are exercised
 * against a real `cmd/api` by `scripts/smoke.sh`, which is where "does the
 * server actually agree" is settled -- the split matches the Go suite's, where
 * unit tests need no database and the integration ones do.
 *
 * What is worth testing here is the behaviour that is hard to see by hand: a
 * 401 recovering silently, four of them sharing one refresh, and the refusal
 * to fall back to a global fetch on the server.
 */

const ORIGIN = 'http://api.test';

/**
 * A stand-in for the fetch SvelteKit hands to load(): it resolves the client's
 * relative BASE against an origin, which is the contract client.js is written
 * against.
 */
function server(routes) {
	const calls = [];

	const impl = vi.fn(async (url, init = {}) => {
		const { pathname, search } = new URL(url, ORIGIN);
		calls.push({ path: pathname + search, init });

		const handler = routes[pathname];
		if (!handler) return json({ error: { code: 'not_found', message: 'no route' } }, 404);
		return handler(init, calls);
	});

	return { impl, calls };
}

function json(body, status = 200) {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function noContent() {
	return new Response(null, { status: 204 });
}

const SESSION = { user: { id: 'u1', email: 'r@k.np' }, access_token: 'fresh.token' };

beforeEach(() => {
	api.setAccessToken(null);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('server-side calls', () => {
	it('refuse to run without the fetch load() provides', async () => {
		// The failure this replaces is `TypeError: Failed to parse URL from
		// /v1/me` thrown from inside Node, which names nothing useful.
		await expect(api.me()).rejects.toThrow(/load\(\)/);
	});

	it('do not fall back to a global fetch that would resolve nothing', async () => {
		const global = vi.fn();
		vi.stubGlobal('fetch', global);

		await expect(api.me()).rejects.toThrow();
		expect(global).not.toHaveBeenCalled();
	});
});

describe('in the browser', () => {
	it('uses the global fetch against the page origin', async () => {
		vi.stubGlobal('window', {});
		vi.stubGlobal('fetch', vi.fn(async () => json(SESSION)));

		await api.login({ email: 'r@k.np', password: 'x' });

		expect(fetch).toHaveBeenCalledOnce();
		expect(fetch.mock.calls[0][0]).toBe('/v1/auth/login');
	});
});

describe('requests', () => {
	it('send the access token only when the call is authenticated', async () => {
		const { impl, calls } = server({
			'/v1/me': () => json({ id: 'u1' }),
			'/v1/courts/c1/availability': () => json({ slots: [] })
		});
		api.setAccessToken('abc');

		await api.me({ fetch: impl });
		await api.availability('c1', '2026-09-02', { fetch: impl });

		expect(calls[0].init.headers.Authorization).toBe('Bearer abc');
		expect(calls[1].init.headers.Authorization).toBeUndefined();
	});

	// The refresh cookie is httpOnly; without this it simply never travels.
	it('always include credentials, so the refresh cookie travels', async () => {
		const { impl, calls } = server({ '/v1/me': () => json({}) });
		await api.me({ fetch: impl });

		expect(calls[0].init.credentials).toBe('include');
	});

	it('encode the date so a query string cannot be injected', async () => {
		const { impl, calls } = server({ '/v1/courts/c1/availability': () => json({ slots: [] }) });
		await api.availability('c1', '2026-09-02&limit=9', { fetch: impl });

		expect(calls[0].path).toBe('/v1/courts/c1/availability?date=2026-09-02%26limit%3D9');
	});

	it('return null for 204 rather than failing to parse an empty body', async () => {
		const { impl } = server({ '/v1/auth/logout': () => noContent() });

		await expect(api.logout({ fetch: impl })).resolves.toBeNull();
	});
});

describe('errors', () => {
	it('become an ApiError carrying the envelope', async () => {
		const { impl } = server({
			'/v1/auth/register': () =>
				json(
					{
						error: {
							code: 'invalid',
							message: 'Check the form.',
							fields: [{ field: 'username', message: 'Too short.' }]
						}
					},
					400
				)
		});

		const err = await api.register({}, { fetch: impl }).catch((e) => e);

		expect(err).toBeInstanceOf(ApiError);
		expect(err.code).toBe('invalid');
		expect(err.message).toBe('Check the form.');
		expect(err.fields).toEqual([{ field: 'username', message: 'Too short.' }]);
	});

	// A proxy's own 502 has no envelope, but pages switch on `code`, so one
	// still has to be inferred rather than left undefined.
	it('infer a code when there is no envelope behind the status', async () => {
		const { impl } = server({ '/v1/me': () => new Response('<html>502</html>', { status: 502 }) });
		api.setAccessToken('abc');

		const err = await api.me({ fetch: impl }).catch((e) => e);

		expect(err).toBeInstanceOf(ApiError);
		expect(err.code).toBe('internal');
	});

	it('turn an unreachable server into an ApiError, not a TypeError', async () => {
		const impl = vi.fn(async () => {
			throw new TypeError('fetch failed');
		});

		const err = await api.me({ fetch: impl }).catch((e) => e);

		expect(err).toBeInstanceOf(ApiError);
		expect(err.code).toBe('unavailable');
	});
});

describe('an expired access token', () => {
	/** A server where the token in SESSION is the only one /v1/me accepts. */
	function withRefresh({ refreshWorks = true } = {}) {
		let accepted = 'stale.token';

		return server({
			'/v1/me': (init) =>
				init.headers.Authorization === `Bearer ${accepted}`
					? json({ id: 'u1', email: 'r@k.np' })
					: json({ error: { code: 'unauthenticated', message: 'Please sign in.' } }, 401),

			'/v1/auth/refresh': () => {
				if (!refreshWorks) {
					return json({ error: { code: 'unauthenticated', message: 'Sign in again.' } }, 401);
				}
				accepted = SESSION.access_token;
				return json(SESSION);
			}
		});
	}

	const refreshes = (calls) => calls.filter((c) => c.path === '/v1/auth/refresh').length;

	it('is refreshed once and the call replayed', async () => {
		const { impl, calls } = withRefresh();
		api.setAccessToken('expired.token');

		await expect(api.me({ fetch: impl })).resolves.toMatchObject({ email: 'r@k.np' });
		expect(refreshes(calls)).toBe(1);
		expect(api.getAccessToken()).toBe(SESSION.access_token);
	});

	// Refreshing rotates the token. Four simultaneous refreshes would mean the
	// last three present a credential the first already retired, which the
	// server is right to treat as reuse of a stolen token.
	it('is refreshed once even when four calls fail at the same moment', async () => {
		const { impl, calls } = withRefresh();
		api.setAccessToken('expired.token');

		const results = await Promise.all([
			api.me({ fetch: impl }),
			api.me({ fetch: impl }),
			api.me({ fetch: impl }),
			api.me({ fetch: impl })
		]);

		expect(results.every((u) => u.email === 'r@k.np')).toBe(true);
		expect(refreshes(calls)).toBe(1);
	});

	it('surfaces the original 401 when the session is genuinely dead', async () => {
		const { impl, calls } = withRefresh({ refreshWorks: false });
		api.setAccessToken('expired.token');

		const err = await api.me({ fetch: impl }).catch((e) => e);

		expect(err).toBeInstanceOf(ApiError);
		expect(err.code).toBe('unauthenticated');
		// One attempt, then it gives up: a retry loop here would hammer the
		// server on every signed-out request.
		expect(refreshes(calls)).toBe(1);
		expect(api.getAccessToken()).toBeNull();
	});

	it('does not try to refresh a failing refresh', async () => {
		const { impl, calls } = server({
			'/v1/auth/refresh': () =>
				json({ error: { code: 'unauthenticated', message: 'Sign in again.' } }, 401)
		});

		await expect(api.refresh({ fetch: impl })).rejects.toBeInstanceOf(ApiError);
		expect(calls).toHaveLength(1);
	});

	it('leaves an unauthenticated 401 alone', async () => {
		// Nothing to refresh on behalf of: this call never carried a token.
		const { impl, calls } = server({
			'/v1/courts/c1/availability': () =>
				json({ error: { code: 'unauthenticated', message: 'Sign in.' } }, 401)
		});

		await expect(api.availability('c1', '2026-09-02', { fetch: impl })).rejects.toBeInstanceOf(
			ApiError
		);
		expect(calls).toHaveLength(1);
	});
});

describe('the access token', () => {
	it('is held in memory only, never handed to storage', async () => {
		api.setAccessToken('abc');
		expect(api.getAccessToken()).toBe('abc');

		api.setAccessToken(null);
		expect(api.getAccessToken()).toBeNull();

		// Nothing in this module should reach for a storage API; persisting a
		// fifteen-minute token buys nothing and adds an XSS-readable copy.
		const source = await import('node:fs').then((fs) =>
			fs.readFileSync(new URL('./client.js', import.meta.url), 'utf8')
		);
		expect(source).not.toMatch(/localStorage|sessionStorage|document\.cookie/);
	});
});
