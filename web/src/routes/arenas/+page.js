import { api } from '$lib/api/index.js';

export function load() {
	return { arenas: api.listArenas() };
}
