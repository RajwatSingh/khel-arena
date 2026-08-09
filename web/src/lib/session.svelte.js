import { api } from './api/index.js';

let current = $state(null);

export const session = {
	/** Picks the signed-in user back up after a reload. Client only. */
	restore() {
		current = api.restore();
	},
	get user() {
		return current?.user ?? null;
	},
	get signedIn() {
		return current !== null;
	},
	async signIn(credentials) {
		current = await api.login(credentials);
		return current;
	},
	async signUp(input) {
		current = await api.register(input);
		return current;
	},
	signOut() {
		api.logout();
		current = null;
	}
};

/** Ticks once a second while any hold is counting down. */
export function clock() {
	let now = $state(Date.now());

	$effect(() => {
		const id = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(id);
	});

	return {
		get now() {
			return now;
		}
	};
}
