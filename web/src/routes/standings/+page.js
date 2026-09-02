import { api } from '$lib/api/index.js';

export async function load({ fetch }) {
	return { standings: await api.standings({ fetch }) };
}
