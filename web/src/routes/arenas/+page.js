import { api } from '$lib/api/index.js';

export async function load({ fetch }) {
	return { arenas: await api.listArenas({ fetch }) };
}
