<script>
	import ArenaRecord from '$lib/components/ArenaRecord.svelte';

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

<section class="head">
	<div class="shell">
		<p class="label">The register</p>
		<h1 class="display display-l">Where the valley plays</h1>
		<p class="lede">
			{data.arenas.length} arenas, {courtCount} courts. Rates are per hour, per court, and the arena
			sets them — nothing here is marked up.
		</p>
	</div>
</section>

<section class="list">
	<div class="shell">
		<ul>
			{#each data.arenas as arena (arena.id)}
				<li><ArenaRecord {arena} /></li>
			{/each}
		</ul>
	</div>
</section>

<style>
	.head {
		padding-block: clamp(2.5rem, 5vw, 4rem) 2rem;
	}

	h1 {
		margin-top: 0.6rem;
	}

	.head .lede {
		margin-top: 1rem;
		max-width: 54ch;
	}

	.list {
		padding-bottom: var(--band);
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
