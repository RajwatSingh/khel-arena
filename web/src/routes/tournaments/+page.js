import { api } from '$lib/api/index.js';

export async function load({ fetch }) {
	return { tournaments: await api.listTournaments({ fetch }) };
}
