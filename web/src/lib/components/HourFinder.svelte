<script>
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { formatNPR, formatTime } from '$lib/time.js';

	/**
	 * The court board, asked in the order people decide: pick a time, see
	 * the arenas with a court free then, pick an arena. Picking one opens
	 * the arena with its cheapest free court at that hour already chosen.
	 */
	let { ledger } = $props();

	const PARTS = [
		{ id: 'morning', label: 'Morning', from: 0, to: 12 },
		{ id: 'afternoon', label: 'Afternoon', from: 12, to: 17 },
		{ id: 'evening', label: 'Evening', from: 17, to: 24 }
	];

	const hourOf = (slot) => Number(formatTime(slot.starts_at).slice(0, 2));
	const pad = (h) => String(h).padStart(2, '0');
	const clock = (h) => `${pad(h)}:00`;
	const inPart = (h, p) => h >= p.from && h < p.to;
	const shortFormat = (format) => format?.replace(/^(\d+)-a-side$/i, '$1v$1') ?? '';

	const calm =
		typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

	// Each court's slots by hour. Hours that have started are left out:
	// they can't be booked.
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

	// How many arenas have at least one court free at each hour.
	const freeArenas = $derived(
		new Map(
			hours.map((h) => [
				h,
				new Set(courts.filter((c) => c.byHour.get(h)?.available).map((c) => c.row.arena_slug)).size
			])
		)
	);

	const parts = $derived(PARTS.filter((p) => hours.some((h) => inPart(h, p))));

	// Opens on the evening, when most games are played, or failing that
	// the first hour with anything free.
	function bestHour(candidates) {
		const open = candidates.filter((h) => freeArenas.get(h) > 0);
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
		chosen = within.find((h) => freeArenas.get(h) > 0) ?? within[0];
	}

	// The arenas with a court free at the chosen hour, each with those
	// courts, cheapest arena first.
	const arenas = $derived.by(() => {
		const groups = new Map();
		for (const c of courts) {
			const slot = c.byHour.get(hour);
			if (!slot?.available) continue;
			// How long the court stays free from this hour, so someone after
			// two hours can see it without opening the arena.
			let to = hour + 1;
			while (c.byHour.get(to)?.available) to++;

			const slug = c.row.arena_slug;
			if (!groups.has(slug)) {
				groups.set(slug, { slug, name: c.row.arena_name, area: c.row.arena_area, courts: [] });
			}
			groups.get(slug).courts.push({ row: c.row, slot, to });
		}
		const list = [...groups.values()];
		for (const a of list) a.courts.sort((x, y) => x.slot.price_npr - y.slot.price_npr);
		return list.sort(
			(a, b) =>
				a.courts[0].slot.price_npr - b.courts[0].slot.price_npr || a.name.localeCompare(b.name)
		);
	});

	// Arenas that are open at this hour but have every court taken.
	const fullArenas = $derived(
		new Set(courts.filter((c) => c.byHour.has(hour)).map((c) => c.row.arena_slug)).size -
			arenas.length
	);

	// When nothing is free at the chosen hour, the nearest hours that do
	// have something, so the next tap is obvious.
	const nearby = $derived(
		arenas.length
			? []
			: hours
					.filter((h) => freeArenas.get(h) > 0)
					.sort((a, b) => Math.abs(a - hour) - Math.abs(b - hour) || a - b)
					.slice(0, 3)
					.sort((a, b) => a - b)
	);

	// Opens the arena with its cheapest free court at this hour picked.
	function href(arena) {
		const { row, slot } = arena.courts[0];
		return `/arenas/${arena.slug}?court=${row.court_id}&date=${ledger.date}&at=${encodeURIComponent(slot.starts_at)}`;
	}
</script>

{#if hours.length === 0}
	<p class="small quiet">Every hour on this day has already been played. Pick another day above.</p>
{:else}
	<div class="finder">
		<div class="pick-time">
			<p class="label">Pick a time</p>

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

			<!-- Every hour is drawn; phones show only the chosen part of the
			     day, wide screens show the whole day in one row. -->
			<div class="hours" role="group" aria-label="Start time">
				{#each hours as h (h)}
					{@const free = freeArenas.get(h)}
					<button
						type="button"
						class="hour"
						class:none={free === 0}
						class:other-part={!partHours.includes(h)}
						aria-pressed={h === hour}
						aria-label="{clock(h)}, {free === 0
							? 'nothing free'
							: `${free} ${free === 1 ? 'arena' : 'arenas'} free`}"
						onclick={() => (chosen = h)}
					>
						<span class="h num">{pad(h)}<span class="mins">:00</span></span>
						<span class="n">{free === 0 ? 'Full' : `${free} free`}</span>
					</button>
				{/each}
			</div>
		</div>

		<h2 class="count" aria-live="polite">
			{#if arenas.length}
				<span class="num">{arenas.length}</span>
				{arenas.length === 1 ? 'arena' : 'arenas'} free at {clock(hour)}
			{:else}
				Nothing free at {clock(hour)}
			{/if}
		</h2>

		{#key `${ledger.date}:${hour}`}
			{#if arenas.length}
				<ul class="list" role="list">
					{#each arenas as arena, i (arena.slug)}
						{@const cheapest = arena.courts[0].slot.price_npr}
						<li
							in:fly={{
								y: 12,
								duration: calm ? 0 : 340,
								delay: calm ? 0 : Math.min(i, 8) * 45,
								easing: cubicOut
							}}
						>
							<a
								class="arena"
								href={href(arena)}
								aria-label="{arena.name}, {arena.area}: {arena.courts.length} {arena.courts
									.length === 1
									? 'court'
									: 'courts'} free at {clock(hour)}, from {formatNPR(cheapest)} rupees"
							>
								<span class="top">
									<span class="info">
										<strong class="venue">{arena.name}</strong>
										<span class="meta">{arena.area}</span>
									</span>
									<span class="price">
										{#if arena.courts.length > 1}<span class="from">from</span>{/if}
										<span class="amount num"><small>NPR</small> {formatNPR(cheapest)}</span>
										<span class="unit">per hour</span>
									</span>
								</span>

								<span class="courts">
									{#each arena.courts as { row, slot, to } (row.court_id)}
										<span class="court">
											<span class="cname">{row.court_name} · {shortFormat(row.format)}</span>
											<span class="tags">
												{#if slot.is_peak}<span class="tag peak">Peak</span>{/if}
												{#if to - hour > 1}<span class="tag run">Free till {clock(to)}</span>{/if}
											</span>
											<span class="cprice num">{formatNPR(slot.price_npr)}</span>
										</span>
									{/each}
								</span>

								<span class="bottom">
									<span class="small quiet">
										{arena.courts.length}
										{arena.courts.length === 1 ? 'court' : 'courts'} free at {clock(hour)}
									</span>
									<span class="go">
										Select arena
										<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
											<path d="M3 8h9.5M8.5 4l4 4-4 4" />
										</svg>
									</span>
								</span>
							</a>
						</li>
					{/each}
				</ul>
				{#if fullArenas > 0}
					<p class="small quiet foot">
						{fullArenas} more {fullArenas === 1 ? 'arena is' : 'arenas are'} fully booked at {clock(
							hour
						)}.
					</p>
				{/if}
			{:else}
				<div class="empty" in:fly={{ y: 8, duration: calm ? 0 : 300, easing: cubicOut }}>
					{#if nearby.length}
						<p class="small quiet">Closest times with an arena free:</p>
						<div class="nearby">
							{#each nearby as h (h)}
								<button type="button" class="btn btn-secondary" onclick={() => (chosen = h)}>
									{clock(h)} · {freeArenas.get(h)} free
								</button>
							{/each}
						</div>
					{:else}
						<p class="small quiet">Every arena is booked for the rest of the day. Try another day.</p>
					{/if}
				</div>
			{/if}
		{/key}
	</div>
{/if}

<style>
	.finder {
		display: grid;
		gap: 1rem;
	}

	.pick-time {
		display: grid;
		gap: 0.6rem;
	}

	.pick-time .label {
		margin: 0;
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
		background: #ffffff;
		color: var(--ink);
		box-shadow: 0 1px 3px color-mix(in srgb, var(--ink) 14%, transparent);
	}

	/* --- times --- */

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
		gap: 0.1rem;
		padding: 0.55rem 0.2rem 0.5rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-sm);
		background: #ffffff;
		font: inherit;
		color: var(--ink);
		cursor: pointer;
		transition:
			background-color 0.2s var(--ease),
			border-color 0.2s var(--ease),
			color 0.2s var(--ease),
			transform 0.15s var(--ease);
	}

	@media (hover: hover) {
		.hour:hover:not([aria-pressed='true']) {
			border-color: var(--pine);
		}
	}

	.hour:active {
		transform: scale(0.95);
	}

	.h {
		font-size: 1.0625rem;
		font-weight: 700;
		white-space: nowrap;
	}

	/* The minutes are always :00, so they sit smaller: the hour is what
	   you scan for, the :00 is what makes it read as a time. */
	.mins {
		font-size: 0.72em;
		font-weight: 600;
		opacity: 0.7;
	}

	.n {
		font-size: 0.6875rem;
		color: var(--muted);
		transition: color 0.2s var(--ease);
	}

	.hour.none {
		background: var(--surface-sunk);
		border-color: var(--line);
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

	/* --- arenas --- */

	.count {
		margin: 0.5rem 0 0;
		font-family: var(--display);
		font-size: 1.4rem;
		font-weight: 700;
		letter-spacing: 0;
		color: var(--ink);
	}

	.list {
		display: grid;
		gap: 0.85rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	/* Pure white with a firm edge and a resting shadow, so a card stands
	   off the warm page ground. It lifts on hover so it's clear which one
	   you're about to pick. */
	.arena {
		display: grid;
		/* Cards in a row share a height; the spare room goes above the
		   footer, not between the venue and its courts. */
		grid-template-rows: auto auto 1fr;
		gap: 0.9rem;
		height: 100%;
		padding: 1.15rem 1.15rem 1rem 1.25rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-md);
		background: #ffffff;
		color: var(--ink);
		text-decoration: none;
		box-shadow:
			0 1px 2px rgba(16, 33, 22, 0.06),
			0 6px 16px -10px rgba(16, 33, 22, 0.22);
		transition:
			transform 0.28s cubic-bezier(0.34, 1.4, 0.64, 1),
			box-shadow 0.28s var(--ease),
			border-color 0.2s var(--ease);
	}

	@media (hover: hover) {
		.arena:hover {
			transform: translateY(-4px);
			border-color: var(--pine);
			box-shadow:
				0 2px 4px rgba(16, 33, 22, 0.06),
				0 20px 38px -16px rgba(16, 33, 22, 0.36);
		}
	}

	.arena:focus-visible {
		outline: 2px solid var(--pine);
		outline-offset: 3px;
		border-color: var(--pine);
	}

	.arena:active {
		transform: translateY(-1px) scale(0.99);
		transition-duration: 0.1s;
	}

	.top {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: start;
		gap: 1rem;
	}

	.info {
		display: grid;
		gap: 0.2rem;
		min-width: 0;
	}

	.venue {
		font-size: 1.1875rem;
		font-weight: 700;
		line-height: 1.25;
		letter-spacing: -0.015em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta {
		font-size: 0.875rem;
		color: var(--muted);
	}

	.price {
		display: grid;
		justify-items: end;
		line-height: 1.15;
	}

	.from,
	.unit {
		font-size: 0.75rem;
		color: var(--muted);
	}

	.amount {
		font-size: 1.375rem;
		font-weight: 800;
		letter-spacing: -0.02em;
		white-space: nowrap;
	}

	.amount small {
		font-size: 0.75rem;
		font-weight: 600;
		letter-spacing: 0.02em;
		color: var(--muted);
	}

	/* The free courts, on a tinted panel inside the white card. */
	.courts {
		display: grid;
		padding: 0.3rem 0.8rem;
		border-radius: var(--r-sm);
		background: var(--field);
	}

	.court {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.6rem;
		padding-block: 0.45rem;
	}

	.court + .court {
		border-top: 1px solid var(--line);
	}

	.cname {
		font-size: 0.875rem;
		font-weight: 600;
		white-space: nowrap;
	}

	.tags {
		display: flex;
		flex-wrap: wrap;
		gap: 0.3rem;
		min-width: 0;
	}

	.tag {
		padding: 0.1rem 0.5rem;
		border-radius: var(--r-pill);
		font-size: 0.6875rem;
		font-weight: 600;
		white-space: nowrap;
	}

	.tag.run {
		background: var(--pine-wash);
		color: var(--pine-deep);
	}

	.tag.peak {
		background: var(--sand);
		color: #7a4524; /* deep terracotta: brick is for errors */
		box-shadow: inset 0 0 0 1px var(--sand-line);
	}

	.cprice {
		font-size: 0.875rem;
		font-weight: 700;
	}

	.bottom {
		display: flex;
		align-self: end;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
	}

	.go {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		margin-left: auto;
		font-size: 0.9rem;
		font-weight: 700;
		color: var(--pine-deep);
		white-space: nowrap;
	}

	.go svg {
		transition: transform 0.28s cubic-bezier(0.34, 1.4, 0.64, 1);
	}

	@media (hover: hover) {
		.arena:hover .go svg {
			transform: translateX(3px);
		}
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

	/* Narrow phones: a court's tags drop under its name instead of
	   squeezing between the name and the price. */
	@media (max-width: 520px) {
		.court {
			grid-template-columns: minmax(0, 1fr) auto;
			grid-template-areas:
				'name price'
				'tags tags';
			row-gap: 0.3rem;
		}

		.cname {
			grid-area: name;
		}

		.cprice {
			grid-area: price;
		}

		.tags {
			grid-area: tags;
		}

		.tags:empty {
			display: none;
		}
	}

	/* Wide screens: the whole day fits in one row, so the part-of-day
	   switch goes, and the arenas sit side by side. */
	@media (min-width: 900px) {
		.finder {
			gap: 1.15rem;
		}

		.parts {
			display: none;
		}

		.hours {
			gap: 0.3rem;
		}

		.hour {
			padding-block: 0.65rem 0.55rem;
		}

		.list {
			grid-template-columns: repeat(auto-fill, minmax(26rem, 1fr));
			gap: 1rem;
		}

		.arena {
			padding: 1.4rem 1.4rem 1.15rem 1.5rem;
			gap: 1rem;
		}

		.venue {
			font-size: 1.3125rem;
		}

		.amount {
			font-size: 1.5rem;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.part,
		.hour,
		.arena,
		.go svg {
			transition: none;
		}
	}
</style>
