<script>
	import '../app.css';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import AmbientLedger from '$lib/components/AmbientLedger.svelte';
	import DotGridSpotlight from '$lib/components/DotGridSpotlight.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { session } from '$lib/session.svelte.js';
	import { haptic } from '$lib/haptic.js';
	import { takeGatewayReturn } from '$lib/gatewayReturn.js';

	let { children } = $props();

	// The server renders signed-out; the browser corrects that once the refresh
	// cookie has been redeemed. Deliberately not returned from the effect: a
	// value returned from $effect is taken as its cleanup function, and
	// restore() is async, so returning it would hand Svelte a Promise to call.
	$effect(() => {
		session.restore();
	});

	// A player who pressed Back on eSewa or Khalti goes to /bookings, where
	// their hold shows with every way to pay and the option to cancel. See
	// $lib/gatewayReturn.js for why the page they came from won't do.
	//
	// A gateway that finished sends them to /bookings anyway, through the
	// payment callback; then the note is simply cleared.
	function returnFromGateway(restoredFromCache) {
		if (!takeGatewayReturn()) return;

		if (restoredFromCache) {
			// The page was frozen as it was left. A real load, not a client-side
			// navigation, so no stale "Opening Khalti…" state survives.
			location.replace('/bookings');
		} else if (location.pathname !== '/bookings') {
			goto('/bookings', { replaceState: true });
		}
	}

	onMount(() => returnFromGateway(false));

	function onPageShow(event) {
		if (event.persisted) returnFromGateway(true);
	}

	// One delegated listener catches every button on the site, present and
	// future, rather than wiring haptic() into each one by hand — a tap on
	// a touch device gets a buzz back, a click anywhere else is a no-op.
	function onClick(e) {
		const button = e.target.closest('button');
		if (button && !button.disabled) haptic();
	}
</script>

<svelte:window onclick={onClick} onpageshow={onPageShow} />

<AmbientLedger />
<DotGridSpotlight />
<a class="skip" href="#main">Skip to content</a>
<SiteHeader />
<main id="main">
	{@render children()}
</main>
<SiteFooter />

<style>
	.skip {
		position: absolute;
		left: 1rem;
		top: -4rem;
		z-index: 100;
		padding: 0.6rem 1rem;
		background: var(--pine);
		color: #ffffff;
		font-size: 0.8125rem;
		font-weight: 600;
		transition: top 0.2s var(--ease);
	}

	.skip:focus {
		top: 1rem;
	}
</style>
