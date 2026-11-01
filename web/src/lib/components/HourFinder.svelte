<script>
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { formatNPR, formatTime } from '$lib/time.js';

	/**
	 * The board turned around for phones: pick when you want to play, then
	 * see every court free at that hour, cheapest first. Same data as the
	 * grid (HourLedger), just asked in the order people decide.
	 */
	let { ledger } = $props();

	const PARTS = [
		{ id: 'morning', label: 'Morning', from: 0, to: 12 },
		{ id: 'afternoon', label: 'Afternoon', from: 12, to: 17 },
		{ id: 'evening', label: 'Evening', from: 17, to: 24 }
	];

	const hourOf = (slot) => Number(formatTime(slot.starts_at).slice(0, 2));
	const clock = (h) => `${String(h).padStart(2, '0')}:00`;
	const inPart = (h, p) => h >= p.from && h < p.to;

	// Each court's slots by hour. Hours that have started are left out, as
	// on the grid: they can't be booked.
	const courts = $derived(
		ledger.rows.map((row) => {
			const byHour = new Map();
			for (const slot of row.slots) if (!slot.is_past) byHour.set(hourOf(slot), slot);
			return { row, byHour };
		})
	);

	const hours = $derived(
		[...new Set(courts.flatMap((c) => [...c.byHour.keys()]))].sort((a, b) => a - b)
	);

	const freeCount = $derived(
		new Map(hours.map((h) => [h, courts.filter((c) => c.byHour.get(h)?.available).length]))
	);

	const parts = $derived(PARTS.filter((p) => hours.some((h) => inPart(h, p))));

	// Opens on the evening, when most games are played, or failing that
	// the first hour with anything free.
	function bestHour(candidates) {
		const open = candidates.filter((h) => freeCount.get(h) > 0);
		return open.find((h) => h >= 17) ?? open[0] ?? candidates[0] ?? null;
	}

	let chosen = $state(null);
	// A new day or filter can drop the chosen hour; fall back rather than
	// show nothing.
	const hour = $derived(hours.includes(chosen) ? chosen : bestHour(hours));
	const part = $derived(PARTS.find((p) => inPart(hour, p)));
	const partHours = $derived(part ? hours.filter((h) => inPart(h, part)) : []);

	function pickPart(p) {
		const within = hours.filter((h) => inPart(h, p));
		chosen = within.find((h) => freeCount.get(h) > 0) ?? within[0];
	}

	const results = $derived(
		courts
			.filter((c) => c.byHour.get(hour)?.available)
			.map((c) => {
				// How long the court stays free from this hour, so someone
				// after two hours can see it without opening the court.
				let to = hour + 1;
				while (c.byHour.get(to)?.available) to++;
				return { ...c, slot: c.byHour.get(hour), to };
			})
			.sort(
				(a, b) =>
					a.slot.price_npr - b.slot.price_npr || a.row.arena_name.localeCompare(b.row.arena_name)
			)
	);

	const booked = $derived(
		courts.filter((c) => c.byHour.has(hour) && !c.byHour.get(hour).available).length
	);

	// When nothing is free at the chosen hour, the nearest hours that do
	// have something, so the next tap is obvious.
	const nearby = $derived(
		results.length
			? []
			: hours
					.filter((h) => freeCount.get(h) > 0)
					.sort((a, b) => Math.abs(a - hour) - Math.abs(b - hour) || a - b)
					.slice(0, 3)
					.sort((a, b) => a - b)
	);

	const calm =
		typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

	const shortFormat = (format) => format?.replace(/^(\d+)-a-side$/i, '$1v$1') ?? '';

	function href(row, slot) {
		return `/arenas/${row.arena_slug}?court=${row.court_id}&date=${ledger.date}&at=${encodeURIComponent(slot.starts_at)}`;
	}
</script>

{#if hours.length === 0}
	<p class="small quiet">Every hour on this day has already been played. Pick another day above.</p>
{:else}
	<div class="finder">
		<div class="parts" role="group" aria-label="Part of the day">
			{#each parts as p (p.id)}
				<button
					type="button"
					class="part"
					aria-pressed={part?.id === p.id}
					onclick={() => pickPart(p)}
				>
					{p.label}
				</button>
			{/each}
		</div>

		<div class="hours" role="group" aria-label="Hour">
			<!-- Every hour is drawn; phones show only the chosen part of the
			     day, wide screens show the whole day in one row. -->
			{#each hours as h (h)}
				{@const free = freeCount.get(h)}
				<button
					type="button"
					class="hour"
					class:none={free === 0}
					class:other-part={!partHours.includes(h)}
					aria-pressed={h === hour}
					aria-label="{clock(h)}, {free === 0 ? 'nothing free' : `${free} free`}"
					onclick={() => (chosen = h)}
				>
					<span class="h num">{String(h).padStart(2, '0')}</span>
					<span class="n">{free === 0 ? 'Full' : `${free} free`}</span>
				</button>
			{/each}
		</div>

		<h2 class="count" aria-live="polite">
			{#if results.length}
				<span class="num">{results.length}</span>
				{results.length === 1 ? 'court' : 'courts'} free at {clock(hour)}
			{:else}
				Nothing free at {clock(hour)}
			{/if}
		</h2>

		{#key `${ledger.date}:${hour}`}
			{#if results.length}
				<ul class="list" role="list">
					{#each results as { row, slot, to }, i (row.court_id)}
						<li in:fly={{ y: 12, duration: calm ? 0 : 340, delay: calm ? 0 : Math.min(i, 8) * 45, easing: cubicOut }}>
							<a class="court" class:peak={slot.is_peak} href={href(row, slot)}>
								<span class="info">
									<strong class="venue">{row.arena_name}</strong>
									<span class="meta">
										{row.court_name} · {shortFormat(row.format)} · {row.arena_area}
									</span>
									{#if to - hour > 1}
										<span class="run">Free till {clock(to)}</span>
									{/if}
								</span>
								<span class="price">
									<span class="amount num">{formatNPR(slot.price_npr)}</span>
									<span class="unit">{slot.is_peak ? 'Peak rate' : 'NPR / hour'}</span>
								</span>
								<span class="go" aria-hidden="true">
									<svg viewBox="0 0 16 16" width="14" height="14"><path d="M6 3.5l4.5 4.5L6 12.5" /></svg>
								</span>
							</a>
						</li>
					{/each}
				</ul>
				{#if booked}
					<p class="small quiet foot">
						{booked} more {booked === 1 ? 'court is' : 'courts are'} already booked at {clock(hour)}.
					</p>
				{/if}
			{:else}
				<div class="empty" in:fly={{ y: 8, duration: calm ? 0 : 300, easing: cubicOut }}>
					{#if nearby.length}
						<p class="small quiet">Closest hours with a court free:</p>
						<div class="nearby">
							{#each nearby as h (h)}
								<button type="button" class="btn btn-secondary" onclick={() => (chosen = h)}>
									{clock(h)} · {freeCount.get(h)} free
								</button>
							{/each}
						</div>
					{:else}
						<p class="small quiet">Every court is booked for the rest of the day. Try another day.</p>
					{/if}
				</div>
			{/if}
		{/key}
	</div>
{/if}

<style>
	.finder {
		display: grid;
		gap: 0.9rem;
	}

	/* --- part of the day: one segmented control --- */

	.parts {
		display: flex;
		padding: 0.25rem;
		border: 1px solid var(--line);
		border-radius: var(--r-pill);
		background: var(--surface-sunk);
	}

	.part {
		flex: 1;
		padding: 0.55rem 0.5rem;
		border: 0;
		border-radius: var(--r-pill);
		background: transparent;
		font: inherit;
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--muted);
		cursor: pointer;
		transition:
			background-color 0.25s var(--ease),
			color 0.25s var(--ease),
			box-shadow 0.25s var(--ease);
	}

	.part[aria-pressed='true'] {
		background: var(--surface);
		color: var(--ink);
		box-shadow: 0 1px 3px color-mix(in srgb, var(--ink) 14%, transparent);
	}

	/* --- hours --- */

	/* One row per part of the day: at most seven hours, which still fit. */
	.hours {
		display: grid;
		grid-auto-flow: column;
		grid-auto-columns: minmax(0, 1fr);
		gap: 0.35rem;
	}

	.hour {
		display: grid;
		justify-items: center;
		gap: 0.05rem;
		padding: 0.5rem 0.25rem 0.45rem;
		border: 1px solid var(--line);
		border-radius: var(--r-sm);
		background: var(--surface);
		font: inherit;
		color: var(--ink);
		cursor: pointer;
		transition:
			background-color 0.2s var(--ease),
			border-color 0.2s var(--ease),
			color 0.2s var(--ease),
			transform 0.15s var(--ease);
	}

	.hour:active {
		transform: scale(0.95);
	}

	.h {
		font-size: 1.0625rem;
		font-weight: 700;
	}

	.n {
		font-size: 0.6875rem;
		color: var(--muted);
		transition: color 0.2s var(--ease);
	}

	.hour.none {
		background: var(--surface-sunk);
		color: var(--faint);
	}

	.hour[aria-pressed='true'] {
		background: var(--pine);
		border-color: var(--pine);
		color: #ffffff;
	}

	.hour[aria-pressed='true'] .n {
		color: color-mix(in srgb, #ffffff 85%, transparent);
	}

	.part:focus-visible,
	.hour:focus-visible {
		outline: 2px solid var(--pine);
		outline-offset: 2px;
	}

	/* --- results --- */

	.count {
		margin: 0.4rem 0 0;
		font-family: var(--display);
		font-size: 1.25rem;
		font-weight: 800;
		letter-spacing: -0.02em;
		color: var(--ink);
	}

	.list {
		display: grid;
		gap: 0.5rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.court {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto auto;
		align-items: center;
		gap: 0.75rem;
		padding: 0.85rem 0.75rem 0.85rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
		color: var(--ink);
		text-decoration: none;
		transition:
			border-color 0.2s var(--ease),
			transform 0.15s var(--ease);
	}

	.court:active {
		transform: scale(0.985);
	}

	.court:hover,
	.court:focus-visible {
		border-color: var(--pine);
	}

	.court.peak {
		background: var(--sand);
		border-color: var(--sand-line);
	}

	.info {
		display: grid;
		gap: 0.15rem;
		min-width: 0;
	}

	.venue {
		font-size: 1rem;
		font-weight: 600;
		letter-spacing: -0.01em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta {
		font-size: 0.8125rem;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.run {
		justify-self: start;
		margin-top: 0.25rem;
		padding: 0.1rem 0.5rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.75rem;
		font-weight: 600;
	}

	.price {
		display: grid;
		justify-items: end;
	}

	.amount {
		font-size: 1.0625rem;
		font-weight: 700;
	}

	.unit {
		font-size: 0.6875rem;
		color: var(--muted);
		white-space: nowrap;
	}

	.go {
		display: grid;
		place-items: center;
		color: var(--pine);
	}

	.go path {
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.foot {
		margin: 0.25rem 0 0;
	}

	.empty {
		display: grid;
		gap: 0.6rem;
	}

	.nearby {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.quiet {
		color: var(--muted);
	}

	/* Phones: one part of the day at a time. */
	@media (max-width: 899px) {
		.hour.other-part {
			display: none;
		}
	}

	/* Wide screens: the whole day fits in one row, so the part-of-day
	   switch goes, and the courts sit side by side. */
	@media (min-width: 900px) {
		.finder {
			gap: 1.1rem;
		}

		.parts {
			display: none;
		}

		.hours {
			gap: 0.3rem;
		}

		.hour {
			padding-block: 0.6rem 0.5rem;
		}

		.list {
			grid-template-columns: repeat(auto-fill, minmax(21rem, 1fr));
			gap: 0.65rem;
		}

		.court {
			padding: 1rem 0.85rem 1rem 1.15rem;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.part,
		.hour,
		.court {
			transition: none;
		}
	}
</style>
