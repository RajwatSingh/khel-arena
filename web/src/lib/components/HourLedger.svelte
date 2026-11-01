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

	// Only hours still to come get a column. An hour that has already started
	// can't be booked, so on today's board it would only take up room.
	const hours = $derived.by(() => {
		const set = new Set();
		for (const row of ledger.rows) {
			for (const slot of row.slots) {
				if (!slot.is_past) set.add(Number(formatTime(slot.starts_at).slice(0, 2)));
			}
		}
		return [...set].sort((a, b) => a - b);
	});

	// A column can still hold a past cell when courts start on different
	// minutes (18:00 and 18:30 share a column), so the key only lists
	// "Already played" when one is on screen.
	const anyGone = $derived(
		byHour.some(({ map }) => hours.some((h) => map.get(h)?.is_past))
	);

	const byHour = $derived(
		ledger.rows.map((row) => {
			const map = new Map();
			for (const slot of row.slots) {
				map.set(Number(formatTime(slot.starts_at).slice(0, 2)), slot);
			}
			return { row, map };
		})
	);

	// "5-a-side" → "5v5", so a court and its format fit one line on phones.
	const shortFormat = (format) => format?.replace(/^(\d+)-a-side$/i, '$1v$1') ?? '';

	function href(row, slot) {
		return `/arenas/${row.arena_slug}?court=${row.court_id}&date=${ledger.date}&at=${encodeURIComponent(slot.starts_at)}`;
	}

	// Long boards open on the first few courts, with the next one peeking out
	// under a fade, and expand to the rest. Not worth it for one extra row.
	const LIMIT = 6;
	const PEEK = 30;
	const canExpand = $derived(byHour.length > LIMIT + 1);
	let expanded = $state(false);
	let fullHeight = $state(0);
	let shortHeight = $state(0);
	let wrapEl = $state();

	// The height is animated between two measured numbers, since CSS can't
	// transition to `auto`. Re-measured whenever the table changes size, so a
	// narrower window or a new day's board keeps the numbers right.
	function measure(table) {
		const update = () => {
			fullHeight = table.offsetHeight;
			const row = table.tBodies[0]?.rows[LIMIT];
			shortHeight = row ? Math.min(row.offsetTop + PEEK, fullHeight) : fullHeight;
		};
		const observer = new ResizeObserver(update);
		observer.observe(table);
		update();
		return { destroy: () => observer.disconnect() };
	}

	// Open, the board keeps a strip of room under the last row for the
	// button, which sits on the board's bottom edge.
	const ROOM = 24;
	const clipHeight = $derived(
		!canExpand || !fullHeight ? null : expanded ? fullHeight + ROOM : shortHeight
	);

	function toggle() {
		expanded = !expanded;
		// Collapsing pulls the page up by the rows that just closed; if the
		// top of the board has scrolled away, bring it back into view.
		if (!expanded && wrapEl) {
			const top = wrapEl.getBoundingClientRect().top;
			if (top < 80) {
				window.scrollTo({ top: window.scrollY + top - 96, behavior: 'smooth' });
			}
		}
	}

	// Day, sport and area all end up changing which courts are listed, when,
	// or both — so one key covering court set + date is enough to catch a
	// switch on any of the three and re-mount the whole board for it.
	const boardKey = $derived(`${ledger.date}:${ledger.rows.map((r) => r.court_id).join(',')}`);
</script>

<div class="ledger">
	{#if hours.length === 0}
	<div class="card done">
		<p class="small">Every hour on this day has already been played. Pick another day above.</p>
	</div>
	{:else}
	<!-- A scrollable region has to be reachable by keyboard, which is exactly
	     what this lint rule assumes it is not. -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	{#key boardKey}
	<div class="board" class:collapsed={canExpand && !expanded} bind:this={wrapEl}>
	<div
		class="card scroll"
		id="court-board"
		role="region"
		aria-label="Court availability by hour"
		tabindex="0"
	>
		<div
			class="clip"
			class:measured={clipHeight !== null}
			style:height={clipHeight === null ? null : `${clipHeight}px`}
		>
		<table use:measure>
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
					<tr
						style="--row: {r}; --i: {r - LIMIT - 1}"
						class:beyond={canExpand && r > LIMIT}
						class:ssr-hidden={canExpand && r > LIMIT && !fullHeight}
						inert={canExpand && !expanded && r >= LIMIT}
					>
						<th scope="row" class="side">
							<a href="/arenas/{row.arena_slug}">
								<MiddleTruncation class="venue" text={row.arena_name} />
								<MiddleTruncation
									class="court"
									text="{row.court_name} · {row.format} · {row.arena_area}"
								/>
								<!-- Phones: a narrow column where the name wraps onto more
								     lines, which leaves the width for hours. -->
								<span class="stacked">
									<span class="s-venue">{row.arena_name}</span>
									<span class="s-court nw">{row.court_name} · {shortFormat(row.format)}</span>
									<span class="s-court nw">{row.arena_area}</span>
								</span>
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
	</div>
	<div class="fade" aria-hidden="true"></div>
	</div>
	{/key}

	{#if canExpand}
		<div class="more">
			<button
				type="button"
				class="expand"
				aria-expanded={expanded}
				aria-controls="court-board"
				onclick={toggle}
			>
				<span>{expanded ? 'Show fewer courts' : `Show all ${byHour.length} courts`}</span>
				<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
					<path d="M3.5 6l4.5 4.5L12.5 6" />
				</svg>
			</button>
		</div>
	{/if}

	<ul class="key small">
		<li><span class="chip free"></span>Free · price per hour, NPR</li>
		<li><span class="chip peak"></span>Peak rate</li>
		<li><span class="chip taken"><i></i></span>Taken</li>
		{#if anyGone}<li><span class="chip gone"></span>Already played</li>{/if}
	</ul>
	{/if}
</div>

<style>
	/* --- expanding --- */

	.board {
		position: relative;
		--expand-ease: cubic-bezier(0.32, 0.72, 0, 1);
	}

	.clip {
		overflow-y: clip;
	}

	.clip.measured {
		transition: height 0.7s var(--expand-ease);
	}

	.collapsed .clip.measured {
		transition-duration: 0.5s;
	}

	/* Before the first measurement (server render) the rows past the peek
	   simply aren't drawn, so the board never flashes open on load. */
	tr.ssr-hidden {
		display: none;
	}

	/* The rows that were out of sight rise in one after another as the
	   board opens, and drop away together when it closes. */
	tr.beyond {
		transition:
			opacity 0.45s var(--ease-reveal),
			transform 0.55s var(--expand-ease),
			filter 0.45s var(--ease-reveal);
		transition-delay: calc(min(var(--i), 10) * 38ms + 60ms);
	}

	.collapsed tr.beyond {
		opacity: 0;
		transform: translateY(14px);
		filter: blur(6px);
		transition-duration: 0.25s;
		transition-delay: 0s;
	}

	.fade {
		position: absolute;
		left: 1px;
		right: 1px;
		bottom: 1px;
		height: 5.5rem;
		border-radius: 0 0 var(--r-md) var(--r-md);
		background: linear-gradient(
			to bottom,
			color-mix(in srgb, var(--surface) 0%, transparent),
			var(--surface) 88%
		);
		pointer-events: none;
		opacity: 0;
		transition: opacity 0.4s var(--ease-reveal);
	}

	.collapsed .fade {
		opacity: 1;
		transition-delay: 0.15s;
	}

	.more {
		display: flex;
		justify-content: center;
		margin-top: -1.35rem;
		position: relative;
		z-index: 3;
	}

	.expand {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.6rem 1.1rem 0.6rem 1.25rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-pill);
		background: var(--surface);
		box-shadow: var(--shadow);
		font: inherit;
		font-size: 0.9rem;
		font-weight: 600;
		color: var(--ink);
		cursor: pointer;
		transition:
			border-color var(--dur-hover) var(--ease),
			color var(--dur-hover) var(--ease),
			transform 0.2s var(--ease);
	}

	.expand:hover {
		border-color: var(--pine);
		color: var(--pine-deep);
		transform: translateY(-1px);
	}

	.expand:active {
		transform: translateY(0) scale(0.97);
	}

	.expand:focus-visible {
		outline: 2px solid var(--pine);
		outline-offset: 2px;
	}

	.expand svg {
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
		transition: transform 0.55s var(--expand-ease);
	}

	.expand[aria-expanded='true'] svg {
		transform: rotate(180deg);
	}

	@media (prefers-reduced-motion: reduce) {
		.clip.measured,
		tr.beyond,
		.fade,
		.expand svg {
			transition: none;
		}
	}

	.done {
		padding: 1.5rem 1.35rem;
		color: var(--muted);
	}

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

	.stacked {
		display: none;
	}

	/* Phones: hours over names. The court column narrows and its text wraps,
	   so rows get taller instead; the hour columns slim down and the board
	   runs to the screen edges. About five hours fit across, up from two. */
	@media (max-width: 720px) {
		.board {
			margin-inline: calc(-1 * var(--gutter));
		}

		.scroll {
			border-radius: 0;
			border-inline: 0;
		}

		.fade {
			left: 0;
			right: 0;
			border-radius: 0;
		}

		.corner,
		.side {
			width: 7.4rem;
			min-width: 7.4rem;
			padding: 0.55rem 0.6rem 0.55rem 0.85rem;
		}

		.corner {
			padding-block: 0.85rem 0.5rem;
		}

		.side :global(.venue),
		.side :global(.court) {
			display: none;
		}

		.stacked {
			display: grid;
			gap: 0.1rem;
		}

		/* Two lines at most, so a long name can't stretch its row. */
		.s-venue {
			display: -webkit-box;
			-webkit-box-orient: vertical;
			-webkit-line-clamp: 2;
			line-clamp: 2;
			overflow: hidden;
			font-size: 0.8125rem;
			font-weight: 600;
			line-height: 1.25;
			letter-spacing: -0.01em;
			overflow-wrap: break-word;
		}

		.s-court {
			display: block;
			font-size: 0.6875rem;
			line-height: 1.3;
			color: var(--faint);
		}

		.nw {
			overflow: hidden;
			white-space: nowrap;
			text-overflow: ellipsis;
		}

		.side a:hover .s-venue {
			color: var(--pine);
		}

		.hour {
			width: 3.4rem;
			min-width: 3.4rem;
			padding-block: 0.85rem 0.5rem;
			font-size: 0.75rem;
		}

		.cell {
			height: 3.1rem;
			margin: 2px;
		}

		.open .price {
			font-size: 0.8125rem;
		}
	}
</style>
