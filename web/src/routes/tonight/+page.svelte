<script>
	import { goto } from '$app/navigation';
	import { api, SPORT_LABELS } from '$lib/api/index.js';
	import HourLedger from '$lib/components/HourLedger.svelte';
	import DateRail from '$lib/components/DateRail.svelte';
	import { formatDateLong, formatNPR } from '$lib/time.js';

	let { data } = $props();

	// The URL owns the board. Every control writes to it, so a filtered view is
	// shareable and the back button walks back through the searches.
	const date = $derived(data.date);
	const sport = $derived(data.sport);
	const area = $derived(data.area);

	const ledger = $derived(api.cityLedger(date, { sport, area }));
	const sportName = $derived(SPORT_LABELS[sport].toLowerCase());

	function set(changes) {
		const next = { date, sport, area, ...changes };
		goto(`/tonight?date=${next.date}&sport=${next.sport}&area=${encodeURIComponent(next.area)}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}
</script>

<svelte:head>
	<title>Find a court — every free hour in the valley | Khel Arena</title>
	<meta
		name="description"
		content="Every court in Kathmandu and Lalitpur, every hour of the day, priced. Pick an hour to hold it."
	/>
</svelte:head>

<section class="head">
	<div class="shell">
		<p class="label">The board</p>
		<h1 class="display display-l">{formatDateLong(date)}</h1>
		<p class="lede">
			{#if ledger.open_hours > 0}
				<strong class="num">{ledger.open_hours}</strong>
				free {ledger.open_hours === 1 ? 'hour' : 'hours'} on
				<strong class="num">{ledger.rows.length}</strong>
				{ledger.rows.length === 1 ? 'court' : 'courts'}{ledger.cheapest_npr
					? `, from NPR ${formatNPR(ledger.cheapest_npr)}`
					: ''}. Tap a price to hold that hour.
			{:else if ledger.rows.length}
				Nothing free here. Try another day, another area, or another sport.
			{:else}
				No {sportName} courts are listed in that area yet.
			{/if}
		</p>
	</div>
</section>

<section class="controls">
	<div class="shell">
		<DateRail dates={data.dates} selected={date} onselect={(d) => set({ date: d })} />

		<div class="filters">
			<div class="group" role="group" aria-label="Filter by sport">
				{#each data.sports as s (s)}
					<button
						type="button"
						class="chip"
						class:on={sport === s}
						aria-pressed={sport === s}
						onclick={() => set({ sport: s })}
					>
						{SPORT_LABELS[s]}
					</button>
				{/each}
			</div>

			<label class="area">
				<span class="sr-only">Filter by area</span>
				<select value={area} onchange={(e) => set({ area: e.currentTarget.value })}>
					<option value="all">Anywhere in the valley</option>
					{#each data.areas as value (value)}
						<option {value}>{value}</option>
					{/each}
				</select>
			</label>
		</div>
	</div>
</section>

<section class="board">
	<div class="shell">
		{#if ledger.rows.length}
			<HourLedger {ledger} />
		{:else}
			<div class="card empty">
				<h2 class="display display-m">Nothing here yet</h2>
				<p class="small">
					No {sportName} courts in {area === 'all' ? 'the valley' : area}. Widen the area or pick another
					sport.
				</p>
				<button class="btn btn-secondary" onclick={() => set({ area: 'all' })}>Search the whole valley</button>
			</div>
		{/if}
	</div>
</section>

<style>
	.head {
		padding-block: clamp(2.5rem, 5vw, 4rem) 1.75rem;
	}

	.head .label {
		color: var(--on-field-faint);
	}

	h1 {
		margin-top: 0.6rem;
		color: var(--on-field);
	}

	.head .lede {
		margin-top: 1rem;
		max-width: 56ch;
		color: var(--on-field-muted);
	}

	.head strong {
		color: var(--on-field);
		font-weight: 600;
	}

	.controls .shell {
		display: grid;
		gap: 1rem;
	}

	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
	}

	.group {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}

	.chip {
		padding: 0.5rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-pill);
		background: var(--surface);
		font-size: 0.9375rem;
		font-weight: 500;
		color: var(--muted);
		cursor: pointer;
		transition:
			border-color 0.18s var(--ease),
			background-color 0.18s var(--ease),
			color 0.18s var(--ease);
	}

	.chip:hover:not(.on) {
		border-color: var(--line-strong);
		color: var(--ink);
	}

	.chip.on {
		background: var(--pine);
		border-color: var(--pine);
		color: #ffffff;
	}

	.area select {
		padding: 0.5rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-pill);
		background: var(--surface);
		font-size: 0.9375rem;
		font-weight: 500;
		color: var(--muted);
		cursor: pointer;
	}

	.board {
		padding-block: 1.5rem var(--band);
	}

	.empty {
		display: grid;
		justify-items: start;
		gap: 0.75rem;
		padding: clamp(2rem, 5vw, 3.5rem);
	}

	.empty p {
		color: var(--muted);
		max-width: 44ch;
	}

	.empty .btn {
		margin-top: 0.75rem;
	}
</style>
