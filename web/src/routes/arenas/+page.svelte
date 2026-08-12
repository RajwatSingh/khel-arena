<script>
	import ArenaRecord from '$lib/components/ArenaRecord.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import { reveal } from '$lib/actions/reveal.js';

	let { data } = $props();

	const courtCount = $derived(data.arenas.reduce((n, a) => n + a.court_count, 0));
</script>

<svelte:head>
	<title>Arenas in Kathmandu and Lalitpur | Khel Arena</title>
	<meta
		name="description"
		content="Every arena bookable through Khel Arena, with opening hours, courts and rates."
	/>
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">The register</p>
		<h1 class="display display-l fade-up">Where the valley plays</h1>
		<p class="lede fade-up fade-up-1">
			<strong class="num">{data.arenas.length}</strong> arenas, <strong class="num"
				>{courtCount}</strong
			> courts. Rates are per hour, per court, and the arena sets them — nothing here is marked up.
		</p>
		<div class="pitch-mark fade-up fade-up-2">
			<CentreMark wide />
		</div>
	</div>
</section>

<section class="list">
	<div class="shell">
		<ul>
			{#each data.arenas as arena, i (arena.id)}
				<li use:reveal={{ delay: Math.min(i, 6) * 60 }}><ArenaRecord {arena} /></li>
			{/each}
		</ul>
	</div>
</section>

<style>
	.head {
		padding-block: clamp(2.5rem, 5vw, 4rem);
	}

	h1 {
		margin-top: 0.6rem;
	}

	.head .lede {
		margin-top: 1rem;
		max-width: 54ch;
	}

	.list {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	ul {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(23rem, 1fr));
		gap: 1rem;
	}

	@media (max-width: 560px) {
		ul {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
