import { api } from '$lib/api/index.js';
import { dates as dateHelper } from '$lib/time.js';

/**
 * The home page's whole payload.
 *
 * `fetch` is threaded into every call: this runs on the server as well as in
 * the browser, and there the client's relative base URL needs the request's
 * own origin to resolve against.
 *
 * The three reads run together rather than in sequence -- they do not depend
 * on each other, and awaiting them one at a time would make the page as slow
 * as their sum.
 */
export async function load({ fetch }) {
	const date = dateHelper.today();

	const [areas, ledger, arenas] = await Promise.all([
		api.listAreas({ fetch }),
		api.cityLedger(date, { fetch }),
		api.listArenas({ fetch })
	]);

	return { date, dates: dateHelper.rail(7), areas, ledger, arenas };
}
