<script>
	import { goto } from '$app/navigation';
	import { api, SPORT_LABELS } from '$lib/api/index.js';
	import HourLedger from '$lib/components/HourLedger.svelte';
	import HourPulse from '$lib/components/HourPulse.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import DateRail from '$lib/components/DateRail.svelte';
	import Listbox from '$lib/components/Listbox.svelte';
	import SportIcon from '$lib/components/SportIcon.svelte';
	import { formatDateLong, formatNPR } from '$lib/time.js';

	let { data } = $props();

	// The URL owns the board. Every control writes to it, so a filtered view is
	// shareable and the back button walks back through the searches.
	const date = $derived(data.date);
	const sport = $derived(data.sport);
	const area = $derived(data.area);

	const ledger = $derived(api.cityLedger(date, { sport, area }));
	const sportName = $derived(SPORT_LABELS[sport].toLowerCase());

	const areaOptions = $derived([
		{ value: 'all', label: 'Anywhere in the valley' },
		...data.areas.map((a) => ({ value: a, label: a }))
	]);

	const sportOptions = $derived(data.sports.map((s) => ({ value: s, label: SPORT_LABELS[s] })));

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
	<title>Find a court · Every free hour in the valley | Khel Arena</title>
	<meta
		name="description"
		content="Every court in Kathmandu and Lalitpur, every hour of the day, priced. Pick an hour to hold it."
	/>
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">The board</p>
		<h1 class="display display-l">{#key date}<TextAnimate text={formatDateLong(date)} />{/key}</h1>
		<div class="pulse-wrap fade-up fade-up-1">
			<HourPulse />
		</div>
		<p class="lede fade-up fade-up-2">
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
		<div class="pitch-mark fade-up fade-up-3">
			<CentreMark wide />
		</div>
	</div>
</section>

<section class="controls">
	<div class="shell">
		<div class="fade-up fade-up-2">
			<DateRail dates={data.dates} selected={date} onselect={(d) => set({ date: d })} />
		</div>

		<div class="filters fade-up fade-up-3">
			<div class="group">
				<Listbox
					label="Filter by sport"
					value={sport}
					options={sportOptions}
					onselect={(v) => set({ sport: v })}
				>
					{#snippet icon(v)}
						<SportIcon sport={v} size={16} />
					{/snippet}
				</Listbox>

				<Listbox
					label="Filter by area"
					value={area}
					options={areaOptions}
					onselect={(v) => set({ area: v })}
				/>
			</div>
		</div>
	</div>
</section>

<section class="board">
	<div class="shell">
		{#if ledger.rows.length}
			<HourLedger {ledger} />
		{:else}
			<div class="empty fade-up">
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
	h1 {
		margin-top: 0.6rem;
	}

	.head .lede {
		margin-top: 1rem;
		max-width: 56ch;
	}

	.pulse-wrap {
		margin-top: 1.4rem;
		max-width: 26rem;
	}

	.controls {
		padding-block: clamp(2rem, 4vw, 3rem) 0.5rem;
	}

	.controls .shell {
		display: grid;
		gap: 1rem;
	}

	/* Grid items default to min-width: auto, which lets a row that wants to
	   be wider than the viewport (the date rail, the filter chips) push the
	   whole page into horizontal scroll instead of clipping/scrolling
	   itself the way its own overflow-x: auto says it should. */
	.controls .shell > * {
		min-width: 0;
	}

	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
	}

	/* Both filters read as one control now — what, then where — sat
	   together on the same edge instead of pulled to opposite ends of the
	   row, so the eye takes them in as a pair. */
	.group {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
	}

	.board {
		padding-block: 1.5rem var(--band);
	}

	.empty {
		display: grid;
		justify-items: start;
		gap: 0.75rem;
		padding-block: clamp(2.5rem, 6vw, 4rem);
		border-top: 1px solid var(--line);
	}

	.empty p {
		color: var(--muted);
		max-width: 44ch;
	}

	.empty .btn {
		margin-top: 0.75rem;
	}
</style>
