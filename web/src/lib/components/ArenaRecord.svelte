<script>
	import { SPORT_LABELS } from '$lib/api/index.js';
	import { formatNPR } from '$lib/time.js';

	let { arena } = $props();

	const sports = $derived([...new Set(arena.courts.map((c) => SPORT_LABELS[c.sport] ?? c.sport))]);
	const shown = $derived(arena.amenities.slice(0, 3));
	const rest = $derived(arena.amenities.length - shown.length);
</script>

<a class="record" href="/arenas/{arena.slug}">
	<div class="line-1">
		<div class="who">
			<h3 class="display display-m">{arena.name}</h3>
			<p class="where small">{arena.area}, {arena.city}</p>
		</div>

		<span class="rating num" aria-label="Rated {arena.rating} out of 5 from {arena.review_count} reviews">
			<svg viewBox="0 0 12 12" aria-hidden="true"
				><path
					d="M6 .8l1.6 3.3 3.6.5-2.6 2.5.6 3.6L6 9l-3.2 1.7.6-3.6L.8 4.6l3.6-.5z"
				/></svg
			>
			{arena.rating}
			<span class="reviews">({arena.review_count})</span>
		</span>

		<p class="price">
			<span class="from">from</span>
			<strong class="num">NPR {formatNPR(arena.from_price_npr)}</strong>
			<span class="per">/ hour</span>
		</p>
	</div>

	<dl class="vitals">
		<div>
			<dt class="label">Courts</dt>
			<dd>{arena.court_count} · {sports.join(', ')}</dd>
		</div>
		<div>
			<dt class="label">Open</dt>
			<dd class="num">{arena.opens_at}–{arena.closes_at}</dd>
		</div>
	</dl>

	<p class="blurb">{arena.description}</p>

	<ul class="amenities small">
		{#each shown as amenity (amenity)}
			<li>{amenity}</li>
		{/each}
		{#if rest > 0}
			<li class="more">+{rest} more</li>
		{/if}
	</ul>
</a>

<style>
	/* A record in the register, not a tile in a grid: one hairline rule opens
	   the row, the row itself carries no surface, border, or shadow of its
	   own. The rule brightens to pine on hover — the same "line goes live"
	   read as picking a row on the board. */
	.record {
		display: block;
		padding-block: 1.5rem;
		border-top: 1px solid var(--line);
		transition: border-color var(--dur-hover) var(--ease);
	}

	.record:hover,
	.record:focus-visible {
		border-top-color: var(--pine);
	}

	.line-1 {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		column-gap: 1.75rem;
		row-gap: 0.5rem;
	}

	.who {
		flex-shrink: 0;
	}

	h3 {
		transition: color var(--dur-hover) var(--ease);
	}

	.record:hover h3 {
		color: var(--pine);
	}

	.where {
		margin-top: 0.15rem;
		color: var(--muted);
	}

	.rating {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		flex-shrink: 0;
		color: var(--faint);
		font-size: 0.875rem;
		font-weight: 600;
	}

	.rating svg {
		width: 12px;
		height: 12px;
		fill: var(--pine);
	}

	.reviews {
		font-weight: 500;
		opacity: 0.7;
	}

	.vitals {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 1.75rem;
		margin-top: 0.7rem;
	}

	.vitals dd {
		margin: 0;
		margin-top: 0.15rem;
		font-size: 0.9375rem;
		color: var(--ink);
	}

	.blurb {
		margin-top: 0.85rem;
		max-width: 60ch;
		color: var(--muted);
		font-size: 1rem;
		line-height: 1.6;
	}

	.amenities {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 1.1rem;
		margin-top: 0.85rem;
	}

	.amenities li {
		color: var(--faint);
		font-size: 0.8125rem;
	}

	.amenities li:not(:last-child)::after {
		content: '·';
		margin-left: 1.1rem;
		color: var(--line-strong);
	}

	.amenities .more {
		color: var(--muted);
	}

	.price {
		flex-shrink: 0;
		white-space: nowrap;
	}

	.from,
	.per {
		font-size: 0.8125rem;
		color: var(--faint);
	}

	.price strong {
		font-family: var(--display);
		font-size: 1.35rem;
		font-weight: 500;
		letter-spacing: -0.02em;
	}

	@media (max-width: 640px) {
		.line-1 {
			flex-direction: column;
			align-items: flex-start;
		}
	}
</style>
