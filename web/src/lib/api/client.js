/**
 * The real transport, ready for when `cmd/api` exists.
 *
 * Every function here matches one row of the endpoint table in apiPlan.md §3,
 * and `request` decodes the error envelope from §4 into the same `ApiError`
 * the mock throws — so pages catch one type either way.
 */

import { ApiError } from './mock.js';

const BASE = '/v1';

let accessToken = null;

export function setAccessToken(token) {
	accessToken = token;
}

async function request(path, { method = 'GET', body, auth = false } = {}) {
	const headers = {};
	if (body !== undefined) headers['Content-Type'] = 'application/json';
	if (auth && accessToken) headers.Authorization = `Bearer ${accessToken}`;

	const response = await fetch(`${BASE}${path}`, {
		method,
		headers,
		credentials: 'include',
		body: body === undefined ? undefined : JSON.stringify(body)
	});

	if (response.status === 204) return null;

	const payload = await response.json().catch(() => null);

	if (!response.ok) {
		const error = payload?.error ?? {};
		throw new ApiError(
			error.code ?? 'internal',
			error.message ?? 'Something went wrong on our side.',
			error.fields ?? []
		);
	}

	return payload;
}

export const login = (credentials) =>
	request('/auth/login', { method: 'POST', body: credentials });

export const register = (input) => request('/auth/register', { method: 'POST', body: input });

export const logout = () => request('/auth/logout', { method: 'POST' });

export const refresh = () => request('/auth/refresh', { method: 'POST' });

export const me = () => request('/me', { auth: true });

export const availability = (courtId, date) =>
	request(`/courts/${courtId}/availability?date=${date}`);

export const listBookings = (limit = 20) => request(`/bookings?limit=${limit}`, { auth: true });

export const createBooking = (input) =>
	request('/bookings', { method: 'POST', body: input, auth: true });

export const cancelBooking = (id) =>
	request(`/bookings/${id}`, { method: 'DELETE', auth: true });
