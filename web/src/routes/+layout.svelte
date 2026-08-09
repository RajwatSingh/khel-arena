<script>
	import '../app.css';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import { session } from '$lib/session.svelte.js';

	let { children } = $props();

	// The server renders signed-out; the browser knows better as soon as it has
	// the stored session in hand.
	$effect(() => session.restore());
</script>

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
		background: var(--sodium);
		color: #17190f;
		font-size: 0.8125rem;
		font-weight: 600;
		transition: top 0.2s var(--ease);
	}

	.skip:focus {
		top: 1rem;
	}
</style>
