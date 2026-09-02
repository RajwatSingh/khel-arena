<script>
	import { formatNPR, formatTime } from '$lib/time.js';
	import MiddleTruncation from './MiddleTruncation.svelte';

	/**
	 * The board: every court down the side, the hours of one day across the top,
	 * the price of each free hour in the cell.
	 *
	 * A court-hour is inventory that expires — at nine o'clock the eight o'clock
	 * slot is not discounted, it is gone — so the whole day is shown at once and
	 * the empty cells make the argument.
	 */
	let { ledger } = $props();

	const hours = $derived.by(() => {
		const set = new Set();
		for (const row of ledger.rows) {
			for (const slot of row.slots) set.add(Number(formatTime(slot.starts_at).slice(0, 2)));
		}
		return [...set].sort((a, b) => a - b);
	});

	const byHour = $derived(
		ledger.rows.map((row) => {
			const map = new Map();
			for (const slot of row.slots) {
				map.set(Number(formatTime(slot.starts_at).slice(0, 2)), slot);
			}
			return { row, map };
		})
	);

	function href(row, slot) {
		return `/arenas/${row.arena_slug}?court=${row.court_id}&date=${ledger.date}&at=${encodeURIComponent(slot.starts_at)}`;
	}

	// Day, sport and area all end up changing which courts are listed, when,
	// or both — so one key covering court set + date is enough to catch a
	// switch on any of the three and re-mount the whole board for it.
	const boardKey = $derived(`${ledger.date}:${ledger.rows.map((r) => r.court_id).join(',')}`);
</script>

<div class="ledger">
	<!-- A scrollable region has to be reachable by keyboard, which is exactly
	     what this lint rule assumes it is not. -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	{#key boardKey}
	<div class="card scroll" role="region" aria-label="Court availability by hour" tabindex="0">
		<table>
			<caption class="sr-only">
				Free hours and the price per hour in Nepali rupees for every court, {ledger.date}
			</caption>
			<thead>
				<tr>
					<th scope="col" class="corner"><span class="label">Court</span></th>
					{#each hours as hour (hour)}
						<th scope="col" class="hour num">{String(hour).padStart(2, '0')}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each byHour as { row, map }, r (`${row.court_id}:${ledger.date}`)}
					<tr style="--row: {r}">
						<th scope="row" class="side">
							<a href="/arenas/{row.arena_slug}">
								<MiddleTruncation class="venue" text={row.arena_name} />
								<MiddleTruncation
									class="court"
									text="{row.court_name} · {row.format} · {row.arena_area}"
								/>
							</a>
						</th>
						{#each hours as hour (hour)}
							{@const slot = map.get(hour)}
							<td>
								{#if !slot}
									<span class="cell closed" aria-label="Closed"></span>
								{:else if slot.available}
									<a
										class="cell open"
										class:peak={slot.is_peak}
										href={href(row, slot)}
										aria-label="{row.arena_name} {row.court_name} at {formatTime(
											slot.starts_at
										)}, {formatNPR(slot.price_npr)} rupees{slot.is_peak ? ', peak rate' : ''}"
									>
										<span class="price num">{formatNPR(slot.price_npr)}</span>
									</a>
								{:else if slot.is_past}
									<span class="cell gone" aria-label="Already played"></span>
								{:else}
									<span class="cell taken" aria-label="Taken"><i></i></span>
								{/if}
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	{/key}

	<ul class="key small">
		<li><span class="chip free"></span>Free · price per hour, NPR</li>
		<li><span class="chip peak"></span>Peak rate</li>
		<li><span class="chip taken"><i></i></span>Taken</li>
		<li><span class="chip gone"></span>Already played</li>
	</ul>
</div>

<style>
	.scroll {
		overflow-x: auto;
		scrollbar-width: thin;
		animation: card-in 0.62s var(--ease-reveal) backwards;
	}

	@keyframes card-in {
		from {
			opacity: 0;
			transform: translateY(10px);
			filter: blur(10px);
		}
	}

	table {
		width: 100%;
		border-collapse: collapse;
		table-layout: fixed;
	}

	th,
	td {
		padding: 0;
		text-align: left;
		font-weight: 400;
	}

	/* Plays on first paint and again on every re-key — a court switching day,
	   sport or area is a new row, not an edit to the old one, so the board
	   re-settles exactly as it did when it first loaded. */
	tbody tr {
		animation: settle 0.52s var(--ease-reveal) backwards;
		animation-delay: calc(var(--row) * 45ms);
	}

	@keyframes settle {
		from {
			opacity: 0;
			transform: translateY(7px);
			filter: blur(8px);
		}
	}

	.corner,
	.side {
		position: sticky;
		left: 0;
		z-index: 2;
		width: 15.5rem;
		min-width: 15.5rem;
		background: var(--surface);
		border-right: 1px solid var(--line);
		padding: 0.7rem 1.15rem 0.7rem 1.35rem;
	}

	.corner {
		padding-block: 1rem 0.6rem;
	}

	.hour {
		width: 4.25rem;
		min-width: 4.25rem;
		padding: 1rem 0 0.6rem;
		font-size: 0.8125rem;
		font-weight: 600;
		color: var(--faint);
		text-align: center;
	}

	.side {
		border-top: 1px solid var(--line);
	}

	/* The truncation spans need a real, definite width to measure and
	   constrain themselves against — an inline <a> won't give its block
	   children one on its own, so this makes the link itself a block that
	   fills the column, letting width: 100% below resolve to something. */
	.side a {
		display: block;
	}

	/* .venue/.court are rendered inside MiddleTruncation's own template now,
	   not this one, so Svelte's scoped-class hash never reaches them —
	   :global() is what lets these rules still find them by class name,
	   kept under .side so the rule can't leak to unrelated "venue"/"court"
	   classes elsewhere in the app. */
	.side :global(.venue) {
		font-size: 1.0625rem;
		font-weight: 600;
		letter-spacing: -0.012em;
		transition: color var(--dur-hover) var(--ease);
	}

	.side a:hover :global(.venue) {
		color: var(--pine);
	}

	.side :global(.court) {
		margin-top: 0.1rem;
		font-size: 0.8125rem;
		line-height: 1.4;
		color: var(--faint);
	}

	.cell {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 3.5rem;
		margin: 3px;
		border-radius: var(--r-sm);
		border: 1px solid transparent;
	}

	.open {
		background: var(--surface);
		border-color: var(--line);
		color: var(--ink);
		transition:
			background-color 0.15s var(--ease),
			border-color 0.15s var(--ease),
			color 0.15s var(--ease),
			transform 0.15s var(--ease);
	}

	.open .price {
		font-size: 0.9375rem;
		font-weight: 500;
	}

	/* Peak is carried by a tint, not by a second text colour — the price stays
	   the same weight and darkness whatever it costs. */
	.open.peak {
		background: var(--sand);
		border-color: var(--sand-line);
	}

	.open:hover,
	.open:focus-visible {
		background: var(--pine);
		border-color: var(--pine);
		color: #ffffff;
		transform: translateY(-2px);
	}

	.taken {
		background: var(--surface-sunk);
	}

	.taken i,
	.chip.taken i {
		display: block;
		width: 1.1rem;
		height: 1px;
		background: var(--line-strong);
	}

	.gone {
		background: repeating-linear-gradient(
			135deg,
			transparent 0 6px,
			color-mix(in srgb, var(--line-strong) 55%, transparent) 6px 7px
		);
	}

	.closed {
		background: var(--surface-sunk);
		opacity: 0.55;
	}

	.key {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem 1.75rem;
		margin-top: 1.25rem;
		color: var(--muted);
	}

	.key li {
		display: flex;
		align-items: center;
		gap: 0.55rem;
	}

	.chip {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 18px;
		border-radius: 6px;
		border: 1px solid var(--line);
		background: var(--surface);
	}

	.chip.peak {
		background: var(--sand);
		border-color: var(--sand-line);
	}

	.chip.taken {
		background: var(--surface-sunk);
		border-color: var(--surface-sunk);
	}

	.chip.gone {
		border-color: transparent;
		background: repeating-linear-gradient(
			135deg,
			transparent 0 6px,
			color-mix(in srgb, var(--line-strong) 55%, transparent) 6px 7px
		);
	}

	@media (max-width: 720px) {
		.corner,
		.side {
			width: 11rem;
			min-width: 11rem;
			padding-inline: 0.9rem;
		}

		.side :global(.venue) {
			font-size: 0.9375rem;
		}
	}
</style>
