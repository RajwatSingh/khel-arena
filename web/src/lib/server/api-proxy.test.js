import { describe, expect, it } from 'vitest';
import { forwardToApi, isApiPath } from './api-proxy.js';

/** A fetch that records what it was asked for and answers with `reply`. */
function recordingFetch(reply = new Response('{}', { status: 200 })) {
	const calls = [];
	const fetch = async (url, init) => {
		calls.push({ url: String(url), init });
		return reply;
	};
	return { fetch, calls };
}

describe('isApiPath', () => {
	it('matches the API prefix and nothing that merely starts with it', () => {
		expect(isApiPath('/v1')).toBe(true);
		expect(isApiPath('/v1/arenas/dhuku')).toBe(true);
		expect(isApiPath('/v10/arenas')).toBe(false);
		expect(isApiPath('/arenas/v1')).toBe(false);
	});
});

describe('forwardToApi', () => {
	it('keeps the path and query, and swaps the origin for the internal one', async () => {
		const { fetch, calls } = recordingFetch();
		const request = new Request('https://khel.example/v1/arenas?sport=futsal&limit=5');

		await forwardToApi(request, { base: 'http://api:8080', clientAddress: '203.0.113.7', fetch });

		expect(calls[0].url).toBe('http://api:8080/v1/arenas?sport=futsal&limit=5');
	});

	it('passes cookies and auth through, and sets the client address itself', async () => {
		const { fetch, calls } = recordingFetch();
		const request = new Request('https://khel.example/v1/me', {
			headers: {
				cookie: 'refresh=abc',
				authorization: 'Bearer t',
				host: 'khel.example',
				// A caller trying to pick their own rate-limit bucket.
				'x-forwarded-for': '1.2.3.4'
			}
		});

		await forwardToApi(request, { base: 'http://api:8080', clientAddress: '203.0.113.7', fetch });

		const headers = calls[0].init.headers;
		expect(headers.get('cookie')).toBe('refresh=abc');
		expect(headers.get('authorization')).toBe('Bearer t');
		expect(headers.get('host')).toBeNull();
		expect(headers.get('x-forwarded-for')).toBe('203.0.113.7');
	});

	it('sends no forwarded address when it does not know one', async () => {
		const { fetch, calls } = recordingFetch();
		const request = new Request('https://khel.example/v1/me', {
			headers: { 'x-forwarded-for': '1.2.3.4' }
		});

		await forwardToApi(request, { base: 'http://api:8080', clientAddress: undefined, fetch });

		expect(calls[0].init.headers.get('x-forwarded-for')).toBeNull();
	});

	it('forwards a body on writes', async () => {
		const { fetch, calls } = recordingFetch();
		const request = new Request('https://khel.example/v1/bookings', {
			method: 'POST',
			body: JSON.stringify({ court_id: 'c1' }),
			headers: { 'content-type': 'application/json' }
		});

		await forwardToApi(request, { base: 'http://api:8080', clientAddress: '203.0.113.7', fetch });

		expect(calls[0].init.method).toBe('POST');
		expect(new TextDecoder().decode(calls[0].init.body)).toBe('{"court_id":"c1"}');
	});

	it('hands back the API response with headers SvelteKit can edit', async () => {
		const reply = new Response('{"ok":true}', {
			status: 201,
			headers: { 'set-cookie': 'refresh=new; HttpOnly' }
		});
		const { fetch } = recordingFetch(reply);

		const response = await forwardToApi(new Request('https://khel.example/v1/auth/refresh'), {
			base: 'http://api:8080',
			clientAddress: '203.0.113.7',
			fetch
		});

		expect(response.status).toBe(201);
		expect(response.headers.get('set-cookie')).toBe('refresh=new; HttpOnly');
		expect(() => response.headers.set('x-test', '1')).not.toThrow();
		expect(await response.json()).toEqual({ ok: true });
	});
});
