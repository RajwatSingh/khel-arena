/**
 * The online gateways a player can be offered. Both are always shown; a venue
 * that has not added its account for one gets a message instead of a
 * checkout when the player taps it.
 */
export const ONLINE_PROVIDERS = [
	{ id: 'esewa', label: 'eSewa' },
	{ id: 'khalti', label: 'Khalti' }
];

export function providerLabel(id) {
	return ONLINE_PROVIDERS.find((p) => p.id === id)?.label ?? id;
}

/**
 * What to tell a player who picked a gateway the venue has not set up,
 * pointing them at whatever the venue does take.
 *
 * @param {string} provider the gateway they tapped
 * @param {string} venue the venue's name
 * @param {string[]} available the gateways the venue has set up
 */
export function notSetUpMessage(provider, venue, available = []) {
	const name = providerLabel(provider);
	const other = available.find((p) => p !== provider);
	const instead = other
		? `Pay with ${providerLabel(other)} or cash at the venue instead.`
		: 'Pay cash at the venue instead.';
	return `${venue || 'This venue'} hasn't added their ${name} account yet, so ${name} payments aren't available here. ${instead}`;
}
