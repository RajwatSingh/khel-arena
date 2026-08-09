<script>
	import { goto } from '$app/navigation';
	import { SPORT_LABELS } from '$lib/api/index.js';
	import { formatDate } from '$lib/time.js';

	/**
	 * The one control on the home page. Three answers — what, where, when — and
	 * it hands you the board already filtered.
	 */
	let { dates, areas, sports } = $props();

	let sport = $state('futsal');
	let area = $state('all');
	let day = $state(null);

	const date = $derived(day ?? dates[0]);

	function show(event) {
		event.preventDefault();
		goto(`/tonight?date=${date}&sport=${sport}&area=${encodeURIComponent(area)}`);
	}
</script>

<form class="card search" onsubmit={show}>
	<div class="pick">
		<label class="label" for="s-sport">Playing</label>
		<select id="s-sport" bind:value={sport}>
			{#each sports as value (value)}
				<option {value}>{SPORT_LABELS[value]}</option>
			{/each}
		</select>
	</div>

	<div class="pick">
		<label class="label" for="s-area">Where</label>
		<select id="s-area" bind:value={area}>
			<option value="all">Anywhere in the valley</option>
			{#each areas as value (value)}
				<option {value}>{value}</option>
			{/each}
		</select>
	</div>

	<div class="pick">
		<label class="label" for="s-date">When</label>
		<select id="s-date" value={date} onchange={(e) => (day = e.currentTarget.value)}>
			{#each dates as value, i (value)}
				<option {value}>{i === 0 ? 'Today' : i === 1 ? 'Tomorrow' : formatDate(value)}</option>
			{/each}
		</select>
	</div>

	<button class="btn btn-primary" type="submit">Show free hours</button>
</form>

<style>
	.search {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr)) auto;
		align-items: end;
		gap: 0.5rem 0.25rem;
		padding: 0.75rem 0.75rem 0.75rem 0.35rem;
		box-shadow: var(--shadow);
	}

	.pick {
		display: grid;
		gap: 0.15rem;
		padding: 0.45rem 1rem;
		border-right: 1px solid var(--line);
	}

	.pick:nth-of-type(3) {
		border-right: 0;
	}

	select {
		width: 100%;
		padding: 0;
		border: 0;
		background: none;
		font-size: 1.0625rem;
		font-weight: 500;
		letter-spacing: -0.01em;
		color: var(--ink);
		cursor: pointer;
	}

	select:focus-visible {
		outline: 2px solid var(--pine);
		outline-offset: 4px;
	}

	.btn {
		padding-block: 0.95em;
	}

	@media (max-width: 860px) {
		.search {
			grid-template-columns: 1fr 1fr;
			padding: 0.75rem;
			gap: 0.75rem;
		}

		.pick {
			padding: 0;
			border-right: 0;
		}

		.pick:nth-of-type(3) {
			grid-column: 1 / -1;
		}

		.btn {
			grid-column: 1 / -1;
		}
	}
</style>
