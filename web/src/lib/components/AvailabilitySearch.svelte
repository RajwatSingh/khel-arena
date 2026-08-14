<script>
	import { goto } from '$app/navigation';
	import { SPORT_LABELS } from '$lib/api/index.js';
	import { formatDate } from '$lib/time.js';
	import Listbox from './Listbox.svelte';

	/**
	 * The one control on the home page. Three answers — what, where, when — and
	 * it hands you the board already filtered.
	 */
	let { dates, areas, sports } = $props();

	let sport = $state('futsal');
	let area = $state('all');
	let day = $state(null);

	const date = $derived(day ?? dates[0]);

	const sportOptions = $derived(sports.map((value) => ({ value, label: SPORT_LABELS[value] })));
	const areaOptions = $derived([
		{ value: 'all', label: 'Anywhere in the valley' },
		...areas.map((value) => ({ value, label: value }))
	]);
	const dateOptions = $derived(
		dates.map((value, i) => ({
			value,
			label: i === 0 ? 'Today' : i === 1 ? 'Tomorrow' : formatDate(value)
		}))
	);

	function show(event) {
		event.preventDefault();
		goto(`/tonight?date=${date}&sport=${sport}&area=${encodeURIComponent(area)}`);
	}
</script>

<form class="card search" onsubmit={show}>
	<div class="pick">
		<span class="label">Playing</span>
		<Listbox label="Playing" value={sport} options={sportOptions} onselect={(v) => (sport = v)} fill />
	</div>

	<div class="pick">
		<span class="label">Where</span>
		<Listbox label="Where" value={area} options={areaOptions} onselect={(v) => (area = v)} fill />
	</div>

	<div class="pick">
		<span class="label">When</span>
		<Listbox label="When" value={date} options={dateOptions} onselect={(v) => (day = v)} fill />
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

	.btn {
		padding-block: 0.95em;
	}

	@media (max-width: 860px) {
		.search {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
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
